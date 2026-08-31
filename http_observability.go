package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"sync/atomic"
	"time"
)

const requestIDHeader = "X-Request-ID"

type requestIDContextKey struct{}

var fallbackRequestIDCounter atomic.Uint64

type statusRecorder struct {
	http.ResponseWriter

	status        int
	wroteResponse bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.wroteResponse {
		return
	}

	r.status = status
	r.wroteResponse = true
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	if !r.wroteResponse {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(body)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func (r *statusRecorder) responseStarted() bool {
	return r.wroteResponse
}

func (r *statusRecorder) markStatus(status int) {
	r.status = status
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := newRequestID()
		w.Header().Set(requestIDHeader, requestID)

		ctx := context.WithValue(r.Context(), requestIDContextKey{}, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func newRequestID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}

	return fmt.Sprintf(
		"%016x%016x",
		uint64(time.Now().UnixNano()),
		fallbackRequestIDCounter.Add(1),
	)
}

func requestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}

func recoveryMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			panicValue := recover()
			if panicValue == nil {
				return
			}

			logger.ErrorContext(
				r.Context(),
				"recovered HTTP panic",
				"requestId", requestIDFromContext(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"panic", panicValue,
				"stack", string(debug.Stack()),
			)

			started := false
			if recorder, ok := w.(interface {
				responseStarted() bool
				markStatus(int)
			}); ok {
				started = recorder.responseStarted()
				recorder.markStatus(http.StatusInternalServerError)
			}

			if !started {
				if err := writeAPIError(
					w,
					r,
					http.StatusInternalServerError,
					"internal_error",
					"internal server error",
				); err != nil {
					logger.ErrorContext(r.Context(), "failed to write panic response", "error", err)
				}
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func observabilityMiddleware(
	logger *slog.Logger,
	metrics *Metrics,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{
			ResponseWriter: w,
			status:         http.StatusOK,
		}

		next.ServeHTTP(recorder, r)

		duration := time.Since(start)
		metrics.ObserveHTTPRequest(recorder.status, duration)

		attributes := []any{
			"requestId", requestIDFromContext(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration", duration,
		}
		if recorder.status >= http.StatusInternalServerError {
			logger.ErrorContext(r.Context(), "http request", attributes...)
			return
		}

		logger.InfoContext(r.Context(), "http request", attributes...)
	})
}
