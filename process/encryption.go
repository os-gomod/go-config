package process

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

// Encryptor encrypts/decrypts strings.
type Encryptor interface {
	Encrypt(string) (string, error)
	Decrypt(string) (string, error)
}

// AESEncryptor implements AES-GCM.
type AESEncryptor struct {
	gcm cipher.AEAD
}

func NewAESEncryptor(key string) (*AESEncryptor, error) {
	block, err := createCipherBlock(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &AESEncryptor{gcm: gcm}, nil
}

func createCipherBlock(key string) (cipher.Block, error) {
	h := sha256.Sum256([]byte(key))
	return aes.NewCipher(h[:])
}

func (a *AESEncryptor) Encrypt(v string) (string, error) {
	nonce, err := generateNonce(a.gcm.NonceSize())
	if err != nil {
		return "", err
	}

	ciphertext := a.gcm.Seal(nonce, nonce, []byte(v), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func generateNonce(size int) ([]byte, error) {
	nonce := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return nonce, nil
}

func (a *AESEncryptor) Decrypt(v string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return "", fmt.Errorf("decrypt: invalid base64: %w", err)
	}

	nonce, ciphertext, err := splitNonceAndCiphertext(b, a.gcm.NonceSize())
	if err != nil {
		return "", err
	}

	plain, err := a.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt failed: %w", err)
	}

	return string(plain), nil
}

func splitNonceAndCiphertext(data []byte, nonceSize int) (nonce, ciphertext []byte, err error) {
	if len(data) < nonceSize {
		return nil, nil, fmt.Errorf(
			"decrypt: ciphertext too short (got %d, need >= %d)",
			len(data), nonceSize,
		)
	}
	return data[:nonceSize], data[nonceSize:], nil
}

// EncryptionProcessor decrypts values with prefix.
type EncryptionProcessor struct {
	enc    Encryptor
	prefix string
}

func NewEncryptionProcessor(enc Encryptor, prefix string) *EncryptionProcessor {
	return &EncryptionProcessor{enc: enc, prefix: prefix}
}

func (p *EncryptionProcessor) Process(data map[string]any) (map[string]any, error) {
	out := make(map[string]any)
	for k, v := range data {
		pv, err := p.walk(v)
		if err != nil {
			return nil, fmt.Errorf("key %s: %w", k, err)
		}
		out[k] = pv
	}
	return out, nil
}

func (p *EncryptionProcessor) walk(v any) (any, error) {
	switch x := v.(type) {
	case string:
		return p.processString(x)
	case map[string]any:
		return p.processMap(x)
	case []any:
		return p.processSlice(x)
	default:
		return v, nil
	}
}

func (p *EncryptionProcessor) processString(s string) (string, error) {
	if strings.HasPrefix(s, p.prefix) {
		return p.enc.Decrypt(strings.TrimPrefix(s, p.prefix))
	}
	return s, nil
}

func (p *EncryptionProcessor) processMap(m map[string]any) (map[string]any, error) {
	result := make(map[string]any)
	for k, v := range m {
		pv, err := p.walk(v)
		if err != nil {
			return nil, err
		}
		result[k] = pv
	}
	return result, nil
}

func (p *EncryptionProcessor) processSlice(arr []any) ([]any, error) {
	result := make([]any, len(arr))
	for i, v := range arr {
		pv, err := p.walk(v)
		if err != nil {
			return nil, err
		}
		result[i] = pv
	}
	return result, nil
}
