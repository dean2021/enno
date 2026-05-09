package fetchurl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/dean2021/enno"
	"github.com/dean2021/enno/builtintools/internal/toolutil"
)

const ToolName = "fetch_url"

const (
	defaultTimeout   = 30 * time.Second
	defaultUserAgent = "Mozilla/5.0 (compatible; Enno/1.0)"
)

type Config struct {
	Timeout        time.Duration
	MaxOutputChars int
	UserAgent      string
}

type tool struct {
	timeout        time.Duration
	maxOutputChars int
	userAgent      string
	client         *http.Client
}

type args struct {
	URL     string `json:"url"`
	Timeout int    `json:"timeout"`
}

type result struct {
	URL             string `json:"url"`
	MarkdownContent string `json:"markdown_content"`
	StatusCode      int    `json:"status_code"`
	ContentLength   int    `json:"content_length"`
}

const toolDescription = `Fetch content from an HTTP/HTTPS URL and convert HTML to readable markdown text.

Use this tool to read a specific page when the user gives a URL or asks for webpage content. After receiving the markdown, synthesize the relevant information naturally instead of dumping raw markdown unless explicitly requested.

- url: required HTTP/HTTPS URL.
- timeout: optional timeout in seconds; capped by the configured timeout.`

func New(config Config) enno.Tool {
	userAgent := strings.TrimSpace(config.UserAgent)
	if userAgent == "" {
		userAgent = defaultUserAgent
	}
	t := &tool{
		timeout:        fetchTimeout(config.Timeout),
		maxOutputChars: toolutil.MaxOutputChars(config.MaxOutputChars),
		userAgent:      userAgent,
		client:         &http.Client{},
	}

	return enno.NewTool(ToolName, toolDescription, map[string]any{
		"url": map[string]any{
			"type":        "string",
			"description": "HTTP or HTTPS URL to fetch.",
		},
		"timeout": map[string]any{
			"type":        "integer",
			"description": "Optional request timeout in seconds.",
		},
	}, []string{"url"}, t.run)
}

func fetchTimeout(value time.Duration) time.Duration {
	if value <= 0 {
		return defaultTimeout
	}
	return value
}

func (t *tool) run(ctx context.Context, raw json.RawMessage) (string, error) {
	var a args
	if err := json.Unmarshal(raw, &a); err != nil {
		return "", fmt.Errorf("invalid %s arguments: %w", ToolName, err)
	}
	out, err := t.fetch(ctx, a)
	if err != nil {
		return "", err
	}
	bytes, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (t *tool) fetch(ctx context.Context, a args) (result, error) {
	rawURL := strings.TrimSpace(a.URL)
	if rawURL == "" {
		return result{}, fmt.Errorf("missing url")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return result{}, fmt.Errorf("invalid url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return result{}, fmt.Errorf("url must use http or https")
	}
	if parsed.Host == "" {
		return result{}, fmt.Errorf("url must include a host")
	}

	timeout := t.timeout
	if a.Timeout > 0 {
		requested := time.Duration(a.Timeout) * time.Second
		if requested < timeout {
			timeout = requested
		}
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(runCtx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return result{}, err
	}
	req.Header.Set("User-Agent", t.userAgent)

	resp, err := t.client.Do(req)
	if err != nil {
		return result{}, fmt.Errorf("fetch url error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(t.maxOutputChars*4)))
	if err != nil {
		return result{}, fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result{}, fmt.Errorf("fetch url error: status %d", resp.StatusCode)
	}

	content, err := responseToMarkdown(resp.Header.Get("Content-Type"), resp.Request.URL.String(), string(body))
	if err != nil {
		return result{}, err
	}
	content = toolutil.TruncateRunes(strings.TrimSpace(content), t.maxOutputChars, toolutil.DefaultTruncationSuffix)
	if content == "" {
		content = "(empty response)"
	}

	return result{
		URL:             resp.Request.URL.String(),
		MarkdownContent: content,
		StatusCode:      resp.StatusCode,
		ContentLength:   len([]rune(content)),
	}, nil
}

func responseToMarkdown(contentType string, finalURL string, body string) (string, error) {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = ""
	}
	switch {
	case mediaType == "text/html" || strings.Contains(body, "<html") || strings.Contains(body, "<HTML"):
		markdown, err := htmltomarkdown.ConvertString(body, converter.WithDomain(finalURL))
		if err != nil {
			return "", fmt.Errorf("convert html to markdown: %w", err)
		}
		return markdown, nil
	default:
		return body, nil
	}
}
