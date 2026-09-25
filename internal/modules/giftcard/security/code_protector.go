package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	dujiaocrypto "github.com/dujiao-next/internal/crypto"
)

const ciphertextPrefix = "gcv1:"

var (
	errEmptyCode            = errors.New("gift card code is empty")
	errProtectorUnavailable = errors.New("gift card code protector is unavailable")
)

var codeAAD = []byte("dujiao-next/gift-card-code/v1")

// CodeProtector 为产品兑换码提供可逆加密与不可逆检索索引。
// 加密密钥与检索密钥从同一个应用主密钥做域分离派生，避免密钥复用。
type CodeProtector struct {
	encryptionKey []byte
	lookupKey     []byte
}

func NewCodeProtector(secret string) *CodeProtector {
	encryptionSeed := sha256.Sum256([]byte("dujiao/giftcard/code/encryption/v1\x00" + secret))
	lookupSeed := sha256.Sum256([]byte("dujiao/giftcard/code/lookup/v1\x00" + secret))
	return &CodeProtector{
		encryptionKey: encryptionSeed[:],
		lookupKey:     lookupSeed[:],
	}
}

func NormalizeCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func (p *CodeProtector) Encrypt(code string) (string, error) {
	if p == nil {
		return "", errProtectorUnavailable
	}
	normalized := NormalizeCode(code)
	if normalized == "" {
		return "", errEmptyCode
	}
	ciphertext, err := dujiaocrypto.EncryptWithAAD(p.encryptionKey, normalized, codeAAD)
	if err != nil {
		return "", err
	}
	return ciphertextPrefix + ciphertext, nil
}

// Decrypt 同时兼容 V0.1 上线前的历史明文，便于一次性原地迁移。
// 业务正常运行时，产品兑换码应全部以 gcv1: 前缀的密文存在数据库中。
func (p *CodeProtector) Decrypt(stored string) (string, error) {
	if p == nil {
		return "", errProtectorUnavailable
	}
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return "", errEmptyCode
	}
	if !p.IsEncrypted(stored) {
		return NormalizeCode(stored), nil
	}
	plaintext, err := dujiaocrypto.DecryptWithAAD(
		p.encryptionKey,
		strings.TrimPrefix(stored, ciphertextPrefix),
		codeAAD,
	)
	if err != nil {
		return "", err
	}
	return NormalizeCode(plaintext), nil
}

func (p *CodeProtector) Hash(code string) string {
	if p == nil {
		return ""
	}
	mac := hmac.New(sha256.New, p.lookupKey)
	_, _ = mac.Write([]byte(NormalizeCode(code)))
	return hex.EncodeToString(mac.Sum(nil))
}

func (p *CodeProtector) IsEncrypted(stored string) bool {
	return strings.HasPrefix(strings.TrimSpace(stored), ciphertextPrefix)
}

// MaskCode 只用于展示，不参与检索与持久化。
func MaskCode(code string) string {
	normalized := NormalizeCode(code)
	switch {
	case normalized == "":
		return ""
	case len(normalized) <= 4:
		return strings.Repeat("*", len(normalized))
	case len(normalized) <= 10:
		return normalized[:2] + strings.Repeat("*", len(normalized)-4) + normalized[len(normalized)-2:]
	default:
		return normalized[:6] + strings.Repeat("*", len(normalized)-10) + normalized[len(normalized)-4:]
	}
}
