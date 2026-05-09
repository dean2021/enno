package taskgraph

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestTaskCreateListGet(t *testing.T) {
	root := t.TempDir()
	tools := New(Config{Root: root, Timeout: 5 * time.Second})
	ctx := context.Background()
	out, err := tools[0].Handler(ctx, jsonRaw(`{"subject":"First"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Created task 1") {
		t.Fatalf("create: %q", out)
	}
	out, err = tools[3].Handler(ctx, jsonRaw(`{"task_id":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"id": 1`) || !strings.Contains(out, "First") {
		t.Fatalf("get: %q", out)
	}
	out, err = tools[2].Handler(ctx, jsonRaw(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Runnable") || !strings.Contains(out, "#1") {
		t.Fatalf("list: %q", out)
	}
}

func TestTaskDependencyUnlock(t *testing.T) {
	root := t.TempDir()
	tg := New(Config{Root: root, Timeout: 5 * time.Second})
	ctx := context.Background()
	_, _ = tg[0].Handler(ctx, jsonRaw(`{"subject":"A"}`))
	_, _ = tg[0].Handler(ctx, jsonRaw(`{"subject":"B","blocked_by":[1]}`))
	out, err := tg[1].Handler(ctx, jsonRaw(`{"task_id":1,"status":"completed"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Updated") {
		t.Fatalf("update: %q", out)
	}
	out, err = tg[3].Handler(ctx, jsonRaw(`{"task_id":2}`))
	if err != nil {
		t.Fatal(err)
	}
	var tk Task
	if err := json.Unmarshal([]byte(out), &tk); err != nil {
		t.Fatal(err)
	}
	if len(tk.BlockedBy) != 0 {
		t.Fatalf("expected unblock, blocked_by=%v", tk.BlockedBy)
	}
}

func TestTaskCycleRejected(t *testing.T) {
	root := t.TempDir()
	tg := New(Config{Root: root, Timeout: 5 * time.Second})
	ctx := context.Background()
	_, _ = tg[0].Handler(ctx, jsonRaw(`{"subject":"A"}`))
	_, _ = tg[0].Handler(ctx, jsonRaw(`{"subject":"B","blocked_by":[1]}`))
	_, err := tg[1].Handler(ctx, jsonRaw(`{"task_id":1,"add_blocked_by":[2]}`))
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle error, got %v", err)
	}
}

func jsonRaw(s string) json.RawMessage {
	return json.RawMessage(s)
}
