// Package summary computes aggregate statistics for a filtered task set.
package summary

import (
	"sort"
	"strings"
	"time"

	"github.com/ryan/mkdone/internal/filter"
	"github.com/ryan/mkdone/internal/model"
)

// PriorityOrder is the canonical order used in summary output.
var PriorityOrder = []string{"highest", "high", "medium", "low", "lowest", "normal"}

// TagCount is the number of tasks carrying a tag.
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// PriorityCount is the number of tasks in a priority slot.
type PriorityCount struct {
	Priority string `json:"priority"`
	Count    int    `json:"count"`
}

// DueCounts contains rolling due-date window counts. The windows include
// today; week is the next seven days and ThirtyDays is the next thirty days.
type DueCounts struct {
	Today      int `json:"today"`
	Tomorrow   int `json:"tomorrow"`
	Week       int `json:"week"`
	ThirtyDays int `json:"30_days"`
}

// Summary aggregates tags, priorities, and due-date windows.
type Summary struct {
	Total      int             `json:"total"`
	Tags       []TagCount      `json:"tags"`
	Priorities []PriorityCount `json:"priorities"`
	Due        DueCounts       `json:"due"`
}

// Build calculates a summary for tasks. now supplies the current local
// calendar date; task due dates are compared by calendar date only.
func Build(tasks []model.Task, now time.Time) Summary {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	tomorrow := today.AddDate(0, 0, 1)
	weekEnd := today.AddDate(0, 0, 7)
	monthEnd := today.AddDate(0, 0, 30)

	tagCounts := make(map[string]int)
	priorityCounts := make(map[string]int)
	for _, priority := range PriorityOrder {
		priorityCounts[priority] = 0
	}

	result := Summary{
		Total: len(tasks),
		Due:   DueCounts{},
	}

	for _, task := range tasks {
		for _, tag := range task.Tags {
			if tag != "" {
				tagCounts[tag]++
			}
		}

		priority := strings.ToLower(strings.TrimSpace(task.Priority))
		if priority == "" {
			priority = "normal"
		}
		priorityCounts[priority]++

		due, ok := filter.ParseDate(task.DueAt)
		if !ok || due.Before(today) {
			continue
		}
		if due.Equal(today) {
			result.Due.Today++
		}
		if due.Equal(tomorrow) {
			result.Due.Tomorrow++
		}
		if !due.After(weekEnd) {
			result.Due.Week++
		}
		if !due.After(monthEnd) {
			result.Due.ThirtyDays++
		}
	}

	result.Tags = make([]TagCount, 0, len(tagCounts))
	for tag, count := range tagCounts {
		result.Tags = append(result.Tags, TagCount{Tag: tag, Count: count})
	}
	sort.Slice(result.Tags, func(i, j int) bool {
		left := strings.ToLower(result.Tags[i].Tag)
		right := strings.ToLower(result.Tags[j].Tag)
		if left != right {
			return left < right
		}
		return result.Tags[i].Tag < result.Tags[j].Tag
	})

	result.Priorities = make([]PriorityCount, 0, len(priorityCounts))
	seen := make(map[string]bool)
	for _, priority := range PriorityOrder {
		result.Priorities = append(result.Priorities, PriorityCount{
			Priority: priority,
			Count:    priorityCounts[priority],
		})
		seen[priority] = true
	}
	extras := make([]string, 0)
	for priority := range priorityCounts {
		if !seen[priority] {
			extras = append(extras, priority)
		}
	}
	sort.Strings(extras)
	for _, priority := range extras {
		result.Priorities = append(result.Priorities, PriorityCount{
			Priority: priority,
			Count:    priorityCounts[priority],
		})
	}

	return result
}
