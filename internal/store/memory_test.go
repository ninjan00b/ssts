package store_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"ssts/internal/store"
)

const testAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func newStore(ttl time.Duration) *store.Store {
	return store.New(ttl)
}

func TestPut_GeneratesValidCode(t *testing.T) {
	s := newStore(5 * time.Minute)

	code, err := s.Put("text", []byte("hello"), "")
	if err != nil {
		t.Fatalf("Put returned error: %v", err)
	}

	if len(code) != 6 {
		t.Errorf("code length = %d, want 6", len(code))
	}

	for _, ch := range code {
		if !strings.ContainsRune(testAlphabet, ch) {
			t.Errorf("code %q contains invalid character %q", code, ch)
		}
	}
}

func TestPut_CodesAreUnique(t *testing.T) {
	s := newStore(5 * time.Minute)
	seen := make(map[string]bool)

	for range 50 {
		code, err := s.Put("text", []byte("x"), "")
		if err != nil {
			t.Fatalf("Put: %v", err)
		}
		if seen[code] {
			t.Errorf("duplicate code generated: %s", code)
		}
		seen[code] = true
	}
}

func TestPopIfExists_RoundTrip(t *testing.T) {
	s := newStore(5 * time.Minute)

	code, err := s.Put("text", []byte("hello world"), "")
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	entry, ok := s.PopIfExists(code)
	if !ok {
		t.Fatal("PopIfExists returned false for existing code")
	}
	if entry.Type != "text" {
		t.Errorf("Type = %q, want %q", entry.Type, "text")
	}
	if string(entry.Data) != "hello world" {
		t.Errorf("Data = %q, want %q", entry.Data, "hello world")
	}
}

func TestPopIfExists_FileRoundTrip(t *testing.T) {
	s := newStore(5 * time.Minute)

	fileData := []byte{0x89, 0x50, 0x4E, 0x47} // PNG magic bytes
	code, err := s.Put("file", fileData, "photo.png")
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	entry, ok := s.PopIfExists(code)
	if !ok {
		t.Fatal("PopIfExists returned false")
	}
	if entry.Filename != "photo.png" {
		t.Errorf("Filename = %q, want %q", entry.Filename, "photo.png")
	}
	if string(entry.Data) != string(fileData) {
		t.Error("Data mismatch")
	}
}

func TestPopIfExists_MissingCode(t *testing.T) {
	s := newStore(5 * time.Minute)

	_, ok := s.PopIfExists("AAAAAA")
	if ok {
		t.Error("PopIfExists returned true for non-existent code")
	}
}

func TestPopIfExists_OneTimeRead(t *testing.T) {
	s := newStore(5 * time.Minute)

	code, _ := s.Put("url", []byte("https://example.com"), "")

	_, first := s.PopIfExists(code)
	_, second := s.PopIfExists(code)

	if !first {
		t.Error("first Pop returned false, want true")
	}
	if second {
		t.Error("second Pop returned true, want false (one-time read)")
	}
}

func TestPopIfExists_ExpiredEntry(t *testing.T) {
	s := newStore(-1 * time.Millisecond) // already expired on creation

	code, err := s.Put("text", []byte("secret"), "")
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	_, ok := s.PopIfExists(code)
	if ok {
		t.Error("PopIfExists returned true for expired entry")
	}
}

func TestCleanup_RemovesExpiredEntries(t *testing.T) {
	s := newStore(-1 * time.Millisecond)

	codes := make([]string, 5)
	for i := range codes {
		code, err := s.Put("text", []byte("x"), "")
		if err != nil {
			t.Fatalf("Put: %v", err)
		}
		codes[i] = code
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.StartCleanup(ctx, 10*time.Millisecond)

	time.Sleep(50 * time.Millisecond)

	for _, code := range codes {
		_, ok := s.PopIfExists(code)
		if ok {
			t.Errorf("code %s still exists after cleanup", code)
		}
	}
}

func TestCleanup_StopsOnContextCancel(t *testing.T) {
	s := newStore(5 * time.Minute)
	ctx, cancel := context.WithCancel(context.Background())

	s.StartCleanup(ctx, 10*time.Millisecond)
	cancel() // should not panic or block

	time.Sleep(30 * time.Millisecond)
}

func TestConcurrent_PutAndPop(t *testing.T) {
	s := newStore(5 * time.Minute)

	var wg sync.WaitGroup
	codes := make(chan string, 100)

	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, err := s.Put("text", []byte("concurrent"), "")
			if err == nil {
				codes <- code
			}
		}()
	}

	wg.Wait()
	close(codes)

	for code := range codes {
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			s.PopIfExists(c)
		}(code)
	}

	wg.Wait()
}
