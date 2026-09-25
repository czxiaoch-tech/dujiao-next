package gormstore

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	appcrypto "github.com/dujiao-next/internal/crypto"
)

const productCodeCipherPrefix = "enc:v1:"

type codeCodec struct {
	encryptionKey []byte
	lookupKey     []byte
}

func newCodeCodec(secret string) *codeCodec {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil
	}
	return &codeCodec{
		encryptionKey: appcrypto.DeriveKey(secret + "\x00gift-card-product-code-encryption-v1"),
		lookupKey:     appcrypto.DeriveKey(secret + "\x00gift-card-product-code-lookup-v1"),
	}
}

func normalizeCode(code string) string {
	return strings.TrimSpace(strings.ToUpper(code))
}

func (c *codeCodec) sealProductCode(code string) (string, *string, string, error) {
	if c == nil {
		return "", nil, "", errors.New("gift card product-code encryption key is unavailable")
	}
	normalized := normalizeCode(code)
	if normalized == "" {
		return "", nil, "", errors.New("gift card product code is empty")
	}
	ciphertext, err := appcrypto.Encrypt(c.encryptionKey, normalized)
	if err != nil {
		return "", nil, "", err
	}
	hash := c.lookupHash(normalized)
	return productCodeCipherPrefix + ciphertext, &hash, maskCode(normalized), nil
}

func (c *codeCodec) revealProductCode(stored string, expectedHash *string) (string, error) {
	if expectedHash == nil || strings.TrimSpace(*expectedHash) == "" {
		return normalizeCode(stored), nil
	}
	if c == nil {
		return "", errors.New("gift card product-code decryption key is unavailable")
	}
	if !strings.HasPrefix(stored, productCodeCipherPrefix) {
		return "", errors.New("gift card product-code ciphertext is invalid")
	}
	plaintext, err := appcrypto.Decrypt(c.encryptionKey, strings.TrimPrefix(stored, productCodeCipherPrefix))
	if err != nil {
		return "", err
	}
	normalized := normalizeCode(plaintext)
	actualHash := c.lookupHash(normalized)
	if !hmac.Equal([]byte(actualHash), []byte(strings.TrimSpace(*expectedHash))) {
		return "", errors.New("gift card product-code integrity check failed")
	}
	return normalized, nil
}

func (c *codeCodec) lookupHash(code string) string {
	if c == nil {
		return ""
	}
	mac := hmac.New(sha256.New, c.lookupKey)
	_, _ = mac.Write([]byte(normalizeCode(code)))
	return hex.EncodeToString(mac.Sum(nil))
}

func maskCode(code string) string {
	normalized := normalizeCode(code)
	runes := []rune(normalized)
	if len(runes) == 0 {
		return ""
	}
	if len(runes) <= 8 {
		return strings.Repeat("*", len(runes))
	}
	return string(runes[:4]) + "********" + string(runes[len(runes)-4:])
}
