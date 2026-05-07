package todo

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/dean2021/enno"
)

type Item struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Status string `json:"status"`
}

type args struct {
	Items []Item `json:"items"`
}

type Manager struct {
	mu    sync.Mutex
	items []Item
}

func New() enno.Tool {
	manager := &Manager{}
	return enno.NewTypedTool("todo", "Update task list. Track progress on multi-step tasks.", map[string]any{
		"items": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":     map[string]any{"type": "string"},
					"text":   map[string]any{"type": "string"},
					"status": map[string]any{"type": "string", "enum": []string{"pending", "in_progress", "completed"}},
				},
				"required": []string{"id", "text", "status"},
			},
		},
	}, []string{"items"}, func(ctx context.Context, input args) (string, error) {
		return manager.Update(input.Items)
	})
}

func (m *Manager) Update(items []Item) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(items) > 20 {
		return "", fmt.Errorf("max 20 todos allowed")
	}

	validated := make([]Item, 0, len(items))
	inProgressCount := 0
	for i, item := range items {
		item.ID = strings.TrimSpace(item.ID)
		item.Text = strings.TrimSpace(item.Text)
		item.Status = strings.ToLower(strings.TrimSpace(item.Status))
		if item.ID == "" {
			item.ID = fmt.Sprintf("%d", i+1)
		}
		if item.Text == "" {
			return "", fmt.Errorf("item %s: text required", item.ID)
		}
		switch item.Status {
		case "pending", "in_progress", "completed":
		default:
			return "", fmt.Errorf("item %s: invalid status %q", item.ID, item.Status)
		}
		if item.Status == "in_progress" {
			inProgressCount++
		}
		validated = append(validated, item)
	}
	if inProgressCount > 1 {
		return "", fmt.Errorf("only one task can be in_progress at a time")
	}

	m.items = validated
	return m.Render(), nil
}

func (m *Manager) Render() string {
	if len(m.items) == 0 {
		return "No todos."
	}

	var lines []string
	completed := 0
	for _, item := range m.items {
		marker := map[string]string{
			"pending":     "[ ]",
			"in_progress": "[>]",
			"completed":   "[x]",
		}[item.Status]
		if item.Status == "completed" {
			completed++
		}
		lines = append(lines, fmt.Sprintf("%s #%s: %s", marker, item.ID, item.Text))
	}
	lines = append(lines, fmt.Sprintf("\n(%d/%d completed)", completed, len(m.items)))
	return strings.Join(lines, "\n")
}
