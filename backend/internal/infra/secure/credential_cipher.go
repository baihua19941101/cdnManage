package secure

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

const cipherPrefix = "v1:"

type CredentialCipher struct {
	key []byte
}

func NewCredentialCipher(key string) *CredentialCipher {
	sum := sha256.Sum256([]byte(key))
	return &CredentialCipher{key: sum[:]}
}

func (c *CredentialCipher) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("build aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("build gcm cipher: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	encrypted := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	payload := append(nonce, encrypted...)

	return cipherPrefix + base64.StdEncoding.EncodeToString(payload), nil
}

func (c *CredentialCipher) Decrypt(ciphertext string) (string, error) {
	if !strings.HasPrefix(ciphertext, cipherPrefix) {
		return "", fmt.Errorf("unsupported ciphertext format")
	}

	encoded := strings.TrimPrefix(ciphertext, cipherPrefix)
	payload, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("build aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("build gcm cipher: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(payload) < nonceSize {
		return "", fmt.Errorf("invalid ciphertext payload")
	}

	nonce := payload[:nonceSize]
	encrypted := payload[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt payload: %w", err)
	}

	return string(plaintext), nil
}
