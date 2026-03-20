package handler

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSigningResponseWriter проверяет функционал signingResponseWriter
func TestSigningResponseWriter(t *testing.T) {
	// Arrange
	recorder := httptest.NewRecorder()
	key := "testkey"
	logger := zerolog.Nop()
	writer := newSigningResponseWriter(recorder, key, logger)
	data1 := []byte("Hello, ")
	data2 := []byte("World!")
	totalData := append(data1, data2...)
	customStatusCode := http.StatusCreated

	// Act
	n1, err := writer.Write(data1)
	require.NoError(t, err)
	n2, err := writer.Write(data2)
	writer.WriteHeader(customStatusCode)
	writer.WriteHeader(http.StatusAccepted) // Повторный вызов метода WriteHeader не изменяет статус

	// Arrange
	require.NoError(t, err)
	assert.Equal(t, len(totalData), n1+n2)
	expectedSignature := computeHMAC(totalData, key)
	actualSignature := recorder.Header().Get("HashSHA256")
	assert.Equal(t, expectedSignature, actualSignature)
	assert.True(t, bytes.Equal(totalData, recorder.Body.Bytes()))
}

// TestSigningResponseWriter_EmptyBody проверяет функционал signingResponseWriter с пустым телом ответа
func TestSigningResponseWriter_EmptyBody(t *testing.T) {
	// Arrange
	recorder := httptest.NewRecorder()
	key := "testkey"
	logger := zerolog.Nop()
	writer := newSigningResponseWriter(recorder, key, logger)

	// Act
	writer.WriteHeader(http.StatusOK)

	// Assert
	assert.Empty(t, recorder.Header().Get("HashSHA256"))
}

// TestSigningResponseWriter_EmptyKey проверяет функционал signingResponseWriter с пустым ключом
func TestSigningResponseWriter_EmptyKey(t *testing.T) {
	// Arrange
	recorder := httptest.NewRecorder()
	key := ""
	logger := zerolog.Nop()
	writer := newSigningResponseWriter(recorder, key, logger)
	data := []byte("Hello, World!")

	// Act
	_, err := writer.Write(data)
	require.NoError(t, err)
	writer.WriteHeader(http.StatusOK)

	// Assert
	assert.Empty(t, recorder.Header().Get("HashSHA256"))
}

// computeHMAC вспомогательная функция для вычисления HMAC-SHA256
func computeHMAC(message []byte, key string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write(message)
	return hex.EncodeToString(h.Sum(nil))
}
