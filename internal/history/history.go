package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Entry represents a single user input recorded in the history file.
type Entry struct {
	Display   string `json:"display"`
	Timestamp int64  `json:"timestamp"`
	Project   string `json:"project,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
}

// Recorder appends user inputs to a JSONL history file.
type Recorder struct {
	mu      sync.Mutex
	file    *os.File
	encoder *json.Encoder
	project string
	session string
}

// NewRecorder creates a Recorder that appends entries to path.
// The file and its parent directories are created if they do not exist.
func NewRecorder(path, project, session string) (*Recorder, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create history directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("open history file: %w", err)
	}
	return &Recorder{
		file:    f,
		encoder: json.NewEncoder(f),
		project: project,
		session: session,
	}, nil
}

// Record appends a single user input to the history file.
func (r *Recorder) Record(display string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry := Entry{
		Display:   display,
		Timestamp: time.Now().UnixMilli(),
		Project:   r.project,
		SessionID: r.session,
	}
	if err := r.encoder.Encode(entry); err != nil {
		return fmt.Errorf("write history entry: %w", err)
	}
	return r.file.Sync()
}

// Close flushes and closes the underlying file.
func (r *Recorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.file.Close()
}

// Path returns the file path the recorder writes to.
func (r *Recorder) Path() string {
	return r.file.Name()
}

// LoadRecent reads the last n entries from a JSONL history file.
// It returns entries in chronological order (oldest first).
// If the file does not exist, it returns nil with no error.
func LoadRecent(path string, n int) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open history file: %w", err)
	}
	defer f.Close()

	var all []Entry
	decoder := json.NewDecoder(f)
	for decoder.More() {
		var entry Entry
		if err := decoder.Decode(&entry); err != nil {
			// Skip malformed lines and continue.
			continue
		}
		all = append(all, entry)
	}
	if n <= 0 || len(all) <= n {
		return all, nil
	}
	return all[len(all)-n:], nil
}

// DefaultPath returns the default history file path: ~/.enno/history.jsonl
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, ".enno", "history.jsonl"), nil
}
