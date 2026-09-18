package statusbar

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRender_Empty(t *testing.T) {
	assert.Equal(t, "", Render(nil))
	assert.Equal(t, "", Render([]Binding{}))
}

func TestRender_ContainsKeysAndLabels(t *testing.T) {
	out := Render([]Binding{
		{Key: "a", Label: "add"},
		{Key: "d", Label: "done"},
		{Key: "q", Label: "quit"},
	})

	for _, want := range []string{"a", "add", "d", "done", "q", "quit"} {
		assert.True(t, strings.Contains(out, want), "expected output to contain %q, got %q", want, out)
	}
}

func TestRender_SeparatesEntries(t *testing.T) {
	out := Render([]Binding{
		{Key: "a", Label: "add"},
		{Key: "d", Label: "done"},
	})

	// Two entries joined by the separator means splitting on it should
	// yield exactly two chunks.
	assert.Len(t, strings.Split(out, separator), 2)
}

func TestRender_SingleBindingHasNoSeparator(t *testing.T) {
	out := Render([]Binding{{Key: "q", Label: "quit"}})
	assert.False(t, strings.Contains(out, separator))
}
