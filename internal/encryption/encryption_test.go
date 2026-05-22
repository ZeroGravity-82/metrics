package encryption

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEncrypt_FailsWhenPublicKeyFileMissing проверяет ошибку при отсутствии файла публичного ключа.
func TestEncrypt_FailsWhenPublicKeyFileMissing(t *testing.T) {
	// Arrange
	data := []byte("metrics")
	missingPath := filepath.Join(t.TempDir(), "missing.pem")

	// Act
	encryptedData, encryptedKey, err := Encrypt(data, missingPath)

	// Assert
	require.Error(t, err)
	assert.Nil(t, encryptedData)
	assert.Nil(t, encryptedKey)
}

// TestDecrypt_FailsWhenEncryptedDataTooShort проверяет ошибку, если в зашифрованных данных нет nonce.
func TestDecrypt_FailsWhenEncryptedDataTooShort(t *testing.T) {
	// Arrange
	privateKeyPath, publicKeyPath := writeTestRSAKeyPair(t, "pkcs1")
	sessionKey := make([]byte, 32)
	_, err := rand.Read(sessionKey)
	require.NoError(t, err)

	publicKey, err := readPublicKey(publicKeyPath)
	require.NoError(t, err)

	encryptedKey, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, sessionKey, nil)
	require.NoError(t, err)

	tooShortEncryptedData := []byte("short")

	// Act
	decryptedData, err := Decrypt(tooShortEncryptedData, encryptedKey, privateKeyPath)

	// Assert
	require.Error(t, err)
	assert.Nil(t, decryptedData)
}

// TestEncryptDecrypt_RoundTrip проверяет, что полный цикл шифрования и дешифрования работает для PKCS#1 и PKCS#8.
func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	// Arrange
	tests := []struct {
		name   string
		format string
	}{
		{
			name:   "pkcs1 private key",
			format: "pkcs1",
		},
		{
			name:   "pkcs8 private key",
			format: "pkcs8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			privateKeyPath, publicKeyPath := writeTestRSAKeyPair(t, tt.format)
			data := []byte("important metrics payload")

			// Act
			encryptedData, encryptedKey, err := Encrypt(data, publicKeyPath)
			require.NoError(t, err)

			decryptedData, err := Decrypt(encryptedData, encryptedKey, privateKeyPath)

			// Assert
			require.NoError(t, err)
			assert.NotEmpty(t, encryptedData)
			assert.NotEmpty(t, encryptedKey)
			assert.Equal(t, data, decryptedData)
		})
	}
}

// Test_readPublicKey_FailsWithUnsupportedPEMType проверяет ошибку для неподдерживаемого PEM-типа публичного ключа.
func Test_readPublicKey_FailsWithUnsupportedPEMType(t *testing.T) {
	// Arrange
	keyPath := filepath.Join(t.TempDir(), "unsupported_public.pem")
	pemData := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("test")})
	err := os.WriteFile(keyPath, pemData, 0o600)
	require.NoError(t, err)

	// Act
	publicKey, err := readPublicKey(keyPath)

	// Assert
	require.Error(t, err)
	assert.Nil(t, publicKey)
}

// Test_readPrivateKey_FailsWithUnsupportedPEMType проверяет ошибку для неподдерживаемого PEM-типа приватного ключа.
func Test_readPrivateKey_FailsWithUnsupportedPEMType(t *testing.T) {
	// Arrange
	keyPath := filepath.Join(t.TempDir(), "unsupported_private.pem")
	pemData := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: []byte("test")})
	err := os.WriteFile(keyPath, pemData, 0o600)
	require.NoError(t, err)

	// Act
	privateKey, err := readPrivateKey(keyPath)

	// Assert
	require.Error(t, err)
	assert.Nil(t, privateKey)
}

// Test_readPrivateKey_FailsWithInvalidPEM проверяет ошибку, если приватный ключ не удается декодировать из PEM.
func Test_readPrivateKey_FailsWithInvalidPEM(t *testing.T) {
	// Arrange
	keyPath := filepath.Join(t.TempDir(), "invalid_private.pem")
	err := os.WriteFile(keyPath, []byte("not a pem"), 0o600)
	require.NoError(t, err)

	// Act
	privateKey, err := readPrivateKey(keyPath)

	// Assert
	require.Error(t, err)
	assert.Nil(t, privateKey)
}

// Test_readPrivateKey_SupportsFormats проверяет загрузку RSA-ключа в форматах PKCS#1 и PKCS#8.
func Test_readPrivateKey_SupportsFormats(t *testing.T) {
	// Arrange
	tests := []struct {
		name   string
		format string
	}{
		{
			name:   "pkcs1 private key",
			format: "pkcs1",
		},
		{
			name:   "pkcs8 private key",
			format: "pkcs8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			privateKeyPath, _ := writeTestRSAKeyPair(t, tt.format)

			// Act
			loadedKey, err := readPrivateKey(privateKeyPath)

			// Assert
			require.NoError(t, err)
			assert.NotNil(t, loadedKey)
		})
	}
}

// writeTestRSAKeyPair создает временную пару RSA-ключей в PEM-формате для тестов.
func writeTestRSAKeyPair(t *testing.T, privateKeyFormat string) (privateKeyPath, publicKeyPath string) {
	t.Helper() // нужен, чтобы место ошибки require.NoError отображалось в тестовых функциях, а не в этом хелпере.

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	privateKeyPEM := encodePrivateKeyPEM(t, privateKey, privateKeyFormat)

	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyDER,
	})

	dir := t.TempDir()
	privateKeyPath = filepath.Join(dir, fmt.Sprintf("private_%s.pem", privateKeyFormat))
	publicKeyPath = filepath.Join(dir, "public.pem")

	err = os.WriteFile(privateKeyPath, privateKeyPEM, 0o600)
	require.NoError(t, err)

	err = os.WriteFile(publicKeyPath, publicKeyPEM, 0o600)
	require.NoError(t, err)

	return privateKeyPath, publicKeyPath
}

// encodePrivateKeyPEM кодирует RSA-приватный ключ в PEM с нужным тесту форматом: PKCS#1 или PKCS#8.
func encodePrivateKeyPEM(t *testing.T, privateKey *rsa.PrivateKey, format string) []byte {
	t.Helper() // нужен, чтобы место ошибки require.NoError отображалось в тестовых функциях, а не в этом хелпере.

	switch format {
	case "pkcs1":
		return pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
		})
	case "pkcs8":
		privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
		require.NoError(t, err)
		return pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: privateKeyDER,
		})
	default:
		t.Fatalf("unsupported private key format: %s", format)
		return nil
	}
}
