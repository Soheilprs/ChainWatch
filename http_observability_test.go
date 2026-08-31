package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPMiddlewareGeneratesRequestIDs(t *testing.T) {
	var logs bytes.Buffer
	server := NewHTTPServerWithObservability(
		&MockTransferReader{},
		nil,
		slog.New(slog.NewJSONHandler(&logs, nil)),
		NewMetrics(),
	)

	requestIDs := make(map[string]struct{}, 2)
	for range 2 {
		request := httptest.NewRequest(http.MethodGet, "/health", nil)
		request.Header.Set(requestIDHeader, strings.Repeat("untrusted", 100))
		response := httptest.NewRecorder()

		server.Handler().ServeHTTP(response, request)

		requestID := response.Header().Get(requestIDHeader)
		if len(requestID) != 32 {
			t.Fatalf("request ID length = %d, want 32", len(requestID))
		}
		if _, err := hex.DecodeString(requestID); err != nil {
			t.Fatalf("request ID %q is not hexadecimal: %v", requestID, err)
		}
		if _, duplicate := requestIDs[requestID]; duplicate {
			t.Fatalf("duplicate request ID generated: %q", requestID)
		}
		requestIDs[requestID] = struct{}{}

		if !strings.Contains(logs.String(), `"requestId":"`+requestID+`"`) {
			t.Fatalf("request ID %q missing from structured logs: %s", requestID, logs.String())
		}
	}
}

func TestHTTPMiddlewareRecoversPanics(t *testing.T) {
	metrics := NewMetrics()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	panicHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("private panic details")
	})
	handler := requestIDMiddleware(
		observabilityMiddleware(
			logger,
			metrics,
			recoveryMiddleware(logger, panicHandler),
		),
	)
	request := httptest.NewRequest(http.MethodGet, "/panic", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json; charset=utf-8" {
		t.Fatalf("content type = %q, want JSON", contentType)
	}
	requestID := response.Header().Get(requestIDHeader)
	if requestID == "" {
		t.Fatal("panic response is missing a request ID")
	}

	var body APIErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode panic response: %v", err)
	}
	if body.Error.Code != "internal_error" || body.Error.RequestID != requestID {
		t.Fatalf("unexpected panic response: %+v", body)
	}
	if strings.Contains(response.Body.String(), "private panic details") {
		t.Fatal("panic details leaked to the client")
	}

	snapshot := metrics.Snapshot()
	if snapshot.HTTPRequests != 1 || snapshot.HTTPErrors != 1 {
		t.Fatalf("unexpected HTTP metrics after panic: %+v", snapshot)
	}
}

func TestHTTPServerMethodErrorsAreStructured(t *testing.T) {
	server := NewHTTPServer(&MockTransferReader{}, nil)
	request := httptest.NewRequest(http.MethodPost, "/transfers", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
	if allow := response.Header().Get("Allow"); allow != http.MethodGet {
		t.Fatalf("Allow header = %q, want %q", allow, http.MethodGet)
	}

	var body APIErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode method error: %v", err)
	}
	if body.Error.Code != "method_not_allowed" || body.Error.RequestID == "" {
		t.Fatalf("unexpected method error: %+v", body)
	}
}
