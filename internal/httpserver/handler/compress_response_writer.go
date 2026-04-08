package handler

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"sync"
)

// compressResponseWriter реализует интерфейс http.ResponseWriter и позволяет прозрачно для сервера сжимать передаваемые
// данные и выставлять правильные HTTP-заголовки.
type compressResponseWriter struct {
	http.ResponseWriter
	zw          *gzip.Writer
	statusCode  int
	wroteHeader bool
}

// gzipWriterPool используется для уменьшения числа аллокаций памяти за счет того, что gzip.Writer не создается новый
// для каждого запроса.
var gzipWriterPool = sync.Pool{
	New: func() any {
		return gzip.NewWriter(nil)
	},
}

func newCompressResponseWriter(w http.ResponseWriter) *compressResponseWriter {
	zw := gzipWriterPool.Get().(*gzip.Writer)
	zw.Reset(w)
	return &compressResponseWriter{
		ResponseWriter: w,
		zw:             zw,
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
	err := w.zw.Close()

	// Возвращаем gzip.Writer обратно в пул.
	w.zw.Reset(nil)
	gzipWriterPool.Put(w.zw)

	if err != nil {
		return fmt.Errorf("compressResponseWriter: failed to close gzip writer: %w", err)
	}
	return nil
}
