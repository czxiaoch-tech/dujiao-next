package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// DeriveKey 从任意长度的密钥字符串派生 32 字节 AES-256 密钥
func DeriveKey(secret string) []byte {
	h := sha256.Sum256([]byte(secret))
	return h[:]
}

// Encrypt AES-256-GCM 加密，返回 hex 编码的密文
func Encrypt(key []byte, plaintext string) (string, error) {
	return EncryptWithAAD(key, plaintext, nil)
}

// EncryptWithAAD 使用 AES-256-GCM 加密，并把 aad 作为不可伪造的上下文绑定到密文。
func EncryptWithAAD(key []byte, plaintext string, aad []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), aad)
	return hex.EncodeToString(ciphertext), nil
}

// Decrypt AES-256-GCM 解密，输入 hex 编码的密文。
func Decrypt(key []byte, ciphertextHex string) (string, error) {
	return DecryptWithAAD(key, ciphertextHex, nil)
}

// DecryptWithAAD 使用与加密时相同的 aad 解密；上下文不一致时认证会失败。
func DecryptWithAAD(key []byte, ciphertextHex string, aad []byte) (string, error) {
	ciphertext, err := hex.DecodeString(ciphertextHex)
	if err != nil {
		return "", fmt.Errorf("decode hex: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}
