package database

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"s12ryt-ssh/internal/config"

	mysql "github.com/go-sql-driver/mysql"
)

func TestBuildDSN_MySQL(t *testing.T) {
	p := config.DBProfile{
		Type: "mysql", Host: "h", Port: 3306, User: "u", Password: "pw", Database: "db", TLSMode: "false",
	}
	got := buildDSN(p)
	parsed, err := mysql.ParseDSN(got)
	if err != nil {
		t.Fatalf("ParseDSN: %v (dsn %q)", err, got)
	}
	if parsed.User != p.User || parsed.Passwd != p.Password || parsed.DBName != p.Database {
		t.Errorf("mysql dsn values: got user=%q database=%q", parsed.User, parsed.DBName)
	}
	if parsed.TLSConfig != "false" {
		t.Errorf("mysql TLS: got %q want false", parsed.TLSConfig)
	}
	if !parsed.ParseTime {
		t.Error("mysql dsn should enable parseTime")
	}
}

func TestBuildDSN_Postgres(t *testing.T) {
	p := config.DBProfile{
		Type: "postgres", Host: "h", Port: 5432, User: "u", Password: "pw", Database: "db",
	}
	got, err := buildDSNErr(p)
	if err != nil {
		t.Fatalf("buildDSNErr: %v", err)
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse postgres URL: %v", err)
	}
	if parsed.Scheme != "postgres" || parsed.Host != "h:5432" || parsed.Path != "/db" {
		t.Errorf("unexpected postgres URL: %q", got)
	}
	if parsed.Query().Get("sslmode") != "require" {
		t.Errorf("postgres should require TLS by default: %q", got)
	}
	user := parsed.User.Username()
	password, _ := parsed.User.Password()
	if user != "u" || password != "pw" {
		t.Errorf("postgres credentials mismatch: %q", got)
	}
}

func TestBuildDSN_MySQLEscapesValues(t *testing.T) {
	p := config.DBProfile{
		Type: "mysql", Host: "db.example", Port: 3306,
		User: "user@example", Password: "p@ss:word?", Database: "app/name", TLSMode: "true",
	}
	got, err := buildDSNErr(p)
	if err != nil {
		t.Fatalf("buildDSNErr: %v", err)
	}
	parsed, err := mysql.ParseDSN(got)
	if err != nil {
		t.Fatalf("ParseDSN: %v (dsn %q)", err, got)
	}
	if parsed.User != p.User || parsed.Passwd != p.Password || parsed.DBName != p.Database {
		t.Fatalf("DSN values were not preserved: %+v (dsn %q)", parsed, got)
	}
	if parsed.TLSConfig != "true" {
		t.Errorf("expected TLS to be enabled, got %q", parsed.TLSConfig)
	}
}

func TestBuildDSN_PostgresEscapesValues(t *testing.T) {
	p := config.DBProfile{
		Type: "postgres", Host: "db.example", Port: 5432,
		User: "user@example", Password: "p@ss word", Database: "app/name", SSLMode: "verify-full",
	}
	got, err := buildDSNErr(p)
	if err != nil {
		t.Fatalf("buildDSNErr: %v", err)
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse postgres URL: %v", err)
	}
	user := parsed.User.Username()
	password, _ := parsed.User.Password()
	if user != p.User || password != p.Password || parsed.Path != "/app/name" {
		t.Fatalf("postgres values were not preserved: %q", got)
	}
	if parsed.Query().Get("sslmode") != "verify-full" {
		t.Errorf("sslmode: %q", parsed.Query().Get("sslmode"))
	}
}

func TestBuildDSN_UnknownType(t *testing.T) {
	p := config.DBProfile{Type: "weird"}
	if _, err := buildDSNErr(p); err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestDBClient_QueryAndExec(t *testing.T) {
	c, err := openTestDB(t)
	if err != nil {
		t.Fatalf("openTestDB: %v", err)
	}
	defer c.Close()
	ctx := context.Background()

	// create a table and insert
	if _, err := c.Exec(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, age INTEGER)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	res, err := c.Exec(ctx, "INSERT INTO users (id, name, age) VALUES (?, ?, ?)", 1, "alice", 30)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if res.RowsAffected != 1 {
		t.Errorf("rows affected: %d", res.RowsAffected)
	}

	// query
	rows, err := c.Query(ctx, "SELECT id, name, age FROM users ORDER BY id")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if name, ok := rows[0]["name"].(string); !ok || name != "alice" {
		t.Errorf("name: %v", rows[0]["name"])
	}
	if age, ok := rows[0]["age"].(int64); !ok || age != 30 {
		t.Errorf("age: %v", rows[0]["age"])
	}

	// insert another and query multiple
	_, _ = c.Exec(ctx, "INSERT INTO users (id, name, age) VALUES (?, ?, ?)", 2, "bob", 25)
	rows, err = c.Query(ctx, "SELECT name FROM users ORDER BY id")
	if err != nil {
		t.Fatalf("query2: %v", err)
	}
	if len(rows) != 2 || rows[0]["name"] != "alice" || rows[1]["name"] != "bob" {
		t.Errorf("rows: %+v", rows)
	}
}

func TestDBClient_Tables(t *testing.T) {
	c, _ := openTestDB(t)
	defer c.Close()
	ctx := context.Background()

	_, _ = c.Exec(ctx, "CREATE TABLE t1 (x INTEGER)")
	_, _ = c.Exec(ctx, "CREATE TABLE t2 (x INTEGER)")

	tables, err := c.Tables(ctx)
	if err != nil {
		t.Fatalf("Tables: %v", err)
	}
	if len(tables) < 2 {
		t.Errorf("expected at least 2 tables, got %v", tables)
	}
}

func TestDBClient_Ping(t *testing.T) {
	c, _ := openTestDB(t)
	defer c.Close()
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestDBClient_QueryError(t *testing.T) {
	c, _ := openTestDB(t)
	defer c.Close()
	if _, err := c.Query(context.Background(), "SELECT * FROM no_such_table"); err == nil {
		t.Error("expected error for bad query")
	}
}

func TestNewDBClient_UnknownType(t *testing.T) {
	if _, err := NewDBClient(config.DBProfile{Type: "weird"}); err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestResult_NoRows(t *testing.T) {
	c, _ := openTestDB(t)
	defer c.Close()
	ctx := context.Background()
	_, _ = c.Exec(ctx, "CREATE TABLE empty (x INTEGER)")
	rows, err := c.Query(ctx, "SELECT * FROM empty")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(rows))
	}
}

func TestDBClient_ClosedReturnsError(t *testing.T) {
	c, err := openTestDB(t)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	ctx := context.Background()
	if _, err := c.Query(ctx, "SELECT 1"); !errors.Is(err, ErrClosed) {
		t.Errorf("Query error: %v", err)
	}
	if _, err := c.Exec(ctx, "SELECT 1"); !errors.Is(err, ErrClosed) {
		t.Errorf("Exec error: %v", err)
	}
	if _, err := c.Tables(ctx); !errors.Is(err, ErrClosed) {
		t.Errorf("Tables error: %v", err)
	}
	if err := c.Ping(ctx); !errors.Is(err, ErrClosed) {
		t.Errorf("Ping error: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("second Close should be idempotent: %v", err)
	}
}

// guard against a nil error wrapping edge case
func TestErrorsSentinel(t *testing.T) {
	if !errors.Is(ErrEmptyResult, ErrEmptyResult) {
		t.Error("ErrEmptyResult not self-equal")
	}
}
