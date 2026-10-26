package secure

import (
	"bytes"
	"testing"
)

func TestSealedTokenIsBoundToItsOwner(t *testing.T) {
	s, err := NewSealer(bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	sealed := s.Seal([]byte("ghu_token"), []byte("user-1"))
	if got, err := s.Open(sealed, []byte("user-1")); err != nil || string(got) != "ghu_token" {
		t.Fatalf("open = %q, %v", got, err)
	}
	// A sealed value copied to another user's row must not decrypt.
	if _, err := s.Open(sealed, []byte("user-2")); err == nil {
		t.Fatal("opened with the wrong owner")
	}
	sealed[len(sealed)-1] ^= 1
	if _, err := s.Open(sealed, []byte("user-1")); err == nil {
		t.Fatal("opened a tampered value")
	}
	other, _ := NewSealer(bytes.Repeat([]byte{2}, 32))
	if _, err := other.Open(s.Seal([]byte("x"), nil), nil); err == nil {
		t.Fatal("opened with another key")
	}
}
