// Copyright 2021 The Bass Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//		 https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
)

type listRoutesCmd struct {
	json bool
}

func newListRoutesCmd() *cobra.Command {
	cmd := new(listRoutesCmd)
	c := &cobra.Command{
		Use:   "list",
		Short: "List routes",
		RunE: func(cc *cobra.Command, args []string) error {
			return cmd.run(cc.Context())
		},
	}
	c.Flags().BoolVar(&cmd.json, "json", false, "show output in JSON format")
	return c
}

func (cmd *listRoutesCmd) run(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("routes list: %w", err)
		}
	}()
	pkgs, err := packages.Load(&packages.Config{
		Context: ctx,
		Mode:    packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
	}, ".")
	if err != nil {
		return err
	}
	if len(pkgs) == 0 {
		return fmt.Errorf("current directory is not a Go package")
	}
	pkg := pkgs[0]
	routingFunc := findInitRouterFunction(pkg)
	if routingFunc == nil {
		return fmt.Errorf("could not find (*application).initRouter")
	}
	type jsonPosition struct {
		Filename string `json:"filename"`
		Line     int    `json:"line,omitempty"`
		Column   int    `json:"column,omitempty"`
	}
	type route struct {
		Method   string       `json:"method"`
		Path     string       `json:"path"`
		Expr     string       `json:"expr"`
		Position jsonPosition `json:"position"`
	}
	var routes []route
	astutil.Apply(routingFunc.Body, func(c *astutil.Cursor) bool {
		switch node := c.Node().(type) {
		case *ast.CallExpr:
			recvType, obj := resolveName(pkg.TypesInfo, node.Fun)
			if recvType == nil || obj == nil {
				return false
			}
			recvPkgPath, recvName := typeName(recvType)
			if recvPkgPath != "github.com/gorilla/mux" || recvName != "Router" || obj.Name() != "Handle" || len(node.Args) < 2 {
				return false
			}
			// TODO(soon): Ensure that the method is being called on app.router.
			pathValue := pkg.TypesInfo.Types[resolveExpr(pkg, node.Args[0])].Value
			if pathValue == nil || pathValue.Kind() != constant.String {
				return false
			}
			handlerExpr := resolveExpr(pkg, node.Args[1])
			if lit := extractMethodHandler(pkg.TypesInfo, handlerExpr); lit != nil {
				for _, elem := range lit.Elts {
					kv, ok := elem.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					httpMethod := pkg.TypesInfo.Types[resolveExpr(pkg, kv.Key)].Value
					if httpMethod == nil || httpMethod.Kind() != constant.String {
						continue
					}
					handlerPos := pkg.Fset.Position(kv.Value.Pos())
					routePos := jsonPosition{
						Filename: handlerPos.Filename,
						Line:     handlerPos.Line,
						Column:   handlerPos.Column,
					}
					routes = append(routes, route{
						Method:   constant.StringVal(httpMethod),
						Path:     constant.StringVal(pathValue),
						Expr:     formatExpr(resolveExpr(pkg, kv.Value)),
						Position: routePos,
					})
				}
			} else {
				handlerPos := pkg.Fset.Position(node.Lparen)
				routePos := jsonPosition{
					Filename: handlerPos.Filename,
					Line:     handlerPos.Line,
					Column:   handlerPos.Column,
				}
				routes = append(routes, route{
					Method:   "*",
					Path:     constant.StringVal(pathValue),
					Expr:     formatExpr(handlerExpr),
					Position: routePos,
				})
			}
			return false
		case *ast.IfStmt, *ast.SwitchStmt, *ast.ForStmt, *ast.GoStmt, *ast.SelectStmt, *ast.DeferStmt:
			// Skip conditional blocks.
			return false
		default:
			return true
		}
	}, nil)
	if cmd.json {
		fmt.Println("[")
		for i, r := range routes {
			line, err := json.Marshal(r)
			if err != nil {
				return err
			}
			if i < len(routes)-1 {
				line = append(line, ',')
			}
			line = append(line, '\n')
			os.Stdout.Write(line)
		}
		fmt.Println("]")
		return nil
	}
	for _, r := range routes {
		fmt.Printf("%-7s %-20s %s\n", r.Method, r.Path, r.Expr)
	}
	return nil
}

func findInitRouterFunction(pkg *packages.Package) *ast.FuncDecl {
	for _, f := range pkg.Syntax {
		for _, decl := range f.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if ok && astReceiverTypeName(funcDecl) == "application" && funcDecl.Name.Name == "initRouter" {
				return funcDecl
			}
		}
	}
	return nil
}

func resolveName(info *types.Info, expr ast.Expr) (recv types.Type, obj types.Object) {
	switch expr := expr.(type) {
	case *ast.Ident:
		return nil, info.ObjectOf(expr)
	case *ast.SelectorExpr:
		sel := info.Selections[expr]
		if sel == nil {
			return nil, nil
		}
		return sel.Recv(), sel.Obj()
	default:
		return nil, nil
	}
}

func astReceiverTypeName(decl *ast.FuncDecl) string {
	if decl.Recv == nil {
		return ""
	}
	recvType := decl.Recv.List[0].Type
	switch recvType := recvType.(type) {
	case *ast.Ident:
		return recvType.Name
	case *ast.StarExpr:
		id, ok := recvType.X.(*ast.Ident)
		if !ok {
			return ""
		}
		return id.Name
	default:
		return ""
	}
}

func typeName(typ types.Type) (pkgPath string, typeName string) {
	for {
		ptr, ok := typ.(*types.Pointer)
		if !ok {
			break
		}
		typ = ptr.Elem()
	}
	named, ok := typ.(*types.Named)
	if !ok {
		return "", ""
	}
	return named.Obj().Pkg().Path(), named.Obj().Name()
}

// extractMethodHandler returns the composite literal for a
// github.com/gorilla/handlers.MethodHandler or nil if expr does not represent
// a MethodHandler.
func extractMethodHandler(info *types.Info, expr ast.Expr) *ast.CompositeLit {
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	typePkgPath, typeName := typeName(info.Types[expr].Type)
	if typePkgPath != "github.com/gorilla/handlers" || typeName != "MethodHandler" {
		return nil
	}
	return lit
}

// resolveExpr descends into the innermost expression, following simple variable
// references and removes parentheses.
func resolveExpr(pkg *packages.Package, expr ast.Expr) ast.Expr {
	for {
		switch e := expr.(type) {
		case *ast.ParenExpr:
			expr = e.X
		case *ast.Ident:
			obj := pkg.TypesInfo.ObjectOf(e)
			switch obj := obj.(type) {
			case *types.Var:
				if obj.IsField() {
					return expr
				}
			case *types.Const:
				// No restrictions on constant references.
			default:
				return expr
			}
			pos := obj.Pos()
			f := fileForPos(pkg.Syntax, pos)
			varPath, _ := astutil.PathEnclosingInterval(f, pos, pos)
			stmt := findAssignmentInPath(varPath)
			if stmt == nil || len(stmt.Lhs) != 1 || len(stmt.Rhs) != 1 {
				return expr
			}
			expr = stmt.Rhs[0]
		default:
			return expr
		}
	}
}

func findAssignmentInPath(path []ast.Node) *ast.AssignStmt {
	for _, node := range path {
		if assign, ok := node.(*ast.AssignStmt); ok {
			return assign
		}
	}
	return nil
}

func fileForPos(files []*ast.File, pos token.Pos) *ast.File {
	for _, f := range files {
		if f.Pos() <= pos && pos < f.End() {
			return f
		}
	}
	return nil
}

func formatExpr(expr ast.Expr) string {
	sb := new(strings.Builder)
	stack := []interface{}{expr}
	for len(stack) > 0 {
		curr := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch curr := curr.(type) {
		case string:
			sb.WriteString(curr)
		case *ast.Ident:
			sb.WriteString(curr.Name)
		case *ast.BasicLit:
			sb.WriteString(curr.Value)
		case *ast.ParenExpr:
			stack = append(stack, ")", curr.X)
			sb.WriteString("(")
		case *ast.SelectorExpr:
			stack = append(stack, curr.Sel, ".", curr.X)
		case *ast.StarExpr:
			stack = append(stack, curr.X)
			sb.WriteString("*")
		case *ast.UnaryExpr:
			stack = append(stack, curr.X)
			sb.WriteString(curr.Op.String())
		case *ast.BinaryExpr:
			stack = append(stack, curr.Y, " "+curr.Op.String()+" ", curr.X)
		case *ast.CompositeLit:
			stack = append(stack, "{ /* ... */ }", curr.Type)
		case *ast.IndexExpr:
			stack = append(stack, "]", curr.Index, "[", curr.X)
		case *ast.SliceExpr:
			stack = append(stack, "]")
			if curr.Max != nil {
				stack = append(stack, curr.Max, ":")
			}
			if curr.High != nil {
				stack = append(stack, curr.High)
			}
			stack = append(stack, ":")
			if curr.Low != nil {
				stack = append(stack, curr.Low)
			}
			stack = append(stack, "[", curr.X)
		case *ast.TypeAssertExpr:
			stack = append(stack, ")", curr.Type, ".(", curr.X)
		case *ast.CallExpr:
			stack = append(stack, ")")
			if len(curr.Args) > 0 {
				if curr.Ellipsis.IsValid() {
					stack = append(stack, "...")
				}
				stack = append(stack, curr.Args[len(curr.Args)-1])
			}
			for i := len(curr.Args) - 2; i >= 0; i-- {
				stack = append(stack, ", ", curr.Args[i])
			}
			stack = append(stack, "(", curr.Fun)
		default:
			sb.WriteString("/* ... */")
		}
	}
	return sb.String()
}
