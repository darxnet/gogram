package sqlitetest_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/darxnet/gogram/sqlstore"
	_ "modernc.org/sqlite"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}

	db.SetMaxOpenConns(1) // one shared in-memory database

	t.Cleanup(func() { _ = db.Close() })

	return db
}

func TestStore_SetGetFinish(t *testing.T) {
	db := openDB(t)
	ctx := t.Context()

	store, err := sqlstore.New(ctx, sqlstore.SQLite, db)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close() //nolint:errcheck

	if err = store.SetState(ctx, 1, 2, "a"); err != nil {
		t.Fatal(err)
	}

	if err = store.SetState(ctx, 1, 2, "b"); err != nil { // upsert
		t.Fatal(err)
	}

	state, err := store.GetState(ctx, 1, 2)
	if err != nil || state != "b" {
		t.Fatalf("GetState = %q, %v; want b", state, err)
	}

	if state, _ = store.GetState(ctx, 9, 9); state != "" {
		t.Fatalf("missing row = %q, want empty", state)
	}

	if err = store.Finish(ctx, 1, 2); err != nil {
		t.Fatal(err)
	}

	if state, _ = store.GetState(ctx, 1, 2); state != "" {
		t.Fatalf("after Finish = %q, want empty", state)
	}

	if n, err := store.Cleanup(ctx); n != 0 || err != nil { // no TTL: no-op
		t.Fatalf("Cleanup = %d, %v", n, err)
	}
}

func TestStore_TTL(t *testing.T) {
	db := openDB(t)
	ctx := t.Context()

	store, err := sqlstore.New(ctx, sqlstore.SQLite, db, sqlstore.WithTTL(time.Second), sqlstore.WithTable("fsm_ttl"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close() //nolint:errcheck

	if err = store.SetState(ctx, 1, 2, "fresh"); err != nil {
		t.Fatal(err)
	}

	if state, _ := store.GetState(ctx, 1, 2); state != "fresh" {
		t.Fatalf("fresh row = %q", state)
	}

	// Age the row past the TTL instead of sleeping.
	if _, err = db.ExecContext(ctx, `UPDATE fsm_ttl SET updated_at = datetime('now', '-5 seconds')`); err != nil {
		t.Fatal(err)
	}

	if state, _ := store.GetState(ctx, 1, 2); state != "" {
		t.Fatalf("expired row = %q, want empty", state)
	}

	n, err := store.Cleanup(ctx)
	if err != nil || n != 1 {
		t.Fatalf("Cleanup = %d, %v; want 1", n, err)
	}
}

func TestNew_UnsupportedDialect(t *testing.T) {
	if _, err := sqlstore.New(t.Context(), "oracle", openDB(t)); err == nil {
		t.Fatal("expected an error for an unsupported dialect")
	}
}
