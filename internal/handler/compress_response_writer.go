package handler

import (
	"compress/gzip"
	"fmt"
	"net/http"
)

// compressResponseWriter реализует интерфейс http.ResponseWriter и позволяет прозрачно для сервера сжимать передаваемые
// данные и выставлять правильные HTTP-заголовки.
type compressResponseWriter struct {
	http.ResponseWriter
	zw          *gzip.Writer
	statusCode  int
	wroteHeader bool
}

func newCompressResponseWriter(w http.ResponseWriter) *compressResponseWriter {
	return &compressResponseWriter{
		ResponseWriter: w,
		zw:             gzip.NewWriter(w),
		statusCode:     http.StatusOK,
	}
}

func (w *compressResponseWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(w.statusCode)
	}
	n, err := w.zw.Write(p)
	if err != nil {
		return n, fmt.Errorf("compressResponseWriter: failed to write compressed data: %w", err)
	}
	return n, nil
}

func (w *compressResponseWriter) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}
	w.ResponseWriter.Header().Set("Content-Encoding", "gzip")
	w.ResponseWriter.WriteHeader(statusCode)
	w.wroteHeader = true
}

// Close закрывает gzip.Writer и досылает все данные из буфера.
func (w *compressResponseWriter) Close() error {
	if err := w.zw.Close(); err != nil {
		return fmt.Errorf("compressResponseWriter: failed to close gzip writer: %w", err)
	}
	return nil
}
