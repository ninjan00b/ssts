package store

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"
)

// alphabet excludes O, 0, I, 1 to avoid visual ambiguity during manual entry.
const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

type Entry struct {
	Type      string
	Data      []byte
	Filename  string
	ExpiresAt time.Time
}

type Store struct {
	mu      sync.Mutex
	entries map[string]Entry
	ttl     time.Duration
}

func New(ttl time.Duration) *Store {
	return &Store{
		entries: make(map[string]Entry),
		ttl:     ttl,
	}
}

func (s *Store) Put(typ string, data []byte, filename string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	code, err := s.generateUniqueCode()
	if err != nil {
		return "", fmt.Errorf("generate code: %w", err)
	}

	s.entries[code] = Entry{
		Type:      typ,
		Data:      data,
		Filename:  filename,
		ExpiresAt: time.Now().Add(s.ttl),
	}
	return code, nil
}

func (s *Store) PopIfExists(code string) (Entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[code]
	if !ok {
		return Entry{}, false
	}
	delete(s.entries, code)
	if time.Now().After(entry.ExpiresAt) {
		return Entry{}, false
	}
	return entry, true
}

func (s *Store) StartCleanup(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.cleanup()
			}
		}
	}()
}

func (s *Store) cleanup() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for code, entry := range s.entries {
		if now.After(entry.ExpiresAt) {
			delete(s.entries, code)
		}
	}
}

func (s *Store) generateUniqueCode() (string, error) {
	buf := make([]byte, 6)
	for range 10 {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		code := make([]byte, 6)
		for i, b := range buf {
			code[i] = alphabet[int(b)%len(alphabet)]
		}
		c := string(code)
		if _, exists := s.entries[c]; !exists {
			return c, nil
		}
	}
	return "", fmt.Errorf("failed to generate unique code after 10 attempts")
}
