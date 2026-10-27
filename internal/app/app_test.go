package app

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryan/mkdone/internal/model"
)

func writeMarkdown(t *testing.T, dir, name, contents string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestRunBasicCSVWithFlagsAfterPath(t *testing.T) {
	dir := t.TempDir()
	writeMarkdown(t, dir, "tasks.md", ""+
		"- [ ] Alpha #work 📅 2026-01-15\n"+
		"- [ ] Beta #personal 📅 2026-01-20\n")

	code, stdout, stderr := run(t, dir, "--tags", "#work")
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr)
	}
	if !strings.HasPrefix(stdout, "status,taskcontent,priority,tags,created_at,due_at") {
		t.Errorf("unexpected CSV header: %s", stdout)
	}
	if !strings.Contains(stdout, "Alpha") {
		t.Errorf("stdout is missing Alpha: %s", stdout)
	}
	if strings.Contains(stdout, "Beta") {
		t.Errorf("stdout unexpectedly contains Beta: %s", stdout)
	}
}

func TestRunDateFieldOverride(t *testing.T) {
	dir := t.TempDir()
	writeMarkdown(t, dir, "tasks.md", ""+
		"- [ ] Due outside ⏳ 2026-01-15 📅 2027-01-01\n"+
		"- [ ] Due inside ⏳ 2027-01-15 📅 2026-01-15\n")

	code, stdout, stderr := run(t, dir,
		"--after", "2026-01-01", "--before", "2026-12-31", "--scheduled")
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr)
	}
	if !strings.Contains(stdout, "Due outside") {
		t.Errorf("stdout missing scheduled match: %s", stdout)
	}
	if strings.Contains(stdout, "Due inside") {
		t.Errorf("stdout contains due-only match: %s", stdout)
	}
}

func TestRunWritesJSONFile(t *testing.T) {
	dir := t.TempDir()
	writeMarkdown(t, dir, "tasks.md", "- [ ] Alpha #work\n")
	out := filepath.Join(dir, "out.json")

	code, stdout, stderr := run(t, dir, "--out", out)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var tasks []model.Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, data)
	}
	if len(tasks) != 1 || tasks[0].Content != "Alpha" {
		t.Errorf("tasks = %#v", tasks)
	}
}

func TestRunSearchNotes(t *testing.T) {
	dir := t.TempDir()
	writeMarkdown(t, dir, "tasks.md", ""+
		"- [ ] Alpha\n"+
		"      - hidden detail about the mouse jiggler\n"+
		"- [ ] Beta\n")

	code, stdout, stderr := run(t, dir, "--search", "mouse jiggler")
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr)
	}
	if !strings.Contains(stdout, "Alpha") || strings.Contains(stdout, "Beta") {
		t.Errorf("unexpected search output: %s", stdout)
	}
}

func TestRunHelpAndVersion(t *testing.T) {
	code, stdout, stderr := run(t, "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("help: code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "Usage:") || !strings.Contains(stdout, "--hidetags") {
		t.Errorf("help output = %s", stdout)
	}

	code, stdout, stderr = run(t, "--version")
	if code != 0 || stderr != "" {
		t.Fatalf("version: code=%d stderr=%s", code, stderr)
	}
	if !strings.HasPrefix(stdout, "mkdone ") {
		t.Errorf("version output = %q", stdout)
	}
}

func TestRunArgumentErrors(t *testing.T) {
	tests := [][]string{
		{"--format", "xml"},
		{"--scheduled", "--completed"},
		{"--after", "nope"},
		{"--unknown"},
		{"a", "b"},
		{"--workers", "-1"},
	}
	for _, args := range tests {
		code, _, stderr := run(t, args...)
		if code != 2 {
			t.Errorf("Run(%#v) exit code = %d, want 2", args, code)
		}
		if !strings.Contains(stderr, "mkdone: error:") {
			t.Errorf("Run(%#v) stderr = %q", args, stderr)
		}
	}
}
