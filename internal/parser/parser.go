// Package parser extracts inline and file tasks from Markdown.
package parser

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ryan/mkdone/internal/model"
)

var (
	taskLineRE = regexp.MustCompile(`^(\s*)(?:[-*+]|\d+[.)])\s+\[(.)\]\s*(.*)$`)
	headingRE  = regexp.MustCompile(`^\s{0,3}#{1,6}\s+(.*?)\s*#*\s*$`)

	customPropertyRE = regexp.MustCompile(`\[([^\]:]+?)\s*::\s*([^\]]*)\]`)
	tagRE            = regexp.MustCompile(`(^|[\s(])#([\p{L}\p{N}_/-]+)`)

	createdRE    = regexp.MustCompile(`➕\s*(\d{4}-\d{2}-\d{2})`)
	dueRE        = regexp.MustCompile(`📅\s*(\d{4}-\d{2}-\d{2})`)
	scheduledRE  = regexp.MustCompile(`⏳\s*(\d{4}-\d{2}-\d{2})`)
	startRE      = regexp.MustCompile(`🛫\s*(\d{4}-\d{2}-\d{2})`)
	completedRE  = regexp.MustCompile(`✅\s*(\d{4}-\d{2}-\d{2})`)
	cancelledRE  = regexp.MustCompile(`❌\s*(\d{4}-\d{2}-\d{2})`)
	timeRE       = regexp.MustCompile(`⏰\s*(\d{1,2}:\d{2}(?:\s*-\s*\d{1,2}:\d{2})?)`)
	idRE         = regexp.MustCompile(`🆔\s*([^\s]+)`)
	dependsRE    = regexp.MustCompile(`⛔\s*([^\s]+)`)
	recurrenceRE = regexp.MustCompile(
		`🔁\s+(.+?)(?:\s+(?:[➕⏳📅🛫✅❌🆔⛔⏰🔺⏫🔼🔽⏬#])|$)`,
	)
)

var priorityMarkers = []struct {
	marker string
	name   string
	rank   int
}{
	{"🔺", "highest", 5},
	{"⏫", "high", 4},
	{"🔼", "medium", 3},
	{"🔽", "low", 2},
	{"⏬", "lowest", 1},
}

// Parse extracts all tasks from a single Markdown document. source is stored
// verbatim on each task and is normally the path passed to the scanner.
func Parse(data []byte, source string) []model.Task {
	text := strings.TrimPrefix(string(data), "\ufeff")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")

	frontmatter, bodyStart, hasFrontmatter := splitFrontmatter(lines)
	tasks := make([]model.Task, 0)

	if hasFrontmatter {
		if task, ok := fileTask(frontmatter, lines[bodyStart:], source); ok {
			tasks = append(tasks, task)
		}
	}

	type stackEntry struct {
		taskIndex int
		indent    int
	}
	stack := make([]stackEntry, 0)
	fenceChar := byte(0)
	fenceLen := 0

	for i := bodyStart; i < len(lines); i++ {
		line := lines[i]

		if ch, n, ok := fenceMarker(line); ok {
			if fenceChar == 0 {
				fenceChar, fenceLen = ch, n
				continue
			}
			if ch == fenceChar && n >= fenceLen {
				fenceChar, fenceLen = 0, 0
			}
			continue
		}
		if fenceChar != 0 {
			continue
		}

		match := taskLineRE.FindStringSubmatch(line)
		if match != nil {
			indent := indentation(match[1])
			task := parseInlineTask(match[2], match[3], source, i+1)

			for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
				stack = stack[:len(stack)-1]
			}
			if len(stack) > 0 {
				parent := stack[len(stack)-1].taskIndex
				task.Parent = tasks[parent].Content
			}

			tasks = append(tasks, task)
			stack = append(stack, stackEntry{taskIndex: len(tasks) - 1, indent: indent})
			continue
		}

		if strings.TrimSpace(line) == "" {
			continue
		}

		// A non-task line belongs to the closest task whose indentation is
		// smaller than this line's indentation. This makes task details
		// searchable without treating nested tasks as details.
		lineIndent := indentation(line)
		for j := len(stack) - 1; j >= 0; j-- {
			if stack[j].indent < lineIndent {
				taskIndex := stack[j].taskIndex
				tasks[taskIndex].Notes = append(tasks[taskIndex].Notes, strings.TrimSpace(line))
				break
			}
		}
	}

	return tasks
}

func parseInlineTask(statusChar, body, source string, line int) model.Task {
	task := model.New()
	task.Type = "inline"
	task.Source = source
	task.Line = line
	task.Status = inlineStatus(statusChar)

	body, properties := extractCustomProperties(body)
	task.Properties = properties

	body, priority := extractPriority(body)
	if priority != "" {
		task.Priority = priority
	}

	body, task.Recurrence = extractRecurrence(body)
	body, task.CreatedAt = takeDate(body, createdRE)
	body, task.DueAt = takeDate(body, dueRE)
	body, task.ScheduledAt = takeDate(body, scheduledRE)
	body, task.StartAt = takeDate(body, startRE)
	body, task.CompletedAt = takeDate(body, completedRE)
	body, task.CancelledAt = takeDate(body, cancelledRE)
	body, task.Time = takeMatch(body, timeRE)
	body, task.ID = takeMatch(body, idRE)
	body, task.DependsOn = takeAllMatches(body, dependsRE)
	body, task.Tags = extractTags(body)

	task.Content = strings.Join(strings.Fields(body), " ")
	return task
}

func splitFrontmatter(lines []string) (map[string]any, int, bool) {
	if len(lines) == 0 {
		return nil, 0, false
	}
	first := strings.TrimSpace(strings.TrimPrefix(lines[0], "\ufeff"))
	if first != "---" {
		return nil, 0, false
	}
	for i := 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "---" || trimmed == "..." {
			return parseFrontmatter(lines[1:i]), i + 1, true
		}
	}
	return nil, 0, false
}

func parseFrontmatter(lines []string) map[string]any {
	result := make(map[string]any)
	for i := 0; i < len(lines); {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || indentation(line) > 0 {
			i++
			continue
		}

		key, raw, ok := splitYAMLKeyValue(trimmed)
		if !ok {
			i++
			continue
		}

		if raw != "" {
			result[key] = parseYAMLValue(raw)
			i++
			continue
		}

		j := i + 1
		children := make([]string, 0)
		for j < len(lines) {
			if strings.TrimSpace(lines[j]) == "" {
				j++
				continue
			}
			if indentation(lines[j]) == 0 {
				break
			}
			children = append(children, lines[j])
			j++
		}
		result[key] = parseYAMLBlock(children)
		i = j
	}
	return result
}

func splitYAMLKeyValue(line string) (string, string, bool) {
	index := strings.IndexRune(line, ':')
	if index <= 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:index])
	key = strings.Trim(key, `"'`)
	if key == "" {
		return "", "", false
	}
	return key, strings.TrimSpace(stripYAMLComment(line[index+1:])), true
}

func parseYAMLBlock(lines []string) any {
	nonBlank := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonBlank = append(nonBlank, line)
		}
	}
	if len(nonBlank) == 0 {
		return ""
	}

	isList := true
	for _, line := range nonBlank {
		trimmed := strings.TrimSpace(line)
		if !(strings.HasPrefix(trimmed, "- ") || trimmed == "-") {
			isList = false
			break
		}
	}
	if isList {
		items := make([]string, 0, len(nonBlank))
		for _, line := range nonBlank {
			item := strings.TrimSpace(line)
			item = strings.TrimSpace(strings.TrimPrefix(item, "-"))
			item = strings.TrimSpace(stripYAMLComment(item))
			items = append(items, yamlScalar(item))
		}
		return items
	}

	nested := make(map[string]any)
	for _, line := range nonBlank {
		key, raw, ok := splitYAMLKeyValue(strings.TrimSpace(line))
		if !ok {
			continue
		}
		nested[key] = parseYAMLValue(raw)
	}
	if len(nested) > 0 {
		return nested
	}
	return strings.Join(nonBlank, " ")
}

func parseYAMLValue(value string) any {
	value = strings.TrimSpace(stripYAMLComment(value))
	if len(value) >= 2 && value[0] == '[' && value[len(value)-1] == ']' {
		inner := strings.TrimSpace(value[1 : len(value)-1])
		if inner == "" {
			return []string{}
		}
		parts := splitYAMLList(inner)
		items := make([]string, 0, len(parts))
		for _, part := range parts {
			items = append(items, yamlScalar(part))
		}
		return items
	}
	return yamlScalar(value)
}

func splitYAMLList(value string) []string {
	parts := make([]string, 0)
	start := 0
	var quote rune
	for i, r := range value {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ',':
			parts = append(parts, strings.TrimSpace(value[start:i]))
			start = i + 1
		}
	}
	parts = append(parts, strings.TrimSpace(value[start:]))
	return parts
}

func yamlScalar(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
		}
	}
	switch strings.ToLower(value) {
	case "null", "~":
		return ""
	}
	return value
}

func stripYAMLComment(value string) string {
	var quote rune
	for i, r := range value {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
		case r == '#':
			if i == 0 || value[i-1] == ' ' || value[i-1] == '\t' {
				return value[:i]
			}
		}
	}
	return value
}

func fileTask(frontmatter map[string]any, body []string, source string) (model.Task, bool) {
	if !looksLikeFileTask(frontmatter) {
		return model.Task{}, false
	}

	task := model.New()
	task.Type = "file"
	task.Source = source
	task.Line = 1
	task.Status = "open"
	if value, ok := lookupFrontmatter(frontmatter, "status"); ok {
		task.Status = fileStatus(textValue(value))
	}
	if value, ok := lookupFrontmatter(frontmatter, "priority"); ok {
		task.Priority = normalizePriority(textValue(value))
	}

	task.Tags = frontmatterTags(frontmatter)
	task.CreatedAt = frontmatterText(frontmatter,
		"dateCreated", "created", "createdAt", "created_at", "created at")
	task.DueAt = frontmatterText(frontmatter,
		"due", "dateDue", "dueDate", "dueAt", "due_at")
	task.ScheduledAt = frontmatterText(frontmatter,
		"scheduled", "dateScheduled", "scheduledDate", "scheduledAt", "scheduled_at")
	task.CompletedAt = frontmatterText(frontmatter,
		"dateCompleted", "completed", "completedAt", "completed_at", "doneAt", "done")
	task.CancelledAt = frontmatterText(frontmatter,
		"dateCancelled", "cancelled", "canceled", "cancelledAt", "cancelled_at")
	task.StartAt = frontmatterText(frontmatter,
		"start", "dateStart", "startDate", "startAt", "start_at")
	task.Parent = frontmatterText(frontmatter, "parent")
	task.ID = frontmatterText(frontmatter, "id", "taskId", "taskID")
	task.Recurrence = frontmatterText(frontmatter, "recurrence", "repeat")
	task.Time = frontmatterText(frontmatter, "time")
	task.DependsOn = frontmatterTagsFromValues(frontmatterValues(frontmatter,
		"dependsOn", "depends", "blockedBy", "depends_on"))

	task.Content = frontmatterText(frontmatter, "title", "name")
	if task.Content == "" {
		task.Content = firstHeading(body)
	}
	if task.Content == "" {
		name := filepath.Base(source)
		task.Content = strings.TrimSuffix(name, filepath.Ext(name))
	}

	for key, value := range frontmatter {
		if isKnownFrontmatterKey(key) {
			continue
		}
		task.Properties[key] = frontmatterValueString(value)
	}

	return task, true
}

func looksLikeFileTask(frontmatter map[string]any) bool {
	if _, ok := lookupFrontmatter(frontmatter, "taskSourceType"); ok {
		return true
	}
	if value, ok := lookupFrontmatter(frontmatter, "type"); ok {
		switch strings.ToLower(textValue(value)) {
		case "task", "tasknote", "tasknotes":
			return true
		}
	}
	if _, ok := lookupFrontmatter(frontmatter, "status"); ok {
		for _, key := range []string{
			"priority", "dateCreated", "created", "due", "dateDue", "scheduled",
			"dateScheduled", "dateCompleted", "dateCancelled", "parent",
			"taskSourceType", "id", "recurrence",
		} {
			if _, ok := lookupFrontmatter(frontmatter, key); ok {
				return true
			}
		}
	}
	return false
}

func isKnownFrontmatterKey(key string) bool {
	switch strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", "")) {
	case "status", "priority", "tags", "tag", "labels", "title", "name",
		"datecreated", "created", "createdat", "due", "datedue", "duedate",
		"dueat", "scheduled", "datescheduled", "scheduleddate", "scheduledat",
		"datecompleted", "completed", "completedat", "doneat", "done",
		"datecancelled", "cancelled", "canceled", "cancelledat",
		"start", "datestart", "startdate", "startat", "parent", "id", "taskid",
		"recurrence", "repeat", "time", "dependson", "depends", "blockedby":
		return true
	}
	return false
}

func lookupFrontmatter(frontmatter map[string]any, key string) (any, bool) {
	for actual, value := range frontmatter {
		if strings.EqualFold(actual, key) {
			return value, true
		}
	}
	return nil, false
}

func frontmatterText(frontmatter map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := lookupFrontmatter(frontmatter, key); ok {
			return textValue(value)
		}
	}
	return ""
}

func frontmatterValues(frontmatter map[string]any, keys ...string) []any {
	for _, key := range keys {
		if value, ok := lookupFrontmatter(frontmatter, key); ok {
			if list, ok := value.([]string); ok {
				values := make([]any, 0, len(list))
				for _, item := range list {
					values = append(values, item)
				}
				return values
			}
			return []any{value}
		}
	}
	return nil
}

func textValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []string:
		return strings.Join(typed, " ")
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			parts = append(parts, fmt.Sprintf("%s=%s", key, textValue(typed[key])))
		}
		return strings.Join(parts, ", ")
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func frontmatterValueString(value any) string {
	return textValue(value)
}

func frontmatterTags(frontmatter map[string]any) []string {
	values := make([]any, 0)
	for _, key := range []string{"tags", "tag", "labels"} {
		if value, ok := lookupFrontmatter(frontmatter, key); ok {
			values = append(values, value)
		}
	}
	return frontmatterTagsFromValues(values)
}

func frontmatterTagsFromValues(values []any) []string {
	tags := make([]string, 0)
	seen := make(map[string]bool)
	for _, value := range values {
		var items []string
		switch typed := value.(type) {
		case []string:
			items = typed
		case map[string]any:
			keys := make([]string, 0, len(typed))
			for key := range typed {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				items = append(items, textValue(typed[key]))
			}
		default:
			items = []string{textValue(value)}
		}
		for _, item := range items {
			for _, part := range strings.FieldsFunc(item, func(r rune) bool {
				return r == ',' || r == ' ' || r == '\t'
			}) {
				tag := normalizeTag(part)
				if tag == "" {
					continue
				}
				key := strings.ToLower(tag)
				if seen[key] {
					continue
				}
				seen[key] = true
				tags = append(tags, tag)
			}
		}
	}
	return tags
}

func firstHeading(lines []string) string {
	fenceChar := byte(0)
	fenceLen := 0
	for _, line := range lines {
		if ch, n, ok := fenceMarker(line); ok {
			if fenceChar == 0 {
				fenceChar, fenceLen = ch, n
			} else if ch == fenceChar && n >= fenceLen {
				fenceChar, fenceLen = 0, 0
			}
			continue
		}
		if fenceChar != 0 {
			continue
		}
		if match := headingRE.FindStringSubmatch(line); match != nil {
			return strings.TrimSpace(match[1])
		}
	}
	return ""
}

func extractCustomProperties(body string) (string, map[string]string) {
	properties := make(map[string]string)
	matches := customPropertyRE.FindAllStringSubmatchIndex(body, -1)
	if len(matches) == 0 {
		return body, properties
	}
	for _, match := range matches {
		key := strings.TrimSpace(body[match[2]:match[3]])
		value := strings.TrimSpace(body[match[4]:match[5]])
		if key != "" {
			properties[key] = value
		}
	}
	for i := len(matches) - 1; i >= 0; i-- {
		match := matches[i]
		body = body[:match[0]] + body[match[1]:]
	}
	return body, properties
}

func extractPriority(body string) (string, string) {
	name := ""
	rank := 0
	for _, priority := range priorityMarkers {
		if strings.Contains(body, priority.marker) {
			if rank == 0 || priority.rank > rank {
				name = priority.name
				rank = priority.rank
			}
			body = strings.ReplaceAll(body, priority.marker, "")
		}
	}
	return body, name
}

func extractRecurrence(body string) (string, string) {
	match := recurrenceRE.FindStringSubmatchIndex(body)
	if match == nil {
		return body, ""
	}
	value := strings.TrimSpace(body[match[2]:match[3]])
	// Remove only the marker and value, leaving a following metadata marker
	// (for example 🛫 or 📅) for its own extractor.
	body = body[:match[0]] + body[match[3]:]
	return body, value
}

func takeDate(body string, expression *regexp.Regexp) (string, string) {
	match := expression.FindStringSubmatchIndex(body)
	if match == nil {
		return body, ""
	}
	value := body[match[2]:match[3]]
	body = body[:match[0]] + body[match[1]:]
	return body, value
}

func takeMatch(body string, expression *regexp.Regexp) (string, string) {
	match := expression.FindStringSubmatchIndex(body)
	if match == nil {
		return body, ""
	}
	value := strings.TrimSpace(body[match[2]:match[3]])
	body = body[:match[0]] + body[match[1]:]
	return body, value
}

func takeAllMatches(body string, expression *regexp.Regexp) (string, []string) {
	matches := expression.FindAllStringSubmatchIndex(body, -1)
	if len(matches) == 0 {
		return body, []string{}
	}
	values := make([]string, 0, len(matches))
	seen := make(map[string]bool)
	for _, match := range matches {
		value := strings.TrimSpace(body[match[2]:match[3]])
		if value != "" && !seen[value] {
			seen[value] = true
			values = append(values, value)
		}
	}
	for i := len(matches) - 1; i >= 0; i-- {
		match := matches[i]
		body = body[:match[0]] + body[match[1]:]
	}
	return body, values
}

func extractTags(body string) (string, []string) {
	matches := tagRE.FindAllStringSubmatchIndex(body, -1)
	if len(matches) == 0 {
		return body, []string{}
	}
	tags := make([]string, 0, len(matches))
	seen := make(map[string]bool)
	for _, match := range matches {
		tag := normalizeTag(body[match[4]:match[5]])
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if seen[key] {
			continue
		}
		seen[key] = true
		tags = append(tags, tag)
	}
	for i := len(matches) - 1; i >= 0; i-- {
		match := matches[i]
		prefix := body[match[2]:match[3]]
		body = body[:match[0]] + prefix + body[match[1]:]
	}
	return body, tags
}

func normalizeTag(tag string) string {
	tag = strings.TrimSpace(tag)
	tag = strings.TrimPrefix(tag, "#")
	tag = strings.Trim(tag, ",")
	if tag == "" {
		return ""
	}
	return "#" + tag
}

func inlineStatus(char string) string {
	switch char {
	case " ":
		return "open"
	case "x", "X":
		return "done"
	case "-":
		return "cancelled"
	case "/":
		return "in_progress"
	case ">":
		return "forwarded"
	case "?":
		return "question"
	case "!":
		return "important"
	default:
		return strings.ToLower(strings.TrimSpace(char))
	}
}

func fileStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "open", "todo", "incomplete":
		return "open"
	case "done", "complete", "completed", "closed":
		return "done"
	case "cancelled", "canceled":
		return "cancelled"
	case "in_progress", "in-progress", "doing", "started":
		return "in_progress"
	default:
		return strings.ToLower(strings.TrimSpace(status))
	}
}

func normalizePriority(priority string) string {
	priority = strings.TrimSpace(priority)
	for _, marker := range priorityMarkers {
		if priority == marker.marker {
			return marker.name
		}
	}
	switch strings.ToLower(priority) {
	case "", "none", "normal":
		return "normal"
	case "highest", "urgent", "critical":
		return "highest"
	case "high":
		return "high"
	case "medium", "med":
		return "medium"
	case "low":
		return "low"
	case "lowest":
		return "lowest"
	default:
		return strings.ToLower(priority)
	}
}

func indentation(line string) int {
	width := 0
	for _, r := range line {
		switch r {
		case ' ':
			width++
		case '\t':
			width += 4
		default:
			return width
		}
	}
	return width
}

func fenceMarker(line string) (byte, int, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	if len(trimmed) < 3 {
		return 0, 0, false
	}
	ch := trimmed[0]
	if ch != '`' && ch != '~' {
		return 0, 0, false
	}
	length := 0
	for length < len(trimmed) && trimmed[length] == ch {
		length++
	}
	if length < 3 {
		return 0, 0, false
	}
	return ch, length, true
}
