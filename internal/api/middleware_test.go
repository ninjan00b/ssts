package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ssts/internal/api"
)

var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func TestRateLimitMiddleware_AllowsUnderLimit(t *testing.T) {
	mw := api.RateLimitMiddleware(10)
	handler := mw(okHandler)

	for i := range 10 {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i+1, w.Code)
		}
	}
}

func TestRateLimitMiddleware_BlocksOverLimit(t *testing.T) {
	// Use burst=2 by setting rps=2 so the token bucket fills to 2
	mw := api.RateLimitMiddleware(2)
	handler := mw(okHandler)

	var lastCode int
	for range 20 {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:9999"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		lastCode = w.Code
	}

	if lastCode != http.StatusTooManyRequests {
		t.Errorf("last request status = %d, want 429", lastCode)
	}
}

func TestRateLimitMiddleware_IsolatesIPs(t *testing.T) {
	mw := api.RateLimitMiddleware(1)
	handler := mw(okHandler)

	for i, ip := range []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip + ":1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d from %s: status = %d, want 200 (different IPs should have separate limits)", i+1, ip, w.Code)
		}
	}
}

func TestRateLimitMiddleware_UsesXForwardedFor(t *testing.T) {
	mw := api.RateLimitMiddleware(1)
	handler := mw(okHandler)

	// First request from XFF header — should be allowed
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.Header.Set("X-Forwarded-For", "203.0.113.5")
	req1.RemoteAddr = "127.0.0.1:1234"
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first XFF request: status = %d, want 200", w1.Code)
	}

	// Second request with same XFF — should be rate limited
	for range 10 {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.5")
		req.RemoteAddr = "127.0.0.1:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("X-Forwarded-For", "203.0.113.5")
	req2.RemoteAddr = "127.0.0.1:1234"
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("over-limit XFF request: status = %d, want 429", w2.Code)
	}
}

func TestRateLimitMiddleware_ErrorBodyIsJSON(t *testing.T) {
	mw := api.RateLimitMiddleware(1)
	handler := mw(okHandler)

	// Exhaust the limit
	for range 20 {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "5.5.5.5:1"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "5.5.5.5:1"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code == http.StatusTooManyRequests {
		ct := w.Header().Get("Content-Type")
		if !strings.Contains(ct, "application/json") {
			t.Errorf("429 Content-Type = %q, want application/json", ct)
		}
	}
}

func TestMaxBodySize_AllowsUnderLimit(t *testing.T) {
	mw := api.MaxBodySize(100)
	handler := mw(okHandler)

	body := strings.NewReader(strings.Repeat("x", 50))
	req := httptest.NewRequest(http.MethodPost, "/", body)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestMaxBodySize_RejectsOverLimit(t *testing.T) {
	// The MaxBodySize middleware wraps with MaxBytesReader.
	// The 413 is only returned if the handler reads the body and the error propagates.
	// We test this via the UploadHandler which reads the body.
	mw := api.MaxBodySize(10)
	uploadMW := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 100)
		_, err := r.Body.Read(buf)
		if err != nil && err.Error() == "http: request body too large" {
			http.Error(w, "too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	body := strings.NewReader(strings.Repeat("x", 100))
	req := httptest.NewRequest(http.MethodPost, "/", body)
	w := httptest.NewRecorder()
	uploadMW.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", w.Code)
	}
}
