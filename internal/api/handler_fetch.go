package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"regexp"

	"ssts/internal/store"
)

type fetchResponse struct {
	Type     string `json:"type"`
	Data     string `json:"data"`
	Filename string `json:"filename"`
}

var validCode = regexp.MustCompile(`^[ABCDEFGHJKLMNPQRSTUVWXYZ23456789]{6}$`)

func FetchHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")
		if !validCode.MatchString(code) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}

		entry, ok := s.PopIfExists(code)
		if !ok {
			writeError(w, http.StatusNotFound, "not found")
			return
		}

		var dataStr string
		if entry.Type == "file" {
			dataStr = base64.StdEncoding.EncodeToString(entry.Data)
		} else {
			dataStr = string(entry.Data)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fetchResponse{
			Type:     entry.Type,
			Data:     dataStr,
			Filename: entry.Filename,
		})
	}
}
