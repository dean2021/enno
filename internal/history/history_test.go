package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRecorder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	rec, err := NewRecorder(path, "/tmp/project", "session-1")
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	if err := rec.Record("hello world"); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if err := rec.Record("second input"); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if err := rec.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open history file: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var entries []Entry
	for scanner.Scan() {
		var entry Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatalf("unmarshal line: %v", err)
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Display != "hello world" {
		t.Errorf("entry[0].Display = %q, want %q", entries[0].Display, "hello world")
	}
	if entries[0].Project != "/tmp/project" {
		t.Errorf("entry[0].Project = %q, want %q", entries[0].Project, "/tmp/project")
	}
	if entries[0].SessionID != "session-1" {
		t.Errorf("entry[0].SessionID = %q, want %q", entries[0].SessionID, "session-1")
	}
	if entries[0].Timestamp <= 0 {
		t.Errorf("entry[0].Timestamp = %d, want positive", entries[0].Timestamp)
	}
	if entries[1].Display != "second input" {
		t.Errorf("entry[1].Display = %q, want %q", entries[1].Display, "second input")
	}
}

func TestRecorderCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "dir", "history.jsonl")

	rec, err := NewRecorder(path, "", "")
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}
	defer rec.Close()

	if err := rec.Record("test"); err != nil {
		t.Fatalf("Record: %v", err)
	}
}

func TestDefaultPath(t *testing.T) {
	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	if filepath.Base(path) != "history.jsonl" {
		t.Errorf("DefaultPath() = %q, want base history.jsonl", path)
	}
}

func TestLoadRecent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	rec, err := NewRecorder(path, "", "")
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}
	for _, text := range []string{"first", "second", "third", "fourth", "fifth"} {
		if err := rec.Record(text); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}
	if err := rec.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	entries, err := LoadRecent(path, 3)
	if err != nil {
		t.Fatalf("LoadRecent: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Display != "third" {
		t.Errorf("entries[0].Display = %q, want %q", entries[0].Display, "third")
	}
	if entries[1].Display != "fourth" {
		t.Errorf("entries[1].Display = %q, want %q", entries[1].Display, "fourth")
	}
	if entries[2].Display != "fifth" {
		t.Errorf("entries[2].Display = %q, want %q", entries[2].Display, "fifth")
	}

	// Load more than available.
	all, err := LoadRecent(path, 100)
	if err != nil {
		t.Fatalf("LoadRecent: %v", err)
	}
	if len(all) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(all))
	}
}

func TestLoadRecentMissingFile(t *testing.T) {
	entries, err := LoadRecent(filepath.Join(t.TempDir(), "nosuch.jsonl"), 10)
	if err != nil {
		t.Fatalf("LoadRecent missing file: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries for missing file, got %d", len(entries))
	}
}
