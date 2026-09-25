package taskwarrior

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Task represents a task item from Taskwarrior JSON export.
type Task struct {
	ID          int          `json:"id"`
	UUID        string       `json:"uuid"`
	Description string       `json:"description"`
	Status      string       `json:"status"`
	Project     string       `json:"project,omitempty"`
	Priority    string       `json:"priority,omitempty"`
	Due         string       `json:"due,omitempty"`
	Entry       string       `json:"entry,omitempty"`
	Modified    string       `json:"modified,omitempty"`
	End         string       `json:"end,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
	Annotations []Annotation `json:"annotations,omitempty"`
	Urgency     float64      `json:"urgency,omitempty"`
	// UrgencyOffset is a lazytask-managed numeric UDA (see Client.EnsureUDA) that
	// nudges a task's position within the urgency-sorted Tasks panel list,
	// added to Urgency to form the effective sort key. It has no meaning to
	// Taskwarrior itself and defaults to 0 (no manual override).
	UrgencyOffset float64 `json:"urgencyoffset,omitempty"`
}

// Annotation represents a single timestamped note attached to a task, as
// found in Taskwarrior's "annotations" export array. Entry is the raw
// Taskwarrior UTC timestamp (taskDueLayout-formatted, e.g.
// "20240115T140000Z") the annotation was added at.
type Annotation struct {
	Entry       string `json:"entry,omitempty"`
	Description string `json:"description"`
}

// ParseTasks decodes Taskwarrior JSON export bytes into a slice of Task structs.
// Returns an empty slice if the input is empty or contains only whitespace.
func ParseTasks(data []byte) ([]Task, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return []Task{}, nil
	}

	var tasks []Task
	if err := json.Unmarshal(trimmed, &tasks); err != nil {
		return nil, fmt.Errorf("failed to parse taskwarrior export JSON: %w", err)
	}

	if tasks == nil {
		return []Task{}, nil
	}

	return tasks, nil
}
