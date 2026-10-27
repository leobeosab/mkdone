package output

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ryan/mkdone/internal/model"
	tasksummary "github.com/ryan/mkdone/internal/summary"
)

func summaryTasks() []model.Task {
	return []model.Task{
		{Tags: []string{"#work"}, Priority: "high", DueAt: "2026-10-27"},
		{Tags: []string{"#work"}, Priority: "medium", DueAt: "2026-10-28"},
	}
}

func TestResolveSummaryFormat(t *testing.T) {
	tests := []struct {
		format string
		out    string
		want   string
		ok     bool
	}{
		{"", "", FormatText, true},
		{"", "summary.json", FormatJSON, true},
		{"", "summary.csv", FormatCSV, true},
		{"", "summary.txt", FormatText, true},
		{"json", "", FormatJSON, true},
		{"TXT", "", FormatText, true},
		{"xml", "", "", false},
	}
	for _, test := range tests {
		got, err := ResolveSummaryFormat(test.format, test.out)
		if test.ok {
			if err != nil {
				t.Errorf("ResolveSummaryFormat(%q, %q) error: %v", test.format, test.out, err)
			} else if got != test.want {
				t.Errorf("ResolveSummaryFormat(%q, %q) = %q, want %q", test.format, test.out, got, test.want)
			}
		} else if err == nil {
			t.Errorf("ResolveSummaryFormat(%q, %q) returned no error", test.format, test.out)
		}
	}
}

func TestWriteSummaryCSV(t *testing.T) {
	var buffer bytes.Buffer
	now := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	if err := WriteSummary(summaryTasks(), FormatCSV, &buffer, now); err != nil {
		t.Fatal(err)
	}

	records, err := csv.NewReader(strings.NewReader(buffer.String())).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if records[0][0] != "category" || records[0][1] != "key" || records[0][2] != "count" {
		t.Fatalf("header = %#v", records[0])
	}
	rows := make(map[string]string)
	for _, record := range records[1:] {
		rows[record[0]+"/"+record[1]] = record[2]
	}
	want := map[string]string{
		"total/tasks":     "2",
		"tag/#work":       "2",
		"priority/high":   "1",
		"priority/medium": "1",
		"priority/normal": "0",
		"due/today":       "1",
		"due/tomorrow":    "1",
		"due/week":        "2",
		"due/30_days":     "2",
	}
	for key, value := range want {
		if rows[key] != value {
			t.Errorf("row %s = %q, want %q", key, rows[key], value)
		}
	}
}

func TestWriteSummaryJSON(t *testing.T) {
	var buffer bytes.Buffer
	now := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	if err := WriteSummary(summaryTasks(), FormatJSON, &buffer, now); err != nil {
		t.Fatal(err)
	}
	var got tasksummary.Summary
	if err := json.Unmarshal(buffer.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Total != 2 {
		t.Errorf("total = %d", got.Total)
	}
	if got.Due.Today != 1 || got.Due.Tomorrow != 1 || got.Due.Week != 2 || got.Due.ThirtyDays != 2 {
		t.Errorf("due = %#v", got.Due)
	}
	if len(got.Tags) != 1 || got.Tags[0].Tag != "#work" || got.Tags[0].Count != 2 {
		t.Errorf("tags = %#v", got.Tags)
	}
}

func TestWriteSummaryText(t *testing.T) {
	var buffer bytes.Buffer
	now := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	if err := WriteSummary(summaryTasks(), FormatText, &buffer, now); err != nil {
		t.Fatal(err)
	}
	output := buffer.String()
	for _, expected := range []string{"Tasks:", "Tags:", "Priorities:", "Due:", "Today", "Tomorrow", "Week", "30 days"} {
		if !strings.Contains(output, expected) {
			t.Errorf("summary text is missing %q:\n%s", expected, output)
		}
	}
	if !regexp.MustCompile(`#work\s+2`).MatchString(output) {
		t.Errorf("summary text is missing the tag count:\n%s", output)
	}
	if !regexp.MustCompile(`high\s+1`).MatchString(output) {
		t.Errorf("summary text is missing the priority count:\n%s", output)
	}
}
