package editbuffer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tkilb/lazytask/internal/taskwarrior"
)

func TestSerializeParseRoundTrip(t *testing.T) {
	task := taskwarrior.Task{
		ID:          7,
		UUID:        "abc-123",
		Description: "Buy milk",
		Status:      "pending",
		Project:     "home",
		Priority:    "H",
		Due:         "20250101T000000Z",
		Entry:       "20241201T000000Z",
		Modified:    "20241202T000000Z",
		Tags:        []string{"errand", "urgent"},
		Urgency:     5.4,
	}

	buf := Serialize(task)
	fields, err := Parse(buf)
	require.NoError(t, err)

	assert.Equal(t, task.Description, fields.Description)
	assert.Equal(t, task.Project, fields.Project)
	assert.Equal(t, task.Priority, fields.Priority)
	assert.Equal(t, task.Due, fields.Due)
	assert.Equal(t, task.Tags, fields.Tags)
}

func TestParse_EmbeddedNewlinesInDescription(t *testing.T) {
	buf := "Description: line one\nline two\nline three\n" +
		"Project: work\n" +
		"Priority: \n" +
		"Due: \n" +
		"Tags: \n" +
		"---\n" +
		"ID: 1\n"

	fields, err := Parse(buf)
	require.NoError(t, err)

	assert.Equal(t, "line one\nline two\nline three", fields.Description)
	assert.Equal(t, "work", fields.Project)
	assert.Nil(t, fields.Tags)
}

func TestParse_MissingBlankOptionalFields(t *testing.T) {
	buf := "Description: just a task\n" +
		"Project: \n" +
		"Priority: \n" +
		"Due: \n" +
		"Tags: \n" +
		"---\n"

	fields, err := Parse(buf)
	require.NoError(t, err)

	assert.Equal(t, "just a task", fields.Description)
	assert.Empty(t, fields.Project)
	assert.Empty(t, fields.Priority)
	assert.Empty(t, fields.Due)
	assert.Nil(t, fields.Tags)
}

func TestParse_TagsWhitespaceTrimmed(t *testing.T) {
	buf := "Description: task\n" +
		"Tags:  one , two ,, three\n" +
		"---\n"

	fields, err := Parse(buf)
	require.NoError(t, err)

	assert.Equal(t, []string{"one", "two", "three"}, fields.Tags)
}

func TestParse_NoDividerStillParsesEditableSection(t *testing.T) {
	buf := "Description: no divider here\nProject: solo\n"

	fields, err := Parse(buf)
	require.NoError(t, err)

	assert.Equal(t, "no divider here", fields.Description)
	assert.Equal(t, "solo", fields.Project)
}

func TestParse_MalformedInputMissingDescriptionErrors(t *testing.T) {
	buf := "Project: work\n---\nID: 1\n"

	_, err := Parse(buf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Description")
}

func TestParse_EditsToReadOnlySectionAreIgnored(t *testing.T) {
	buf := "Description: task\n" +
		"---\n" +
		"ID: 999\n" +
		"UUID: tampered-uuid\n" +
		"Status: completed\n"

	fields, err := Parse(buf)
	require.NoError(t, err)
	assert.Equal(t, "task", fields.Description)
	// EditableFields has no room for ID/UUID/Status at all: the read-only
	// section is structurally impossible to feed back into a re-import.
}

func TestParse_GarbageAfterDividerDoesNotError(t *testing.T) {
	buf := "Description: task\n---\nthis is not a valid key: value\n@@@garbage###\n"

	fields, err := Parse(buf)
	require.NoError(t, err)
	assert.Equal(t, "task", fields.Description)
}

func TestApply_PreservesReadOnlyFields(t *testing.T) {
	original := taskwarrior.Task{
		ID:            7,
		UUID:          "abc-123",
		Status:        "pending",
		Entry:         "20241201T000000Z",
		Modified:      "20241202T000000Z",
		End:           "",
		Urgency:       5.4,
		UrgencyOffset: 3.25,

		Description: "old description",
		Project:     "old project",
	}
	fields := EditableFields{
		Description: "new description",
		Project:     "new project",
		Priority:    "M",
		Due:         "20250101T000000Z",
		Tags:        []string{"a", "b"},
	}

	updated := Apply(original, fields)

	assert.Equal(t, original.ID, updated.ID)
	assert.Equal(t, original.UUID, updated.UUID)
	assert.Equal(t, original.Status, updated.Status)
	assert.Equal(t, original.Entry, updated.Entry)
	assert.Equal(t, original.Modified, updated.Modified)
	assert.Equal(t, original.End, updated.End)
	assert.Equal(t, original.Urgency, updated.Urgency)
	assert.Equal(t, original.UrgencyOffset, updated.UrgencyOffset)

	assert.Equal(t, fields.Description, updated.Description)
	assert.Equal(t, fields.Project, updated.Project)
	assert.Equal(t, fields.Priority, updated.Priority)
	assert.Equal(t, fields.Due, updated.Due)
	assert.Equal(t, fields.Tags, updated.Tags)
}

func TestSerialize_ContainsReadOnlyReferenceSection(t *testing.T) {
	task := taskwarrior.Task{
		ID:            3,
		UUID:          "uuid-1",
		Status:        "pending",
		Entry:         "entry-ts",
		Modified:      "modified-ts",
		End:           "end-ts",
		Urgency:       1,
		UrgencyOffset: 2.5,
	}

	buf := Serialize(task)

	assert.Contains(t, buf, divider)
	assert.Contains(t, buf, "ID: 3")
	assert.Contains(t, buf, "UUID: uuid-1")
	assert.Contains(t, buf, "Status: pending")
	assert.Contains(t, buf, "Entry: entry-ts")
	assert.Contains(t, buf, "Modified: modified-ts")
	assert.Contains(t, buf, "End: end-ts")
	assert.Contains(t, buf, "Urgency: 1")
	assert.Contains(t, buf, "UrgencyOffset: 2.5")
}
