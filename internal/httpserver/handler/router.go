package handler

import (
	"bytes"
	"compress/gzip"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
)

// MetricRouter собирает и возвращает HTTP-роутер сервиса метрик.
//
// Доступные ручки:
//
//	POST /update/{type}/{name}/{value}
//	GET  /value/{type}/{name}
//	GET  /
//	POST /update
//	POST /updates
//	POST /value
//	GET  /ping
//
// Если key не пустой, включается middleware подписи запросов/ответов.
func MetricRouter(s Storage, a AuditPublisher, key string, logger zerolog.Logger) chi.Router {
	r := chi.NewRouter()
	r.Use(
		middleware.StripSlashes,
		middleware.RealIP,
		withLogging(logger),
		withGzip(logger),
	)
	if key != "" {
		r.Use(withSignature(key, logger))
	}
	h := New(s, a, logger)

	textPlainContentType := middleware.AllowContentType("text/plain")
	r.With(textPlainContentType).Post("/update/{mType}/{mName}/{mValue}", h.updateMetric)
	r.With(textPlainContentType).Get("/value/{mType}/{mName}", h.getMetric)
	r.With(textPlainContentType).Get("/", h.getMetricList)

	applicationJSONContentType := middleware.AllowContentType("application/json")
	r.With(applicationJSONContentType).Post("/update", h.update)
	r.With(applicationJSONContentType).Post("/updates", h.updates)
	r.With(applicationJSONContentType).Post("/value", h.get)

	r.Get("/ping", h.ping)
	return r
}

func withLogging(logger zerolog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			URI := r.RequestURI
			method := r.Method
			start := time.Now()

			responseData := &responseData{
				status: http.StatusOK,
				size:   0,
			}
			lw := newLoggingResponseWriter(w, responseData)
			next.ServeHTTP(lw, r)

			duration := time.Since(start)
			logger.Info().
				Str("uri", URI).
				Str("method", method).
				Str("duration", duration.String()).
				Str("status", strconv.Itoa(responseData.status)).
				Str("size", strconv.Itoa(responseData.size)).
				Msg("Request processed")
		})
	}
}

func withSignature(key string, logger zerolog.Logger) func(next http.Handler) http.Handler {
	const signatureHeaderName = "HashSHA256"

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, fmt.Sprintf("unable to read request body: %s", err.Error()), http.StatusBadRequest)
				return
			}
			if len(body) > 0 {
				if r.Header.Get(signatureHeaderName) == "" {
					http.Error(w, fmt.Sprintf("header %s is not provided", signatureHeaderName), http.StatusBadRequest)
					return
				}
				err = validateSignature(r.Header.Get(signatureHeaderName), body, key)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
			}
			r.Body = io.NopCloser(bytes.NewBuffer(body))
			ow := newSigningResponseWriter(w, key, logger)
			next.ServeHTTP(ow, r)
		})
	}
}

func validateSignature(signature string, body []byte, key string) error {
	decodedSig, err := hex.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("unable to decode signature string: %w", err)
	}
	if !hmac.Equal(generateSignature(body, key), decodedSig) {
		return errors.New("invalid request signature")
	}

	return nil
}

func generateSignature(data []byte, key string) []byte {
	h := hmac.New(sha256.New, []byte(key))
	h.Write(data)
	return h.Sum(nil)
}

func withGzip(logger zerolog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ow := w
			acceptEncoding := r.Header.Get("Accept-Encoding")
			supportGzip := strings.Contains(acceptEncoding, "gzip")
			if supportGzip {
				cw := newCompressResponseWriter(w)
				defer cw.Close()
				ow = cw
			}

			contentEncoding := r.Header.Get("Content-Encoding")
			sendsGzip := strings.Contains(contentEncoding, "gzip")
			if sendsGzip {
				cr, err := newCompressRequestReader(r.Body)
				if err != nil {
					if errors.Is(err, gzip.ErrChecksum) || errors.Is(err, gzip.ErrHeader) {
						ow.WriteHeader(http.StatusBadRequest)
						return
					}
					ow.WriteHeader(http.StatusInternalServerError)
					logError(err, "Request error", logger)
					return
				}
				defer cr.Close()
				r.Body = cr
			}
			next.ServeHTTP(ow, r)
		})
	}
}
