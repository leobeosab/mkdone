package output

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ryan/mkdone/internal/model"
)

func TestWriteCSV(t *testing.T) {
	task := model.Task{
		Status:      "open",
		Content:     "Fix it",
		Priority:    "high",
		Tags:        []string{"#work", "#api"},
		CreatedAt:   "2026-01-01",
		DueAt:       "2026-02-01",
		ScheduledAt: "2026-01-15",
		CompletedAt: "2026-02-02",
		Parent:      "Parent",
		Source:      "file.md",
		Line:        7,
		Type:        "inline",
		ID:          "id-1",
		Notes:       []string{"note one", "note two"},
		Properties:  map[string]string{"b": "2", "a": "1"},
		StartAt:     "2026-01-01",
		CancelledAt: "2026-01-02",
		Recurrence:  "every week",
		DependsOn:   []string{"dep-a", "dep-b"},
		Time:        "14:30",
	}

	var buffer bytes.Buffer
	if err := Write([]model.Task{task}, FormatCSV, &buffer); err != nil {
		t.Fatal(err)
	}

	records, err := csv.NewReader(strings.NewReader(buffer.String())).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d CSV records, want 2", len(records))
	}
	if strings.Join(records[0], ",") != strings.Join(csvHeader, ",") {
		t.Errorf("header = %#v", records[0])
	}
	want := []string{
		"open", "Fix it", "high", "#work #api", "2026-01-01", "2026-02-01",
		"2026-01-15", "2026-02-02", "Parent", "file.md", "7", "inline", "id-1",
		"note one | note two", "a=1; b=2", "2026-01-01", "2026-01-02",
		"every week", "dep-a dep-b", "14:30",
	}
	if len(records[1]) != len(want) {
		t.Fatalf("record has %d fields, want %d: %#v", len(records[1]), len(want), records[1])
	}
	for i := range want {
		if records[1][i] != want[i] {
			t.Errorf("field %d (%s) = %q, want %q", i, csvHeader[i], records[1][i], want[i])
		}
	}
}

func TestWriteJSON(t *testing.T) {
	task := model.New()
	task.Content = "Fix it"
	task.Tags = []string{"#work"}
	task.Notes = []string{"detail"}
	task.Properties["custom"] = "value"

	var buffer bytes.Buffer
	if err := Write([]model.Task{task}, FormatJSON, &buffer); err != nil {
		t.Fatal(err)
	}
	var decoded []model.Task
	if err := json.Unmarshal(buffer.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 1 {
		t.Fatalf("got %d tasks, want 1", len(decoded))
	}
	if decoded[0].Content != "Fix it" || decoded[0].Priority != "normal" {
		t.Errorf("decoded task = %#v", decoded[0])
	}
	if len(decoded[0].Tags) != 1 || decoded[0].Tags[0] != "#work" {
		t.Errorf("decoded tags = %#v", decoded[0].Tags)
	}
}

func TestWriteEmptyJSONIsArray(t *testing.T) {
	var buffer bytes.Buffer
	if err := Write(nil, FormatJSON, &buffer); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buffer.String()) != "[]" {
		t.Errorf("empty JSON = %q, want []", buffer.String())
	}
}

func TestResolveFormat(t *testing.T) {
	tests := []struct {
		format string
		out    string
		want   string
		ok     bool
	}{
		{"", "", FormatPretty, true},
		{"", "tasks.json", FormatJSON, true},
		{"", "tasks.csv", FormatCSV, true},
		{"JSON", "", FormatJSON, true},
		{"pretty", "", FormatPretty, true},
		{"text", "", FormatPretty, true},
		{"xml", "", "", false},
	}
	for _, test := range tests {
		got, err := ResolveFormat(test.format, test.out)
		if test.ok {
			if err != nil {
				t.Errorf("ResolveFormat(%q, %q) error: %v", test.format, test.out, err)
			} else if got != test.want {
				t.Errorf("ResolveFormat(%q, %q) = %q, want %q", test.format, test.out, got, test.want)
			}
		} else if err == nil {
			t.Errorf("ResolveFormat(%q, %q) returned no error", test.format, test.out)
		}
	}
}

func TestWritePretty(t *testing.T) {
	tasks := []model.Task{
		{
			Status:    "open",
			Content:   "Fix it",
			Priority:  "high",
			Tags:      []string{"#work", "#api"},
			CreatedAt: "2026-01-01",
			DueAt:     "2026-02-01",
		},
		{
			Status:   "done",
			Content:  "Old task",
			Priority: "normal",
		},
	}
	var buffer bytes.Buffer
	if err := Write(tasks, FormatPretty, &buffer); err != nil {
		t.Fatal(err)
	}
	output := buffer.String()
	for _, expected := range []string{"STATUS", "PRIORITY", "CREATED", "DUE", "TAGS", "TASK", "[ ]", "[x]", "Fix it", "#work #api", "2026-02-01", "Old task"} {
		if !strings.Contains(output, expected) {
			t.Errorf("pretty output is missing %q:\n%s", expected, output)
		}
	}
}

func TestWritePrettyEmpty(t *testing.T) {
	var buffer bytes.Buffer
	if err := Write(nil, FormatPretty, &buffer); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buffer.String()) != "No tasks found." {
		t.Errorf("empty pretty output = %q", buffer.String())
	}
}
