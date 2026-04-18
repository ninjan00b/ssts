package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultServer = "http://localhost:8080"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New(usage())
	}

	server := serverURL()

	// Global --server flag
	if len(args) >= 2 && args[0] == "--server" {
		server = args[1]
		args = args[2:]
	}

	if len(args) == 0 {
		return errors.New(usage())
	}

	switch args[0] {
	case "send":
		return cmdSend(server, args[1:])
	case "recv":
		return cmdRecv(server, args[1:])
	default:
		return fmt.Errorf("unknown command %q\n%s", args[0], usage())
	}
}

func cmdSend(server string, args []string) error {
	typ := "text"
	var data string
	var filename string

	i := 0
	for i < len(args) {
		switch args[i] {
		case "--type", "-t":
			if i+1 >= len(args) {
				return errors.New("--type requires a value")
			}
			typ = args[i+1]
			i += 2
		default:
			data = args[i]
			i++
		}
	}

	if typ != "text" && typ != "url" && typ != "file" {
		return fmt.Errorf("invalid type %q: must be text, url or file", typ)
	}

	var encodedData string

	switch typ {
	case "file":
		if data == "" {
			return errors.New("provide a file path as argument")
		}
		raw, err := os.ReadFile(data)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
		if len(raw) > 1048576 {
			return errors.New("file exceeds 1 MB limit")
		}
		encodedData = base64.StdEncoding.EncodeToString(raw)
		filename = filepath.Base(data)
	case "text":
		if data == "" {
			// Read from stdin
			raw, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
			data = strings.TrimRight(string(raw), "\n")
		}
		if data == "" {
			return errors.New("data cannot be empty")
		}
		encodedData = data
	default:
		if data == "" {
			return errors.New("provide data as argument")
		}
		encodedData = data
	}

	body := map[string]string{
		"type":     typ,
		"data":     encodedData,
		"filename": filename,
	}

	raw, _ := json.Marshal(body)
	resp, err := http.Post(server+"/upload", "application/json", bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var e struct{ Error string }
		json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("server error %d: %s", resp.StatusCode, e.Error)
	}

	var result struct {
		Code      string    `json:"code"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	fmt.Println()
	fmt.Println("  Transfer code:")
	fmt.Println()
	fmt.Printf("      %s\n", result.Code)
	fmt.Println()
	fmt.Printf("  Expires at: %s\n", result.ExpiresAt.Local().Format("15:04:05"))
	fmt.Println()

	return nil
}

func cmdRecv(server string, args []string) error {
	if len(args) == 0 {
		return errors.New("provide a 6-character code")
	}
	code := strings.ToUpper(strings.TrimSpace(args[0]))
	if len(code) != 6 {
		return errors.New("code must be exactly 6 characters")
	}

	resp, err := http.Get(server + "/fetch/" + code)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return errors.New("code not found, already used, or expired")
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return errors.New("rate limit exceeded — try again in a minute")
	}
	if resp.StatusCode != http.StatusOK {
		var e struct{ Error string }
		json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("server error %d: %s", resp.StatusCode, e.Error)
	}

	var result struct {
		Type     string `json:"type"`
		Data     string `json:"data"`
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	switch result.Type {
	case "text", "url":
		fmt.Println(result.Data)
	case "file":
		raw, err := base64.StdEncoding.DecodeString(result.Data)
		if err != nil {
			return fmt.Errorf("decode file data: %w", err)
		}
		name := result.Filename
		if name == "" {
			name = "download"
		}
		// Avoid path traversal
		name = filepath.Base(name)
		if err := os.WriteFile(name, raw, 0644); err != nil {
			return fmt.Errorf("write file: %w", err)
		}
		fmt.Printf("Saved: %s (%d bytes)\n", name, len(raw))
	default:
		fmt.Println(result.Data)
	}

	return nil
}

func serverURL() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return defaultServer
	}
	cfgPath := filepath.Join(home, ".ssts", "config")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return defaultServer
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "server_url") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				return strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			}
		}
	}
	return defaultServer
}

func usage() string {
	return `Usage:
  ssts-cli [--server URL] send [--type text|url|file] <data|path>
  ssts-cli [--server URL] recv <CODE>

Examples:
  ssts-cli send "my secret text"
  ssts-cli send --type url "https://example.com"
  ssts-cli send --type file ./photo.jpg
  ssts-cli recv A3K9PZ
  ssts-cli --server http://192.168.1.42:8080 send "hello"`
}
