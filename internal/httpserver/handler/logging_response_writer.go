package handler

import (
	"fmt"
	"net/http"
)

type (
	// responseData хранит аналитические данные HTTP-ответа
	responseData struct {
		status int
		size   int
	}
	// loggingResponseWriter реализует интерфейс http.ResponseWriter и позволяет прозрачно для сервера собирать
	// аналитические данные HTTP-ответов.
	loggingResponseWriter struct {
		http.ResponseWriter
		responseData *responseData
		statusCode   int
		wroteHeader  bool
	}
)

func newLoggingResponseWriter(w http.ResponseWriter, responseData *responseData) *loggingResponseWriter {
	return &loggingResponseWriter{
		ResponseWriter: w,
		responseData:   responseData,
		statusCode:     http.StatusOK,
	}
}

func (w *loggingResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(w.statusCode)
	}
	size, err := w.ResponseWriter.Write(b)
	w.responseData.size += size
	if err != nil {
		return size, fmt.Errorf("loggingResponseWriter: failed to write data: %w", err)
	}
	return size, nil
}

func (w *loggingResponseWriter) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}
	w.responseData.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
	w.wroteHeader = true
}
