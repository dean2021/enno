package fetchurl

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchURLConvertsHTMLToMarkdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); !strings.Contains(got, "Enno") {
			t.Fatalf("User-Agent = %q", got)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><style>.x{}</style></head><body><h1>Title</h1><p>Hello <a href="/docs">docs</a>.</p><script>x()</script></body></html>`))
	}))
	defer server.Close()

	tool := New(Config{Timeout: time.Second})
	out, err := tool.Handler(context.Background(), mustArgs(t, map[string]any{"url": server.URL}))
	if err != nil {
		t.Fatalf("handler: %v", err)
	}

	var got result
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal output: %v\n%s", err, out)
	}
	if got.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", got.StatusCode)
	}
	if !strings.Contains(got.MarkdownContent, "# Title") {
		t.Fatalf("markdown missing heading: %q", got.MarkdownContent)
	}
	if !strings.Contains(got.MarkdownContent, "[docs]("+server.URL+"/docs)") {
		t.Fatalf("markdown missing link: %q", got.MarkdownContent)
	}
	if strings.Contains(got.MarkdownContent, "x()") {
		t.Fatalf("markdown should skip script content: %q", got.MarkdownContent)
	}
	if got.ContentLength <= 0 {
		t.Fatalf("content_length = %d", got.ContentLength)
	}
}

func TestFetchURLRejectsNonHTTPURL(t *testing.T) {
	tool := New(Config{})
	_, err := tool.Handler(context.Background(), mustArgs(t, map[string]any{"url": "file:///etc/passwd"}))
	if err == nil || !strings.Contains(err.Error(), "http or https") {
		t.Fatalf("expected scheme error, got %v", err)
	}
}

func TestFetchURLTruncatesOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(strings.Repeat("a", 200)))
	}))
	defer server.Close()

	tool := New(Config{MaxOutputChars: 40})
	out, err := tool.Handler(context.Background(), mustArgs(t, map[string]any{"url": server.URL}))
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	var got result
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if got.ContentLength > 40 {
		t.Fatalf("content_length = %d, want <= 40", got.ContentLength)
	}
	if !strings.Contains(got.MarkdownContent, "[truncated]") {
		t.Fatalf("expected truncation marker, got %q", got.MarkdownContent)
	}
}

func mustArgs(t *testing.T, value map[string]any) json.RawMessage {
	t.Helper()
	bytes, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return bytes
}
