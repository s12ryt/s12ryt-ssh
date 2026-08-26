package vault

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"s12ryt-ssh/internal/database"
	"s12ryt-ssh/internal/storage"
)

// ErrNotFound means that the requested remote vault record does not exist.
var ErrNotFound = errors.New("vault: record not found")

// Backend stores and retrieves an encrypted vault envelope.
type Backend interface {
	Save(ctx context.Context, id string, payload []byte) error
	Load(ctx context.Context, id string) ([]byte, error)
	Delete(ctx context.Context, id string) error
}

// ObjectBackend stores one envelope as an object in an S3-compatible bucket.
type ObjectBackend struct {
	store  storage.Storage
	prefix string
}

// NewObjectBackend creates an object-backed vault repository.
func NewObjectBackend(store storage.Storage) *ObjectBackend {
	return &ObjectBackend{store: store, prefix: "s12ryt/vault/"}
}

// Save writes the encrypted envelope under the vault identity.
func (b *ObjectBackend) Save(ctx context.Context, id string, payload []byte) error {
	key, err := objectKey(id)
	if err != nil {
		return err
	}
	if b == nil || b.store == nil {
		return errors.New("vault: object backend is not configured")
	}
	return b.store.Put(ctx, key, payload)
}

// Load reads the encrypted envelope for the vault identity.
func (b *ObjectBackend) Load(ctx context.Context, id string) ([]byte, error) {
	key, err := objectKey(id)
	if err != nil {
		return nil, err
	}
	if b == nil || b.store == nil {
		return nil, errors.New("vault: object backend is not configured")
	}
	payload, err := b.store.Get(ctx, key)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return payload, err
}

// Delete removes the encrypted envelope.
func (b *ObjectBackend) Delete(ctx context.Context, id string) error {
	key, err := objectKey(id)
	if err != nil {
		return err
	}
	if b == nil || b.store == nil {
		return errors.New("vault: object backend is not configured")
	}
	return b.store.Delete(ctx, key)
}

// SQLBackend stores one envelope in a database table.
type SQLBackend struct {
	db      database.Database
	dialect string
}

// NewSQLBackend creates a SQL-backed vault repository. dialect accepts mysql,
// postgres/postgresql/pg, or sqlite (the latter is useful for local tests).
func NewSQLBackend(db database.Database, dialect string) (*SQLBackend, error) {
	if db == nil {
		return nil, errors.New("vault: SQL backend requires a database")
	}
	dialect = normalizeDialect(dialect)
	switch dialect {
	case "mysql", "postgres", "sqlite":
		return &SQLBackend{db: db, dialect: dialect}, nil
	default:
		return nil, fmt.Errorf("vault: unsupported SQL dialect %q", dialect)
	}
}

// Save creates the vault table if necessary and upserts the envelope.
func (b *SQLBackend) Save(ctx context.Context, id string, payload []byte) error {
	if err := b.validate(id); err != nil {
		return err
	}
	if err := b.ensureSchema(ctx); err != nil {
		return err
	}
	_, err := b.db.Exec(ctx, b.saveQuery(), id, string(payload))
	return err
}

// Load returns the envelope for id or ErrNotFound.
func (b *SQLBackend) Load(ctx context.Context, id string) ([]byte, error) {
	if err := b.validate(id); err != nil {
		return nil, err
	}
	if err := b.ensureSchema(ctx); err != nil {
		return nil, err
	}
	rows, err := b.db.Query(ctx, b.loadQuery(), id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNotFound
	}
	value, ok := rows[0]["payload"]
	if !ok {
		return nil, fmt.Errorf("vault: SQL row has no payload")
	}
	switch value := value.(type) {
	case string:
		return []byte(value), nil
	case []byte:
		return append([]byte(nil), value...), nil
	default:
		return nil, fmt.Errorf("vault: unsupported SQL payload type %T", value)
	}
}

// Delete removes the envelope for id.
func (b *SQLBackend) Delete(ctx context.Context, id string) error {
	if err := b.validate(id); err != nil {
		return err
	}
	if err := b.ensureSchema(ctx); err != nil {
		return err
	}
	_, err := b.db.Exec(ctx, b.deleteQuery(), id)
	return err
}

func (b *SQLBackend) validate(id string) error {
	if b == nil || b.db == nil {
		return errors.New("vault: SQL backend is not configured")
	}
	_, err := objectKey(id)
	return err
}

func (b *SQLBackend) ensureSchema(ctx context.Context) error {
	_, err := b.db.Exec(ctx, `CREATE TABLE IF NOT EXISTS s12ryt_vault (
  vault_id VARCHAR(255) PRIMARY KEY,
  payload TEXT NOT NULL,
  updated_at TIMESTAMP NOT NULL
)`)
	return err
}

func (b *SQLBackend) saveQuery() string {
	switch b.dialect {
	case "mysql":
		return `INSERT INTO s12ryt_vault (vault_id, payload, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP) ON DUPLICATE KEY UPDATE payload = VALUES(payload), updated_at = CURRENT_TIMESTAMP`
	case "postgres":
		return `INSERT INTO s12ryt_vault (vault_id, payload, updated_at) VALUES ($1, $2, CURRENT_TIMESTAMP) ON CONFLICT (vault_id) DO UPDATE SET payload = EXCLUDED.payload, updated_at = CURRENT_TIMESTAMP`
	default:
		return `INSERT INTO s12ryt_vault (vault_id, payload, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP) ON CONFLICT (vault_id) DO UPDATE SET payload = excluded.payload, updated_at = CURRENT_TIMESTAMP`
	}
}

func (b *SQLBackend) loadQuery() string {
	if b.dialect == "postgres" {
		return `SELECT payload FROM s12ryt_vault WHERE vault_id = $1`
	}
	return `SELECT payload FROM s12ryt_vault WHERE vault_id = ?`
}

func (b *SQLBackend) deleteQuery() string {
	if b.dialect == "postgres" {
		return `DELETE FROM s12ryt_vault WHERE vault_id = $1`
	}
	return `DELETE FROM s12ryt_vault WHERE vault_id = ?`
}

func objectKey(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", errors.New("vault: id is required")
	}
	if strings.ContainsAny(id, `/\\`) || id == "." || id == ".." {
		return "", errors.New("vault: invalid id")
	}
	return "s12ryt/vault/" + id + ".json", nil
}

func normalizeDialect(dialect string) string {
	switch strings.ToLower(strings.TrimSpace(dialect)) {
	case "postgresql", "pg", "pgx":
		return "postgres"
	default:
		return strings.ToLower(strings.TrimSpace(dialect))
	}
}
