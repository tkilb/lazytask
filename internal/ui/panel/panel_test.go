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

func TestFrame_WithFooter_EmbedsFooterInBottomBorder(t *testing.T) {
	view := Frame("Status", "hello", 40, 10, false, "3/10")

	lines := strings.Split(view, "\n")
	assert.Contains(t, lines[len(lines)-1], "3/10")
}

func TestFrame_NoFooter_OmitsFooter(t *testing.T) {
	view := Frame("Status", "hello", 40, 10, false)

	lines := strings.Split(view, "\n")
	assert.NotContains(t, lines[len(lines)-1], "/")
}

func TestFrame_FooterTooWideIsDropped(t *testing.T) {
	view := Frame("Status", "hello", 10, 5, false, "way-too-long-footer-for-this-width")

	lines := strings.Split(view, "\n")
	assert.NotContains(t, lines[len(lines)-1], "way-too-long")
	assert.NotEmpty(t, lines[len(lines)-1])
}

func TestFrameTabs_WithFooter_EmbedsFooterInBottomBorder(t *testing.T) {
	tabs := []Tab{{Label: "Todo", Active: true}, {Label: "Done"}, {Label: "Deleted"}}
	view := FrameTabs('2', tabs, "hello", 60, 10, false, "1/1")

	lines := strings.Split(view, "\n")
	assert.Contains(t, lines[len(lines)-1], "1/1")
}

func TestScrollWindow_FitsWithoutScrolling(t *testing.T) {
	start, end := ScrollWindow(0, 3, 10)
	assert.Equal(t, 0, start)
	assert.Equal(t, 3, end)
}

func TestScrollWindow_CursorAtTop(t *testing.T) {
	start, end := ScrollWindow(0, 20, 5)
	assert.Equal(t, 0, start)
	assert.Equal(t, 5, end)
}

func TestScrollWindow_CursorScrollsWindowDown(t *testing.T) {
	start, end := ScrollWindow(9, 20, 5)
	assert.Equal(t, 5, start)
	assert.Equal(t, 10, end)
}

func TestScrollWindow_CursorNearEndClampsToMaxStart(t *testing.T) {
	start, end := ScrollWindow(19, 20, 5)
	assert.Equal(t, 15, start)
	assert.Equal(t, 20, end)
}

func TestScrollWindow_CursorAlwaysWithinWindow(t *testing.T) {
	const total = 37
	const visible = 6
	for cursor := 0; cursor < total; cursor++ {
		start, end := ScrollWindow(cursor, total, visible)
		assert.GreaterOrEqual(t, cursor, start, "cursor %d below window start %d", cursor, start)
		assert.Less(t, cursor, end, "cursor %d not below window end %d", cursor, end)
		assert.LessOrEqual(t, end-start, visible)
	}
}

func TestScrollWindow_ZeroItems(t *testing.T) {
	start, end := ScrollWindow(0, 0, 5)
	assert.Equal(t, 0, start)
	assert.Equal(t, 0, end)
}
