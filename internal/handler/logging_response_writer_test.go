package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoggingResponseWriter проверяет функционал loggingResponseWriter
func TestLoggingResponseWriter(t *testing.T) {
	// Arrange
	recorder := httptest.NewRecorder()
	responseData := &responseData{}
	writer := newLoggingResponseWriter(recorder, responseData)
	data1 := []byte("Hello, ")
	data2 := []byte("World!")
	totalData := append(data1, data2...)
	customStatusCode := http.StatusCreated

	// Act
	writer.WriteHeader(customStatusCode)
	writer.WriteHeader(http.StatusAccepted) // Повторный вызов метода WriteHeader не изменяет статус
	n1, err := writer.Write(data1)
	require.NoError(t, err)
	n2, err := writer.Write(data2)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, len(totalData), n1+n2)
	assert.True(t, bytes.Equal(totalData, recorder.Body.Bytes()))
	assert.Equal(t, customStatusCode, recorder.Result().StatusCode)
	assert.Equal(t, len(totalData), responseData.size)
	assert.Equal(t, customStatusCode, responseData.status)
}
