package editbuffer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tkilb/lazytask/internal/taskwarrior"
)

func TestSerialize_DisplaysDueAsYYYYMMDD(t *testing.T) {
	task := taskwarrior.Task{Description: "Buy milk", Due: "20250101T000000Z"}

	buf := Serialize(task)

	wantDue, err := time.Parse(taskDueLayout, task.Due)
	require.NoError(t, err)
	assert.Contains(t, buf, "Due: "+wantDue.Local().Format(displayDueLayout)+"\n")
}

func TestSerialize_BlankDueStaysBlank(t *testing.T) {
	task := taskwarrior.Task{Description: "Buy milk"}

	buf := Serialize(task)

	assert.Contains(t, buf, "Due: \n")
}

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
		Annotations: []taskwarrior.Annotation{
			{Entry: "20241201T010000Z", Description: "call the store"},
			{Entry: "20241201T020000Z", Description: "2% milk"},
		},
		Urgency: 5.4,
	}

	buf := Serialize(task)
	fields, err := Parse(buf)
	require.NoError(t, err)

	assert.Equal(t, task.Description, fields.Description)
	assert.Equal(t, task.Project, fields.Project)
	assert.Equal(t, task.Priority, fields.Priority)
	assert.Equal(t, []string{"call the store", "2% milk"}, fields.Annotations)
	// The Due field is round-tripped through its human-friendly
	// YYYY-MM-DD display form (see displayDue), not the raw taskwarrior
	// timestamp, so compare against that same conversion rather than the
	// raw value.
	wantDue, err := time.Parse(taskDueLayout, task.Due)
	require.NoError(t, err)
	assert.Equal(t, wantDue.Local().Format(displayDueLayout), fields.Due)
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

func TestParse_PriorityNormalization(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"H", "H"},
		{"h", "H"},
		{"HIGH", "H"},
		{"high", "H"},
		{"High", "H"},
		{"M", "M"},
		{"m", "M"},
		{"med", "M"},
		{"MED", "M"},
		{"medium", "M"},
		{"Medium", "M"},
		{"L", "L"},
		{"l", "L"},
		{"low", "L"},
		{"LOW", "L"},
		{"", ""},
		{"  ", ""},
		{"  m  ", "M"},
	}

	for _, tc := range cases {
		buf := "Description: task\n" +
			"Priority: " + tc.input + "\n" +
			"---\n"

		fields, err := Parse(buf)
		require.NoError(t, err)
		assert.Equal(t, tc.want, fields.Priority, "input %q", tc.input)
	}
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

func TestParse_AnnotationsSection(t *testing.T) {
	buf := "Description: task\n" +
		"Annotations: first note\n" +
		"  second note\n" +
		"Project: home\n" +
		"---\n"

	fields, err := Parse(buf)
	require.NoError(t, err)
	assert.Equal(t, []string{"first note", "second note"}, fields.Annotations)
	assert.Equal(t, "home", fields.Project)
}

func TestParse_AnnotationsSectionEmpty(t *testing.T) {
	buf := "Description: task\n" +
		"Annotations:\n" +
		"Project: home\n" +
		"---\n"

	fields, err := Parse(buf)
	require.NoError(t, err)
	assert.Nil(t, fields.Annotations)
}

func TestParse_AnnotationsEmbeddedNewlineEscaped(t *testing.T) {
	buf := `Description: task
Annotations: line one\nline two
---
`

	fields, err := Parse(buf)
	require.NoError(t, err)
	require.Len(t, fields.Annotations, 1)
	assert.Equal(t, "line one\nline two", fields.Annotations[0])
}

func TestParse_AnnotationsHeaderInlineTextIsFirstAnnotation(t *testing.T) {
	// Annotations reads like every other "Key: value" field: the first
	// annotation's text goes directly after the colon on the header line
	// itself, not on a separate bullet line, so it must end the
	// Description continuation block (rather than being silently
	// absorbed into Description, which was a real bug fixed alongside
	// this format) and be captured as the first annotation's text.
	buf := "Description: task\n" +
		"Annotations: foo\n" +
		"Project: home\n" +
		"---\n"

	fields, err := Parse(buf)
	require.NoError(t, err)
	assert.Equal(t, "task", fields.Description)
	assert.Equal(t, []string{"foo"}, fields.Annotations)
	assert.Equal(t, "home", fields.Project)
}

func TestParse_BlankContinuationLineIsDroppedNotEmptyAnnotation(t *testing.T) {
	// A blank (or whitespace-only) continuation line under Annotations —
	// e.g. left over from editing — must be dropped rather than becoming
	// a phantom empty-string annotation.
	buf := "Description: task\n" +
		"Annotations: foo\n" +
		"  \n" +
		"Project: home\n" +
		"---\n"

	fields, err := Parse(buf)
	require.NoError(t, err)
	assert.Equal(t, []string{"foo"}, fields.Annotations)
}

func TestSerialize_AnnotationsEmbeddedNewlineEscaped(t *testing.T) {
	task := taskwarrior.Task{
		Description: "task",
		Annotations: []taskwarrior.Annotation{{Description: "line one\nline two"}},
	}

	buf := Serialize(task)

	assert.Contains(t, buf, `Annotations: line one\nline two`+"\n")
}

func TestApply_AnnotationsPreservesEntryForUnchangedText(t *testing.T) {
	original := taskwarrior.Task{
		Annotations: []taskwarrior.Annotation{
			{Entry: "20241201T010000Z", Description: "unchanged note"},
			{Entry: "20241201T020000Z", Description: "old text"},
		},
	}
	fields := EditableFields{
		Description: "task",
		Annotations: []string{"unchanged note", "edited text", "brand new note"},
	}

	updated := Apply(original, fields)

	require.Len(t, updated.Annotations, 3)
	assert.Equal(t, taskwarrior.Annotation{Entry: "20241201T010000Z", Description: "unchanged note"}, updated.Annotations[0])
	assert.Equal(t, taskwarrior.Annotation{Entry: "", Description: "edited text"}, updated.Annotations[1])
	assert.Equal(t, taskwarrior.Annotation{Entry: "", Description: "brand new note"}, updated.Annotations[2])
}

func TestApply_AnnotationsRemovedWhenDeletedFromBuffer(t *testing.T) {
	original := taskwarrior.Task{
		Annotations: []taskwarrior.Annotation{
			{Entry: "20241201T010000Z", Description: "keep me"},
			{Entry: "20241201T020000Z", Description: "delete me"},
		},
	}
	fields := EditableFields{
		Description: "task",
		Annotations: []string{"keep me"},
	}

	updated := Apply(original, fields)

	require.Len(t, updated.Annotations, 1)
	assert.Equal(t, "keep me", updated.Annotations[0].Description)
}

func TestApply_AnnotationsAllDeletedYieldsNil(t *testing.T) {
	original := taskwarrior.Task{
		Annotations: []taskwarrior.Annotation{{Description: "gone"}},
	}
	fields := EditableFields{Description: "task"}

	updated := Apply(original, fields)

	assert.Nil(t, updated.Annotations)
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
