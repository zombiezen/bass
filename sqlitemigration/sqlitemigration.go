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

// Package sqlitemigration provides a connection pool type that guarantees a
// series of SQL scripts has been run once successfully before making
// connections available to the application. This is frequently useful for
// ensuring tables are created.
package sqlitemigration

import (
	"context"
	"fmt"
	"sync"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"golang.org/x/xerrors"
)

// Schema defines the migrations for the application.
type Schema struct {
	// AppID is saved to the database file to identify the application.
	// It's used to prevent opening database files for a different
	// application. It should be a positive number, but should not change
	// between runs of the same program. A common way of setting this is with
	// a compile-time constant that was randomly generated.
	AppID int32

	// Migrations is a list of SQL scripts to run. Each script is wrapped in a
	// transaction which is rolled back on any error.
	Migrations []string
}

// Options specifies optional behaviors for the pool.
type Options struct {
	// Flags is interpreted the same way as the argument to sqlitex.Open.
	Flags sqlite.OpenFlags

	// PoolSize sets an explicit size to the pool. If less than 1, a reasonable
	// default is used.
	PoolSize int

	// OnStartMigrate is called after the pool has successfully opened a
	// connection to the database but before any migrations have been run.
	OnStartMigrate SignalFunc
	// OnReady is called after the pool has connected to the database and run any
	// necessary migrations.
	OnReady SignalFunc
	// OnError is called when the pool encounters errors while applying the
	// migration. This is typically used for logging errors.
	OnError ReportFunc
}

func (opts Options) realPoolSize() int {
	if opts.PoolSize < 1 {
		return 10
	}
	return opts.PoolSize
}

// Pool is a pool of SQLite connections.
type Pool struct {
	retry  chan<- struct{}
	cancel context.CancelFunc

	ready <-chan struct{} // protects the following fields
	pool  *sqlitex.Pool
	err   error

	closedMu sync.RWMutex
	closed   bool
}

// NewPool opens a new pool of SQLite connections.
func NewPool(uri string, schema Schema, opts Options) *Pool {
	ready := make(chan struct{})
	retry := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		ready:  ready,
		retry:  retry,
		cancel: cancel,
	}
	go func() {
		defer close(ready)
		defer cancel()
		p.pool, p.err = openPool(ctx, uri, schema, opts, retry)
		if p.err != nil {
			opts.OnError.call(p.err)
		}
	}()
	return p
}

// Close closes all connections in the Pool, potentially interrupting
// a migration.
func (p *Pool) Close() error {
	p.closedMu.Lock()
	if p.closed {
		p.closedMu.Unlock()
		return xerrors.New("close sqlite pool: already closed")
	}
	p.closed = true
	p.closedMu.Unlock()

	p.cancel()
	<-p.ready
	if p.pool == nil {
		return nil
	}
	return p.pool.Close()
}

// Get gets an SQLite connection from the pool.
func (p *Pool) Get(ctx context.Context) (*sqlite.Conn, error) {
	select {
	case <-p.ready:
	case <-ctx.Done():
		return nil, xerrors.Errorf("get sqlite conn: %w", ctx.Err())
	}
	if p.err != nil {
		return nil, xerrors.Errorf("get sqlite conn: %w", p.err)
	}
	conn := p.pool.Get(ctx)
	if conn == nil {
		if err := ctx.Err(); err != nil {
			return nil, xerrors.Errorf("get sqlite conn: %w", err)
		}
		return nil, xerrors.New("get sqlite conn: pool closed")
	}
	return conn, nil
}

// Put puts an SQLite connection back into the pool.
// See sqlitex.Pool for details.
func (p *Pool) Put(conn *sqlite.Conn) {
	select {
	case <-p.ready:
	default:
		panic("Pool.Put before pool is ready")
	}
	if p.err != nil {
		panic("Pool.Put on failed pool")
	}
	p.pool.Put(conn)
}

// CheckHealth returns an error if the migration has not completed.
// Closed pools may report healthy.
func (p *Pool) CheckHealth() error {
	p.closedMu.RLock()
	closed := p.closed
	p.closedMu.RUnlock()
	if closed {
		return xerrors.New("sqlite pool health: closed")
	}

	select {
	case <-p.ready:
		if p.err != nil {
			return xerrors.Errorf("sqlite pool health: %w", p.err)
		}
		return nil
	default:
		return xerrors.New("sqlite pool health: not ready")
	}
}

func openPool(ctx context.Context, uri string, schema Schema, opts Options, retry <-chan struct{}) (*sqlitex.Pool, error) {
	for first := true; ; first = false {
		if !first {
			select {
			case <-retry:
			case <-ctx.Done():
				return nil, xerrors.New("closed before successful migration")
			}
		}

		pool, err := sqlitex.Open(uri, opts.Flags, opts.realPoolSize())
		if err != nil {
			opts.OnError.call(err)
			continue
		}
		conn := pool.Get(ctx)
		if conn == nil {
			// Canceled.
			pool.Close()
			return nil, xerrors.New("closed before successful migration")
		}
		err = migrateDB(ctx, conn, schema, opts.OnStartMigrate)
		pool.Put(conn)
		if err != nil {
			if closeErr := pool.Close(); closeErr != nil {
				opts.OnError.call(xerrors.Errorf("close after failed migration: %w", closeErr))
			}
			return nil, err
		}
		opts.OnReady.call()
		return pool, nil
	}
}

func migrateDB(ctx context.Context, conn *sqlite.Conn, schema Schema, onStart SignalFunc) (err error) {
	defer conn.SetInterrupt(conn.SetInterrupt(ctx.Done()))
	defer sqlitex.Save(conn)(&err)

	var hasSchema bool
	err = sqlitex.ExecTransient(conn, "VALUES ((SELECT COUNT(*) FROM sqlite_master) > 0);", func(stmt *sqlite.Stmt) error {
		hasSchema = stmt.ColumnInt(0) != 0
		return nil
	})
	if err != nil {
		return xerrors.Errorf("migrate database: %w", err)
	}
	var dbAppID int32
	err = sqlitex.ExecTransient(conn, "PRAGMA application_id;", func(stmt *sqlite.Stmt) error {
		dbAppID = stmt.ColumnInt32(0)
		return nil
	})
	if err != nil {
		return xerrors.Errorf("migrate database: %w", err)
	}
	if dbAppID != schema.AppID && !(dbAppID == 0 && !hasSchema) {
		return xerrors.Errorf("migrate database: database application_id = %#x (expected %#x)", dbAppID, schema.AppID)
	}
	var schemaVersion int
	err = sqlitex.ExecTransient(conn, "PRAGMA user_version;", func(stmt *sqlite.Stmt) error {
		schemaVersion = stmt.ColumnInt(0)
		return nil
	})
	if err != nil {
		return xerrors.Errorf("migrate database: %w", err)
	}
	// Using Sprintf because PRAGMAs don't permit arbitrary expressions, and thus
	// don't permit using parameter substitution.
	err = sqlitex.ExecTransient(conn, fmt.Sprintf("PRAGMA application_id = %d;", schema.AppID), nil)
	if err != nil {
		return xerrors.Errorf("migrate database: %w", err)
	}
	onStart.call()
	for schemaVersion < len(schema.Migrations) {
		err := sqlitex.ExecScript(conn, fmt.Sprintf("%s;\nPRAGMA user_version = %d;\n", schema.Migrations[schemaVersion], schemaVersion+1))
		if err != nil {
			return xerrors.Errorf("migrate database: apply migrations[%d]: %w", schemaVersion, err)
		}
		schemaVersion++
	}
	return nil
}

// A SignalFunc is called at most once when a particular event in a Pool's
// lifecycle occurs.
type SignalFunc func()

func (f SignalFunc) call() {
	if f == nil {
		return
	}
	f()
}

// A ReportFunc is called for transient errors the pool encounters while
// running the migrations.
type ReportFunc func(error)

func (f ReportFunc) call(err error) {
	if f == nil {
		return
	}
	f(err)
}
