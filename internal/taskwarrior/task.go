package taskwarrior

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Task represents a task item from Taskwarrior JSON export.
type Task struct {
	ID          int      `json:"id"`
	UUID        string   `json:"uuid"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Project     string   `json:"project,omitempty"`
	Priority    string   `json:"priority,omitempty"`
	Due         string   `json:"due,omitempty"`
	Entry       string   `json:"entry,omitempty"`
	Modified    string   `json:"modified,omitempty"`
	End         string   `json:"end,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Urgency     float64  `json:"urgency,omitempty"`
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
