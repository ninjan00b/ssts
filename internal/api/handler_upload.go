package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"ssts/internal/store"
)

type uploadRequest struct {
	Type     string `json:"type"`
	Data     string `json:"data"`
	Filename string `json:"filename"`
}

type uploadResponse struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}

func UploadHandler(s *store.Store, maxPayloadBytes int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req uploadRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if err.Error() == "http: request body too large" {
				writeError(w, http.StatusRequestEntityTooLarge, "payload too large")
				return
			}
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		switch req.Type {
		case "text", "url", "file":
		default:
			writeError(w, http.StatusBadRequest, "type must be text, url or file")
			return
		}

		if req.Data == "" {
			writeError(w, http.StatusBadRequest, "data must not be empty")
			return
		}

		var data []byte
		switch req.Type {
		case "file":
			decoded, err := base64.StdEncoding.DecodeString(req.Data)
			if err != nil {
				writeError(w, http.StatusBadRequest, "data is not valid base64")
				return
			}
			if int64(len(decoded)) > maxPayloadBytes {
				writeError(w, http.StatusRequestEntityTooLarge, "file exceeds size limit")
				return
			}
			data = decoded
		default:
			data = []byte(req.Data)
			if int64(len(data)) > maxPayloadBytes {
				writeError(w, http.StatusRequestEntityTooLarge, "payload too large")
				return
			}
		}

		code, err := s.Put(req.Type, data, req.Filename)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to store entry")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(uploadResponse{
			Code:      code,
			ExpiresAt: time.Now().Add(5 * time.Minute),
		})
	}
}
