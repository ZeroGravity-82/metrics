package handler

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompressWriter проверяет функционал compressResponseWriter
func TestCompressWriter(t *testing.T) {
	// Arrange
	recorder := httptest.NewRecorder()
	writer := newCompressResponseWriter(recorder)
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
	err = writer.Close()
	require.NoError(t, err)
	assert.Equal(t, len(totalData), n1+n2)
	assert.Equal(t, "gzip", recorder.Header().Get("Content-Encoding"))
	assert.Equal(t, customStatusCode, recorder.Result().StatusCode)

	gzReader, err := gzip.NewReader(recorder.Body) // Проверяем сжатые данные
	require.NoError(t, err)
	defer gzReader.Close()

	decompressedData, err := io.ReadAll(gzReader)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(totalData, decompressedData))
}
