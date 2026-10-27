// Package model contains the data shared by the mkdone parser, filters, and
// output writers.
package model

// Task is a single task found in a Markdown file. Inline tasks come from
// checkbox list items; file tasks come from task-like YAML frontmatter.
//
// Date fields are kept as strings in the form they appeared in the source.
// The DateTime helpers in the filter package understand both date-only values
// and RFC3339 timestamps.
type Task struct {
	Status      string            `json:"status"`
	Content     string            `json:"taskcontent"`
	Priority    string            `json:"priority"`
	Tags        []string          `json:"tags"`
	CreatedAt   string            `json:"created_at"`
	DueAt       string            `json:"due_at"`
	ScheduledAt string            `json:"scheduled_at"`
	CompletedAt string            `json:"completed_at"`
	Parent      string            `json:"parent"`
	Source      string            `json:"source"`
	Line        int               `json:"line"`
	Type        string            `json:"type"`
	ID          string            `json:"id"`
	Notes       []string          `json:"notes"`
	Properties  map[string]string `json:"properties"`
	StartAt     string            `json:"start_at"`
	CancelledAt string            `json:"cancelled_at"`
	Recurrence  string            `json:"recurrence"`
	DependsOn   []string          `json:"depends_on"`
	Time        string            `json:"time"`
}

// New returns a Task with non-nil collections so JSON output is stable.
func New() Task {
	return Task{
		Priority:   "normal",
		Tags:       []string{},
		Notes:      []string{},
		Properties: map[string]string{},
		DependsOn:  []string{},
	}
}
