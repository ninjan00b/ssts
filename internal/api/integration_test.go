package api_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ssts/internal/api"
	"ssts/internal/store"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	s := store.New(5 * time.Minute)
	mux := http.NewServeMux()
	mux.Handle("POST /upload", api.UploadHandler(s, 1048576))
	mux.Handle("GET /fetch/{code}", api.FetchHandler(s))
	return httptest.NewServer(mux)
}

func TestIntegration_TextRoundTrip(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	// Upload
	body, _ := json.Marshal(map[string]string{"type": "text", "data": "integration test"})
	resp, err := http.Post(srv.URL+"/upload", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload status = %d, want 201", resp.StatusCode)
	}

	var uploadResp struct {
		Code string `json:"code"`
	}
	json.NewDecoder(resp.Body).Decode(&uploadResp)

	if uploadResp.Code == "" {
		t.Fatal("empty code in upload response")
	}

	// Fetch
	getResp, err := http.Get(srv.URL + "/fetch/" + uploadResp.Code)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("fetch status = %d, want 200", getResp.StatusCode)
	}

	var fetchResp struct {
		Type string `json:"type"`
		Data string `json:"data"`
	}
	json.NewDecoder(getResp.Body).Decode(&fetchResp)

	if fetchResp.Type != "text" {
		t.Errorf("type = %q, want text", fetchResp.Type)
	}
	if fetchResp.Data != "integration test" {
		t.Errorf("data = %q, want %q", fetchResp.Data, "integration test")
	}
}

func TestIntegration_FileRoundTrip(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	original := []byte("fake file content for testing")
	b64 := base64.StdEncoding.EncodeToString(original)

	body, _ := json.Marshal(map[string]string{
		"type":     "file",
		"data":     b64,
		"filename": "report.txt",
	})
	uploadResp, err := http.Post(srv.URL+"/upload", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer uploadResp.Body.Close()

	var up struct {
		Code string `json:"code"`
	}
	json.NewDecoder(uploadResp.Body).Decode(&up)

	fetchResp, err := http.Get(srv.URL + "/fetch/" + up.Code)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	defer fetchResp.Body.Close()

	var fr struct {
		Type     string `json:"type"`
		Data     string `json:"data"`
		Filename string `json:"filename"`
	}
	json.NewDecoder(fetchResp.Body).Decode(&fr)

	decoded, err := base64.StdEncoding.DecodeString(fr.Data)
	if err != nil {
		t.Fatalf("decode base64: %v", err)
	}
	if string(decoded) != string(original) {
		t.Errorf("file data mismatch: got %q, want %q", decoded, original)
	}
	if fr.Filename != "report.txt" {
		t.Errorf("filename = %q, want report.txt", fr.Filename)
	}
}

func TestIntegration_OneTimeEnforced(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"type": "text", "data": "one-time"})
	uploadResp, err := http.Post(srv.URL+"/upload", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer uploadResp.Body.Close()

	var up struct{ Code string }
	json.NewDecoder(uploadResp.Body).Decode(&up)

	first, _ := http.Get(srv.URL + "/fetch/" + up.Code)
	first.Body.Close()

	second, _ := http.Get(srv.URL + "/fetch/" + up.Code)
	second.Body.Close()

	if first.StatusCode != http.StatusOK {
		t.Errorf("first fetch: status = %d, want 200", first.StatusCode)
	}
	if second.StatusCode != http.StatusNotFound {
		t.Errorf("second fetch: status = %d, want 404", second.StatusCode)
	}
}

func TestIntegration_RateLimitOnFetch(t *testing.T) {
	s := store.New(5 * time.Minute)
	mux := http.NewServeMux()
	rateMW := api.RateLimitMiddleware(2)
	mux.Handle("POST /upload", api.UploadHandler(s, 1048576))
	mux.Handle("GET /fetch/{code}", rateMW(api.FetchHandler(s)))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	var got429 bool
	for range 30 {
		resp, err := http.Get(srv.URL + "/fetch/AAAAAA")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}

	if !got429 {
		t.Error("expected 429 after exceeding rate limit, got none")
	}
}
