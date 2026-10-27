// Package app implements the mkdone command line interface.
package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/ryan/mkdone/internal/filter"
	"github.com/ryan/mkdone/internal/model"
	"github.com/ryan/mkdone/internal/output"
	"github.com/ryan/mkdone/internal/scanner"
)

// Version is replaced at build time with -ldflags "-X .../app.Version=...".
var Version = "dev"

type config struct {
	path       string
	tags       []string
	hideTags   []string
	after      string
	before     string
	search     string
	format     string
	out        string
	workers    int
	due        bool
	scheduled  bool
	completed  bool
	help       bool
	version    bool
	dateField  string
	positional []string
}

// Run executes mkdone and returns a process exit code.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "mkdone: error: %v\n\n", err)
		writeUsage(stderr)
		return 2
	}
	if cfg.help {
		writeUsage(stdout)
		return 0
	}
	if cfg.version {
		fmt.Fprintf(stdout, "mkdone %s\n", Version)
		return 0
	}

	if cfg.scheduled && cfg.completed {
		fmt.Fprintln(stderr, "mkdone: error: --scheduled and --completed cannot be used together")
		return 2
	}
	switch {
	case cfg.scheduled:
		cfg.dateField = "scheduled"
	case cfg.completed:
		cfg.dateField = "completed"
	default:
		cfg.dateField = "due"
	}

	taskFilter, err := filter.New(filter.Options{
		Tags:      cfg.tags,
		HideTags:  cfg.hideTags,
		Search:    cfg.search,
		After:     cfg.after,
		Before:    cfg.before,
		DateField: cfg.dateField,
	})
	if err != nil {
		fmt.Fprintf(stderr, "mkdone: error: %v\n", err)
		return 2
	}

	format, err := output.ResolveFormat(cfg.format, cfg.out)
	if err != nil {
		fmt.Fprintf(stderr, "mkdone: error: %v\n", err)
		return 2
	}

	result, err := scanner.Scan(ctx, cfg.path, scanner.Options{Workers: cfg.workers})
	if err != nil {
		fmt.Fprintf(stderr, "mkdone: error: %v\n", err)
		return 1
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(stderr, "mkdone: warning: %v\n", warning)
	}

	tasks := taskFilter.Apply(result.Tasks)
	if err := writeOutput(tasks, format, cfg.out, stdout); err != nil {
		fmt.Fprintf(stderr, "mkdone: error: %v\n", err)
		return 1
	}
	return 0
}

func parseArgs(args []string) (config, error) {
	cfg := config{
		path:    ".",
		workers: 0,
	}
	positionals := make([]string, 0)

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if arg == "" || arg == "-" || !strings.HasPrefix(arg, "-") {
			positionals = append(positionals, arg)
			continue
		}

		name, value, hasValue := splitFlag(arg)
		switch name {
		case "h", "help":
			cfg.help = boolFlag(value, hasValue)
		case "v", "version":
			cfg.version = boolFlag(value, hasValue)
		case "due":
			cfg.due = boolFlag(value, hasValue)
		case "scheduled":
			cfg.scheduled = boolFlag(value, hasValue)
		case "completed", "complete":
			cfg.completed = boolFlag(value, hasValue)
		case "tags", "tag":
			value, i, hasValue = nextValue(args, i, value, hasValue)
			if !hasValue {
				return cfg, fmt.Errorf("--%s requires a value", name)
			}
			cfg.tags = append(cfg.tags, value)
		case "hidetags", "hide-tags", "hide_tags":
			value, i, hasValue = nextValue(args, i, value, hasValue)
			if !hasValue {
				return cfg, fmt.Errorf("--%s requires a value", name)
			}
			cfg.hideTags = append(cfg.hideTags, value)
		case "after":
			value, i, hasValue = nextValue(args, i, value, hasValue)
			if !hasValue {
				return cfg, fmt.Errorf("--after requires a date")
			}
			cfg.after = value
		case "before":
			value, i, hasValue = nextValue(args, i, value, hasValue)
			if !hasValue {
				return cfg, fmt.Errorf("--before requires a date")
			}
			cfg.before = value
		case "search":
			value, i, hasValue = nextValue(args, i, value, hasValue)
			if !hasValue {
				return cfg, fmt.Errorf("--search requires a value")
			}
			cfg.search = value
		case "format":
			value, i, hasValue = nextValue(args, i, value, hasValue)
			if !hasValue {
				return cfg, fmt.Errorf("--format requires a value")
			}
			cfg.format = value
		case "out", "o":
			value, i, hasValue = nextValue(args, i, value, hasValue)
			if !hasValue {
				return cfg, fmt.Errorf("--out requires a file name")
			}
			cfg.out = value
		case "workers":
			value, i, hasValue = nextValue(args, i, value, hasValue)
			if !hasValue {
				return cfg, fmt.Errorf("--workers requires a number")
			}
			workers, err := strconv.Atoi(value)
			if err != nil || workers < 0 {
				return cfg, fmt.Errorf("--workers must be a non-negative integer, got %q", value)
			}
			cfg.workers = workers
		default:
			return cfg, fmt.Errorf("unknown option %q", arg)
		}
	}

	if len(positionals) > 1 {
		return cfg, fmt.Errorf("expected at most one path, got %d", len(positionals))
	}
	if len(positionals) == 1 {
		cfg.path = positionals[0]
	}
	return cfg, nil
}

func splitFlag(arg string) (name, value string, hasValue bool) {
	trimmed := strings.TrimLeft(arg, "-")
	if before, after, found := strings.Cut(trimmed, "="); found {
		return strings.ToLower(strings.ReplaceAll(before, "_", "-")), after, true
	}
	return strings.ToLower(strings.ReplaceAll(trimmed, "_", "-")), "", false
}

func nextValue(args []string, index int, value string, hasValue bool) (string, int, bool) {
	if hasValue {
		return value, index, true
	}
	if index+1 >= len(args) {
		return "", index, false
	}
	return args[index+1], index + 1, true
}

func boolFlag(value string, hasValue bool) bool {
	if !hasValue {
		return true
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		// Unknown values are treated as true; the flag is primarily a switch.
		return true
	}
	return parsed
}

func writeOutput(tasks []model.Task, format, out string, stdout io.Writer) error {
	writer := stdout
	var file *os.File
	if out != "" && out != "-" {
		created, err := os.Create(out)
		if err != nil {
			return err
		}
		file = created
		writer = file
	}

	err := output.Write(tasks, format, writer)
	if file != nil {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}
	return err
}

func writeUsage(writer io.Writer) {
	fmt.Fprint(writer, `Usage:
  mkdone [path] [options]

Reads inline checkbox tasks and task-like YAML frontmatter from Markdown files.

Arguments:
  path                         File or directory to scan (default ".")

Filters:
  --tags TAG[,TAG...]          Only tasks with at least one matching tag.
                               Parent tags match subtags (#work matches #work/api).
  --hidetags TAG[,TAG...]      Exclude tasks with any matching tag.
  --after DATE                 Include tasks on or after DATE.
  --before DATE                Include tasks on or before DATE.
  --due                        Compare dates against due_at (default).
  --scheduled                  Compare dates against scheduled_at instead.
  --completed, --complete      Compare dates against completed_at instead.
  --search TEXT                Case-insensitive search in task content and notes.

Output:
  --format csv|json            Output format (default csv).
  --out FILE                   Write to FILE instead of stdout. "-" is stdout.
                               With no --format, .json/.csv infers the format.
  --workers N                  Parallel file readers (default: number of CPUs).
  -h, --help                   Show this help.
  -v, --version                Show the version.

Examples:
  mkdone ./
  mkdone ./ --tags '#projects/tech' --hidetags '#someday'
  mkdone ./ --after 2025-10-01 --before 2026-06-10 --scheduled
  mkdone ./ --search 'mouse jiggler' --format json --out tasks.json

Dates use YYYY-MM-DD. --after and --before are inclusive. Tasks without the
selected date are omitted when a date range is active.
`)
}
