//go:build windows

package securestore

import (
	"bytes"
	"testing"
)

func TestDPAPIStoreRoundTrip(t *testing.T) {
	s := NewDPAPIStore()
	secret := []byte("bootstrap-secret")
	if err := s.Save("s12ryt-test", secret); err != nil {
		t.Fatalf("Save: %v", err)
	}
	t.Cleanup(func() { _ = s.Delete("s12ryt-test") })
	secret[0] = 'X'

	got, err := s.Load("s12ryt-test")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !bytes.Equal(got, []byte("bootstrap-secret")) {
		t.Fatalf("secret was not protected/copied: %q", got)
	}
}
