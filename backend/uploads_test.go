package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Two uploads with the same stem (the clock's, on a fast machine) must not
// overwrite each other.
func TestSaveUploadNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	a, err := saveUpload(dir, "editor", "1-42", "x.png", strings.NewReader("first"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := saveUpload(dir, "editor", "1-42", "x.png", strings.NewReader("second"))
	if err != nil {
		t.Fatal(err)
	}
	if a == b || a != "editor/1-42.png" || b != "editor/1-42-1.png" {
		t.Fatalf("names: %q %q", a, b)
	}
	got, _ := os.ReadFile(filepath.Join(dir, filepath.FromSlash(a)))
	if string(got) != "first" {
		t.Fatalf("first file overwritten: %q", got)
	}
}
