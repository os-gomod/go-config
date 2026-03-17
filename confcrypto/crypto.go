// Package confcrypto provides AES-GCM encryption for sensitive configuration values.
// Implements zero plaintext persistence and secure key handling.
package confcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"sync"

	"github.com/os-gomod/go-config/types"
)

// Encryptor provides encryption and decryption for configuration values.
type Encryptor interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
}

// AESGCMEncryptor implements AES-GCM encryption.
type AESGCMEncryptor struct {
	gcm cipher.AEAD
	mu  sync.Mutex // Protect against concurrent nonce generation
}

// NewAESGCMEncryptor creates a new AES-GCM encryptor.
// The key must be 16, 24, or 32 bytes for AES-128, AES-192, or AES-256.
func NewAESGCMEncryptor(key []byte) (*AESGCMEncryptor, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, types.NewError(types.ErrCryptoError, "failed to create cipher",
			types.WithCause(err))
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, types.NewError(types.ErrCryptoError, "failed to create GCM",
			types.WithCause(err))
	}

	return &AESGCMEncryptor{gcm: gcm}, nil
}

// Encrypt encrypts plaintext using AES-GCM.
func (e *AESGCMEncryptor) Encrypt(plaintext []byte) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Generate random nonce
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, types.NewError(types.ErrCryptoError, "failed to generate nonce",
			types.WithCause(err))
	}

	// Encrypt and append nonce
	ciphertext := e.gcm.Seal(nonce, nonce, plaintext, nil)

	return ciphertext, nil
}

// Decrypt decrypts ciphertext using AES-GCM.
func (e *AESGCMEncryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	nonceSize := e.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, types.NewError(types.ErrCryptoError, "ciphertext too short")
	}

	// Extract nonce and decrypt
	nonce := ciphertext[:nonceSize]
	ciphertext = ciphertext[nonceSize:]

	plaintext, err := e.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, types.NewError(types.ErrCryptoError, "decryption failed",
			types.WithCause(err))
	}

	return plaintext, nil
}

// KeyDerivation derives an encryption key from a password.
type KeyDerivation struct {
	salt []byte
}

// NewKeyDerivation creates a new key derivation instance.
func NewKeyDerivation(salt []byte) *KeyDerivation {
	return &KeyDerivation{salt: salt}
}

// Derive derives a key from a password using SHA-256.
// For production, consider using scrypt or argon2.
func (k *KeyDerivation) Derive(password string, keyLen int) []byte {
	h := sha256.New()
	h.Write([]byte(password))
	h.Write(k.salt)

	hash := h.Sum(nil)

	// Extend or truncate to desired length
	result := make([]byte, keyLen)
	copy(result, hash)

	// If we need more bytes, hash again
	for i := sha256.Size; i < keyLen; i++ {
		h = sha256.New()
		h.Write(hash)
		hash = h.Sum(nil)
		result[i] = hash[0]
	}

	return result
}

// SecureBytes holds sensitive data with secure wiping.
type SecureBytes struct {
	data []byte
	mu   sync.RWMutex
}

// NewSecureBytes creates a new secure bytes holder.
func NewSecureBytes(data []byte) *SecureBytes {
	// Copy data to prevent external references
	d := make([]byte, len(data))
	copy(d, data)

	return &SecureBytes{data: d}
}

// Get returns a copy of the data.
func (s *SecureBytes) Get() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]byte, len(s.data))
	copy(result, s.data)

	return result
}

// Wipe securely erases the data.
func (s *SecureBytes) Wipe() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Overwrite with zeros
	for i := range s.data {
		s.data[i] = 0
	}
	s.data = nil
}

// KeyProvider provides encryption keys.
type KeyProvider interface {
	GetKey() ([]byte, error)
}

// StaticKeyProvider provides a static key.
type StaticKeyProvider struct {
	key *SecureBytes
}

// NewStaticKeyProvider creates a new static key provider.
func NewStaticKeyProvider(key []byte) *StaticKeyProvider {
	return &StaticKeyProvider{
		key: NewSecureBytes(key),
	}
}

// GetKey returns the encryption key.
func (p *StaticKeyProvider) GetKey() ([]byte, error) {
	return p.key.Get(), nil
}

// Wipe securely erases the key.
func (p *StaticKeyProvider) Wipe() {
	p.key.Wipe()
}

// EncryptedValue represents an encrypted configuration value.
type EncryptedValue struct {
	Ciphertext string `json:"ciphertext"`
	KeyID      string `json:"key_id,omitempty"`
}

// Encoder handles encoding/decoding of encrypted values.
type Encoder struct {
	encryptor   Encryptor
	keyProvider KeyProvider
}

// NewEncoder creates a new encoder.
func NewEncoder(encryptor Encryptor, provider KeyProvider) *Encoder {
	return &Encoder{
		encryptor:   encryptor,
		keyProvider: provider,
	}
}

// EncryptString encrypts a string value.
func (e *Encoder) EncryptString(plaintext string) (*EncryptedValue, error) {
	ciphertext, err := e.encryptor.Encrypt([]byte(plaintext))
	if err != nil {
		return nil, err
	}

	return &EncryptedValue{
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}, nil
}

// DecryptString decrypts an encrypted string.
func (e *Encoder) DecryptString(ev *EncryptedValue) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(ev.Ciphertext)
	if err != nil {
		return "", types.NewError(types.ErrCryptoError, "failed to decode ciphertext",
			types.WithCause(err))
	}

	plaintext, err := e.encryptor.Decrypt(ciphertext)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// CryptoManager manages encryption for configuration values.
type CryptoManager struct {
	encryptors map[string]Encryptor
	defaultID  string
	mu         sync.RWMutex
	pool       sync.Pool
}

// NewCryptoManager creates a new crypto manager.
func NewCryptoManager(defaultKey []byte) (*CryptoManager, error) {
	if len(defaultKey) == 0 {
		return nil, types.NewError(types.ErrCryptoError, "invalid encryption key")
	}

	normalizedKey := normalizeAESKey(defaultKey)
	enc, err := NewAESGCMEncryptor(normalizedKey)
	if err != nil {
		return nil, err
	}

	m := &CryptoManager{
		encryptors: make(map[string]Encryptor),
		defaultID:  "default",
		pool: sync.Pool{
			New: func() any {
				return make([]byte, 0, 1024)
			},
		},
	}

	m.encryptors["default"] = enc

	return m, nil
}

func normalizeAESKey(key []byte) []byte {
	switch len(key) {
	case 16, 24, 32:
		out := make([]byte, len(key))
		copy(out, key)

		return out
	default:
		sum := sha256.Sum256(key)
		out := make([]byte, len(sum))
		copy(out, sum[:])

		return out
	}
}

// Register adds an encryptor with the given ID.
func (m *CryptoManager) Register(id string, enc Encryptor) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.encryptors[id] = enc
}

// Get retrieves an encryptor by ID.
func (m *CryptoManager) Get(id string) (Encryptor, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	enc, ok := m.encryptors[id]

	return enc, ok
}

// Default returns the default encryptor.
func (m *CryptoManager) Default() Encryptor {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.encryptors[m.defaultID]
}

// Encrypt encrypts a value using the default encryptor.
func (m *CryptoManager) Encrypt(plaintext []byte) ([]byte, error) {
	enc := m.Default()
	if enc == nil {
		return nil, types.NewError(types.ErrCryptoError, "no encryptor available")
	}

	return enc.Encrypt(plaintext)
}

// Decrypt decrypts a value using the default encryptor.
func (m *CryptoManager) Decrypt(ciphertext []byte) ([]byte, error) {
	enc := m.Default()
	if enc == nil {
		return nil, types.NewError(types.ErrCryptoError, "no encryptor available")
	}

	return enc.Decrypt(ciphertext)
}

// EncryptValue encrypts a configuration value.
func (m *CryptoManager) EncryptValue(value types.Value) (*EncryptedValue, error) {
	plaintext := value.String()

	return m.EncryptString(plaintext)
}

// DecryptValue decrypts an encrypted value.
func (m *CryptoManager) DecryptValue(ev *EncryptedValue) (types.Value, error) {
	plaintext, err := m.DecryptString(ev)
	if err != nil {
		return types.Value{}, err
	}

	return types.NewValue(plaintext, types.TypeString, types.SourceMemory, 0), nil
}

// EncryptString encrypts a string.
func (m *CryptoManager) EncryptString(plaintext string) (*EncryptedValue, error) {
	ciphertext, err := m.Encrypt([]byte(plaintext))
	if err != nil {
		return nil, err
	}

	return &EncryptedValue{
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}, nil
}

// DecryptString decrypts a string.
func (m *CryptoManager) DecryptString(ev *EncryptedValue) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(ev.Ciphertext)
	if err != nil {
		return "", types.NewError(types.ErrCryptoError, "failed to decode",
			types.WithCause(err))
	}

	plaintext, err := m.Decrypt(ciphertext)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// Errors.
var (
	ErrInvalidKey       = errors.New("invalid encryption key")
	ErrDecryptionFailed = errors.New("decryption failed")
	ErrKeyNotFound      = errors.New("key not found")
)

// GenerateKey generates a random encryption key.
func GenerateKey(size int) ([]byte, error) {
	key := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, types.NewError(types.ErrCryptoError, "failed to generate key",
			types.WithCause(err))
	}

	return key, nil
}
