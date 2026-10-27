// Package filter implements the task filters exposed by the mkdone CLI.
package filter

import (
	"fmt"
	"strings"
	"time"

	"github.com/ryan/mkdone/internal/model"
)

// Options are the user-facing filter options. Empty dates disable that side
// of the range. DateField defaults to "due".
type Options struct {
	Tags      []string
	HideTags  []string
	Search    string
	After     string
	Before    string
	DateField string
}

// Filter is a compiled set of task predicates.
type Filter struct {
	tags      []string
	hideTags  []string
	search    string
	after     time.Time
	before    time.Time
	hasAfter  bool
	hasBefore bool
	dateField string
}

// New validates and compiles filter options.
func New(options Options) (Filter, error) {
	dateField := strings.ToLower(strings.TrimSpace(options.DateField))
	if dateField == "" {
		dateField = "due"
	}
	switch dateField {
	case "due", "scheduled", "completed":
	default:
		return Filter{}, fmt.Errorf("unsupported date field %q (use due, scheduled, or completed)", options.DateField)
	}

	filter := Filter{
		tags:      normalizeTags(options.Tags),
		hideTags:  normalizeTags(options.HideTags),
		search:    strings.ToLower(strings.TrimSpace(options.Search)),
		dateField: dateField,
	}

	if strings.TrimSpace(options.After) != "" {
		after, ok := ParseDate(options.After)
		if !ok {
			return Filter{}, fmt.Errorf("invalid --after date %q (expected YYYY-MM-DD or RFC3339)", options.After)
		}
		filter.after = after
		filter.hasAfter = true
	}
	if strings.TrimSpace(options.Before) != "" {
		before, ok := ParseDate(options.Before)
		if !ok {
			return Filter{}, fmt.Errorf("invalid --before date %q (expected YYYY-MM-DD or RFC3339)", options.Before)
		}
		filter.before = before
		filter.hasBefore = true
	}
	if filter.hasAfter && filter.hasBefore && filter.after.After(filter.before) {
		return Filter{}, fmt.Errorf("--after (%s) must not be later than --before (%s)",
			options.After, options.Before)
	}

	return filter, nil
}

// Match reports whether task passes all configured filters.
func (f Filter) Match(task model.Task) bool {
	if len(f.hideTags) > 0 {
		for _, tag := range task.Tags {
			if matchesAnyTag(tag, f.hideTags) {
				return false
			}
		}
	}

	if len(f.tags) > 0 {
		matched := false
		for _, tag := range task.Tags {
			if matchesAnyTag(tag, f.tags) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	if f.search != "" {
		haystack := strings.ToLower(task.Content)
		if !strings.Contains(haystack, f.search) {
			found := false
			for _, note := range task.Notes {
				if strings.Contains(strings.ToLower(note), f.search) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	if f.hasAfter || f.hasBefore {
		value := selectedDate(task, f.dateField)
		date, ok := ParseDate(value)
		if !ok {
			return false
		}
		if f.hasAfter && date.Before(f.after) {
			return false
		}
		if f.hasBefore && date.After(f.before) {
			return false
		}
	}

	return true
}

// Apply returns the tasks that pass the filter, preserving order.
func (f Filter) Apply(tasks []model.Task) []model.Task {
	result := make([]model.Task, 0, len(tasks))
	for _, task := range tasks {
		if f.Match(task) {
			result = append(result, task)
		}
	}
	return result
}

// ParseDate parses a date or timestamp from task metadata. Calendar dates are
// returned at midnight UTC. Timestamp values are reduced to their calendar
// date, which is what the --after/--before filters compare.
func ParseDate(value string) (time.Time, bool) {
	value = strings.TrimSpace(strings.Trim(value, `"'`))
	if value == "" {
		return time.Time{}, false
	}

	if len(value) >= 10 {
		if date, err := time.Parse("2006-01-02", value[:10]); err == nil {
			return date, true
		}
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006/01/02",
		"2006/01/02 15:04:05",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC), true
		}
	}
	return time.Time{}, false
}

func selectedDate(task model.Task, field string) string {
	switch field {
	case "scheduled":
		return task.ScheduledAt
	case "completed":
		return task.CompletedAt
	default:
		return task.DueAt
	}
}

func normalizeTags(tags []string) []string {
	result := make([]string, 0, len(tags))
	seen := make(map[string]bool)
	for _, tag := range tags {
		for _, part := range strings.FieldsFunc(tag, func(r rune) bool {
			return r == ',' || r == ' ' || r == '\t'
		}) {
			part = strings.TrimPrefix(strings.TrimSpace(part), "#")
			if part == "" {
				continue
			}
			key := strings.ToLower(part)
			if seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, key)
		}
	}
	return result
}

func matchesAnyTag(tag string, filters []string) bool {
	tag = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(tag), "#"))
	for _, wanted := range filters {
		if tag == wanted || strings.HasPrefix(tag, wanted+"/") {
			return true
		}
	}
	return false
}
