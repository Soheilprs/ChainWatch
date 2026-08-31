package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateProfilingAddressAcceptsLoopback(t *testing.T) {
	for _, address := range []string{
		"127.0.0.1:6060",
		"localhost:6060",
		"[::1]:6060",
	} {
		t.Run(address, func(t *testing.T) {
			if err := validateProfilingAddress(address); err != nil {
				t.Fatalf("validate %q: %v", address, err)
			}
		})
	}
}

func TestProfilingHandlerServesProfiles(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	response := httptest.NewRecorder()

	newProfilingHandler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), "goroutine") {
		t.Fatalf("profiling index did not list goroutine profile: %s", response.Body.String())
	}
}

func TestPublicHTTPServerDoesNotExposeProfiling(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	response := httptest.NewRecorder()

	NewHTTPServer(&MockTransferReader{}, nil).Handler().ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("public profiling status = %d, want %d", response.Code, http.StatusNotFound)
	}
}
