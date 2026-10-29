package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommittedDemoIsValidRealRecording(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "demo", "sloth.cast")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("recording has no header")
	}
	var header struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(scanner.Bytes(), &header); err != nil || header.Version != 2 {
		t.Fatalf("recording header = %+v, error = %v", header, err)
	}
	var output strings.Builder
	events := 0
	for scanner.Scan() {
		var event []json.RawMessage
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil || len(event) != 3 {
			t.Fatalf("invalid cast event %q: %v", scanner.Text(), err)
		}
		var text string
		if err := json.Unmarshal(event[2], &text); err != nil {
			t.Fatal(err)
		}
		output.WriteString(text)
		events++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if events < 6 {
		t.Fatalf("recording has %d events", events)
	}
	text := output.String()
	for _, required := range []string{"sloth 0.0.1-next", "commit 2392dfb09e508c2455fad5f2fdc768b21ffcc570", "Sloth diagnosis", "Metrics above are genuine"} {
		if !strings.Contains(text, required) {
			t.Errorf("recording does not contain %q", required)
		}
	}
	if strings.ContainsRune(text, '\x1b') {
		t.Fatal("recording contains ANSI escape sequences")
	}
}
