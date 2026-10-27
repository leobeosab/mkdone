package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ryan/mkdone/internal/model"
	tasksummary "github.com/ryan/mkdone/internal/summary"
)

// ResolveSummaryFormat validates a summary format. When format is empty it is
// inferred from the output extension; summaries default to pretty.
func ResolveSummaryFormat(format, out string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		switch strings.ToLower(filepath.Ext(out)) {
		case ".json":
			format = FormatJSON
		case ".csv":
			format = FormatCSV
		default:
			format = FormatPretty
		}
	}
	switch format {
	case FormatPretty, "text", "txt":
		return FormatPretty, nil
	case FormatCSV, FormatJSON:
		return format, nil
	default:
		return "", fmt.Errorf("unsupported summary format %q (use pretty, csv, or json)", format)
	}
}

// WriteSummary aggregates tasks and writes the result in text, CSV, or JSON.
func WriteSummary(tasks []model.Task, format string, writer io.Writer, now time.Time) error {
	data := tasksummary.Build(tasks, now)
	switch strings.ToLower(format) {
	case FormatPretty, "text", "txt":
		return writeSummaryPretty(data, writer)
	case FormatCSV:
		return writeSummaryCSV(data, writer)
	case FormatJSON:
		return writeSummaryJSON(data, writer)
	default:
		return fmt.Errorf("unsupported summary format %q (use pretty, csv, or json)", format)
	}
}

func writeSummaryPretty(data tasksummary.Summary, writer io.Writer) error {
	table := tabwriter.NewWriter(writer, 0, 4, 2, ' ', 0)
	fmt.Fprintf(table, "Tasks:\t%d\n\n", data.Total)

	fmt.Fprintln(table, "Tags:")
	if len(data.Tags) == 0 {
		fmt.Fprintln(table, "  (none)")
	} else {
		for _, tag := range data.Tags {
			fmt.Fprintf(table, "  %s\t%d\n", tag.Tag, tag.Count)
		}
	}
	fmt.Fprintln(table)

	fmt.Fprintln(table, "Priorities:")
	for _, priority := range data.Priorities {
		fmt.Fprintf(table, "  %s\t%d\n", priority.Priority, priority.Count)
	}
	fmt.Fprintln(table)

	fmt.Fprintln(table, "Due:")
	fmt.Fprintf(table, "  Today\t%d\n", data.Due.Today)
	fmt.Fprintf(table, "  Tomorrow\t%d\n", data.Due.Tomorrow)
	fmt.Fprintf(table, "  Week\t%d\n", data.Due.Week)
	fmt.Fprintf(table, "  30 days\t%d\n", data.Due.ThirtyDays)

	return table.Flush()
}

func writeSummaryCSV(data tasksummary.Summary, writer io.Writer) error {
	csvWriter := csv.NewWriter(writer)
	if err := csvWriter.Write([]string{"category", "key", "count"}); err != nil {
		return err
	}
	if err := csvWriter.Write([]string{"total", "tasks", strconv.Itoa(data.Total)}); err != nil {
		return err
	}
	for _, tag := range data.Tags {
		if err := csvWriter.Write([]string{"tag", tag.Tag, strconv.Itoa(tag.Count)}); err != nil {
			return err
		}
	}
	for _, priority := range data.Priorities {
		if err := csvWriter.Write([]string{"priority", priority.Priority, strconv.Itoa(priority.Count)}); err != nil {
			return err
		}
	}
	due := []struct {
		key   string
		count int
	}{
		{"today", data.Due.Today},
		{"tomorrow", data.Due.Tomorrow},
		{"week", data.Due.Week},
		{"30_days", data.Due.ThirtyDays},
	}
	for _, row := range due {
		if err := csvWriter.Write([]string{"due", row.key, strconv.Itoa(row.count)}); err != nil {
			return err
		}
	}
	csvWriter.Flush()
	return csvWriter.Error()
}

func writeSummaryJSON(data tasksummary.Summary, writer io.Writer) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
