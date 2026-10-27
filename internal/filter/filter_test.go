package filter

import (
	"testing"
	"time"

	"github.com/ryan/mkdone/internal/model"
)

func TestTagFilters(t *testing.T) {
	tasks := []model.Task{
		{Tags: []string{"#work", "#work/api"}},
		{Tags: []string{"#personal"}},
		{Tags: []string{"#work", "#dontshow"}},
	}

	byTag, err := New(Options{Tags: []string{"work"}})
	if err != nil {
		t.Fatal(err)
	}
	got := byTag.Apply(tasks)
	if len(got) != 2 {
		t.Fatalf("tag filter returned %d tasks, want 2", len(got))
	}
	if len(got[0].Tags) != 2 || got[0].Tags[1] != "#work/api" {
		t.Errorf("parent tag should match a subtag: %#v", got[0].Tags)
	}

	hidden, err := New(Options{HideTags: []string{"#dontshow"}})
	if err != nil {
		t.Fatal(err)
	}
	got = hidden.Apply(tasks)
	if len(got) != 2 {
		t.Fatalf("hide filter returned %d tasks, want 2", len(got))
	}
	for _, task := range got {
		for _, tag := range task.Tags {
			if tag == "#dontshow" {
				t.Errorf("hidden task was not removed: %#v", task.Tags)
			}
		}
	}
}

func TestSearchInContentAndNotes(t *testing.T) {
	tasks := []model.Task{
		{Content: "Fix the thing"},
		{Content: "Other", Notes: []string{"details about the THING"}},
		{Content: "Unrelated"},
	}
	taskFilter, err := New(Options{Search: "thing"})
	if err != nil {
		t.Fatal(err)
	}
	got := taskFilter.Apply(tasks)
	if len(got) != 2 {
		t.Fatalf("search returned %d tasks, want 2", len(got))
	}
}

func TestDateFiltersAreInclusive(t *testing.T) {
	tasks := []model.Task{
		{DueAt: "2025-09-30"},
		{DueAt: "2025-10-01"},
		{DueAt: "2026-06-10"},
		{DueAt: "2026-06-11"},
		{DueAt: ""},
	}
	taskFilter, err := New(Options{After: "2025-10-01", Before: "2026-06-10"})
	if err != nil {
		t.Fatal(err)
	}
	got := taskFilter.Apply(tasks)
	if len(got) != 2 {
		t.Fatalf("date filter returned %d tasks, want 2", len(got))
	}
	if got[0].DueAt != "2025-10-01" || got[1].DueAt != "2026-06-10" {
		t.Errorf("unexpected dates: %#v", got)
	}
}

func TestDateFieldOverride(t *testing.T) {
	tasks := []model.Task{
		{DueAt: "2027-01-01", ScheduledAt: "2026-01-01"},
		{DueAt: "2026-01-01", ScheduledAt: "2027-01-01"},
	}
	scheduled, err := New(Options{
		After:     "2026-01-01",
		Before:    "2026-12-31",
		DateField: "scheduled",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := scheduled.Apply(tasks)
	if len(got) != 1 || got[0].ScheduledAt != "2026-01-01" {
		t.Fatalf("scheduled filter = %#v", got)
	}

	completed, err := New(Options{
		After:     "2026-01-01",
		Before:    "2026-12-31",
		DateField: "completed",
	})
	if err != nil {
		t.Fatal(err)
	}
	tasks = []model.Task{
		{CompletedAt: "2026-02-01"},
		{CompletedAt: ""},
	}
	got = completed.Apply(tasks)
	if len(got) != 1 {
		t.Fatalf("completed filter returned %d tasks, want 1", len(got))
	}
}

func TestInvalidFilterOptions(t *testing.T) {
	tests := []Options{
		{After: "not-a-date"},
		{Before: "not-a-date"},
		{After: "2026-02-01", Before: "2026-01-01"},
		{DateField: "created"},
	}
	for _, options := range tests {
		if _, err := New(options); err == nil {
			t.Errorf("New(%#v) returned no error", options)
		}
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{"2026-10-27", "2026-10-27"},
		{"2026-10-21T20:52:27.570-04:00", "2026-10-21"},
		{"'2026-10-21'", "2026-10-21"},
	}
	for _, test := range tests {
		got, ok := ParseDate(test.value)
		if !ok {
			t.Fatalf("ParseDate(%q) failed", test.value)
		}
		if got.Format("2006-01-02") != test.want {
			t.Errorf("ParseDate(%q) = %s, want %s", test.value, got.Format(time.DateOnly), test.want)
		}
	}
	if _, ok := ParseDate("nope"); ok {
		t.Error("ParseDate accepted an invalid date")
	}
}
