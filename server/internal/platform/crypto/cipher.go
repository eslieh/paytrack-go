// Package crypto provides envelope-style encryption for secrets at rest
// (Daraja credentials, webhook signing secrets).
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

const version byte = 1

type Cipher struct {
	aead cipher.AEAD
}

// NewCipher builds an AES-256-GCM cipher from a 32-byte key.
func NewCipher(key []byte) (*Cipher, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

// Encrypt returns version || nonce || ciphertext.
func (c *Cipher) Encrypt(plain []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := append([]byte{version}, nonce...)
	return c.aead.Seal(out, nonce, plain, nil), nil
}

func (c *Cipher) Decrypt(b []byte) ([]byte, error) {
	ns := c.aead.NonceSize()
	if len(b) < 1+ns || b[0] != version {
		return nil, errors.New("crypto: malformed ciphertext")
	}
	return c.aead.Open(nil, b[1:1+ns], b[1+ns:], nil)
}
