package app

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ryan/mkdone/internal/model"
	tasksummary "github.com/ryan/mkdone/internal/summary"
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

	code, stdout, stderr := run(t, dir, "--format", "csv", "--tags", "#work")
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

func TestRunPrettyDefault(t *testing.T) {
	dir := t.TempDir()
	writeMarkdown(t, dir, "tasks.md", "- [ ] Alpha #work ⏫ 📅 2026-01-15\n")

	code, stdout, stderr := run(t, dir)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr)
	}
	for _, expected := range []string{"STATUS", "PRIORITY", "CREATED", "DUE", "TAGS", "TASK", "[ ]", "Alpha", "#work", "2026-01-15"} {
		if !strings.Contains(stdout, expected) {
			t.Errorf("default output is missing %q:\n%s", expected, stdout)
		}
	}
	if strings.Contains(stdout, "status,taskcontent") {
		t.Errorf("default output unexpectedly used CSV: %s", stdout)
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
		{"--status", "blocked"},
		{"--limit", "-1"},
		{"--limit", "abc"},
		{"--limit"},
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

func TestRunSummaryTextRespectsFilters(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	today := now.Format("2006-01-02")
	tomorrow := now.AddDate(0, 0, 1).Format("2006-01-02")
	week := now.AddDate(0, 0, 5).Format("2006-01-02")
	writeMarkdown(t, dir, "tasks.md", ""+
		"- [ ] Today #work ⏫ 📅 "+today+"\n"+
		"- [ ] Tomorrow #work 📅 "+tomorrow+"\n"+
		"- [ ] Week #work 🔼 📅 "+week+"\n"+
		"- [ ] Personal #personal ⏫ 📅 "+today+"\n")

	code, stdout, stderr := run(t, dir, "-s", "--tags", "#work")
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr)
	}
	if strings.Contains(stdout, "status,taskcontent") {
		t.Errorf("summary unexpectedly contained the task CSV header: %s", stdout)
	}
	if strings.Contains(stdout, "#personal") {
		t.Errorf("summary did not respect --tags: %s", stdout)
	}
	checks := map[string]string{
		"total":    `Tasks:\s+3`,
		"tag":      `#work\s+3`,
		"today":    `Today\s+1`,
		"tomorrow": `Tomorrow\s+1`,
		"week":     `Week\s+3`,
		"30 days":  `30 days\s+3`,
	}
	for name, expression := range checks {
		if !regexp.MustCompile(expression).MatchString(stdout) {
			t.Errorf("summary is missing %s (%s):\n%s", name, expression, stdout)
		}
	}
}

func TestRunSummaryJSON(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	today := now.Format("2006-01-02")
	tomorrow := now.AddDate(0, 0, 1).Format("2006-01-02")
	writeMarkdown(t, dir, "tasks.md", ""+
		"- [ ] Today #work ⏫ 📅 "+today+"\n"+
		"- [ ] Tomorrow #work 📅 "+tomorrow+"\n")

	out := filepath.Join(dir, "summary.json")
	code, stdout, stderr := run(t, dir, "--summary", "--format", "json", "--out", out)
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
	var got tasksummary.Summary
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("invalid summary JSON: %v\n%s", err, data)
	}
	if got.Total != 2 {
		t.Errorf("total = %d, want 2", got.Total)
	}
	if got.Due.Today != 1 || got.Due.Tomorrow != 1 || got.Due.Week != 2 || got.Due.ThirtyDays != 2 {
		t.Errorf("due = %#v", got.Due)
	}
	if len(got.Tags) != 1 || got.Tags[0].Tag != "#work" || got.Tags[0].Count != 2 {
		t.Errorf("tags = %#v", got.Tags)
	}
}

func TestRunStatusFilter(t *testing.T) {
	dir := t.TempDir()
	writeMarkdown(t, dir, "tasks.md", ""+
		"- [ ] Open task\n"+
		"- [x] Done task\n"+
		"- [-] Cancelled task\n")

	tests := []struct {
		args      []string
		want      []string
		avoid     []string
		summary   string
		summaryNo string
	}{
		{
			args:      nil,
			want:      []string{"Open task"},
			avoid:     []string{"Done task", "Cancelled task"},
			summary:   `Tasks:\s+1`,
			summaryNo: `Tasks:\s+3`,
		},
		{
			args:    []string{"--status", "done"},
			want:    []string{"Done task"},
			avoid:   []string{"Open task", "Cancelled task"},
			summary: `Tasks:\s+1`,
		},
		{
			args:    []string{"--status", "any"},
			want:    []string{"Open task", "Done task", "Cancelled task"},
			summary: `Tasks:\s+3`,
		},
	}
	for _, test := range tests {
		args := append([]string{dir}, test.args...)
		code, stdout, stderr := run(t, args...)
		if code != 0 {
			t.Fatalf("Run(%#v) exit code = %d, stderr = %s", args, code, stderr)
		}
		for _, want := range test.want {
			if !strings.Contains(stdout, want) {
				t.Errorf("Run(%#v) output is missing %q: %s", args, want, stdout)
			}
		}
		for _, avoid := range test.avoid {
			if strings.Contains(stdout, avoid) {
				t.Errorf("Run(%#v) output unexpectedly contains %q: %s", args, avoid, stdout)
			}
		}

		summaryArgs := append([]string{dir, "--summary"}, test.args...)
		code, stdout, stderr = run(t, summaryArgs...)
		if code != 0 {
			t.Fatalf("Run(%#v) exit code = %d, stderr = %s", summaryArgs, code, stderr)
		}
		if !regexp.MustCompile(test.summary).MatchString(stdout) {
			t.Errorf("Run(%#v) summary is missing %s: %s", summaryArgs, test.summary, stdout)
		}
		if test.summaryNo != "" && regexp.MustCompile(test.summaryNo).MatchString(stdout) {
			t.Errorf("Run(%#v) summary unexpectedly matches %s: %s", summaryArgs, test.summaryNo, stdout)
		}
	}
}

func taskContents(t *testing.T, csvOutput string) []string {
	t.Helper()
	records, err := csv.NewReader(strings.NewReader(csvOutput)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	contents := make([]string, 0, len(records)-1)
	for _, record := range records[1:] {
		contents = append(contents, record[1])
	}
	return contents
}

func TestOrderTasks(t *testing.T) {
	base := []model.Task{
		{Content: "A"}, {Content: "B"}, {Content: "C"},
		{Content: "D"}, {Content: "E"},
	}
	original := append([]model.Task(nil), base...)

	got := orderTasks(base, false, 2, nil)
	if len(got) != 2 || got[0].Content != "A" || got[1].Content != "B" {
		t.Errorf("limited order = %#v", got)
	}
	if got := orderTasks(base, false, -1, nil); len(got) != len(base) {
		t.Errorf("unlimited order returned %d tasks", len(got))
	}
	if got := orderTasks(base, false, 0, nil); len(got) != 0 {
		t.Errorf("limit 0 returned %d tasks", len(got))
	}

	first := orderTasks(base, true, 3, rand.New(rand.NewSource(1)))
	second := orderTasks(base, true, 3, rand.New(rand.NewSource(2)))
	if len(first) != 3 || len(second) != 3 {
		t.Fatalf("random limit lengths = %d, %d", len(first), len(second))
	}
	if reflect.DeepEqual(first, second) {
		t.Errorf("different seeds produced the same sample: %#v", first)
	}
	allowed := make(map[string]bool)
	for _, task := range base {
		allowed[task.Content] = true
	}
	seen := make(map[string]bool)
	for _, task := range first {
		if !allowed[task.Content] {
			t.Errorf("random sample contains unknown task %q", task.Content)
		}
		if seen[task.Content] {
			t.Errorf("random sample contains duplicate task %q", task.Content)
		}
		seen[task.Content] = true
	}
	if !reflect.DeepEqual(base, original) {
		t.Errorf("orderTasks mutated its input: %#v", base)
	}
}

func TestRunLimit(t *testing.T) {
	dir := t.TempDir()
	writeMarkdown(t, dir, "tasks.md", ""+
		"- [ ] A\n- [ ] B\n- [ ] C\n- [ ] D\n- [ ] E\n")

	code, stdout, stderr := run(t, dir, "--format", "csv", "--limit", "2")
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr)
	}
	if got := taskContents(t, stdout); !reflect.DeepEqual(got, []string{"A", "B"}) {
		t.Errorf("limited tasks = %#v", got)
	}

	code, stdout, stderr = run(t, dir, "--format", "csv", "--limit", "0")
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr)
	}
	if got := taskContents(t, stdout); len(got) != 0 {
		t.Errorf("limit 0 returned %#v", got)
	}

	code, stdout, stderr = run(t, dir, "--format", "csv", "--limit", "99")
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr)
	}
	if got := taskContents(t, stdout); len(got) != 5 {
		t.Errorf("large limit returned %d tasks, want 5", len(got))
	}
}

func TestRunRandomLimitAndSummaryIsUnaffected(t *testing.T) {
	dir := t.TempDir()
	var contents strings.Builder
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&contents, "- [ ] Task %02d\n", i)
	}
	writeMarkdown(t, dir, "tasks.md", contents.String())

	code, stdout, stderr := run(t, dir, "--format", "csv", "-r", "--limit", "3")
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr)
	}
	got := taskContents(t, stdout)
	if len(got) != 3 {
		t.Fatalf("random limit returned %d tasks, want 3", len(got))
	}
	seen := make(map[string]bool)
	for _, content := range got {
		if seen[content] {
			t.Errorf("random limit returned duplicate task %q", content)
		}
		seen[content] = true
	}

	code, stdout, stderr = run(t, dir, "--summary", "--random", "--limit", "2")
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr)
	}
	if !regexp.MustCompile(`Tasks:\s+20`).MatchString(stdout) {
		t.Errorf("summary was affected by --limit/--random: %s", stdout)
	}
}
