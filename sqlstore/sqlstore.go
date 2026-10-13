// Package sqlstore provides a [gogram.Storer] backed by database/sql
// (PostgreSQL, MySQL, SQLite).
package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/darxnet/gogram"
)

var errNilDatabase = errors.New("sqlstore: nil database")

// Dialect defines the type of SQL database being used.
type Dialect string

// Supported dialects.
const (
	PostgreSQL Dialect = "postgres"
	MySQL      Dialect = "mysql"
	SQLite     Dialect = "sqlite3"
)

// Storer is a persistent, SQL-backed [gogram.Storer].
type Storer struct {
	db      *sql.DB
	dialect Dialect
	// Prepared statements for performance and security.
	upsertStmt *sql.Stmt
	selectStmt *sql.Stmt
	deleteStmt *sql.Stmt
}

var _ gogram.Storer = (*Storer)(nil)

// New creates a SQL-backed state storer.
// It creates the table if needed and prepares the statements.
// The context is used for the initial setup queries.
func New(ctx context.Context, dialect Dialect, db *sql.DB) (*Storer, error) {
	if db == nil {
		return nil, errNilDatabase
	}
	s := &Storer{
		db:      db,
		dialect: dialect,
	}

	if err := s.initSchema(ctx); err != nil {
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	if err := s.prepareStatements(ctx); err != nil {
		return nil, fmt.Errorf("failed to prepare statements: %w", err)
	}

	return s, nil
}

func (s *Storer) initSchema(ctx context.Context) error {
	var query string
	switch s.dialect {
	case PostgreSQL:
		query = `
		CREATE TABLE IF NOT EXISTS gogram_fsm_states (
			user_id BIGINT NOT NULL,
			chat_id BIGINT NOT NULL,
			state VARCHAR(255) NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (user_id, chat_id)
		);`
	case MySQL:
		query = `
		CREATE TABLE IF NOT EXISTS gogram_fsm_states (
			user_id BIGINT NOT NULL,
			chat_id BIGINT NOT NULL,
			state VARCHAR(255) NOT NULL,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, chat_id)
		);`
	case SQLite:
		query = `
		CREATE TABLE IF NOT EXISTS gogram_fsm_states (
			user_id INTEGER NOT NULL,
			chat_id INTEGER NOT NULL,
			state TEXT NOT NULL,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, chat_id)
		);`
	default:
		return fmt.Errorf("unsupported sql dialect: %s", s.dialect) //nolint:err113
	}

	_, err := s.db.ExecContext(ctx, query)
	return err
}

func (s *Storer) prepareStatements(ctx context.Context) (err error) {
	var upsertQuery string
	switch s.dialect {
	case PostgreSQL:
		upsertQuery = `
		INSERT INTO gogram_fsm_states (user_id, chat_id, state, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (user_id, chat_id) DO UPDATE SET state = EXCLUDED.state, updated_at = NOW();`
	case MySQL:
		upsertQuery = `
		INSERT INTO gogram_fsm_states (user_id, chat_id, state, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON DUPLICATE KEY UPDATE state = VALUES(state), updated_at = CURRENT_TIMESTAMP;`
	case SQLite:
		upsertQuery = `
		INSERT INTO gogram_fsm_states (user_id, chat_id, state, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT (user_id, chat_id) DO UPDATE SET state = excluded.state, updated_at = CURRENT_TIMESTAMP;`
	}

	selectQuery := `SELECT state FROM gogram_fsm_states WHERE user_id = ? AND chat_id = ?;`
	deleteQuery := `DELETE FROM gogram_fsm_states WHERE user_id = ? AND chat_id = ?;`

	// Adjust placeholders for PostgreSQL ($1/$2 style).
	if s.dialect == PostgreSQL {
		selectQuery = strings.Replace(selectQuery, "?", "$1", 1)
		selectQuery = strings.Replace(selectQuery, "?", "$2", 1)
		deleteQuery = strings.Replace(deleteQuery, "?", "$1", 1)
		deleteQuery = strings.Replace(deleteQuery, "?", "$2", 1)
	}

	if s.upsertStmt, err = s.db.PrepareContext(ctx, upsertQuery); err != nil {
		return fmt.Errorf("failed to prepare upsert statement: %w", err)
	}

	if s.selectStmt, err = s.db.PrepareContext(ctx, selectQuery); err != nil {
		_ = s.upsertStmt.Close()
		return fmt.Errorf("failed to prepare select statement: %w", err)
	}

	if s.deleteStmt, err = s.db.PrepareContext(ctx, deleteQuery); err != nil {
		_ = s.upsertStmt.Close()
		_ = s.selectStmt.Close()
		return fmt.Errorf("failed to prepare delete statement: %w", err)
	}

	return nil
}

// SetState saves or updates the state for a user-chat pair.
// The context is forwarded to the underlying SQL driver for cancellation support.
func (s *Storer) SetState(ctx context.Context, userID, chatID int64, state gogram.State) error {
	_, err := s.upsertStmt.ExecContext(ctx, userID, chatID, string(state))
	return err
}

// GetState retrieves the state for a user-chat pair.
// Returns ("", nil) when no state is found.
// The context is forwarded to the underlying SQL driver for cancellation support.
func (s *Storer) GetState(ctx context.Context, userID, chatID int64) (gogram.State, error) {
	var stateStr string

	err := s.selectStmt.QueryRowContext(ctx, userID, chatID).Scan(&stateStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}

	return gogram.State(stateStr), nil
}

// Finish removes the state for a user-chat pair.
// The context is forwarded to the underlying SQL driver for cancellation support.
func (s *Storer) Finish(ctx context.Context, userID, chatID int64) error {
	_, err := s.deleteStmt.ExecContext(ctx, userID, chatID)
	return err
}

// Close closes the prepared statements.
// It does not close the underlying *sql.DB – that remains the caller's responsibility.
func (s *Storer) Close() error {
	if s == nil {
		return nil
	}

	var err error
	if s.upsertStmt != nil {
		err = errors.Join(err, s.upsertStmt.Close())
	}
	if s.selectStmt != nil {
		err = errors.Join(err, s.selectStmt.Close())
	}
	if s.deleteStmt != nil {
		err = errors.Join(err, s.deleteStmt.Close())
	}
	return err
}
