package api_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ssts/internal/api"
	"ssts/internal/store"
)

type fetchResp struct {
	Type     string `json:"type"`
	Data     string `json:"data"`
	Filename string `json:"filename"`
	Error    string `json:"error"`
}

// fetchViaRouter routes through a real ServeMux so PathValue works.
func fetchViaRouter(t *testing.T, s *store.Store, code string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /fetch/{code}", api.FetchHandler(s))
	req := httptest.NewRequest(http.MethodGet, "/fetch/"+code, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func TestFetchHandler_TextSuccess(t *testing.T) {
	s := store.New(5 * time.Minute)
	code, _ := s.Put("text", []byte("hello world"), "")

	w := fetchViaRouter(t, s, code)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp fetchResp
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Type != "text" {
		t.Errorf("type = %q, want %q", resp.Type, "text")
	}
	if resp.Data != "hello world" {
		t.Errorf("data = %q, want %q", resp.Data, "hello world")
	}
}

func TestFetchHandler_URLSuccess(t *testing.T) {
	s := store.New(5 * time.Minute)
	code, _ := s.Put("url", []byte("https://example.com"), "")

	w := fetchViaRouter(t, s, code)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp fetchResp
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Data != "https://example.com" {
		t.Errorf("data = %q, want url", resp.Data)
	}
}

func TestFetchHandler_FileReturnedAsBase64(t *testing.T) {
	s := store.New(5 * time.Minute)
	fileData := []byte{0x89, 0x50, 0x4E, 0x47}
	code, _ := s.Put("file", fileData, "img.png")

	w := fetchViaRouter(t, s, code)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp fetchResp
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Type != "file" {
		t.Errorf("type = %q, want file", resp.Type)
	}
	if resp.Filename != "img.png" {
		t.Errorf("filename = %q, want img.png", resp.Filename)
	}

	decoded, err := base64.StdEncoding.DecodeString(resp.Data)
	if err != nil {
		t.Fatalf("data is not valid base64: %v", err)
	}
	if string(decoded) != string(fileData) {
		t.Error("decoded file data does not match original")
	}
}

func TestFetchHandler_NotFound(t *testing.T) {
	s := store.New(5 * time.Minute)

	w := fetchViaRouter(t, s, "AAAAAA")

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestFetchHandler_OneTimeRead(t *testing.T) {
	s := store.New(5 * time.Minute)
	code, _ := s.Put("text", []byte("secret"), "")

	first := fetchViaRouter(t, s, code)
	second := fetchViaRouter(t, s, code)

	if first.Code != http.StatusOK {
		t.Errorf("first: status = %d, want 200", first.Code)
	}
	if second.Code != http.StatusNotFound {
		t.Errorf("second: status = %d, want 404 (one-time read)", second.Code)
	}
}

func TestFetchHandler_InvalidCodeFormat(t *testing.T) {
	s := store.New(5 * time.Minute)

	cases := []string{
		"AAAAA",       // too short
		"AAAAAAA",     // too long
		"aaaaaa",      // lowercase
		"AAAA0O",      // contains O and 0 (excluded from alphabet)
		"AAAA1I",      // contains I and 1 (excluded)
		"A A AA",      // spaces
	}

	for _, code := range cases {
		t.Run(code, func(t *testing.T) {
			w := fetchViaRouter(t, s, code)
			if w.Code != http.StatusNotFound {
				t.Errorf("code %q: status = %d, want 404", code, w.Code)
			}
		})
	}
}

func TestFetchHandler_ResponseContentType(t *testing.T) {
	s := store.New(5 * time.Minute)
	code, _ := s.Put("text", []byte("hi"), "")

	w := fetchViaRouter(t, s, code)

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
