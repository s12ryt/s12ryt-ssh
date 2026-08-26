package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"s12ryt-ssh/internal/config"

	// Database drivers registered for remote connections.
	mysql "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// ErrEmptyResult is returned when a query unexpectedly yields no rows.
var ErrEmptyResult = errors.New("database: empty result")

// ErrClosed is returned when an operation is attempted after Close.
var ErrClosed = errors.New("database: client is closed")

// Result describes the outcome of an Exec statement.
type Result struct {
	Columns      []string
	RowsAffected int64
	LastInsertID int64
}

// Row is a single record keyed by column name.
type Row = map[string]interface{}

// Database is the common interface for SQL database backends.
type Database interface {
	Query(ctx context.Context, query string, args ...interface{}) ([]Row, error)
	Exec(ctx context.Context, query string, args ...interface{}) (Result, error)
	Tables(ctx context.Context) ([]string, error)
	Ping(ctx context.Context) error
	Close() error
}

// DBClient implements Database over database/sql.
type DBClient struct {
	db     *sql.DB
	driver string
}

// NewDBClient opens a connection to the database described by profile.
func NewDBClient(p config.DBProfile) (*DBClient, error) {
	driver, err := driverFor(p.Type)
	if err != nil {
		return nil, err
	}
	dsn, err := buildDSNErr(p)
	if err != nil {
		return nil, err
	}
	return openDB(driver, dsn)
}

// openDB opens a database/sql connection with the given driver and DSN.
func openDB(driverName, dsn string) (*DBClient, error) {
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(2)
	return &DBClient{db: db, driver: driverName}, nil
}

// driverFor maps a profile type to its database/sql driver name.
func driverFor(typ string) (string, error) {
	switch strings.ToLower(typ) {
	case "mysql":
		return "mysql", nil
	case "postgres", "postgresql", "pg":
		return "pgx", nil
	default:
		return "", fmt.Errorf("database: unsupported type %q", typ)
	}
}

// buildDSNErr builds a connection DSN for the profile.
func buildDSNErr(p config.DBProfile) (string, error) {
	switch strings.ToLower(p.Type) {
	case "mysql":
		cfg := mysql.Config{
			User:      p.User,
			Passwd:    p.Password,
			Net:       "tcp",
			Addr:      net.JoinHostPort(p.Host, strconv.Itoa(p.Port)),
			DBName:    p.Database,
			ParseTime: true,
			TLSConfig: mysqlTLSMode(p.TLSMode),
		}
		return cfg.FormatDSN(), nil
	case "postgres", "postgresql", "pg":
		sslMode := strings.TrimSpace(p.SSLMode)
		if sslMode == "" {
			sslMode = "require"
		}
		dsn := url.URL{
			Scheme: "postgres",
			Host:   net.JoinHostPort(p.Host, strconv.Itoa(p.Port)),
			Path:   "/" + strings.TrimPrefix(p.Database, "/"),
			User:   url.UserPassword(p.User, p.Password),
		}
		query := dsn.Query()
		query.Set("sslmode", sslMode)
		dsn.RawQuery = query.Encode()
		return dsn.String(), nil
	default:
		return "", fmt.Errorf("database: unsupported type %q", p.Type)
	}
}

func mysqlTLSMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		return "true"
	}
	return mode
}

// buildDSN is a convenience wrapper that returns "" on error.
func buildDSN(p config.DBProfile) string {
	dsn, _ := buildDSNErr(p)
	return dsn
}

// Query runs a SELECT-like statement and returns all rows as maps.
func (c *DBClient) Query(ctx context.Context, query string, args ...interface{}) ([]Row, error) {
	db, err := c.connection()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var out []Row
	for rows.Next() {
		values := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := Row{}
		for i, col := range cols {
			row[col] = normalizeValue(values[i])
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// Exec runs a statement that does not return rows.
func (c *DBClient) Exec(ctx context.Context, query string, args ...interface{}) (Result, error) {
	db, err := c.connection()
	if err != nil {
		return Result{}, err
	}
	res, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return Result{}, err
	}
	affected, _ := res.RowsAffected()
	lastID, _ := res.LastInsertId()
	return Result{RowsAffected: affected, LastInsertID: lastID}, nil
}

// Tables lists user tables in the current database.
func (c *DBClient) Tables(ctx context.Context) ([]string, error) {
	db, err := c.connection()
	if err != nil {
		return nil, err
	}
	query := tablesQuery(c.driver)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

// tablesQuery returns the table-listing SQL for the given driver.
func tablesQuery(driver string) string {
	switch driver {
	case "mysql":
		return `SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() ORDER BY table_name`
	case "pgx":
		return `SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name`
	default: // sqlite and any unknown driver
		return `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`
	}
}

// Ping verifies the connection is alive.
func (c *DBClient) Ping(ctx context.Context) error {
	db, err := c.connection()
	if err != nil {
		return err
	}
	return db.PingContext(ctx)
}

// Close releases the connection pool.
func (c *DBClient) Close() error {
	if c == nil || c.db == nil {
		return nil
	}
	err := c.db.Close()
	c.db = nil
	return err
}

func (c *DBClient) connection() (*sql.DB, error) {
	if c == nil || c.db == nil {
		return nil, ErrClosed
	}
	return c.db, nil
}

// normalizeValue converts database driver-specific types into plain values.
func normalizeValue(v interface{}) interface{} {
	switch t := v.(type) {
	case []byte:
		return string(t)
	case *[]byte:
		if t == nil {
			return nil
		}
		return string(*t)
	default:
		return v
	}
}
