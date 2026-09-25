package taskwarrior

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTasks(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		wantTasks []Task
		wantErr   bool
	}{
		{
			name:      "empty input",
			input:     []byte(""),
			wantTasks: []Task{},
			wantErr:   false,
		},
		{
			name:      "whitespace only input",
			input:     []byte("   \n\t  "),
			wantTasks: []Task{},
			wantErr:   false,
		},
		{
			name:      "empty json array",
			input:     []byte("[]"),
			wantTasks: []Task{},
			wantErr:   false,
		},
		{
			name:  "single task fully populated",
			input: []byte(`[{"id":1,"uuid":"ebeeab00-ccf8-464b-8b58-f7f2d606edfb","description":"Fix login bug","status":"pending","project":"Core","priority":"H","due":"20260920T000000Z","entry":"20260916T120000Z","modified":"20260916T123000Z","tags":["bug","urgent"],"urgency":12.5}]`),
			wantTasks: []Task{
				{
					ID:          1,
					UUID:        "ebeeab00-ccf8-464b-8b58-f7f2d606edfb",
					Description: "Fix login bug",
					Status:      "pending",
					Project:     "Core",
					Priority:    "H",
					Due:         "20260920T000000Z",
					Entry:       "20260916T120000Z",
					Modified:    "20260916T123000Z",
					Tags:        []string{"bug", "urgent"},
					Urgency:     12.5,
				},
			},
			wantErr: false,
		},
		{
			name:  "task with annotations",
			input: []byte(`[{"id":3,"uuid":"u3","description":"Ship release","status":"pending","annotations":[{"entry":"20260916T120000Z","description":"Waiting on QA sign-off"},{"entry":"20260917T090000Z","description":"Multi-line note\nsecond line"}]}]`),
			wantTasks: []Task{
				{
					ID:          3,
					UUID:        "u3",
					Description: "Ship release",
					Status:      "pending",
					Annotations: []Annotation{
						{Entry: "20260916T120000Z", Description: "Waiting on QA sign-off"},
						{Entry: "20260917T090000Z", Description: "Multi-line note\nsecond line"},
					},
				},
			},
			wantErr: false,
		},
		{
			name:  "multiple tasks with optional fields omitted",
			input: []byte(`[{"id":1,"uuid":"u1","description":"Task one","status":"pending"},{"id":2,"uuid":"u2","description":"Task two","status":"completed","end":"20260916T130000Z"}]`),
			wantTasks: []Task{
				{
					ID:          1,
					UUID:        "u1",
					Description: "Task one",
					Status:      "pending",
				},
				{
					ID:          2,
					UUID:        "u2",
					Description: "Task two",
					Status:      "completed",
					End:         "20260916T130000Z",
				},
			},
			wantErr: false,
		},
		{
			name:      "invalid json syntax",
			input:     []byte(`[{"id": 1, "description": broken]`),
			wantTasks: nil,
			wantErr:   true,
		},
		{
			name:      "json object instead of array",
			input:     []byte(`{"id": 1, "description": "Single"}`),
			wantTasks: nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTasks(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantTasks, got)
		})
	}
}
