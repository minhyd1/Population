package crypto_test

import (
	"testing"

	"population-service/pkg/crypto"
)

func TestEncryptDecrypt(t *testing.T) {
	// Generate a test key
	keyB64, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	enc, err := crypto.New(keyB64)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	testCases := []string{
		"079123456789",        // CCCD 12 số
		"0912345678",          // SĐT
		"test@example.com",
		"123 Đường ABC, Quận 1, TP.HCM",
		"",                    // empty string
	}

	for _, tc := range testCases {
		encrypted, err := enc.Encrypt(tc)
		if err != nil {
			t.Errorf("Encrypt(%q) failed: %v", tc, err)
			continue
		}

		decrypted, err := enc.Decrypt(encrypted)
		if err != nil {
			t.Errorf("Decrypt failed: %v", err)
			continue
		}

		if decrypted != tc {
			t.Errorf("Round-trip failed: got %q, want %q", decrypted, tc)
		}
	}
}

func TestEncryptNonDeterministic(t *testing.T) {
	keyB64, _ := crypto.GenerateKey()
	enc, _ := crypto.New(keyB64)

	plain := "079123456789"
	enc1, _ := enc.Encrypt(plain)
	enc2, _ := enc.Encrypt(plain)

	// Same plaintext → different ciphertexts (due to random IV)
	if enc1 == enc2 {
		t.Error("Expected different ciphertexts for same plaintext (IV should be random)")
	}
}
