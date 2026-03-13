package handler

import (
	"bytes"
	"compress/gzip"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompressRequestReader проверяет функционал compressRequestReader
func TestCompressRequestReader(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	data := []byte("Hello, World!")
	_, err := gzWriter.Write(data)
	require.NoError(t, err)
	gzWriter.Close()

	reader, err := newCompressRequestReader(io.NopCloser(&buf))
	require.NoError(t, err)
	defer reader.Close()

	decompressedData := make([]byte, len(data))

	// Act
	n, err := reader.Read(decompressedData)

	// Assert
	require.True(t, err == nil || err == io.EOF)

	assert.Equal(t, len(data), n)
	assert.True(t, bytes.Equal(data, decompressedData))
}
