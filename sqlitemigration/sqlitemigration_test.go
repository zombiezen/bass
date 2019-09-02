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

package sqlitemigration

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"golang.org/x/xerrors"
)

func TestPool(t *testing.T) {
	dir, err := ioutil.TempDir("", "sqlitemigration_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("cleaning up: %v", err)
		}
	}()
	ctx := context.Background()

	t.Run("NoMigrations", func(t *testing.T) {
		schema := Schema{
			AppID: 0xedbeef,
		}
		state := new(eventRecorder)
		pool := NewPool(filepath.Join(dir, "no-migrations.db"), schema, Options{
			Flags:          sqlite.SQLITE_OPEN_READWRITE | sqlite.SQLITE_OPEN_CREATE | sqlite.SQLITE_OPEN_NOMUTEX,
			OnStartMigrate: state.startMigrateFunc(),
			OnReady:        state.readyFunc(),
		})
		defer func() {
			if err := pool.Close(); err != nil {
				t.Error("pool.Close:", err)
			}
			if err := pool.CheckHealth(); err == nil {
				t.Error("after Close, CheckHealth() = <nil>; want error")
			}
		}()
		conn, err := pool.Get(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if state.migrationStarted != 1 {
			t.Errorf("OnStartMigrate called %d times; want 1", state.migrationStarted)
		}
		if state.ready != 1 {
			t.Errorf("OnReady called %d times; want 1", state.ready)
		}
		if err := pool.CheckHealth(); err != nil {
			t.Errorf("after successful Get, CheckHealth = %v; want <nil>", err)
		}
		called := false
		err = sqlitex.ExecTransient(conn, "PRAGMA application_id;", func(stmt *sqlite.Stmt) error {
			called = true
			if got, want := stmt.ColumnInt32(0), int32(0xedbeef); got != want {
				t.Errorf("application_id = %#x; want %#x", got, want)
			}
			return nil
		})
		if err != nil {
			t.Errorf("PRAGMA application_id: %v", err)
		} else if !called {
			t.Error("PRAGMA application_id not called")
		}
		pool.Put(conn)
	})

	t.Run("DoesNotMigrateDifferentDatabase", func(t *testing.T) {
		// Create another.db with a single table.
		// Don't set application ID.
		err := withTestConn(dir, "another.db", func(conn *sqlite.Conn) error {
			err := sqlitex.ExecTransient(conn, `create table foo ( id integer primary key not null );`, nil)
			if err != nil {
				return xerrors.Errorf("create table: %v", err)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}

		// Try to open the pool.
		schema := Schema{
			AppID: 0xedbeef,
		}
		state := new(eventRecorder)
		pool := NewPool(filepath.Join(dir, "another.db"), schema, Options{
			Flags:          sqlite.SQLITE_OPEN_READWRITE | sqlite.SQLITE_OPEN_CREATE | sqlite.SQLITE_OPEN_NOMUTEX,
			OnStartMigrate: state.startMigrateFunc(),
			OnReady:        state.readyFunc(),
		})
		defer func() {
			if err := pool.Close(); err != nil {
				t.Error("pool.Close:", err)
			}
			if err := pool.CheckHealth(); err == nil {
				t.Error("after Close, CheckHealth() = <nil>; want error")
			}
		}()
		conn, err := pool.Get(ctx)
		t.Logf("pool.Get error: %v", err)
		if err == nil {
			pool.Put(conn)
			return
		}
		if state.migrationStarted != 0 {
			t.Errorf("OnStartMigrate called %d times; want 0", state.migrationStarted)
		}
		if state.ready != 0 {
			t.Errorf("OnReady called %d times; want 0", state.ready)
		}
		if err := pool.CheckHealth(); err == nil {
			t.Errorf("CheckHealth = <nil>; want error")
		}

		// Verify that application ID is not set.
		err = withTestConn(dir, "another.db", func(conn *sqlite.Conn) error {
			called := false
			err = sqlitex.ExecTransient(conn, "PRAGMA application_id;", func(stmt *sqlite.Stmt) error {
				called = true
				if got, want := stmt.ColumnInt32(0), int32(0); got != want {
					t.Errorf("application_id = %#x; want %#x", got, want)
				}
				return nil
			})
			if err != nil {
				t.Errorf("PRAGMA application_id: %v", err)
			} else if !called {
				t.Error("PRAGMA application_id not called")
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("OneMigration", func(t *testing.T) {
		schema := Schema{
			AppID: 0xedbeef,
			Migrations: []string{
				`create table foo ( id integer primary key not null );`,
			},
		}
		state := new(eventRecorder)
		pool := NewPool(filepath.Join(dir, "one-migration.db"), schema, Options{
			Flags:          sqlite.SQLITE_OPEN_READWRITE | sqlite.SQLITE_OPEN_CREATE | sqlite.SQLITE_OPEN_NOMUTEX,
			OnStartMigrate: state.startMigrateFunc(),
			OnReady:        state.readyFunc(),
		})
		defer func() {
			if err := pool.Close(); err != nil {
				t.Error("pool.Close:", err)
			}
			if err := pool.CheckHealth(); err == nil {
				t.Error("after Close, CheckHealth() = <nil>; want error")
			}
		}()
		conn, err := pool.Get(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer pool.Put(conn)
		if state.migrationStarted != 1 {
			t.Errorf("OnStartMigrate called %d times; want 1", state.migrationStarted)
		}
		if state.ready != 1 {
			t.Errorf("OnReady called %d times; want 1", state.ready)
		}
		if err := pool.CheckHealth(); err != nil {
			t.Errorf("after successful Get, CheckHealth = %v; want <nil>", err)
		}
		err = sqlitex.ExecTransient(conn, "insert into foo values (42);", nil)
		if err != nil {
			t.Error(err)
		}
	})

	t.Run("TwoMigrations", func(t *testing.T) {
		schema := Schema{
			AppID: 0xedbeef,
			Migrations: []string{
				`create table foo ( id integer primary key not null );`,
				`insert into foo values (42);`,
			},
		}
		state := new(eventRecorder)
		pool := NewPool(filepath.Join(dir, "two-migrations.db"), schema, Options{
			Flags:          sqlite.SQLITE_OPEN_READWRITE | sqlite.SQLITE_OPEN_CREATE | sqlite.SQLITE_OPEN_NOMUTEX,
			OnStartMigrate: state.startMigrateFunc(),
			OnReady:        state.readyFunc(),
		})
		defer func() {
			if err := pool.Close(); err != nil {
				t.Error("pool.Close:", err)
			}
		}()
		conn, err := pool.Get(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer pool.Put(conn)
		if state.migrationStarted != 1 {
			t.Errorf("OnStartMigrate called %d times; want 1", state.migrationStarted)
		}
		if state.ready != 1 {
			t.Errorf("OnReady called %d times; want 1", state.ready)
		}
		if err := pool.CheckHealth(); err != nil {
			t.Errorf("after successful Get, CheckHealth = %v; want <nil>", err)
		}
		var got int
		err = sqlitex.ExecTransient(conn, "select id from foo order by id;", func(stmt *sqlite.Stmt) error {
			got = stmt.ColumnInt(0)
			return nil
		})
		if err != nil {
			t.Error(err)
		} else if got != 42 {
			t.Errorf("select id = %d; want 42", got)
		}
	})

	t.Run("PartialMigration", func(t *testing.T) {
		schema := Schema{
			AppID: 0xedbeef,
			Migrations: []string{
				`create table foo ( id integer primary key not null ); insert into foo values (1);`,
				`insert into foo values (42); insert into bar values (57);`,
			},
		}
		state := new(eventRecorder)
		pool := NewPool(filepath.Join(dir, "partial-migration.db"), schema, Options{
			Flags:          sqlite.SQLITE_OPEN_READWRITE | sqlite.SQLITE_OPEN_CREATE | sqlite.SQLITE_OPEN_NOMUTEX,
			OnStartMigrate: state.startMigrateFunc(),
			OnReady:        state.readyFunc(),
		})
		defer func() {
			if err := pool.Close(); err != nil {
				t.Error("pool.Close:", err)
			}
			if err := pool.CheckHealth(); err == nil {
				t.Error("after Close, CheckHealth() = <nil>; want error")
			}
		}()
		conn, err := pool.Get(ctx)
		t.Logf("pool.Get error: %v", err)
		if err == nil {
			pool.Put(conn)
			return
		}
		if state.migrationStarted != 1 {
			t.Errorf("OnStartMigrate called %d times; want 1", state.migrationStarted)
		}
		if state.ready != 0 {
			t.Errorf("OnReady called %d times; want 0", state.ready)
		}
		if err := pool.CheckHealth(); err == nil {
			t.Error("CheckHealth() = <nil>; want error")
		}

		// Verify that the first migration is applied and that none of the second
		// migration is applied.
		withTestConn(dir, "partial-migration.db", func(conn *sqlite.Conn) error {
			var got int
			err = sqlitex.ExecTransient(conn, "select id from foo order by id;", func(stmt *sqlite.Stmt) error {
				got = stmt.ColumnInt(0)
				return nil
			})
			if err != nil {
				return err
			}
			if got != 1 {
				t.Errorf("select id = %d; want 1", got)
			}
			return nil
		})
	})

	t.Run("MigrationsDontRepeat", func(t *testing.T) {
		schema := Schema{
			AppID: 0xedbeef,
			Migrations: []string{
				`create table foo ( id integer primary key not null );`,
			},
		}

		// Run 1
		pool := NewPool(filepath.Join(dir, "migrations-dont-repeat.db"), schema, Options{
			Flags: sqlite.SQLITE_OPEN_READWRITE | sqlite.SQLITE_OPEN_CREATE | sqlite.SQLITE_OPEN_NOMUTEX,
		})
		conn, err := pool.Get(ctx)
		if err != nil {
			pool.Close()
			t.Fatal(err)
		}
		err = sqlitex.ExecTransient(conn, "insert into foo values (42);", nil)
		if err != nil {
			t.Error(err)
		}
		pool.Put(conn)
		if err := pool.Close(); err != nil {
			t.Error("pool.Close:", err)
		}

		// Run 2
		pool = NewPool(filepath.Join(dir, "migrations-dont-repeat.db"), schema, Options{
			Flags: sqlite.SQLITE_OPEN_READWRITE | sqlite.SQLITE_OPEN_CREATE | sqlite.SQLITE_OPEN_NOMUTEX,
		})
		conn, err = pool.Get(ctx)
		if err != nil {
			pool.Close()
			t.Fatal(err)
		}
		err = sqlitex.ExecTransient(conn, "insert into foo values (56);", nil)
		if err != nil {
			t.Error(err)
		}
		pool.Put(conn)
		if err := pool.Close(); err != nil {
			t.Error("pool.Close:", err)
		}
	})

	t.Run("IncrementalMigration", func(t *testing.T) {
		schema1 := Schema{
			AppID: 0xedbeef,
			Migrations: []string{
				`create table foo ( id integer primary key not null );`,
			},
		}
		schema2 := Schema{
			AppID: 0xedbeef,
			Migrations: []string{
				`create table foo ( id integer primary key not null );`,
				`insert into foo values (42);`,
			},
		}

		// Run 1
		pool := NewPool(filepath.Join(dir, "incremental-migration.db"), schema1, Options{
			Flags: sqlite.SQLITE_OPEN_READWRITE | sqlite.SQLITE_OPEN_CREATE | sqlite.SQLITE_OPEN_NOMUTEX,
		})
		conn, err := pool.Get(ctx)
		if err != nil {
			pool.Close()
			t.Fatal(err)
		}
		pool.Put(conn)
		if err := pool.Close(); err != nil {
			t.Error("pool.Close:", err)
		}

		// Run 2
		pool = NewPool(filepath.Join(dir, "incremental-migration.db"), schema2, Options{
			Flags: sqlite.SQLITE_OPEN_READWRITE | sqlite.SQLITE_OPEN_CREATE | sqlite.SQLITE_OPEN_NOMUTEX,
		})
		conn, err = pool.Get(ctx)
		if err != nil {
			pool.Close()
			t.Fatal(err)
		}
		var got int
		err = sqlitex.ExecTransient(conn, "select id from foo order by id;", func(stmt *sqlite.Stmt) error {
			got = stmt.ColumnInt(0)
			return nil
		})
		if err != nil {
			t.Error(err)
		} else if got != 42 {
			t.Errorf("select id = %d; want 42", got)
		}
		pool.Put(conn)
		if err := pool.Close(); err != nil {
			t.Error("pool.Close:", err)
		}
	})

	t.Run("FutureVersion", func(t *testing.T) {
		schema1 := Schema{
			AppID: 0xedbeef,
			Migrations: []string{
				`create table foo ( id integer primary key not null );`,
				`insert into foo values (42);`,
			},
		}
		schema2 := Schema{
			AppID: 0xedbeef,
			Migrations: []string{
				`create table foo ( id integer primary key not null );`,
			},
		}

		// Run 1
		pool := NewPool(filepath.Join(dir, "future-version.db"), schema1, Options{
			Flags: sqlite.SQLITE_OPEN_READWRITE | sqlite.SQLITE_OPEN_CREATE | sqlite.SQLITE_OPEN_NOMUTEX,
		})
		conn, err := pool.Get(ctx)
		if err != nil {
			pool.Close()
			t.Fatal(err)
		}
		pool.Put(conn)
		if err := pool.Close(); err != nil {
			t.Error("pool.Close:", err)
		}

		// Run 2
		pool = NewPool(filepath.Join(dir, "future-version.db"), schema2, Options{
			Flags: sqlite.SQLITE_OPEN_READWRITE | sqlite.SQLITE_OPEN_CREATE | sqlite.SQLITE_OPEN_NOMUTEX,
		})
		conn, err = pool.Get(ctx)
		if err != nil {
			pool.Close()
			t.Fatal(err)
		}
		var got int
		err = sqlitex.ExecTransient(conn, "select id from foo order by id;", func(stmt *sqlite.Stmt) error {
			got = stmt.ColumnInt(0)
			return nil
		})
		if err != nil {
			t.Error(err)
		} else if got != 42 {
			t.Errorf("select id = %d; want 42", got)
		}
		pool.Put(conn)
		if err := pool.Close(); err != nil {
			t.Error("pool.Close:", err)
		}
	})
}

// withTestConn makes an independent connection to the given database.
func withTestConn(dir, name string, f func(*sqlite.Conn) error) error {
	conn, err := sqlite.OpenConn(filepath.Join(dir, name), sqlite.SQLITE_OPEN_READWRITE|sqlite.SQLITE_OPEN_CREATE|sqlite.SQLITE_OPEN_NOMUTEX)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := f(conn); err != nil {
		return err
	}
	return nil
}

type eventRecorder struct {
	migrationStarted int
	ready            int
}

func (rec *eventRecorder) startMigrateFunc() SignalFunc {
	return func() {
		rec.migrationStarted++
	}
}

func (rec *eventRecorder) readyFunc() SignalFunc {
	return func() {
		rec.ready++
	}
}
