package summary

import (
	"reflect"
	"testing"
	"time"

	"github.com/ryan/mkdone/internal/model"
)

func TestBuild(t *testing.T) {
	now := time.Date(2026, 10, 27, 15, 30, 0, 0, time.Local)
	tasks := []model.Task{
		{Tags: []string{"#work", "#api"}, Priority: "high", DueAt: "2026-10-27"},
		{Tags: []string{"#work"}, Priority: "medium", DueAt: "2026-10-28"},
		{Tags: []string{"#personal"}, Priority: "normal", DueAt: "2026-11-01"},
		{Tags: []string{"#work"}, Priority: "low", DueAt: "2026-11-16"},
		{Priority: "lowest", DueAt: "2026-11-30"},
		{Priority: "", DueAt: ""},
	}

	got := Build(tasks, now)
	if got.Total != 6 {
		t.Errorf("total = %d, want 6", got.Total)
	}
	wantTags := []TagCount{
		{Tag: "#api", Count: 1},
		{Tag: "#personal", Count: 1},
		{Tag: "#work", Count: 3},
	}
	if !reflect.DeepEqual(got.Tags, wantTags) {
		t.Errorf("tags = %#v, want %#v", got.Tags, wantTags)
	}
	wantPriorities := []PriorityCount{
		{Priority: "highest", Count: 0},
		{Priority: "high", Count: 1},
		{Priority: "medium", Count: 1},
		{Priority: "low", Count: 1},
		{Priority: "lowest", Count: 1},
		{Priority: "normal", Count: 2},
	}
	if !reflect.DeepEqual(got.Priorities, wantPriorities) {
		t.Errorf("priorities = %#v, want %#v", got.Priorities, wantPriorities)
	}
	wantDue := DueCounts{Today: 1, Tomorrow: 1, Week: 3, ThirtyDays: 4}
	if got.Due != wantDue {
		t.Errorf("due = %#v, want %#v", got.Due, wantDue)
	}
}

func TestBuildIncludesUnknownPriority(t *testing.T) {
	got := Build([]model.Task{{Priority: "urgent"}}, time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC))
	if len(got.Priorities) != len(PriorityOrder)+1 {
		t.Fatalf("priorities = %#v", got.Priorities)
	}
	last := got.Priorities[len(got.Priorities)-1]
	if last.Priority != "urgent" || last.Count != 1 {
		t.Errorf("last priority = %#v", last)
	}
}

func TestBuildEmpty(t *testing.T) {
	got := Build(nil, time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC))
	if got.Total != 0 || len(got.Tags) != 0 {
		t.Errorf("summary = %#v", got)
	}
	if len(got.Priorities) != len(PriorityOrder) {
		t.Errorf("priorities = %#v", got.Priorities)
	}
	for _, priority := range got.Priorities {
		if priority.Count != 0 {
			t.Errorf("priority %s = %d, want 0", priority.Priority, priority.Count)
		}
	}
}
