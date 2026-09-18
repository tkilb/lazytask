// Package editbuffer implements the structured, human-friendly plain-text
// buffer format used for the `$EDITOR` edit flow (see requirements.md §7,
// chunk 10). The buffer has two sections separated by a "---" divider line:
//
//	Description: <text, may span multiple lines>
//	Project: <text>
//	Priority: <text>
//	Due: <text>
//	Tags: tag1, tag2
//	---
//	ID: <int>
//	UUID: <string>
//	Status: <string>
//	Entry: <string>
//	Modified: <string>
//	End: <string>
//	Urgency: <float>
//
// Everything above the divider is user-editable and re-imported on save.
// Everything below the divider is read-only reference: it is rendered for
// display only and is never parsed back into the edited task, so any edits
// a user makes there are silently ignored rather than treated as an error.
package editbuffer

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tkilb/lazytask/internal/taskwarrior"
)

// divider marks the boundary between the editable section and the
// read-only reference section.
const divider = "---"

// editableKeys are the recognized field labels in the editable section, in
// the order they're written by Serialize.
var editableKeys = []string{"Description", "Project", "Priority", "Due", "Tags"}

// EditableFields holds the subset of a Task's fields that are user-editable
// via the buffer, as parsed back from edited buffer content.
type EditableFields struct {
	Description string
	Project     string
	Priority    string
	Due         string
	Tags        []string
}

// Serialize renders t as an edit buffer: the editable fields, a "---"
// divider, then the read-only reference fields for display.
func Serialize(t taskwarrior.Task) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Description: %s\n", t.Description)
	fmt.Fprintf(&b, "Project: %s\n", t.Project)
	fmt.Fprintf(&b, "Priority: %s\n", t.Priority)
	fmt.Fprintf(&b, "Due: %s\n", t.Due)
	fmt.Fprintf(&b, "Tags: %s\n", strings.Join(t.Tags, ", "))
	b.WriteString(divider + "\n")
	fmt.Fprintf(&b, "ID: %d\n", t.ID)
	fmt.Fprintf(&b, "UUID: %s\n", t.UUID)
	fmt.Fprintf(&b, "Status: %s\n", t.Status)
	fmt.Fprintf(&b, "Entry: %s\n", t.Entry)
	fmt.Fprintf(&b, "Modified: %s\n", t.Modified)
	fmt.Fprintf(&b, "End: %s\n", t.End)
	fmt.Fprintf(&b, "Urgency: %s\n", formatUrgency(t.Urgency))

	return b.String()
}

// Parse extracts the editable fields from edit buffer content. Only the
// section above the "---" divider (if any) is consulted; the read-only
// reference section, and any edits made to it, are always ignored rather
// than causing a parse error. Parse returns an error only if the buffer is
// missing a Description line, since every task requires one.
func Parse(content string) (EditableFields, error) {
	lines := strings.Split(content, "\n")

	var fields EditableFields
	var descLines []string
	sawDescription := false
	inDescription := false

	for _, line := range lines {
		if strings.TrimSpace(line) == divider {
			break
		}

		if key, value, ok := matchKey(line); ok {
			inDescription = key == "Description"
			switch key {
			case "Description":
				sawDescription = true
				descLines = []string{value}
			case "Project":
				fields.Project = value
			case "Priority":
				fields.Priority = value
			case "Due":
				fields.Due = value
			case "Tags":
				fields.Tags = parseTags(value)
			}
			continue
		}

		if inDescription {
			descLines = append(descLines, line)
		}
		// Unrecognized lines outside an in-progress Description block are
		// tolerated silently (malformed input should not be a hard error).
	}

	if !sawDescription {
		return EditableFields{}, fmt.Errorf("edit buffer is missing a Description field")
	}
	fields.Description = strings.Join(descLines, "\n")

	return fields, nil
}

// Apply returns a copy of original with its editable fields replaced by
// fields, leaving every read-only reference field (ID, UUID, Status,
// Entry, Modified, End, Urgency) untouched.
func Apply(original taskwarrior.Task, fields EditableFields) taskwarrior.Task {
	updated := original
	updated.Description = fields.Description
	updated.Project = fields.Project
	updated.Priority = fields.Priority
	updated.Due = fields.Due
	updated.Tags = fields.Tags
	return updated
}

// matchKey checks whether line begins with one of editableKeys followed by
// ":", returning the key and the trimmed remainder if so.
func matchKey(line string) (key string, value string, ok bool) {
	for _, k := range editableKeys {
		prefix := k + ":"
		if strings.HasPrefix(line, prefix) {
			return k, strings.TrimSpace(strings.TrimPrefix(line, prefix)), true
		}
	}
	return "", "", false
}

// parseTags splits a comma-separated Tags value into individual tags,
// trimming whitespace and dropping empty entries. Returns nil (not an
// empty slice) when there are no tags, matching taskwarrior.Task's
// omitempty JSON tag.
func parseTags(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	var tags []string
	for _, part := range strings.Split(value, ",") {
		if t := strings.TrimSpace(part); t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

// formatUrgency renders a task's urgency as a plain decimal string,
// omitting a trailing zero value's ".0" the way strconv.FormatFloat's 'g'
// format would but without switching to scientific notation for
// typical urgency magnitudes.
func formatUrgency(u float64) string {
	return strconv.FormatFloat(u, 'f', -1, 64)
}
