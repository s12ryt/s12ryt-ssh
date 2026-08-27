package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"s12ryt-ssh/internal/securestore"
)

type proxyFixture struct {
	server                 *httptest.Server
	mu                     sync.Mutex
	upload                 []byte
	paths                  []string
	query                  sqlRequest
	exec                   sqlRequest
	queryParametersPresent bool
}

type sqlRequest struct {
	Statement  string `json:"statement"`
	Parameters []any  `json:"parameters"`
}

func newProxyFixture(t *testing.T) *proxyFixture {
	t.Helper()
	fixture := &proxyFixture{}
	fixture.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer access-proxy" {
			writeAPIError(w, http.StatusUnauthorized, "invalid_token", "invalid access token")
			return
		}
		fixture.mu.Lock()
		fixture.paths = append(fixture.paths, r.URL.EscapedPath())
		fixture.mu.Unlock()

		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/s3/objects"):
			if r.URL.Query().Get("prefix") != "folder/space name/" {
				http.Error(w, "unexpected prefix", http.StatusBadRequest)
				return
			}
			writeJSON(w, map[string]any{"objects": []S3Object{{Key: "folder/space name/file.txt", Size: 7, LastModified: "2026-08-27T00:00:00Z"}}})
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/s3/objects/"):
			if r.Header.Get("Content-Type") != "application/octet-stream" {
				http.Error(w, "unexpected content type", http.StatusBadRequest)
				return
			}
			payload, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			fixture.mu.Lock()
			fixture.upload = payload
			fixture.mu.Unlock()
			writeJSON(w, UploadResult{Bytes: int64(len(payload))})
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/s3/download/"):
			w.Header().Set("Content-Type", "text/plain")
			w.Header().Set("Content-Length", "10")
			_, _ = w.Write([]byte("downloaded"))
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/s3/objects/"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/sql/tables"):
			writeJSON(w, map[string]any{"tables": []string{"users", "jobs"}})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/sql/query"):
			data, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(data, &raw); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			_, fixture.queryParametersPresent = raw["parameters"]
			if err := json.Unmarshal(data, &fixture.query); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, SQLQueryResult{Columns: []string{"id"}, Rows: [][]any{{float64(1)}}, Truncated: false})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/sql/exec"):
			if err := json.NewDecoder(r.Body).Decode(&fixture.exec); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, SQLExecResult{RowsAffected: 2, LastInsertID: "7"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(fixture.server.Close)
	return fixture
}

func TestSessionOmitsNilSQLParametersForFastifySchema(t *testing.T) {
	fixture := newProxyFixture(t)
	session := newProxySession(t, fixture)
	if _, err := session.Query(context.Background(), "database-1", "SELECT 1", nil); err != nil {
		t.Fatal(err)
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.queryParametersPresent {
		t.Fatal("nil SQL parameters must be omitted instead of encoded as null")
	}
}

func newProxySession(t *testing.T, fixture *proxyFixture) *Session {
	t.Helper()
	client, err := NewClient(fixture.server.URL+"/gateway", fixture.server.Client())
	if err != nil {
		t.Fatal(err)
	}
	session, err := newSession(client, securestore.NewMemoryStore(), "alice", "desktop-a", TokenPair{
		AccessToken: "access-proxy", AccessExpiresAt: time.Now().Add(time.Hour).UnixMilli(),
		RefreshToken: "refresh-proxy", RefreshExpiresAt: time.Now().Add(24 * time.Hour).UnixMilli(),
		Account: Account{ID: "account-1", Username: "alice"}, SessionID: "session-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func TestSessionProxiesS3OperationsWithoutCredentials(t *testing.T) {
	fixture := newProxyFixture(t)
	session := newProxySession(t, fixture)

	objects, err := session.ListObjects(context.Background(), "storage id", "folder/space name/")
	if err != nil {
		t.Fatal(err)
	}
	if len(objects) != 1 || objects[0].Key != "folder/space name/file.txt" || objects[0].Size != 7 {
		t.Fatalf("objects = %+v", objects)
	}

	payload := bytes.NewReader([]byte("payload"))
	uploaded, err := session.UploadObject(context.Background(), "storage id", "folder/a b.txt", payload, payload.Size())
	if err != nil {
		t.Fatal(err)
	}
	if uploaded.Bytes != 7 {
		t.Fatalf("upload = %+v", uploaded)
	}

	download, err := session.DownloadObject(context.Background(), "storage id", "folder/a b.txt")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(download.Body)
	closeErr := download.Body.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("download read=%v close=%v", err, closeErr)
	}
	if string(body) != "downloaded" || download.ContentType != "text/plain" || download.ContentLength != 10 {
		t.Fatalf("download = type %q length %d body %q", download.ContentType, download.ContentLength, body)
	}

	if err := session.DeleteObject(context.Background(), "storage id", "folder/a b.txt"); err != nil {
		t.Fatal(err)
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if string(fixture.upload) != "payload" {
		t.Fatalf("upload body = %q", fixture.upload)
	}
	wantEscaped := "/gateway/api/v1/resources/storage%20id/s3/objects/folder/a%20b.txt"
	if !containsString(fixture.paths, wantEscaped) {
		t.Fatalf("escaped paths = %v, missing %q", fixture.paths, wantEscaped)
	}
}

func TestSessionProxiesSQLTablesQueryAndExec(t *testing.T) {
	fixture := newProxyFixture(t)
	session := newProxySession(t, fixture)

	tables, err := session.Tables(context.Background(), "database-1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(tables, ",") != "users,jobs" {
		t.Fatalf("tables = %v", tables)
	}
	query, err := session.Query(context.Background(), "database-1", "SELECT id FROM users WHERE id = ?", []any{1})
	if err != nil {
		t.Fatal(err)
	}
	if len(query.Rows) != 1 || query.Columns[0] != "id" || query.Truncated {
		t.Fatalf("query = %+v", query)
	}
	exec, err := session.Exec(context.Background(), "database-1", "INSERT INTO users(name) VALUES (?)", []any{"alice"})
	if err != nil {
		t.Fatal(err)
	}
	if exec.RowsAffected != 2 || exec.LastInsertID != "7" {
		t.Fatalf("exec = %+v", exec)
	}
	if fixture.query.Statement != "SELECT id FROM users WHERE id = ?" || len(fixture.query.Parameters) != 1 {
		t.Fatalf("query request = %+v", fixture.query)
	}
	if fixture.exec.Statement != "INSERT INTO users(name) VALUES (?)" || fixture.exec.Parameters[0] != "alice" {
		t.Fatalf("exec request = %+v", fixture.exec)
	}
}

func TestResourceAllowsOnlyAssignedOperations(t *testing.T) {
	resource := Resource{Operations: []Operation{OperationS3Read, OperationS3Write}}
	if !resource.Allows(OperationS3Read) || resource.Allows(OperationS3Delete) {
		t.Fatalf("operation check failed: %+v", resource.Operations)
	}
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		decoded, err := url.PathUnescape(value)
		if value == wanted || (err == nil && decoded == strings.ReplaceAll(wanted, "%20", " ")) {
			return true
		}
	}
	return false
}
