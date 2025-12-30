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
	http.ResponseWriter
	zw          *gzip.Writer
	statusCode  int
	wroteHeader bool
}

func newCompressWriter(w http.ResponseWriter) *compressWriter {
	return &compressWriter{
		ResponseWriter: w,
		zw:             gzip.NewWriter(w),
		statusCode:     http.StatusOK,
	}
}

func (w *compressWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(w.statusCode)
	}
	n, err := w.zw.Write(p)
	if err != nil {
		return n, fmt.Errorf("compressWriter: failed to write compressed data: %w", err)
	}
	return n, nil
}

func (w *compressWriter) WriteHeader(statusCode int) {
	if !w.wroteHeader {
		w.wroteHeader = true
		w.ResponseWriter.WriteHeader(statusCode)
	}
}

// Close закрывает gzip.Writer и досылает все данные из буфера.
func (w *compressWriter) Close() error {
	if err := w.zw.Close(); err != nil {
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

func (r *compressReader) Read(p []byte) (int, error) {
	n, err := r.zr.Read(p)
	if err != nil && err != io.EOF {
		return n, fmt.Errorf("compressReader: failed to read compressed data: %w", err)
	}

	return n, err
}

func (r *compressReader) Close() error {
	if err := r.r.Close(); err != nil {
		return fmt.Errorf("compressReader: failed to close gzip reader: %w", err)
	}
	return r.zr.Close()
}
