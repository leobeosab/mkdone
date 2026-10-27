// Package output writes tasks as CSV or JSON.
package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ryan/mkdone/internal/model"
)

// Supported output formats.
const (
	FormatCSV  = "csv"
	FormatJSON = "json"
)

var csvHeader = []string{
	"status",
	"taskcontent",
	"priority",
	"tags",
	"created_at",
	"due_at",
	"scheduled_at",
	"completed_at",
	"parent",
	"source",
	"line",
	"type",
	"id",
	"notes",
	"properties",
	"start_at",
	"cancelled_at",
	"recurrence",
	"depends_on",
	"time",
}

// ResolveFormat validates format and, when format is empty, infers it from the
// output file extension. CSV is the default.
func ResolveFormat(format, out string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		switch strings.ToLower(filepath.Ext(out)) {
		case ".json":
			format = FormatJSON
		case ".csv":
			format = FormatCSV
		default:
			format = FormatCSV
		}
	}
	switch format {
	case FormatCSV, FormatJSON:
		return format, nil
	default:
		return "", fmt.Errorf("unsupported format %q (use csv or json)", format)
	}
}

// Write encodes tasks in the requested format. format must be csv or json.
func Write(tasks []model.Task, format string, writer io.Writer) error {
	switch strings.ToLower(format) {
	case FormatCSV:
		return writeCSV(tasks, writer)
	case FormatJSON:
		return writeJSON(tasks, writer)
	default:
		return fmt.Errorf("unsupported format %q (use csv or json)", format)
	}
}

func writeCSV(tasks []model.Task, writer io.Writer) error {
	csvWriter := csv.NewWriter(writer)
	if err := csvWriter.Write(csvHeader); err != nil {
		return err
	}
	for _, task := range tasks {
		record := []string{
			task.Status,
			task.Content,
			task.Priority,
			strings.Join(task.Tags, " "),
			task.CreatedAt,
			task.DueAt,
			task.ScheduledAt,
			task.CompletedAt,
			task.Parent,
			task.Source,
			strconv.Itoa(task.Line),
			task.Type,
			task.ID,
			strings.Join(task.Notes, " | "),
			formatProperties(task.Properties),
			task.StartAt,
			task.CancelledAt,
			task.Recurrence,
			strings.Join(task.DependsOn, " "),
			task.Time,
		}
		if err := csvWriter.Write(record); err != nil {
			return err
		}
	}
	csvWriter.Flush()
	return csvWriter.Error()
}

func writeJSON(tasks []model.Task, writer io.Writer) error {
	if tasks == nil {
		tasks = []model.Task{}
	}
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(tasks)
}

func formatProperties(properties map[string]string) string {
	if len(properties) == 0 {
		return ""
	}
	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+properties[key])
	}
	return strings.Join(parts, "; ")
}
