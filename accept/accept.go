// Copyright 2019 The Bass Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package accept provides functions for handling HTTP content negotiation
// headers.
package accept

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// A Header represents a set of media ranges as sent in the Accept header
// of an HTTP request.
//
// http://tools.ietf.org/html/rfc2616#section-14.1
type Header []MediaRange

// String formats the media ranges in the format for an Accept header.
func (h Header) String() string {
	parts := make([]string, len(h))
	for i := range h {
		mr := &h[i]
		parts[i] = mr.String()
	}
	return strings.Join(parts, ",")
}

// Quality returns the quality of a content type based on the media ranges in h.
func (h Header) Quality(contentType string, params map[string][]string) float32 {
	results := make(mediaRangeMatches, 0, len(h))
	for i := range h {
		mr := &h[i]
		if m := mr.match(contentType, params); m.Valid {
			results = append(results, m)
		}
	}
	if len(results) == 0 {
		return 0.0
	}

	// find most specific
	i := 0
	for j := 1; j < results.Len(); j++ {
		if results.Less(j, i) {
			i = j
		}
	}
	return results[i].MediaRange.Quality
}

// ParseHeader parses an Accept header of an HTTP request.  The media
// ranges are unsorted.
func ParseHeader(accept string) (Header, error) {
	var h Header
	p := &parser{s: accept}
	p.space()
	for !p.eof() {
		if len(h) > 0 {
			if !p.consume(",") {
				return nil, fmt.Errorf("parse accept header: expected ',', found %s", p.first())
			}
			p.space()
		}

		r, err := parseMediaRange(p)
		if err != nil {
			return nil, fmt.Errorf("parse accept header: %w", err)
		}
		quality, params, err := parseParams(p)
		if err != nil {
			return nil, fmt.Errorf("parse accept header: %w", err)
		}
		h = append(h, MediaRange{Range: r, Quality: quality, Params: params})
	}
	return h, nil
}

func parseMediaRange(p *parser) (string, error) {
	const sep = "/"
	input := p.s
	typ := p.token()
	if len(typ) == 0 {
		return "", fmt.Errorf("parse media range: expected token, found %s", p.first())
	}
	if !p.consume(sep) {
		return "", fmt.Errorf("parse media range: expected %q, found %s", sep, p.first())
	}
	subtype := p.token()
	if len(subtype) == 0 {
		return "", fmt.Errorf("parse media range: expected subtype, found %s", p.first())
	}
	return string(input[:len(typ)+len(sep)+len(subtype)]), nil
}

func parseParams(p *parser) (float32, map[string][]string, error) {
	quality, params := float32(1.0), make(map[string][]string)
	p.space()
	for p.consume(";") {
		p.space()
		key := string(p.token())
		p.space()
		if !p.consume("=") {
			return 0, nil, fmt.Errorf("parse parameters: expected '=', found %s", p.first())
		}
		p.space()
		var value string
		if s, err := p.quotedString(); errors.Is(err, errNotQuotedString) {
			value = string(p.token())
		} else if err != nil {
			return 0, nil, fmt.Errorf("parse parameters: %w", err)
		} else {
			value = string(s)
		}
		p.space()

		if key == "q" {
			// check for qvalue
			q, err := strconv.ParseFloat(value, 64)
			if err != nil || q < 0 || 1 < q {
				return 0, nil, fmt.Errorf("parse parameters: invalid q value %q", value)
			}
			quality = float32(q)
		} else {
			params[key] = append(params[key], value)
		}
	}
	return quality, params, nil
}

// A MediaRange represents a set of MIME types as sent in the Accept header of
// an HTTP request.
type MediaRange struct {
	Range   string
	Quality float32
	Params  map[string][]string
}

// Match reports whether the range applies to a content type.
func (mr *MediaRange) Match(contentType string, params map[string][]string) bool {
	return mr.match(contentType, params).Valid
}

type mediaRangeMatch struct {
	MediaRange *MediaRange
	Valid      bool
	Type       int
	Subtype    int
	Params     int
}

type mediaRangeMatches []mediaRangeMatch

func (m mediaRangeMatches) Len() int      { return len(m) }
func (m mediaRangeMatches) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m mediaRangeMatches) Less(i, j int) bool {
	mi, mj := &m[i], &m[j]
	switch {
	case !mi.Valid && !mj.Valid:
		return false
	case !mi.Valid && mj.Valid:
		return false
	case mi.Valid && !mj.Valid:
		return true
	}
	if mi.Params != mj.Params {
		return mi.Params > mj.Params
	}
	if mi.Subtype != mj.Subtype {
		return mi.Subtype > mj.Subtype
	}
	return mi.Type > mj.Type
}

func (mr *MediaRange) match(contentType string, params map[string][]string) mediaRangeMatch {
	mrType, mrSubtype := splitContentType(mr.Range)
	ctType, ctSubtype := splitContentType(contentType)
	match := mediaRangeMatch{MediaRange: mr}

	if !(mrSubtype == "*" || mrSubtype == ctSubtype) || !(mrType == "*" || mrType == ctType) {
		return match
	}
	if mrType != "*" {
		match.Type++
	}
	if mrSubtype != "*" {
		match.Subtype++
	}

	for k, v1 := range mr.Params {
		v2, ok := params[k]
		if !ok {
			return match
		}
		if len(v1) != len(v2) {
			return match
		}
		for i := range v1 {
			if v1[i] != v2[i] {
				return match
			}
		}
		match.Params++
	}
	match.Valid = true
	return match
}

func splitContentType(s string) (string, string) {
	i := strings.IndexRune(s, '/')
	if i == -1 {
		return "", ""
	}
	return s[:i], s[i+1:]
}

func (mr *MediaRange) String() string {
	parts := make([]string, 0, len(mr.Params)+1)
	parts = append(parts, mr.Range)
	if mr.Quality != 1.0 {
		parts = append(parts, "q="+strconv.FormatFloat(float64(mr.Quality), 'f', 3, 32))
	}
	for k, vs := range mr.Params {
		for _, v := range vs {
			parts = append(parts, k+"="+quoteHTTP(v))
		}
	}
	return strings.Join(parts, ";")
}

func quoteHTTP(s string) string {
	if s == "" {
		return `""`
	}
	isToken := true
	for i := 0; i < len(s); i++ {
		if !isTokenChar(s[i]) {
			isToken = false
			break
		}
	}
	if isToken {
		return s
	}
	sb := make([]byte, 0, len(s)+2)
	sb = append(sb, '"')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\\', '"':
			sb = append(sb, '\\', c)
		default:
			sb = append(sb, c)
		}
	}
	sb = append(sb, '"')
	return string(sb)
}

type parser struct {
	s string
}

func (p *parser) eof() bool {
	return p.s == ""
}

func (p *parser) peek() byte {
	if len(p.s) == 0 {
		return 0
	}
	return p.s[0]
}

func (p *parser) first() string {
	if len(p.s) == 0 {
		return "EOF"
	}
	return strconv.QuoteRuneToASCII(rune(p.s[0]))
}

func (p *parser) consume(literal string) bool {
	if !strings.HasPrefix(p.s, literal) {
		return false
	}
	p.s = p.s[len(literal):]
	return true
}

func isTokenChar(c byte) bool {
	const chars = "!#$%&'*+-.0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ^_`abcdefghijklmnopqrstuvwxyz|~"
	return strings.IndexByte(chars, c) != -1
}

func (p *parser) token() string {
	i := 0
	for ; i < len(p.s); i++ {
		if !isTokenChar(p.s[i]) {
			break
		}
	}
	run := p.s[:i]
	p.s = p.s[i:]
	return run
}

var errNotQuotedString = errors.New("not a quoted string")

func (p *parser) quotedString() (string, error) {
	if len(p.s) == 0 || p.s[0] != '"' {
		return "", errNotQuotedString
	}
	sb := new(strings.Builder)
	i := 1
	for ; i < len(p.s); i++ {
		switch c := p.s[i]; c {
		case '"':
			p.s = p.s[i+1:]
			return sb.String(), nil
		case '\\':
			i++
			if i >= len(p.s) {
				p.s = p.s[i:]
				return "", io.ErrUnexpectedEOF
			}
			sb.WriteByte(p.s[i])
		default:
			sb.WriteByte(c)
		}
	}
	p.s = p.s[i:]
	return "", io.ErrUnexpectedEOF
}

func (p *parser) space() string {
	i := 0
	for ; i < len(p.s); i++ {
		if c := p.s[i]; c != ' ' && c != '\t' {
			break
		}
	}
	run := p.s[:i]
	p.s = p.s[i:]
	return run
}
