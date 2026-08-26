package securestore

import (
	"bytes"
	"errors"
	"testing"
)

func TestMemoryStoreCopiesSecrets(t *testing.T) {
	s := NewMemoryStore()
	secret := []byte("bootstrap-secret")
	if err := s.Save("vault-1", secret); err != nil {
		t.Fatalf("Save: %v", err)
	}
	secret[0] = 'X'

	got, err := s.Load("vault-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !bytes.Equal(got, []byte("bootstrap-secret")) {
		t.Fatalf("secret was not copied: %q", got)
	}
	got[0] = 'Y'
	again, err := s.Load("vault-1")
	if err != nil {
		t.Fatalf("Load again: %v", err)
	}
	if !bytes.Equal(again, []byte("bootstrap-secret")) {
		t.Fatalf("returned secret was not copied: %q", again)
	}
}

func TestMemoryStoreMissingAndDelete(t *testing.T) {
	s := NewMemoryStore()
	if _, err := s.Load("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if err := s.Save("vault-1", []byte("secret")); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("vault-1"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("vault-1"); err != nil {
		t.Fatalf("Delete should be idempotent: %v", err)
	}
	if _, err := s.Load("vault-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestMemoryStoreValidatesKey(t *testing.T) {
	s := NewMemoryStore()
	for _, operation := range []func() error{
		func() error { return s.Save("", []byte("secret")) },
		func() error { _, err := s.Load(""); return err },
		func() error { return s.Delete("") },
	} {
		if err := operation(); err == nil {
			t.Fatal("expected empty key error")
		}
	}
}
