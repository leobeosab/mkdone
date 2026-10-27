# mkdone

`mkdone` extracts tasks from Markdown files and prints them as CSV or JSON.
It understands inline checkbox tasks (Obsidian Tasks style) and file tasks
described by YAML frontmatter.

It is designed to be used by a TODO printer or piped into other tools:

```bash
mkdone ./
mkdone ./ --tags '#work' --format json --out tasks.json
```

## Features

- Inline tasks with status, priority, tags, dates, recurrence, IDs, dependencies,
  custom properties, parents, and searchable notes.
- File tasks from task-like YAML frontmatter.
- Filters for status, tags, hidden tags, search text, and due/scheduled/completed dates.
- Pretty, CSV, and JSON task output plus tag/priority/due-date summaries.
- Random sampling and result limits for task output.
- Markdown discovery through `ripgrep`, with a concurrent worker pool for parsing.
- A fallback directory walk when `ripgrep` is not installed.

## Build

Requirements: Go 1.23 or newer. `ripgrep` is recommended at runtime.

```bash
make build
./bin/mkdone --help
```

Or build directly:

```bash
go build -o mkdone .
```

## Docker

```bash
make docker-build
docker run --rm -v "$PWD:/work" -w /work mkdone:latest .
```

The image contains `ripgrep`. The Makefile also provides:

```bash
make docker-run ARGS="./ --tags '#work'"
make docker-test
```

Quote tag values containing `#`; otherwise the shell treats the rest of the
line as a comment.

## Usage

```text
mkdone [path] [options]
```

`path` may be a Markdown file or a directory. It defaults to `.`. Options may
appear before or after the path.

### Basic output

```bash
mkdone ./
```

By default tasks are printed as a human-readable `pretty` table:

```text
STATUS  PRIORITY  CREATED     DUE         TAGS             TASK
[ ]     normal    2025-12-31  -           #projects/tech   Design a personal pocket PC
[ ]     medium    2026-10-27  2026-10-31  #work            inline test +[[$4 mouse jiggler]]
```

Status is shown as a checkbox-like marker. Empty values are shown as `-`.
When no tasks match, pretty output prints `No tasks found.`

Use `--format csv` or `--format json` for machine-readable output.

### Filters

| Option | Behavior |
| --- | --- |
| `--tags TAG[,TAG...]` | Keep tasks with at least one matching tag. Parent tags match subtags: `#work` matches `#work/api`. Repeat the option to add more tags. |
| `--hidetags TAG[,TAG...]` | Drop tasks with any matching tag. Parent tags also match subtags. |
| `--search TEXT` | Case-insensitive search in task content and attached notes/details. |
| `--status any\|done\|todo` | Filter by status. Defaults to `todo`. `todo` includes open, in-progress, and other unfinished tasks; it excludes `done` and `cancelled`. `done` matches only completed tasks. `any` disables status filtering. |
| `--after DATE` | Keep tasks on or after `DATE` (inclusive). |
| `--before DATE` | Keep tasks on or before `DATE` (inclusive). |
| `--due` | Compare date filters with `due_at`. This is the default. |
| `--scheduled` | Compare date filters with `scheduled_at` instead. |
| `--completed`, `--complete` | Compare date filters with `completed_at` instead. |

All filter types are combined with AND. Tag filters use OR within the values
given to `--tags`. Dates use `YYYY-MM-DD` (RFC3339 timestamps are also accepted
in `--after`/`--before`). When a date range is active, a task without the
selected date is omitted. The default `--status todo` means completed and
cancelled tasks are hidden unless you pass `--status any` or `--status done`.

`--scheduled` and `--completed` (or `--complete`) cannot be used together. If either is supplied,
it overrides the default due-date field.

```bash
# Only tasks tagged #work or a subtag of it
mkdone ./ --tags '#work'

# Hide tasks tagged #someday
mkdone ./ --hidetags '#someday'

# Tasks due between two dates, inclusive
mkdone ./ --after 2025-10-01 --before 2026-06-10

# Same range, but compare scheduled dates
mkdone ./ --after 2025-10-01 --before 2026-06-10 --scheduled

# Search task text and indented notes
mkdone ./ --search 'mouse jiggler'

# Include completed and cancelled tasks too
mkdone ./ --status any

# Only completed tasks
mkdone ./ --status done
```

### Limiting and randomizing task output

| Option | Behavior |
| --- | --- |
| `--limit N` | Return at most `N` tasks after filtering. The default is unlimited. `--limit 0` returns no tasks. |
| `-r`, `--random` | Shuffle the filtered task list. When combined with `--limit`, the limit is taken from the shuffled list, producing a random sample. |

Both options only affect the task list. They are ignored by `--summary`.

```bash
# At most 10 filtered tasks
mkdone ./ --limit 10

# A random sample of 3 todo tasks
mkdone ./ --random --limit 3
mkdone ./ -r --limit 3 --tags '#work'

# All filtered tasks in random order
mkdone ./ --random
```

Ordering is applied after all normal filters: filters first, then random shuffle,
then the limit.

### Summary

Use `-s` or `--summary` to print aggregate information instead of the task list:

```bash
mkdone ./ --summary
mkdone ./ -s --tags '#work'
mkdone ./ -s --format json --out summary.json
```

All normal filters still apply before the summary is calculated. Because the
default status filter is `todo`, completed and cancelled tasks are excluded from
summaries unless you pass `--status any` or `--status done`.

The default summary is human-readable `pretty` output:

```text
Tasks:  3

Tags:
  #work    3

Priorities:
  highest   0
  high      1
  medium    1
  low       0
  lowest    0
  normal    1

Due:
  Today      1
  Tomorrow   1
  Week       3
  30 days    3
```

Summary details:

- `Tasks` is the total number of filtered tasks.
- Each tag shows how many tasks carry that exact tag. A task with multiple tags
  is counted under each tag, and untagged tasks appear only in the total.
- All six priority slots are always shown, including zeros. Unexpected priority
  values are appended after the standard slots.
- Due windows use `due_at` and are rolling windows that include today:
  - `Today`: due today.
  - `Tomorrow`: due tomorrow.
  - `Week`: due within the next 7 days, including today and tomorrow.
  - `30 days`: due within the next 30 days.
- Overdue tasks and tasks without a due date are not included in due windows.

Summary output also supports `pretty`, CSV, and JSON via `--format`. With no
`--format`, `.json` and `.csv` output files infer the matching summary format;
otherwise the summary defaults to pretty.

```bash
mkdone ./ -s --format csv --out summary.csv
mkdone ./ -s --out summary.json   # inferred as JSON
```

Summary CSV uses three columns, `category,key,count`, with categories `total`,
`tag`, `priority`, and `due`. Summary JSON looks like:

```json
{
  "total": 3,
  "tags": [{"tag": "#work", "count": 3}],
  "priorities": [
    {"priority": "highest", "count": 0},
    {"priority": "high", "count": 1},
    {"priority": "medium", "count": 1},
    {"priority": "low", "count": 0},
    {"priority": "lowest", "count": 0},
    {"priority": "normal", "count": 1}
  ],
  "due": {"today": 1, "tomorrow": 1, "week": 3, "30_days": 3}
}
```

### Output format and file

| Option | Behavior |
| --- | --- |
| `--format pretty\|csv\|json` | Select the format. Defaults to `pretty` for both task lists and summaries. |
| `--out FILE` | Write to `FILE` instead of stdout. `--out -` means stdout. |
| `--workers N` | Number of concurrent file readers. Defaults to the CPU count. `0` also means the CPU count. |

When `--format` is omitted, a `.json` output file implies JSON and a `.csv`
output file implies CSV. Everything else defaults to pretty.

```bash
mkdone ./ --format json --out out.json
mkdone ./ --out out.json          # format inferred as JSON
mkdone ./ --format csv --out tasks.csv
```

Pretty task output:

- Columns are `STATUS`, `PRIORITY`, `CREATED`, `DUE`, `TAGS`, and `TASK`.
- Status uses checkbox-style markers: `[ ]`, `[x]`, `[-]`, `[/]`, `[>]`,
  `[?]`, and `[!]`.
- Empty values are shown as `-`.
- For `--summary`, pretty output is the human-readable summary block shown
  above instead of the task table.

CSV conventions:

- The first columns are `status,taskcontent,priority,tags,created_at,due_at`.
- Additional columns are `scheduled_at,completed_at,parent,source,line,type,id,notes,properties,start_at,cancelled_at,recurrence,depends_on,time`.
- `tags` is space-separated and every tag includes its leading `#`.
- `notes` is joined with ` | `.
- `properties` is sorted `key=value` pairs joined with `; `.
- `depends_on` is space-separated.
- Normal CSV quoting is used for values containing commas, quotes, or newlines.

JSON is a top-level array of task objects. Empty collections are `[]` or `{}`,
not `null`.

## Task syntax

### Inline tasks

Inline tasks are Markdown list items with a checkbox:

```markdown
- [ ] Task content
- [x] Completed task
    - [ ] Nested task
        - notes and details
```

`-`, `*`, `+`, and numbered list markers are accepted. Leading indentation
establishes parent/child relationships.

Metadata is read from Obsidian Tasks compatible emoji markers:

| Field | Marker and value | Output column |
| --- | --- | --- |
| Created | `➕ 2026-10-27` | `created_at` |
| Due | `📅 2026-10-31` | `due_at` |
| Scheduled | `⏳ 2026-10-27` | `scheduled_at` |
| Start | `🛫 2026-10-27` | `start_at` |
| Completed | `✅ 2026-10-27` | `completed_at` |
| Cancelled | `❌ 2026-10-27` | `cancelled_at` |
| Time | `⏰ 14:30` | `time` |
| Recurrence | `🔁 every week` | `recurrence` |
| ID | `🆔 task-id` | `id` |
| Dependency | `⛔ task-id` | `depends_on` |
| Tag | `#work`, `#projects/tech` | `tags` |
| Property | `[custom:: value]` | `properties` |

Priority markers:

| Marker | Priority |
| --- | --- |
| `🔺` | `highest` |
| `⏫` | `high` |
| `🔼` | `medium` |
| `🔽` | `low` |
| `⏬` | `lowest` |
| none | `normal` |

Checkbox statuses:

| Checkbox | Status |
| --- | --- |
| `[ ]` | `open` |
| `[x]`, `[X]` | `done` |
| `[-]` | `cancelled` |
| `[/]` | `in_progress` |
| `[>]` | `forwarded` |
| `[?]` | `question` |
| `[!]` | `important` |

Metadata markers, tags, and custom properties are removed from `taskcontent`.
For example:

```markdown
- [ ] inline test #work +[[$4 mouse jiggler]] 🔼 ➕ 2026-10-27 ⏳ 2026-10-27 📅 2026-10-31 [custom:: property]
      - notes and details
```

produces content `inline test +[[$4 mouse jiggler]]`, tags `#work`, priority
`medium`, the three dates, and property `custom=property`.

Checkbox examples inside fenced code blocks (```` ``` ```` or `~~~`) are ignored.

Tasks are linked to their nearest less-indented task as `parent`. Non-task
lines indented deeper than a task are attached as its notes. Notes are included
in `--search` but are not part of the task content.

### File tasks

A Markdown file can itself be a task when it has task-like YAML frontmatter:

```markdown
---
status: open
priority: medium
dateCreated: 2026-10-21T20:52:27.570-04:00
tags:
  - task
  - demo
taskSourceType: taskNotes
---

## What Is the Obsidian Tasks Format?

Body text.
```

This produces a task with `type: file`, content from `title` (or the first
heading, or the filename), and the frontmatter metadata.

A file is treated as a file task when at least one of these is true:

- It has a `taskSourceType` key.
- `type` is `task`, `taskNote`, or `taskNotes`.
- It has `status` plus another task field such as `priority`, `dateCreated`,
  `due`, `scheduled`, `dateCompleted`, `dateCancelled`, parent, ID, or
  recurrence.

A regular note with only `status` and `tags` is not treated as a file task.
Inline tasks inside a file task are still extracted as separate inline tasks.

Recognized frontmatter fields and aliases:

| Output | Frontmatter keys |
| --- | --- |
| `created_at` | `dateCreated`, `created`, `createdAt`, `created_at`, `created at` |
| `due_at` | `due`, `dateDue`, `dueDate`, `dueAt`, `due_at` |
| `scheduled_at` | `scheduled`, `dateScheduled`, `scheduledDate`, `scheduledAt`, `scheduled_at` |
| `completed_at` | `dateCompleted`, `completed`, `completedAt`, `completed_at`, `doneAt`, `done` |
| `cancelled_at` | `dateCancelled`, `cancelled`, `canceled`, `cancelledAt`, `cancelled_at` |
| `start_at` | `start`, `dateStart`, `startDate`, `startAt`, `start_at` |
| `parent` | `parent` |
| `id` | `id`, `taskId`, `taskID` |
| `recurrence` | `recurrence`, `repeat` |
| `time` | `time` |
| `depends_on` | `dependsOn`, `depends`, `blockedBy`, `depends_on` |
| `tags` | `tags`, `tag`, `labels` |
| `status` | `status` |
| `priority` | `priority` |

Unknown frontmatter keys become `properties`. The included frontmatter reader
supports scalar values, inline lists (`[a, b]`), and indented `- item` lists.
Nested maps are flattened for display in the `properties` string.

## File discovery and concurrency

For a directory, `mkdone` runs:

```text
rg --files --null --hidden --no-ignore --iglob '*.md' --iglob '*.markdown' ...
```

This means hidden Markdown files and files normally excluded by `.gitignore`
are included, except anything inside `.git`. Both `.md` and `.markdown` are
case-insensitive extensions. If `ripgrep` is missing, the tool falls back to a
normal directory walk.

Files are parsed by a worker pool (`--workers`, CPU count by default). Results
are sorted by source path, line number, and content so output is deterministic.

## Make targets

| Target | Purpose |
| --- | --- |
| `make build` | Build `bin/mkdone`. |
| `make run ARGS="./ --tags '#work'"` | Build and run. |
| `make test` | Run all tests. |
| `make test-race` | Run tests with the race detector. |
| `make vet` | Run `go vet`. |
| `make fmt` | Format Go files. |
| `make fmt-check` | Fail if any Go file is not formatted. |
| `make check` | Run format check, vet, and tests. |
| `make docker-build` | Build the Docker image. |
| `make docker-run ARGS="./ --format json"` | Run the image against the current directory. |
| `make docker-test` | Run tests in the Docker build stage. |
| `make clean` | Remove `bin/`. |

## Tests

```bash
make test
# or
go test ./...
```

The tests cover inline and frontmatter parsing, code fences, parents/notes,
filters, date comparisons, CSV/JSON task output, summaries, random sampling,
limits, ripgrep discovery, and CLI behavior.

## Limitations

- Date filtering compares calendar dates. For timestamps, the date portion as
  written in the source is used; timezone conversion is not applied.
- The frontmatter reader intentionally supports a small, task-focused YAML
  subset rather than the entire YAML specification.
- Inline notes must be indented deeper than their task to be attached.
- Only Markdown files (`.md`, `.markdown`) are discovered in directory scans.
