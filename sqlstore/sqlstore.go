// Package sqlstore provides a [gogram.StateStore] backed by database/sql
// (PostgreSQL, MySQL, SQLite), with an optional TTL for inactive states.
package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/darxnet/gogram"
)

var (
	errNilDatabase = errors.New("sqlstore: nil database")
	errDialect     = errors.New("sqlstore: unsupported dialect")
)

// Dialect selects the SQL flavour of the database.
type Dialect string

// Supported dialects.
const (
	PostgreSQL Dialect = "postgres"
	MySQL      Dialect = "mysql"
	SQLite     Dialect = "sqlite3"
)

const defaultTable = "gogram_fsm_states"

// Option configures a [Store].
type Option func(*Store)

// WithTTL makes states older than ttl invisible to GetState. Expired rows
// are removed by [Store.Cleanup], or periodically with [WithCleanupInterval].
func WithTTL(ttl time.Duration) Option {
	return func(s *Store) {
		s.ttl = ttl
	}
}

// WithCleanupInterval runs [Store.Cleanup] every interval in a goroutine
// stopped by [Store.Close]. It requires [WithTTL].
func WithCleanupInterval(interval time.Duration) Option {
	return func(s *Store) {
		s.cleanupEvery = interval
	}
}

// WithTable sets the table name. Default: "gogram_fsm_states".
func WithTable(table string) Option {
	return func(s *Store) {
		s.table = table
	}
}

// Store is a persistent, SQL-backed [gogram.StateStore].
type Store struct {
	db           *sql.DB
	dialect      Dialect
	table        string
	ttl          time.Duration
	cleanupEvery time.Duration

	upsert, selectOne, deleteOne, cleanup *sql.Stmt

	closeOnce sync.Once
	done      chan struct{}
}

var _ gogram.StateStore = (*Store)(nil)

// New creates the table if needed, prepares the statements and returns the
// store. ctx bounds the setup queries only.
func New(ctx context.Context, dialect Dialect, db *sql.DB, opts ...Option) (*Store, error) {
	if db == nil {
		return nil, errNilDatabase
	}

	s := &Store{db: db, dialect: dialect, table: defaultTable, done: make(chan struct{})}
	for _, opt := range opts {
		opt(s)
	}

	d, ok := dialects[dialect]
	if !ok {
		return nil, fmt.Errorf("%w: %q", errDialect, dialect)
	}

	if _, err := db.ExecContext(ctx, fmt.Sprintf(d.create, s.table)); err != nil {
		return nil, fmt.Errorf("sqlstore: create table: %w", err)
	}

	if err := s.prepare(ctx, d); err != nil {
		_ = s.Close()
		return nil, err
	}

	if s.ttl > 0 && s.cleanupEvery > 0 {
		go s.cleanupLoop() //nolint:gosec,contextcheck // the loop outlives the setup ctx; Close stops it
	}

	return s, nil
}

// dialectSQL holds the statements of one SQL flavour; %s is the table name,
// %d in expired the TTL in seconds.
type dialectSQL struct {
	create, upsert, expired string
	placeholder             func(n int) string
}

var dialects = map[Dialect]dialectSQL{
	PostgreSQL: {
		create: `CREATE TABLE IF NOT EXISTS %s (
			user_id BIGINT NOT NULL,
			chat_id BIGINT NOT NULL,
			state VARCHAR(255) NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (user_id, chat_id))`,
		upsert: `INSERT INTO %s (user_id, chat_id, state, updated_at) VALUES ($1, $2, $3, NOW())
			ON CONFLICT (user_id, chat_id) DO UPDATE SET state = EXCLUDED.state, updated_at = NOW()`,
		expired:     `updated_at <= NOW() - INTERVAL '%d seconds'`,
		placeholder: func(n int) string { return "$" + strconv.Itoa(n) },
	},
	MySQL: {
		create: `CREATE TABLE IF NOT EXISTS %s (
			user_id BIGINT NOT NULL,
			chat_id BIGINT NOT NULL,
			state VARCHAR(255) NOT NULL,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, chat_id))`,
		upsert: `INSERT INTO %s (user_id, chat_id, state, updated_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)
			ON DUPLICATE KEY UPDATE state = VALUES(state), updated_at = CURRENT_TIMESTAMP`,
		expired:     `updated_at <= NOW() - INTERVAL %d SECOND`,
		placeholder: func(int) string { return "?" },
	},
	SQLite: {
		create: `CREATE TABLE IF NOT EXISTS %s (
			user_id INTEGER NOT NULL,
			chat_id INTEGER NOT NULL,
			state TEXT NOT NULL,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, chat_id))`,
		upsert: `INSERT INTO %s (user_id, chat_id, state, updated_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT (user_id, chat_id) DO UPDATE SET state = excluded.state, updated_at = CURRENT_TIMESTAMP`,
		expired:     `updated_at <= datetime('now', '-%d seconds')`,
		placeholder: func(int) string { return "?" },
	},
}

func (s *Store) prepare(ctx context.Context, d dialectSQL) error {
	key := "user_id = " + d.placeholder(1) + " AND chat_id = " + d.placeholder(2)

	selectOne := "SELECT state FROM " + s.table + " WHERE " + key
	if s.ttl > 0 {
		selectOne += " AND NOT (" + s.expired(d) + ")"
	}

	statements := []struct {
		dst   **sql.Stmt
		query string
	}{
		{&s.upsert, fmt.Sprintf(d.upsert, s.table)},
		{&s.selectOne, selectOne},
		{&s.deleteOne, "DELETE FROM " + s.table + " WHERE " + key},
	}

	if s.ttl > 0 {
		statements = append(statements, struct {
			dst   **sql.Stmt
			query string
		}{&s.cleanup, "DELETE FROM " + s.table + " WHERE " + s.expired(d)})
	}

	for _, st := range statements {
		stmt, err := s.db.PrepareContext(ctx, st.query) //nolint:sqlclosecheck // closed in Store.Close
		if err != nil {
			return fmt.Errorf("sqlstore: prepare %q: %w", strings.Fields(st.query)[0], err)
		}

		*st.dst = stmt
	}

	return nil
}

func (s *Store) expired(d dialectSQL) string {
	return fmt.Sprintf(d.expired, max(int64(s.ttl/time.Second), 1))
}

func (s *Store) cleanupLoop() {
	ticker := time.NewTicker(s.cleanupEvery)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), s.cleanupEvery)
			_, _ = s.Cleanup(ctx)
			cancel()
		}
	}
}

// SetState saves or updates the state for a user-chat pair.
func (s *Store) SetState(ctx context.Context, userID, chatID int64, state gogram.State) error {
	_, err := s.upsert.ExecContext(ctx, userID, chatID, string(state))
	return err
}

// GetState retrieves the state for a user-chat pair; ("", nil) when there is
// none or it has expired.
func (s *Store) GetState(ctx context.Context, userID, chatID int64) (gogram.State, error) {
	var state string

	err := s.selectOne.QueryRowContext(ctx, userID, chatID).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}

	return gogram.State(state), err
}

// Finish removes the state for a user-chat pair.
func (s *Store) Finish(ctx context.Context, userID, chatID int64) error {
	_, err := s.deleteOne.ExecContext(ctx, userID, chatID)
	return err
}

// Cleanup deletes expired rows and returns their number. Without a TTL it
// is a no-op.
func (s *Store) Cleanup(ctx context.Context) (int64, error) {
	if s.cleanup == nil {
		return 0, nil
	}

	res, err := s.cleanup.ExecContext(ctx)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}

// Close stops the cleanup goroutine and closes the prepared statements.
// It does not close the *sql.DB; that remains the caller's responsibility.
func (s *Store) Close() error {
	s.closeOnce.Do(func() { close(s.done) })

	var err error

	for _, stmt := range []*sql.Stmt{s.upsert, s.selectOne, s.deleteOne, s.cleanup} {
		if stmt != nil {
			err = errors.Join(err, stmt.Close())
		}
	}

	return err
}
