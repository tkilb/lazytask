package panel

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRender_ContainsTitleAndBody(t *testing.T) {
	view := Render("Status", "hello world", 40, 10, false)

	assert.Contains(t, view, "Status")
	assert.Contains(t, view, "hello world")
}

func TestRender_ClampsToMinimumSize(t *testing.T) {
	// Absurdly small dimensions shouldn't panic or produce something
	// unusable; the panel should clamp to its minimum footprint.
	view := Render("Tiny", "x", 1, 1, false)

	assert.NotEmpty(t, view)
	assert.True(t, strings.Contains(view, "Tiny"))
}

func TestBorderStyle_FocusChangesColor(t *testing.T) {
	focused := BorderStyle(true).GetBorderTopForeground()
	unfocused := BorderStyle(false).GetBorderTopForeground()

	assert.NotEqual(t, focused, unfocused)
}

func TestTitleStyle_FocusChangesColor(t *testing.T) {
	focused := TitleStyle(true).GetForeground()
	unfocused := TitleStyle(false).GetForeground()

	assert.NotEqual(t, focused, unfocused)
}

func TestFrameTabs_ContainsAllTabsAndBody(t *testing.T) {
	tabs := []Tab{{Label: "Todo", Active: true}, {Label: "Done"}, {Label: "Deleted"}}
	view := FrameTabs('2', tabs, "hello world", 60, 10, false)

	assert.Contains(t, view, "[2]-Todo - Done - Deleted")
	assert.Contains(t, view, "hello world")
}

func TestFrameTabs_FallsBackToPlainTitleWhenTooNarrow(t *testing.T) {
	tabs := []Tab{{Label: "Todo", Active: true}, {Label: "Done"}, {Label: "Deleted"}}
	view := FrameTabs('2', tabs, "x", 5, 3, false)

	assert.NotEmpty(t, view)
}
