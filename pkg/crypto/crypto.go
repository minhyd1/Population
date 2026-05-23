// Package crypto cung cấp mã hóa/giải mã AES-256-GCM cho các trường nhạy cảm.
// Frontend sử dụng cùng key để giải mã các trường được đánh dấu encrypted_fields.
//
// Flow:
//   DB (encrypted) → Service (decrypt for internal use) → Handler (re-encrypt for response) → Client
//
// Frontend decrypt (JavaScript example):
//
//	async function decryptField(base64Ciphertext, base64Key) {
//	  const keyBytes = Uint8Array.from(atob(base64Key), c => c.charCodeAt(0));
//	  const cipherBytes = Uint8Array.from(atob(base64Ciphertext), c => c.charCodeAt(0));
//	  const iv = cipherBytes.slice(0, 12);
//	  const data = cipherBytes.slice(12);
//	  const cryptoKey = await crypto.subtle.importKey("raw", keyBytes, "AES-GCM", false, ["decrypt"]);
//	  const decrypted = await crypto.subtle.decrypt({ name: "AES-GCM", iv }, cryptoKey, data);
//	  return new TextDecoder().decode(decrypted);
//	}
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// Encryptor handles AES-256-GCM encryption/decryption
type Encryptor struct {
	key []byte // 32-byte AES-256 key
}

// New creates a new Encryptor from a base64-encoded key string
func New(base64Key string) (*Encryptor, error) {
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		// Try raw key if not base64
		key = []byte(base64Key)
	}

	if len(key) != 32 {
		return nil, errors.New("encryption key must be exactly 32 bytes (AES-256)")
	}

	return &Encryptor{key: key}, nil
}

// NewFromBytes creates a new Encryptor from raw key bytes
func NewFromBytes(key []byte) (*Encryptor, error) {
	if len(key) != 32 {
		return nil, errors.New("encryption key must be exactly 32 bytes (AES-256)")
	}
	return &Encryptor{key: key}, nil
}

// Encrypt mã hóa plaintext bằng AES-256-GCM.
// Output format: base64(iv[12] + ciphertext + tag[16])
// IV được generate ngẫu nhiên mỗi lần → cùng plaintext cho ciphertext khác nhau.
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Random 12-byte IV (nonce)
	iv := make([]byte, gcm.NonceSize()) // 12 bytes
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	// Seal: iv + encrypt(plaintext) + auth_tag
	ciphertext := gcm.Seal(iv, iv, []byte(plaintext), nil)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt giải mã base64 ciphertext về plaintext.
// Chỉ dùng internally (service layer) khi cần xử lý dữ liệu gốc.
func (e *Encryptor) Decrypt(base64Ciphertext string) (string, error) {
	if base64Ciphertext == "" {
		return "", nil
	}

	data, err := base64.StdEncoding.DecodeString(base64Ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	iv, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// MustEncrypt encrypts and panics on error (use only in tests/init)
func (e *Encryptor) MustEncrypt(plaintext string) string {
	result, err := e.Encrypt(plaintext)
	if err != nil {
		panic(err)
	}
	return result
}

// GenerateKey tạo ngẫu nhiên 32-byte key và trả về base64
func GenerateKey() (string, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// SensitiveFields danh sách các trường nhạy cảm được mã hóa
var SensitiveFields = []string{
	"national_id",
	"phone_number",
	"email",
	"permanent_address",
}
