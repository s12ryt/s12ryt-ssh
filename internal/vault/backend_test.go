package vault

import (
	"context"
	"errors"
	"strings"
	"testing"

	"s12ryt-ssh/internal/database"
	"s12ryt-ssh/internal/storage"
)

func TestObjectBackendRoundTrip(t *testing.T) {
	backend := NewObjectBackend(storage.NewMemoryStorage())
	ctx := context.Background()
	payload := []byte(`{"version":1,"ciphertext":"opaque"}`)

	if err := backend.Save(ctx, "vault-1", payload); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := backend.Load(ctx, "vault-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("payload mismatch: %q", got)
	}
	got[0] = 'X'
	again, err := backend.Load(ctx, "vault-1")
	if err != nil {
		t.Fatalf("Load again: %v", err)
	}
	if string(again) != string(payload) {
		t.Fatalf("backend returned mutable payload: %q", again)
	}
	if err := backend.Delete(ctx, "vault-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := backend.Load(ctx, "vault-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestSQLBackendRoundTripAndMissing(t *testing.T) {
	db := newVaultDBStub()
	backend, err := NewSQLBackend(db, "postgres")
	if err != nil {
		t.Fatalf("NewSQLBackend: %v", err)
	}
	ctx := context.Background()
	payload := []byte(`{"version":1,"ciphertext":"opaque"}`)

	if _, err := backend.Load(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected missing record error, got %v", err)
	}
	if err := backend.Save(ctx, "vault-1", payload); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := backend.Save(ctx, "vault-1", []byte("updated")); err != nil {
		t.Fatalf("Save update: %v", err)
	}
	got, err := backend.Load(ctx, "vault-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if string(got) != "updated" {
		t.Fatalf("payload mismatch: %q", got)
	}
	if err := backend.Delete(ctx, "vault-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := backend.Load(ctx, "vault-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected missing after delete, got %v", err)
	}
	for _, query := range db.queries {
		if strings.Contains(strings.ToLower(query), "vault_id = '") {
			t.Fatalf("query interpolated vault id: %q", query)
		}
	}
}

type vaultDBStub struct {
	payload string
	queries []string
}

func newVaultDBStub() *vaultDBStub { return &vaultDBStub{} }

func (s *vaultDBStub) Query(_ context.Context, query string, args ...interface{}) ([]database.Row, error) {
	s.queries = append(s.queries, query)
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(query)), "SELECT") && s.payload != "" {
		return []database.Row{{"payload": s.payload}}, nil
	}
	return nil, nil
}

func (s *vaultDBStub) Exec(_ context.Context, query string, args ...interface{}) (database.Result, error) {
	s.queries = append(s.queries, query)
	upper := strings.ToUpper(strings.TrimSpace(query))
	switch {
	case strings.HasPrefix(upper, "INSERT"):
		if len(args) != 2 {
			return database.Result{}, errors.New("expected id and payload args")
		}
		s.payload, _ = args[1].(string)
	case strings.HasPrefix(upper, "DELETE"):
		s.payload = ""
	}
	return database.Result{}, nil
}

func (s *vaultDBStub) Tables(context.Context) ([]string, error) { return nil, nil }
func (s *vaultDBStub) Ping(context.Context) error               { return nil }
func (s *vaultDBStub) Close() error                             { return nil }
