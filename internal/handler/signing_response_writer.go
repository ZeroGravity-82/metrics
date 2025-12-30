package handler

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/rs/zerolog"
)

type (
	signingResponseWriter struct {
		http.ResponseWriter
		body   *bytes.Buffer
		key    string
		logger zerolog.Logger
	}
)

func newSigningWriter(w http.ResponseWriter, key string, logger zerolog.Logger) *signingResponseWriter {
	return &signingResponseWriter{
		ResponseWriter: w,
		body:           new(bytes.Buffer),
		key:            key,
		logger:         logger,
	}
}

func (w *signingResponseWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

func (w *signingResponseWriter) WriteHeader(statusCode int) {
	bodyBz := w.body.Bytes()
	if len(bodyBz) > 0 {
		h := hmac.New(sha256.New, []byte(w.key))
		h.Write(bodyBz)
		signature := hex.EncodeToString(h.Sum(nil))
		w.ResponseWriter.Header().Set("HashSHA256", signature)
	}
	w.ResponseWriter.WriteHeader(statusCode)
	_, err := w.ResponseWriter.Write(bodyBz)
	if err != nil {
		w.logger.Error().Str("error", err.Error()).Msg("Error on sending signed response")
	}
}
