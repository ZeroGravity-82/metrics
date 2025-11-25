package handler

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
)

// compressWriter реализует интерфейс http.ResponseWriter и позволяет прозрачно для сервера
// сжимать передаваемые данные и выставлять правильные HTTP-заголовки
type compressWriter struct {
	w  http.ResponseWriter
	zw *gzip.Writer
}

func newCompressWriter(w http.ResponseWriter) *compressWriter {
	return &compressWriter{
		w:  w,
		zw: gzip.NewWriter(w),
	}
}

func (c *compressWriter) Header() http.Header {
	return c.w.Header()
}

func (c *compressWriter) Write(p []byte) (int, error) {
	n, err := c.zw.Write(p)
	if err != nil {
		return n, fmt.Errorf("compressWriter: failed to write compressed data: %w", err)
	}
	return n, nil
}

func (c *compressWriter) WriteHeader(statusCode int) {
	c.w.WriteHeader(statusCode)
}

// Close закрывает gzip.Writer и досылает все данные из буфера.
func (c *compressWriter) Close() error {
	if err := c.zw.Close(); err != nil {
		return fmt.Errorf("compressWriter: failed to close gzip writer: %w", err)
	}
	return nil
}

// compressReader реализует интерфейс io.ReadCloser и позволяет прозрачно для сервера
// декомпрессировать получаемые от клиента данные
type compressReader struct {
	r  io.ReadCloser
	zr *gzip.Reader
}

func newCompressReader(r io.ReadCloser) (*compressReader, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("compressReader: failed to create gzip reader: %w", err)
	}

	return &compressReader{
		r:  r,
		zr: zr,
	}, nil
}

func (c compressReader) Read(p []byte) (int, error) {
	n, err := c.zr.Read(p)
	if err != nil && err != io.EOF {
		return n, fmt.Errorf("compressReader: failed to read compressed data: %w", err)
	}

	return n, err
}

func (c *compressReader) Close() error {
	if err := c.r.Close(); err != nil {
		return fmt.Errorf("compressReader: failed to close gzip reader: %w", err)
	}
	return c.zr.Close()
}
