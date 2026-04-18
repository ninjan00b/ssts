package api_test

import (
	"bytes"
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

func newTestStore() *store.Store {
	return store.New(5 * time.Minute)
}

type uploadResp struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
	Error     string    `json:"error"`
}

func doUpload(t *testing.T, s *store.Store, body any, maxBytes int64) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/upload", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	api.UploadHandler(s, maxBytes).ServeHTTP(w, req)
	return w
}

func TestUploadHandler_TextSuccess(t *testing.T) {
	s := newTestStore()
	w := doUpload(t, s, map[string]string{"type": "text", "data": "hello"}, 1048576)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}

	var resp uploadResp
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Code) != 6 {
		t.Errorf("code length = %d, want 6", len(resp.Code))
	}
	if resp.ExpiresAt.IsZero() {
		t.Error("expires_at is zero")
	}
	if resp.ExpiresAt.Before(time.Now()) {
		t.Error("expires_at is in the past")
	}
}

func TestUploadHandler_URLSuccess(t *testing.T) {
	s := newTestStore()
	w := doUpload(t, s, map[string]string{"type": "url", "data": "https://example.com"}, 1048576)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}
}

func TestUploadHandler_FileSuccess(t *testing.T) {
	s := newTestStore()
	fileData := []byte("fake png content")
	b64 := base64.StdEncoding.EncodeToString(fileData)

	w := doUpload(t, s, map[string]string{
		"type":     "file",
		"data":     b64,
		"filename": "test.png",
	}, 1048576)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}

	var resp uploadResp
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Code == "" {
		t.Error("code is empty")
	}
}

func TestUploadHandler_InvalidType(t *testing.T) {
	s := newTestStore()
	w := doUpload(t, s, map[string]string{"type": "binary", "data": "x"}, 1048576)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestUploadHandler_MissingType(t *testing.T) {
	s := newTestStore()
	w := doUpload(t, s, map[string]string{"data": "x"}, 1048576)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestUploadHandler_EmptyData(t *testing.T) {
	s := newTestStore()
	w := doUpload(t, s, map[string]string{"type": "text", "data": ""}, 1048576)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestUploadHandler_InvalidJSON(t *testing.T) {
	s := newTestStore()
	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader("{not json"))
	w := httptest.NewRecorder()
	api.UploadHandler(s, 1048576).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestUploadHandler_InvalidBase64ForFile(t *testing.T) {
	s := newTestStore()
	w := doUpload(t, s, map[string]string{"type": "file", "data": "not-base64!!!"}, 1048576)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestUploadHandler_FileTooLarge(t *testing.T) {
	s := newTestStore()
	big := make([]byte, 100)
	b64 := base64.StdEncoding.EncodeToString(big)
	w := doUpload(t, s, map[string]string{"type": "file", "data": b64}, 50) // limit 50 bytes

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", w.Code)
	}
}

func TestUploadHandler_TextTooLarge(t *testing.T) {
	s := newTestStore()
	w := doUpload(t, s, map[string]string{"type": "text", "data": strings.Repeat("x", 200)}, 100)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", w.Code)
	}
}

func TestUploadHandler_ResponseContentType(t *testing.T) {
	s := newTestStore()
	w := doUpload(t, s, map[string]string{"type": "text", "data": "hi"}, 1048576)

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
