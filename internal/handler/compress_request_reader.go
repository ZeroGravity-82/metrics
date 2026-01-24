package handler

import (
	"compress/gzip"
	"fmt"
	"io"
)

// compressRequestReader реализует интерфейс io.ReadCloser и позволяет прозрачно для сервера декомпрессировать
// получаемые от клиента данные.
type compressRequestReader struct {
	r  io.ReadCloser
	zr *gzip.Reader
}

func newCompressRequestReader(r io.ReadCloser) (*compressRequestReader, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("compressRequestReader: failed to create gzip reader: %w", err)
	}

	return &compressRequestReader{
		r:  r,
		zr: zr,
	}, nil
}

func (r *compressRequestReader) Read(p []byte) (int, error) {
	n, err := r.zr.Read(p)
	if err != nil && err != io.EOF {
		return n, fmt.Errorf("compressRequestReader: failed to read compressed data: %w", err)
	}

	return n, err
}

func (r *compressRequestReader) Close() error {
	if err := r.r.Close(); err != nil {
		return fmt.Errorf("compressRequestReader: failed to close gzip reader: %w", err)
	}
	return r.zr.Close()
}
