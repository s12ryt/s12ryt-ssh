package database

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// openTestDB returns a fresh in-memory SQLite DBClient so database tests run
// without an external server and without CGO. SQLite is in-process (no
// networking), so each test can safely own a private database. A single
// pooled connection keeps the in-memory schema visible across queries.
func openTestDB(t *testing.T) (*DBClient, error) {
	t.Helper()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	c := &DBClient{db: db, driver: "sqlite"}
	t.Cleanup(func() { c.Close() })
	return c, nil
}
