// Package sqlitetest is a test-only module: it runs the sqlstore tests
// against an in-memory SQLite database. It is its own module so that the
// SQLite driver never enters the library's dependency graph.
package sqlitetest
