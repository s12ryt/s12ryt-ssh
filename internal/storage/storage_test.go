package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	"s12ryt-ssh/internal/config"
)

func TestMain(m *testing.M) {
	code := m.Run()
	if s3Srv != nil {
		s3Srv.Close()
	}
	os.Exit(code)
}

// retryClient wraps http.Client to retry transient network errors (notably
// the intermittent "connection refused" seen on WSL2 loopback).
func retryClient() *http.Client {
	return &http.Client{
		Transport: &retryTransport{base: http.DefaultTransport},
	}
}

type retryTransport struct{ base http.RoundTripper }

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error
	for attempt := 0; attempt < 10; attempt++ {
		// Clone the body so retries can resend it.
		var body io.ReadCloser
		if req.Body != nil {
			buf, _ := io.ReadAll(req.Body)
			req.Body.Close()
			body = io.NopCloser(bytes.NewReader(buf))
		}
		req.Body = body
		resp, err = t.base.RoundTrip(req)
		if err == nil {
			return resp, nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return resp, err
}

func TestMemoryStorage_CRUD(t *testing.T) {
	s := NewMemoryStorage()
	ctx := context.Background()

	// Put then Get
	data := []byte("hello world")
	if err := s.Put(ctx, "folder/file.txt", data); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := s.Get(ctx, "folder/file.txt")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("Get mismatch: %q", got)
	}

	// List
	objs, err := s.List(ctx, "folder/")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(objs) != 1 || objs[0].Key != "folder/file.txt" {
		t.Errorf("List mismatch: %+v", objs)
	}
	if objs[0].Size != int64(len(data)) {
		t.Errorf("Size mismatch: %d", objs[0].Size)
	}

	// Delete
	if err := s.Delete(ctx, "folder/file.txt"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(ctx, "folder/file.txt"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestMemoryStorage_GetMissing(t *testing.T) {
	s := NewMemoryStorage()
	if _, err := s.Get(context.Background(), "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestS3Config_BuildsResolver(t *testing.T) {
	p := config.S3Profile{
		Name:         "r2",
		Endpoint:     "https://abc.r2.cloudflarestorage.com",
		Region:       "auto",
		AccessKey:    "ak",
		SecretKey:    "sk",
		Bucket:       "data",
		UsePathStyle: true,
	}
	cfg, err := buildS3Config(p)
	if err != nil {
		t.Fatalf("buildS3Config: %v", err)
	}
	if cfg.Region != "auto" {
		t.Errorf("region: %q", cfg.Region)
	}
	// credentials should resolve to the static keys
	creds, err := cfg.Credentials.Retrieve(context.Background())
	if err != nil {
		t.Fatalf("retrieve creds: %v", err)
	}
	if creds.AccessKeyID != "ak" || creds.SecretAccessKey != "sk" {
		t.Errorf("creds mismatch: %+v", creds)
	}
}

// sharedS3Server lazily starts a single in-process S3 fake for all tests.
// Creating one listener (rather than one per test) sidesteps WSL2 quirks where
// later loopback listeners intermittently refuse connections.
var (
	s3SrvOnce sync.Once
	s3Srv     *httptest.Server
	s3SrvFake *s3Fake
)

func sharedS3Server() (*httptest.Server, *s3Fake) {
	s3SrvOnce.Do(func() {
		s3SrvFake = newS3Fake("data")
		s3Srv = httptest.NewServer(http.HandlerFunc(s3SrvFake.handle))
	})
	return s3Srv, s3SrvFake
}

// newTestS3 builds an S3Storage against the shared fake server. Each test
// isolates its objects under a unique key prefix derived from its name.
func newTestS3(t *testing.T) (*S3Storage, string) {
	t.Helper()
	srv, _ := sharedS3Server()
	prefix := t.Name() + "/"
	s, err := NewS3Storage(config.S3Profile{
		Endpoint: srv.URL, Region: "us-east-1",
		AccessKey: "ak", SecretKey: "sk", Bucket: "data", UsePathStyle: true,
	}, WithHTTPClient(retryClient()))
	if err != nil {
		t.Fatalf("NewS3Storage: %v", err)
	}
	return s, prefix
}

func TestS3Storage_PutGetDelete(t *testing.T) {
	s, prefix := newTestS3(t)
	ctx := context.Background()
	key := prefix + "a/b.txt"

	if err := s.Put(ctx, key, []byte("payload")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := s.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "payload" {
		t.Errorf("Get mismatch: %q", got)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestS3Storage_List(t *testing.T) {
	s, prefix := newTestS3(t)
	ctx := context.Background()

	_ = s.Put(ctx, prefix+"logs/1.log", []byte("aaa"))
	_ = s.Put(ctx, prefix+"logs/2.log", []byte("bbbb"))
	_ = s.Put(ctx, prefix+"other.txt", []byte("c"))

	objs, err := s.List(ctx, prefix+"logs/")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(objs) != 2 {
		t.Fatalf("expected 2 objects, got %d: %+v", len(objs), objs)
	}
	for _, o := range objs {
		if o.Size == 0 {
			t.Errorf("zero size for %q", o.Key)
		}
		if o.Modified.IsZero() || o.Modified.After(time.Now().Add(time.Second)) {
			t.Errorf("bad modified time for %q: %v", o.Key, o.Modified)
		}
	}
}

func TestS3Storage_ListPaginates(t *testing.T) {
	s, prefix := newTestS3(t)
	ctx := context.Background()
	want := []string{
		prefix + "object-1",
		prefix + "object-2",
		prefix + "object-3",
		prefix + "object-4",
		prefix + "object-5",
	}
	for _, key := range want {
		if err := s.Put(ctx, key, []byte(key)); err != nil {
			t.Fatalf("Put %q: %v", key, err)
		}
	}

	objects, err := s.List(ctx, prefix)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	got := make([]string, 0, len(objects))
	for _, object := range objects {
		got = append(got, object.Key)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("expected all paginated objects, got %d: %v", len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("object %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestS3Storage_GetMissing(t *testing.T) {
	s, prefix := newTestS3(t)
	if _, err := s.Get(context.Background(), prefix+"missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// s3Fake is a tiny in-memory S3 backend for tests.
type s3Fake struct {
	bucket string
	store  map[string][]byte
}

func newS3Fake(bucket string) *s3Fake { return &s3Fake{bucket: bucket, store: map[string][]byte{}} }

func (f *s3Fake) handle(w http.ResponseWriter, r *http.Request) {
	// Path is "/<bucket>" for list, "/<bucket>/<key...>" for objects.
	base := "/" + f.bucket
	if r.URL.Path == base || r.URL.Path == base+"/" {
		f.handleList(w, r)
		return
	}
	key := r.URL.Path[len(base)+1:] // strip "/<bucket>/"
	switch r.Method {
	case http.MethodPut:
		body, _ := io.ReadAll(r.Body)
		f.store[key] = body
		w.WriteHeader(http.StatusOK)
	case http.MethodGet:
		data, ok := f.store[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write(data)
	case http.MethodDelete:
		delete(f.store, key)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (f *s3Fake) handleList(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	var keys []string
	for k := range f.store {
		if prefix == "" || len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	start := 0
	if token := r.URL.Query().Get("continuation-token"); token != "" {
		for i, key := range keys {
			if key > token {
				start = i
				break
			}
			start = len(keys)
		}
	}
	page := keys[start:]
	const pageSize = 2
	truncated := len(page) > pageSize
	if truncated {
		page = page[:pageSize]
	}
	w.Header().Set("Content-Type", "application/xml")
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>`))
	w.Write([]byte(`<ListBucketResult>`))
	if truncated {
		w.Write([]byte(`<IsTruncated>true</IsTruncated>`))
		w.Write([]byte(`<NextContinuationToken>` + page[len(page)-1] + `</NextContinuationToken>`))
	}
	for _, k := range page {
		w.Write([]byte(`<Contents><Key>` + k + `</Key><Size>` + itoa(len(f.store[k])) + `</Size><LastModified>2026-01-01T00:00:00.000Z</LastModified></Contents>`))
	}
	w.Write([]byte(`</ListBucketResult>`))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
