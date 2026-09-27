package sensitiveform

import (
	"errors"
	"strings"

	appcrypto "github.com/dujiao-next/internal/crypto"
)

const Prefix = "enc:v1:"

type Codec struct {
	key []byte
}

func New(secret string) *Codec {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil
	}
	return &Codec{
		key: appcrypto.DeriveKey(secret + "\x00manual-form-sensitive-v1"),
	}
}

func (c *Codec) Seal(raw string) (string, error) {
	if c == nil || len(c.key) == 0 {
		return "", errors.New("sensitive form encryption key unavailable")
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("sensitive form value empty")
	}
	encrypted, err := appcrypto.Encrypt(c.key, raw)
	if err != nil {
		return "", err
	}
	return Prefix + encrypted, nil
}

func (c *Codec) Open(stored string) (string, error) {
	if c == nil || len(c.key) == 0 {
		return "", errors.New("sensitive form encryption key unavailable")
	}
	stored = strings.TrimSpace(stored)
	if !strings.HasPrefix(stored, Prefix) {
		return "", errors.New("sensitive form ciphertext invalid")
	}
	return appcrypto.Decrypt(c.key, strings.TrimPrefix(stored, Prefix))
}
