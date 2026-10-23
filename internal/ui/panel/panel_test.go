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

func TestCenteredScrollWindow_FitsWithoutScrolling(t *testing.T) {
	start, end := CenteredScrollWindow(2, 3, 10)
	assert.Equal(t, 0, start)
	assert.Equal(t, 3, end)
}

func TestCenteredScrollWindow_CursorInMiddle(t *testing.T) {
	start, end := CenteredScrollWindow(10, 20, 5)
	assert.Equal(t, 8, start, "two rows above the cursor")
	assert.Equal(t, 13, end, "two rows below the cursor")
}

func TestCenteredScrollWindow_ClampsAtEnds(t *testing.T) {
	start, end := CenteredScrollWindow(0, 20, 5)
	assert.Equal(t, 0, start)
	assert.Equal(t, 5, end)
	start, end = CenteredScrollWindow(1, 20, 5)
	assert.Equal(t, 0, start)
	assert.Equal(t, 5, end)
	start, end = CenteredScrollWindow(19, 20, 5)
	assert.Equal(t, 15, start)
	assert.Equal(t, 20, end)
}

func TestCenteredScrollWindow_CursorAlwaysWithinWindow(t *testing.T) {
	const total = 37
	for _, visible := range []int{1, 2, 5, 6} {
		for cursor := 0; cursor < total; cursor++ {
			start, end := CenteredScrollWindow(cursor, total, visible)
			assert.GreaterOrEqual(t, cursor, start)
			assert.Less(t, cursor, end)
			assert.Equal(t, visible, end-start)
		}
	}
}

func TestTabAt_MapsColumnsToTabs(t *testing.T) {
	tabs := []Tab{{Label: "Todo"}, {Label: "Done"}, {Label: "Deleted"}}
	// "╭──[2]-Todo - Done - Deleted": Todo at 7-10, Done at 14-17, Deleted at 21-27.
	tests := []struct {
		x    int
		want int
		ok   bool
	}{
		{0, 0, false}, {6, 0, false},
		{7, 0, true}, {10, 0, true}, {11, 0, false}, {13, 0, false},
		{14, 1, true}, {17, 1, true},
		{21, 2, true}, {27, 2, true}, {28, 0, false},
	}
	for _, tt := range tests {
		got, ok := TabAt(tabs, 60, tt.x)
		assert.Equal(t, tt.ok, ok, "x=%d", tt.x)
		if tt.ok {
			assert.Equal(t, tt.want, got, "x=%d", tt.x)
		}
	}
}

func TestTabAt_NarrowPanelHasNoTabs(t *testing.T) {
	_, ok := TabAt([]Tab{{Label: "Todo"}, {Label: "Done"}, {Label: "Deleted"}}, 10, 8)
	assert.False(t, ok)
}

func TestListRowAt_FollowsScrollWindow(t *testing.T) {
	// Focused: cursor at the end of 10 items in 4 rows shows items 6-9.
	i, ok := ListRowAt(0, 9, 10, 4, true)
	assert.True(t, ok)
	assert.Equal(t, 6, i)
	// Unfocused windows are centered on the cursor: cursor 5 shows 3-6.
	i, ok = ListRowAt(3, 5, 10, 4, false)
	assert.True(t, ok)
	assert.Equal(t, 6, i)
	_, ok = ListRowAt(4, 5, 10, 4, false)
	assert.False(t, ok)
	_, ok = ListRowAt(-1, 0, 10, 4, true)
	assert.False(t, ok)
	_, ok = ListRowAt(2, 0, 2, 4, true)
	assert.False(t, ok, "past the last item")
}
