// Package secure provides token sealing, random identifiers, and hashing helpers.
package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
)

// KeyVersion identifies the active sealing key. Increment it when TOKEN_ENCRYPTION_KEY rotates.
const KeyVersion int16 = 1

// Sealer encrypts secrets at rest with AES-256-GCM. Output is nonce || ciphertext || tag.
type Sealer struct{ aead cipher.AEAD }

// NewSealer builds a Sealer from a 32-byte key.
func NewSealer(key []byte) (*Sealer, error) {
	if len(key) != 32 {
		return nil, errors.New("sealing key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Sealer{aead: aead}, nil
}

// Seal encrypts plaintext. additional binds the ciphertext to a context such as a user id.
func (s *Sealer) Seal(plaintext, additional []byte) []byte {
	nonce := make([]byte, s.aead.NonceSize(), s.aead.NonceSize()+len(plaintext)+s.aead.Overhead())
	_, _ = rand.Read(nonce) // crypto/rand.Read never returns an error on supported platforms.
	return s.aead.Seal(nonce, nonce, plaintext, additional)
}

// Open decrypts a value produced by Seal with the same additional data.
func (s *Sealer) Open(sealed, additional []byte) ([]byte, error) {
	n := s.aead.NonceSize()
	if len(sealed) < n+s.aead.Overhead() {
		return nil, errors.New("sealed value is too short")
	}
	return s.aead.Open(nil, sealed[:n], sealed[n:], additional)
}

// RandomToken returns a URL-safe random string with 256 bits of entropy.
func RandomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// Hash returns the SHA-256 digest of a token. Only hashes of session ids are stored.
func Hash(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
