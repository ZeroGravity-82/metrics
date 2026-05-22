package handler

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zerogravity-82/metrics/internal/encryption"
)

// TestEncryptRequestReader_Read проверяет, что encryptRequestReader корректно расшифровывает данные.
func TestEncryptRequestReader_Read(t *testing.T) {
	// Arrange
	privateKeyPath, publicKeyPath := writeTestRSAKeyPair(t)
	originalData := []byte("compressed metrics payload")
	encryptedData, encryptedKey, err := encryption.Encrypt(originalData, publicKeyPath)
	require.NoError(t, err)

	reader := newEncryptRequestReader(io.NopCloser(bytes.NewReader(encryptedData)), encryptedKey, privateKeyPath)
	decryptedData := make([]byte, len(originalData))

	// Act
	n, err := reader.Read(decryptedData)

	// Assert
	require.True(t, err == nil || err == io.EOF)
	assert.Equal(t, len(originalData), n)
	assert.Equal(t, originalData, decryptedData)
}

// TestEncryptRequestReader_ReadReturnsErrorOnInvalidEncryptedData проверяет ошибку, если payload невозможно расшифровать.
func TestEncryptRequestReader_ReadReturnsErrorOnInvalidEncryptedData(t *testing.T) {
	// Arrange
	privateKeyPath, publicKeyPath := writeTestRSAKeyPair(t)
	_, encryptedKey, err := encryption.Encrypt([]byte("payload"), publicKeyPath)
	require.NoError(t, err)

	invalidEncryptedData := []byte("invalid")
	reader := newEncryptRequestReader(io.NopCloser(bytes.NewReader(invalidEncryptedData)), encryptedKey, privateKeyPath)
	buf := make([]byte, 32)

	// Act
	n, err := reader.Read(buf)

	// Assert
	require.Error(t, err)
	assert.Zero(t, n)
	assert.ErrorContains(t, err, "failed to decrypt data")
}

func writeTestRSAKeyPair(t *testing.T) (privateKeyPath, publicKeyPath string) {
	t.Helper() // нужен, чтобы место ошибки require.NoError отображалось в тестовых функциях, а не в этом хелпере.

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyDER,
	})

	dir := t.TempDir()
	privateKeyPath = filepath.Join(dir, "private.pem")
	publicKeyPath = filepath.Join(dir, "public.pem")

	err = os.WriteFile(privateKeyPath, privateKeyPEM, 0o600)
	require.NoError(t, err)

	err = os.WriteFile(publicKeyPath, publicKeyPEM, 0o600)
	require.NoError(t, err)

	return privateKeyPath, publicKeyPath
}
