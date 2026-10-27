package parser

import (
	"reflect"
	"testing"
)

func TestParseInlineTasks(t *testing.T) {
	input := "# Tasks\n\n" +
		"- [ ] Design a personal pocket PC #projects/tech ➕ 2025-12-31\n" +
		"      - [ ] inline test #work +[[$4 mouse jiggler]] 🔼 ➕ 2026-10-27 ⏳ 2026-10-27 📅 2026-10-31 [custom:: property]\n" +
		"        - notes and details\n"

	tasks := Parse([]byte(input), "tasks.md")
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks, want 2: %#v", len(tasks), tasks)
	}

	parent := tasks[0]
	if parent.Status != "open" {
		t.Errorf("parent status = %q, want open", parent.Status)
	}
	if parent.Content != "Design a personal pocket PC" {
		t.Errorf("parent content = %q", parent.Content)
	}
	if parent.Priority != "normal" {
		t.Errorf("parent priority = %q, want normal", parent.Priority)
	}
	if !reflect.DeepEqual(parent.Tags, []string{"#projects/tech"}) {
		t.Errorf("parent tags = %#v", parent.Tags)
	}
	if parent.CreatedAt != "2025-12-31" {
		t.Errorf("parent created = %q", parent.CreatedAt)
	}
	if parent.Line != 3 {
		t.Errorf("parent line = %d, want 3", parent.Line)
	}
	if parent.Type != "inline" {
		t.Errorf("parent type = %q", parent.Type)
	}
	if len(parent.Notes) != 0 {
		t.Errorf("parent notes = %#v, want none", parent.Notes)
	}

	child := tasks[1]
	if child.Content != "inline test +[[$4 mouse jiggler]]" {
		t.Errorf("child content = %q", child.Content)
	}
	if child.Priority != "medium" {
		t.Errorf("child priority = %q, want medium", child.Priority)
	}
	if !reflect.DeepEqual(child.Tags, []string{"#work"}) {
		t.Errorf("child tags = %#v", child.Tags)
	}
	if child.CreatedAt != "2026-10-27" || child.ScheduledAt != "2026-10-27" || child.DueAt != "2026-10-31" {
		t.Errorf("child dates = created %q scheduled %q due %q", child.CreatedAt, child.ScheduledAt, child.DueAt)
	}
	if child.Parent != "Design a personal pocket PC" {
		t.Errorf("child parent = %q", child.Parent)
	}
	if child.Line != 4 {
		t.Errorf("child line = %d, want 4", child.Line)
	}
	if !reflect.DeepEqual(child.Notes, []string{"- notes and details"}) {
		t.Errorf("child notes = %#v", child.Notes)
	}
	if !reflect.DeepEqual(child.Properties, map[string]string{"custom": "property"}) {
		t.Errorf("child properties = %#v", child.Properties)
	}
}

func TestParseInlineMetadata(t *testing.T) {
	input := "- [x] Finished #work ⏫ ✅ 2026-01-02 🆔 task-1 ⛔ dep-a ⛔ dep-b ⏰ 09:30\n" +
		"- [-] Cancelled ❌ 2026-02-01\n" +
		"- [ ] Weekly 🔁 every week 🛫 2026-01-01 📅 2026-02-01\n"

	tasks := Parse([]byte(input), "tasks.md")
	if len(tasks) != 3 {
		t.Fatalf("got %d tasks, want 3", len(tasks))
	}

	finished := tasks[0]
	if finished.Status != "done" {
		t.Errorf("status = %q, want done", finished.Status)
	}
	if finished.Content != "Finished" {
		t.Errorf("content = %q, want Finished", finished.Content)
	}
	if finished.Priority != "high" {
		t.Errorf("priority = %q, want high", finished.Priority)
	}
	if finished.CompletedAt != "2026-01-02" {
		t.Errorf("completed = %q", finished.CompletedAt)
	}
	if finished.ID != "task-1" {
		t.Errorf("id = %q", finished.ID)
	}
	if !reflect.DeepEqual(finished.DependsOn, []string{"dep-a", "dep-b"}) {
		t.Errorf("depends = %#v", finished.DependsOn)
	}
	if finished.Time != "09:30" {
		t.Errorf("time = %q", finished.Time)
	}

	cancelled := tasks[1]
	if cancelled.Status != "cancelled" {
		t.Errorf("status = %q, want cancelled", cancelled.Status)
	}
	if cancelled.Content != "Cancelled" {
		t.Errorf("content = %q", cancelled.Content)
	}
	if cancelled.CancelledAt != "2026-02-01" {
		t.Errorf("cancelled at = %q", cancelled.CancelledAt)
	}

	weekly := tasks[2]
	if weekly.Content != "Weekly" {
		t.Errorf("content = %q, want Weekly", weekly.Content)
	}
	if weekly.Recurrence != "every week" {
		t.Errorf("recurrence = %q", weekly.Recurrence)
	}
	if weekly.StartAt != "2026-01-01" {
		t.Errorf("start = %q", weekly.StartAt)
	}
	if weekly.DueAt != "2026-02-01" {
		t.Errorf("due = %q", weekly.DueAt)
	}
}

func TestParsePriorityMarkers(t *testing.T) {
	tests := []struct {
		marker string
		want   string
	}{
		{"🔺", "highest"},
		{"⏫", "high"},
		{"🔼", "medium"},
		{"🔽", "low"},
		{"⏬", "lowest"},
	}
	for _, test := range tests {
		tasks := Parse([]byte("- [ ] Priority "+test.marker+"\n"), "tasks.md")
		if len(tasks) != 1 {
			t.Fatalf("got %d tasks", len(tasks))
		}
		if tasks[0].Priority != test.want {
			t.Errorf("marker %s priority = %q, want %q", test.marker, tasks[0].Priority, test.want)
		}
		if tasks[0].Content != "Priority" {
			t.Errorf("marker %s content = %q", test.marker, tasks[0].Content)
		}
	}
}

func TestIgnoresTasksInCodeFences(t *testing.T) {
	input := "```md\n- [ ] not a task\n```\n" +
		"~~~\n- [ ] also not a task\n~~~\n" +
		"- [ ] real task\n"

	tasks := Parse([]byte(input), "tasks.md")
	if len(tasks) != 1 {
		t.Fatalf("got %d tasks, want 1: %#v", len(tasks), tasks)
	}
	if tasks[0].Content != "real task" {
		t.Errorf("content = %q", tasks[0].Content)
	}
}

func TestFileTaskFrontmatter(t *testing.T) {
	input := `---
status: open
priority: medium
dateCreated: 2026-10-21T20:52:27.570-04:00
tags:
  - task
  - demo
taskSourceType: taskNotes
---

## What Is the Obsidian Tasks Format?

Text about tasks.

` + "```md" + `
- [ ] Example inside a fence
` + "```" + `
`

	tasks := Parse([]byte(input), "file-task.md")
	if len(tasks) != 1 {
		t.Fatalf("got %d tasks, want 1: %#v", len(tasks), tasks)
	}
	task := tasks[0]
	if task.Type != "file" {
		t.Errorf("type = %q, want file", task.Type)
	}
	if task.Status != "open" {
		t.Errorf("status = %q", task.Status)
	}
	if task.Priority != "medium" {
		t.Errorf("priority = %q", task.Priority)
	}
	if task.CreatedAt != "2026-10-21T20:52:27.570-04:00" {
		t.Errorf("created = %q", task.CreatedAt)
	}
	if !reflect.DeepEqual(task.Tags, []string{"#task", "#demo"}) {
		t.Errorf("tags = %#v", task.Tags)
	}
	if task.Content != "What Is the Obsidian Tasks Format?" {
		t.Errorf("content = %q", task.Content)
	}
	if task.Line != 1 {
		t.Errorf("line = %d, want 1", task.Line)
	}
	if !reflect.DeepEqual(task.Properties, map[string]string{"taskSourceType": "taskNotes"}) {
		t.Errorf("properties = %#v", task.Properties)
	}
}

func TestFileTaskInlineTagsAndDates(t *testing.T) {
	input := `---
status: done
priority: high
tags: [task, work]
dateDue: 2026-06-10
dateCompleted: 2026-06-09
title: Finish report
---

Body.
`
	tasks := Parse([]byte(input), "report.md")
	if len(tasks) != 1 {
		t.Fatalf("got %d tasks, want 1", len(tasks))
	}
	task := tasks[0]
	if task.Status != "done" || task.Priority != "high" {
		t.Errorf("status/priority = %q/%q", task.Status, task.Priority)
	}
	if task.Content != "Finish report" {
		t.Errorf("content = %q", task.Content)
	}
	if !reflect.DeepEqual(task.Tags, []string{"#task", "#work"}) {
		t.Errorf("tags = %#v", task.Tags)
	}
	if task.DueAt != "2026-06-10" || task.CompletedAt != "2026-06-09" {
		t.Errorf("due/completed = %q/%q", task.DueAt, task.CompletedAt)
	}
}

func TestPlainFrontmatterNoteIsNotAFileTask(t *testing.T) {
	tests := []string{
		"---\ntitle: A normal note\ntags: [notes]\n---\n# Heading\n",
		"---\nstatus: active\ntags: [notes]\n---\n# Heading\n",
	}
	for _, input := range tests {
		tasks := Parse([]byte(input), "note.md")
		if len(tasks) != 0 {
			t.Errorf("got %d tasks for plain frontmatter, want 0: %#v", len(tasks), tasks)
		}
	}
}

func TestFileTaskFallsBackToFilename(t *testing.T) {
	input := "---\ntaskSourceType: taskNotes\n---\nNo heading here.\n"
	tasks := Parse([]byte(input), "/tmp/my-task.md")
	if len(tasks) != 1 {
		t.Fatalf("got %d tasks, want 1", len(tasks))
	}
	if tasks[0].Content != "my-task" {
		t.Errorf("content = %q, want my-task", tasks[0].Content)
	}
}
