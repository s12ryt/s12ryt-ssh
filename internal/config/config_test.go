package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManager_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	mgr := NewManager(path)

	store := &Store{
		SSH: []SSHProfile{
			{Name: "web-server", Host: "10.0.0.1", Port: 22, User: "root", KeyPath: "/tmp/id_rsa"},
		},
		S3: []S3Profile{
			{Name: "r2-bucket", Endpoint: "https://xxx.r2.cloudflarestorage.com", Region: "auto", AccessKey: "ak", SecretKey: "sk", Bucket: "data", UsePathStyle: true},
		},
		DB: []DBProfile{
			{Name: "prod-mysql", Type: "mysql", Host: "db.local", Port: 3306, User: "admin", Password: "pw", Database: "app"},
			{Name: "prod-pg", Type: "postgres", Host: "pg.local", Port: 5432, User: "admin", Password: "pw", Database: "app"},
		},
	}

	if err := mgr.Save(store); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	loaded, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.SSH) != 1 || loaded.SSH[0].Name != "web-server" {
		t.Errorf("SSH mismatch: %+v", loaded.SSH)
	}
	if len(loaded.S3) != 1 || loaded.S3[0].Bucket != "data" {
		t.Errorf("S3 mismatch: %+v", loaded.S3)
	}
	if len(loaded.DB) != 2 || loaded.DB[0].Type != "mysql" || loaded.DB[1].Type != "postgres" {
		t.Errorf("DB mismatch: %+v", loaded.DB)
	}
}

func TestManager_LoadEmptyReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.json")
	mgr := NewManager(path)

	store, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load missing file: %v", err)
	}
	if store == nil || store.SSH != nil || store.S3 != nil || store.DB != nil {
		t.Errorf("expected empty default store, got %+v", store)
	}
}

func TestManager_FindSSH(t *testing.T) {
	store := &Store{
		SSH: []SSHProfile{
			{Name: "a", Host: "1.1.1.1", Port: 22, User: "u"},
			{Name: "b", Host: "2.2.2.2", Port: 2222, User: "u2"},
		},
	}
	p, ok := store.FindSSH("b")
	if !ok || p.Port != 2222 {
		t.Errorf("FindSSH b: %+v ok=%v", p, ok)
	}
	if _, ok := store.FindSSH("missing"); ok {
		t.Error("FindSSH should return false for missing")
	}
}

func TestManager_FindS3(t *testing.T) {
	store := &Store{S3: []S3Profile{{Name: "r2", Bucket: "x"}}}
	if p, ok := store.FindS3("r2"); !ok || p.Bucket != "x" {
		t.Errorf("FindS3: %+v ok=%v", p, ok)
	}
}

func TestManager_FindDB(t *testing.T) {
	store := &Store{DB: []DBProfile{{Name: "mysql1", Type: "mysql"}}}
	if p, ok := store.FindDB("mysql1"); !ok || p.Type != "mysql" {
		t.Errorf("FindDB: %+v ok=%v", p, ok)
	}
}

func TestManager_DefaultPath(t *testing.T) {
	mgr := NewManager("")
	if mgr.Path() == "" {
		t.Error("default path should not be empty")
	}
}
