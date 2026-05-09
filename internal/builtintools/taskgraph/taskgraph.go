// Package taskgraph provides persistent DAG task tools (task_create / task_update / task_list / task_get)
// backed by JSON files under a configurable directory, aligned with s07-style task systems.
package taskgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dean2021/enno"
)

const (
	ToolCreate = "task_create"
	ToolUpdate = "task_update"
	ToolList   = "task_list"
	ToolGet    = "task_get"

	maxTasks = 50
)

// Config sets the workspace root and optional tasks directory; Timeout bounds each tool call.
// If TasksDir is empty, JSON files default to filepath.Join(Root, ".tasks") (library use).
// If TasksDir is a non-empty absolute path (CLI passes ~/.enno/tasks/<session_id>), that directory is the sole store.
type Config struct {
	Root     string
	TasksDir string
	Timeout  time.Duration
}

// Task is stored as JSON at TasksDir/task_<id>.json
type Task struct {
	ID          int    `json:"id"`
	Subject     string `json:"subject"`
	Description string `json:"description"`
	Status      string `json:"status"` // pending, in_progress, completed
	BlockedBy   []int  `json:"blocked_by"`
	Owner       string `json:"owner"`
}

type manager struct {
	mu       sync.Mutex
	root     string
	tasksDir string
	timeout  time.Duration
}

// New returns four tools: task_create, task_update, task_list, task_get.
func New(config Config) []enno.Tool {
	root := config.Root
	if root == "" {
		root = "."
	}
	td := config.TasksDir
	if strings.TrimSpace(td) == "" {
		td = filepath.Join(root, ".tasks")
	}
	to := config.Timeout
	if to == 0 {
		to = 120 * time.Second
	}
	m := &manager{root: root, tasksDir: td, timeout: to}

	return []enno.Tool{
		m.toolCreate(),
		m.toolUpdate(),
		m.toolList(),
		m.toolGet(),
	}
}

func (m *manager) runCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, m.timeout)
}

func (m *manager) absTasksDir() (string, error) {
	rootAbs, err := filepath.Abs(m.root)
	if err != nil {
		return "", err
	}
	td := m.tasksDir
	if filepath.IsAbs(td) {
		return filepath.Clean(td), nil
	}
	return filepath.Join(rootAbs, td), nil
}

func (m *manager) ensureDir() error {
	dir, err := m.absTasksDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(dir, 0755)
}

func taskFilePath(dir string, id int) string {
	return filepath.Join(dir, fmt.Sprintf("task_%d.json", id))
}

func (m *manager) loadID(dir string, id int) (Task, error) {
	data, err := os.ReadFile(taskFilePath(dir, id))
	if err != nil {
		return Task{}, err
	}
	var t Task
	if err := json.Unmarshal(data, &t); err != nil {
		return Task{}, err
	}
	return t, nil
}

func (m *manager) save(dir string, t Task) error {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(taskFilePath(dir, t.ID), data, 0644)
}

func (m *manager) nextID(dir string) (int, error) {
	tasks, err := m.loadAllFromDir(dir)
	if err != nil {
		return 0, err
	}
	max := 0
	for _, t := range tasks {
		if t.ID > max {
			max = t.ID
		}
	}
	return max + 1, nil
}

func (m *manager) loadAllFromDir(dir string) ([]Task, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Task
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "task_") || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var t Task
		if err := json.Unmarshal(data, &t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

func normalizeStatus(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func validateStatus(s string) bool {
	switch s {
	case "pending", "in_progress", "completed":
		return true
	default:
		return false
	}
}

// --- cycle detection: edge prereq -> task if task.blocked_by contains prereq ---
func buildDependents(tasks []Task) map[int][]int {
	dep := make(map[int][]int)
	for _, t := range tasks {
		for _, p := range t.BlockedBy {
			dep[p] = append(dep[p], t.ID)
		}
	}
	return dep
}

func reachesFrom(dependents map[int][]int, start, target int, seen map[int]bool) bool {
	if start == target {
		return true
	}
	if seen[start] {
		return false
	}
	seen[start] = true
	for _, next := range dependents[start] {
		if reachesFrom(dependents, next, target, seen) {
			return true
		}
	}
	return false
}

func (m *manager) wouldCreateCycle(tasks []Task, taskID int, newBlocked []int) bool {
	// simulate task taskID with blocked_by = newBlocked merged
	var sim []Task
	for _, t := range tasks {
		cp := t
		if t.ID == taskID {
			cp.BlockedBy = append([]int(nil), newBlocked...)
		}
		sim = append(sim, cp)
	}
	dep := buildDependents(sim)
	// For each new prereq p added to taskID: cycle if path taskID -> ... -> p exists
	for _, p := range newBlocked {
		seen := make(map[int]bool)
		if reachesFrom(dep, taskID, p, seen) {
			return true
		}
	}
	return false
}

func (m *manager) clearDependency(dir string, completedID int) error {
	tasks, err := m.loadAllFromDir(dir)
	if err != nil {
		return err
	}
	for _, t := range tasks {
		if t.ID == completedID {
			continue
		}
		var nb []int
		for _, b := range t.BlockedBy {
			if b != completedID {
				nb = append(nb, b)
			}
		}
		if len(nb) != len(t.BlockedBy) {
			t.BlockedBy = nb
			if err := m.save(dir, t); err != nil {
				return err
			}
		}
	}
	return nil
}

type createArgs struct {
	Subject     string `json:"subject"`
	Description string `json:"description"`
	BlockedBy   []int  `json:"blocked_by"`
	Owner       string `json:"owner"`
}

func (m *manager) toolCreate() enno.Tool {
	desc := `Create a persisted task in the configured task store (per-task JSON files). Optional blocked_by lists task IDs that must complete before this task becomes runnable.

Use task_create, task_update, task_list, and task_get to plan and track multi-step work. Status starts as pending. Use blocked_by for dependencies.`
	return enno.NewTypedTool(ToolCreate, desc, map[string]any{
		"subject":     map[string]any{"type": "string", "description": "Short title for the task."},
		"description": map[string]any{"type": "string"},
		"blocked_by":  map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
		"owner":       map[string]any{"type": "string"},
	}, []string{"subject"}, func(ctx context.Context, a createArgs) (string, error) {
		ctx, cancel := m.runCtx(ctx)
		defer cancel()
		return m.create(ctx, a)
	})
}

func (m *manager) create(ctx context.Context, a createArgs) (string, error) {
	if strings.TrimSpace(a.Subject) == "" {
		return "", fmt.Errorf("subject is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.ensureDir(); err != nil {
		return "", err
	}
	dir, err := m.absTasksDir()
	if err != nil {
		return "", err
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	tasks, err := m.loadAllFromDir(dir)
	if err != nil {
		return "", err
	}
	if len(tasks) >= maxTasks {
		return "", fmt.Errorf("max %d tasks reached", maxTasks)
	}
	ids := map[int]bool{}
	for _, t := range tasks {
		ids[t.ID] = true
	}
	for _, b := range a.BlockedBy {
		if !ids[b] {
			return "", fmt.Errorf("blocked_by references unknown task id %d", b)
		}
	}
	id, err := m.nextID(dir)
	if err != nil {
		return "", err
	}
	t := Task{
		ID:          id,
		Subject:     strings.TrimSpace(a.Subject),
		Description: strings.TrimSpace(a.Description),
		Status:      "pending",
		BlockedBy:   append([]int(nil), a.BlockedBy...),
		Owner:       strings.TrimSpace(a.Owner),
	}
	if m.wouldCreateCycle(append(tasks, t), t.ID, t.BlockedBy) {
		return "", fmt.Errorf("blocked_by would create a dependency cycle")
	}
	if err := m.save(dir, t); err != nil {
		return "", err
	}
	return fmt.Sprintf("Created task %d: %s", t.ID, t.Subject), nil
}

type updateArgs struct {
	TaskID          int    `json:"task_id"`
	Status          string `json:"status"`
	AddBlockedBy    []int  `json:"add_blocked_by"`
	RemoveBlockedBy []int  `json:"remove_blocked_by"`
}

func (m *manager) toolUpdate() enno.Tool {
	desc := `Update a persisted task: status (pending, in_progress, completed), or edit blocked_by edges. Use this to keep the task graph current as work starts, completes, or dependencies change. Completing a task removes its id from other tasks' blocked_by (unblocks dependents).`
	return enno.NewTypedTool(ToolUpdate, desc, map[string]any{
		"task_id":           map[string]any{"type": "integer"},
		"status":            map[string]any{"type": "string", "enum": []string{"pending", "in_progress", "completed"}},
		"add_blocked_by":    map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
		"remove_blocked_by": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
	}, []string{"task_id"}, func(ctx context.Context, a updateArgs) (string, error) {
		ctx, cancel := m.runCtx(ctx)
		defer cancel()
		return m.update(ctx, a)
	})
}

func (m *manager) update(ctx context.Context, a updateArgs) (string, error) {
	if a.TaskID <= 0 {
		return "", fmt.Errorf("invalid task_id")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.ensureDir(); err != nil {
		return "", err
	}
	dir, err := m.absTasksDir()
	if err != nil {
		return "", err
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	t, err := m.loadID(dir, a.TaskID)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("unknown task_id %d", a.TaskID)
		}
		return "", err
	}
	tasks, err := m.loadAllFromDir(dir)
	if err != nil {
		return "", err
	}
	ids := map[int]bool{}
	for _, x := range tasks {
		ids[x.ID] = true
	}
	if st := strings.TrimSpace(a.Status); st != "" {
		st = normalizeStatus(st)
		if !validateStatus(st) {
			return "", fmt.Errorf("invalid status %q", a.Status)
		}
		old := normalizeStatus(t.Status)
		t.Status = st
		if old != "completed" && st == "completed" {
			if err := m.save(dir, t); err != nil {
				return "", err
			}
			if err := m.clearDependency(dir, t.ID); err != nil {
				return "", err
			}
			t, err = m.loadID(dir, a.TaskID)
			if err != nil {
				return "", err
			}
			tasks, err = m.loadAllFromDir(dir)
			if err != nil {
				return "", err
			}
		}
	}
	bset := make(map[int]struct{})
	for _, b := range t.BlockedBy {
		if b == t.ID {
			return "", fmt.Errorf("invalid blocked_by: self-reference")
		}
		bset[b] = struct{}{}
	}
	for _, r := range a.RemoveBlockedBy {
		delete(bset, r)
	}
	for _, add := range a.AddBlockedBy {
		if add == t.ID {
			return "", fmt.Errorf("add_blocked_by: cannot depend on self")
		}
		if !ids[add] {
			return "", fmt.Errorf("add_blocked_by: unknown task id %d", add)
		}
		bset[add] = struct{}{}
	}
	var newBlocked []int
	for b := range bset {
		newBlocked = append(newBlocked, b)
	}
	sort.Ints(newBlocked)
	simTasks := make([]Task, 0, len(tasks))
	for _, x := range tasks {
		if x.ID == t.ID {
			cp := t
			cp.BlockedBy = append([]int(nil), newBlocked...)
			simTasks = append(simTasks, cp)
		} else {
			simTasks = append(simTasks, x)
		}
	}
	if m.wouldCreateCycle(simTasks, t.ID, newBlocked) {
		return "", fmt.Errorf("blocked_by would create a dependency cycle")
	}
	t.BlockedBy = newBlocked
	if err := m.save(dir, t); err != nil {
		return "", err
	}
	return fmt.Sprintf("Updated task %d.", t.ID), nil
}

func (m *manager) toolList() enno.Tool {
	desc := `List all tasks in the persistent task graph, grouped into runnable (pending, no blockers), blocked (waiting on dependencies), in progress, and completed. Use this to inspect current plan state before choosing next work.`
	return enno.NewTypedTool(ToolList, desc, map[string]any{}, []string{}, func(ctx context.Context, _ struct{}) (string, error) {
		ctx, cancel := m.runCtx(ctx)
		defer cancel()
		return m.list(ctx)
	})
}

func (m *manager) list(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.ensureDir(); err != nil {
		return "", err
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	dir, err := m.absTasksDir()
	if err != nil {
		return "", err
	}
	tasks, err := m.loadAllFromDir(dir)
	if err != nil {
		return "", err
	}
	if len(tasks) == 0 {
		return "No tasks.", nil
	}
	var runnable, blocked, inprog, done []Task
	for _, t := range tasks {
		switch normalizeStatus(t.Status) {
		case "completed":
			done = append(done, t)
		case "in_progress":
			inprog = append(inprog, t)
		default:
			if len(t.BlockedBy) == 0 {
				runnable = append(runnable, t)
			} else {
				blocked = append(blocked, t)
			}
		}
	}
	var b strings.Builder
	b.WriteString("Task graph:\n\n")
	writeSec := func(title string, ts []Task) {
		b.WriteString(title)
		b.WriteString("\n")
		if len(ts) == 0 {
			b.WriteString("  (none)\n")
			return
		}
		for _, t := range ts {
			line := fmt.Sprintf("  #%d [%s] %s", t.ID, t.Status, t.Subject)
			if len(t.BlockedBy) > 0 {
				line += fmt.Sprintf(" blocked_by=%v", t.BlockedBy)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	writeSec("Runnable (pending, unblocked):", runnable)
	writeSec("Blocked (pending, waiting on dependencies):", blocked)
	writeSec("In progress:", inprog)
	writeSec("Completed:", done)
	return b.String(), nil
}

type getArgs struct {
	TaskID int `json:"task_id"`
}

func (m *manager) toolGet() enno.Tool {
	return enno.NewTypedTool(ToolGet, `Get one persisted task by id as JSON, including status and dependency metadata.`, map[string]any{
		"task_id": map[string]any{"type": "integer"},
	}, []string{"task_id"}, func(ctx context.Context, a getArgs) (string, error) {
		ctx, cancel := m.runCtx(ctx)
		defer cancel()
		return m.get(ctx, a)
	})
}

func (m *manager) get(ctx context.Context, a getArgs) (string, error) {
	if a.TaskID <= 0 {
		return "", fmt.Errorf("invalid task_id")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.ensureDir(); err != nil {
		return "", err
	}
	dir, err := m.absTasksDir()
	if err != nil {
		return "", err
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	t, err := m.loadID(dir, a.TaskID)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("unknown task_id %d", a.TaskID)
		}
		return "", err
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ReminderToolNames lists tools that count as "planning" for agent reminders.
var ReminderToolNames = []string{ToolCreate, ToolUpdate, ToolList, ToolGet}

// IsReminderTool reports whether name is a task graph tool used for plan reminders.
func IsReminderTool(name string) bool {
	switch name {
	case ToolCreate, ToolUpdate, ToolList, ToolGet:
		return true
	default:
		return false
	}
}
