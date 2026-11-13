package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBoundedFileLimitsRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(path, []byte("12345"), 0600); err != nil {
		t.Fatal(err)
	}
	if data, err := readBoundedFile(path, 5); err != nil || string(data) != "12345" {
		t.Fatalf("exact limit: %q %v", data, err)
	}
	if data, err := readBoundedFile(path, 4); err == nil || data != nil {
		t.Fatalf("oversize file accepted: %q %v", data, err)
	}
}
