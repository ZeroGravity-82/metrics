package handler

import (
	"bytes"
	"fmt"
	"io"

	"zerogravity-82/metrics/internal/encryption"
)

// encryptRequestReader реализует интерфейс io.ReadCloser и позволяет прозрачно для сервера дешифровать
// получаемые от клиента данные.
type encryptRequestReader struct {
	r              io.ReadCloser
	er             *bytes.Reader
	encryptedKey   []byte
	privateKeyPath string
}

func newEncryptRequestReader(
	r io.ReadCloser,
	encryptedKey []byte,
	privateKeyPath string,
) *encryptRequestReader {
	return &encryptRequestReader{
		r:              r,
		encryptedKey:   encryptedKey,
		privateKeyPath: privateKeyPath,
	}
}

func (r *encryptRequestReader) Read(p []byte) (int, error) {
	if r.er == nil {
		encryptedData, err := io.ReadAll(r.r)
		if err != nil {
			return 0, fmt.Errorf("encryptRequestReader: failed to read encrypted data: %w", err)
		}
		decryptedData, err := encryption.Decrypt(encryptedData, r.encryptedKey, r.privateKeyPath)
		if err != nil {
			return 0, fmt.Errorf("encryptRequestReader: failed to decrypt data: %w", err)
		}
		r.er = bytes.NewReader(decryptedData)
	}
	n, err := r.er.Read(p)
	if err != nil && err != io.EOF {
		return n, fmt.Errorf("encryptRequestReader: failed to read decrypted data: %w", err)
	}
	return n, err
}

func (r *encryptRequestReader) Close() error {
	if err := r.r.Close(); err != nil {
		return fmt.Errorf("encryptRequestReader: failed to close encrypt reader: %w", err)
	}
	return nil
}
