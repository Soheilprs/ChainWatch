package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProfilingHandlerServesProfiles(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	response := httptest.NewRecorder()

	NewProfilingHandler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), "goroutine") {
		t.Fatalf("profiling index did not list goroutine profile: %s", response.Body.String())
	}
}
