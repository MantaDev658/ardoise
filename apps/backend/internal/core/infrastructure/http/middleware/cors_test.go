package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSMiddleware(t *testing.T) {
	const origin = "http://localhost:5173"

	wrap := func(called *bool) http.Handler {
		return CORSMiddleware(origin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*called = true
			w.WriteHeader(http.StatusOK)
		}))
	}

	t.Run("sets CORS headers on regular requests", func(t *testing.T) {
		called := false
		req := httptest.NewRequest("GET", "/expenses", nil)
		rr := httptest.NewRecorder()
		wrap(&called).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		if !called {
			t.Error("expected next handler to be called")
		}
		if got := rr.Header().Get("Access-Control-Allow-Origin"); got != origin {
			t.Errorf("expected Allow-Origin %q, got %q", origin, got)
		}
		if got := rr.Header().Get("Access-Control-Allow-Methods"); got == "" {
			t.Error("expected Access-Control-Allow-Methods to be set")
		}
	})

	t.Run("returns 204 for OPTIONS preflight without calling next", func(t *testing.T) {
		called := false
		req := httptest.NewRequest("OPTIONS", "/expenses", nil)
		rr := httptest.NewRecorder()
		wrap(&called).ServeHTTP(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d", rr.Code)
		}
		if called {
			t.Error("next handler must not be called for OPTIONS preflight")
		}
		if got := rr.Header().Get("Access-Control-Allow-Origin"); got != origin {
			t.Errorf("expected Allow-Origin %q on preflight, got %q", origin, got)
		}
	})

	t.Run("respects configured origin", func(t *testing.T) {
		const customOrigin = "https://app.example.com"
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		CORSMiddleware(customOrigin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rr, req)

		if got := rr.Header().Get("Access-Control-Allow-Origin"); got != customOrigin {
			t.Errorf("expected Allow-Origin %q, got %q", customOrigin, got)
		}
	})
}
