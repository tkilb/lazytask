package helppopup

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tkilb/lazytask/internal/ui/statusbar"
)

// testScreenHeight is a representative terminal height used by tests that
// don't care about VisibleRows' exact scaling, chosen to give a
// VisibleRows() budget (14) small enough that manyBindingSections(30)
// still overflows and forces scrolling, matching prior test sizing.
const testScreenHeight = 20

func TestBox_ContainsTitleInBorderAndSectionsAndBindings(t *testing.T) {
	out := Box([]Section{
		{Title: "Local", Bindings: []statusbar.Binding{
			{Key: "d", Label: "done"},
		}},
		{Title: "Global", Bindings: []statusbar.Binding{
			{Key: "q", Label: "quit"},
		}},
	}, 100, testScreenHeight, 0)

	for _, want := range []string{"Keybindings", "--- Local ---", "--- Global ---", "q", "quit", "d", "done"} {
		assert.True(t, strings.Contains(out, want), "expected output to contain %q, got %q", want, out)
	}
}

func TestBox_ContainsDismissHint(t *testing.T) {
	out := Box([]Section{{Title: "Global", Bindings: []statusbar.Binding{{Key: "q", Label: "quit"}}}}, 100, testScreenHeight, 0)
	assert.True(t, strings.Contains(out, "close"))
}

func TestBox_RightAlignsKeyColumn(t *testing.T) {
	// "q" (1 char) and "ctrl+j/k" (8 chars) should end at the same
	// column, i.e. "q" gets left-padded to line up with the longer key.
	out := Box([]Section{{Title: "Global", Bindings: []statusbar.Binding{
		{Key: "q", Label: "quit"},
		{Key: "ctrl+j/k", Label: "reorder"},
	}}}, 100, testScreenHeight, 0)

	assert.True(t, strings.Contains(out, "       q  quit"), "expected 'q' to be right-padded/aligned against the longer key, got %q", out)
}

func TestBox_SectionHeaderIndentedUnderDescriptionColumn(t *testing.T) {
	// The header should start after the key column's width (matching
	// lazygit's own menu, where section headers occupy the description
	// column rather than the key column), not flush at column 0.
	out := Box([]Section{{Title: "Global", Bindings: []statusbar.Binding{
		{Key: "ctrl+j/k", Label: "reorder"},
	}}}, 100, testScreenHeight, 0)

	assert.True(t, strings.Contains(out, "          --- Global ---"), "expected the header indented past the key column, got %q", out)
}

func TestBox_EmptySectionsStillRendersTitle(t *testing.T) {
	out := Box(nil, 100, testScreenHeight, 0)
	assert.True(t, strings.Contains(out, "Keybindings"))
}

func TestBox_WidthCappedRegardlessOfTerminalWidth(t *testing.T) {
	narrow := Box([]Section{{Title: "Global", Bindings: []statusbar.Binding{{Key: "q", Label: "quit"}}}}, 200, testScreenHeight, 0)
	wider := Box([]Section{{Title: "Global", Bindings: []statusbar.Binding{{Key: "q", Label: "quit"}}}}, 500, testScreenHeight, 0)
	// Both should be capped at the same maxWidth regardless of how wide
	// the terminal is, so the rendered line lengths should match.
	firstLineWidth := func(s string) int {
		lines := strings.Split(s, "\n")
		return len(lines[0])
	}
	assert.Equal(t, firstLineWidth(narrow), firstLineWidth(wider))
}

func TestBox_WidthShrinksForNarrowTerminal(t *testing.T) {
	sections := []Section{{Title: "Global", Bindings: []statusbar.Binding{{Key: "q", Label: "quit"}}}}
	normal := Box(sections, 100, testScreenHeight, 0)
	narrow := Box(sections, 44, testScreenHeight, 0)

	firstLineWidth := func(s string) int {
		lines := strings.Split(s, "\n")
		return len(lines[0])
	}
	assert.True(t, firstLineWidth(narrow) < firstLineWidth(normal))
}

func TestPadLeft(t *testing.T) {
	assert.Equal(t, "        ab", padLeft("ab", 10))
	assert.Equal(t, "abcdefghij", padLeft("abcdefghij", 10))
}

// manyBindingSections builds n single-binding sections ("Section 0" ..
// "Section n-1", each with one distinctly-labeled key), enough to force
// scrolling in tests regardless of VisibleRows' exact value at
// testScreenHeight.
func manyBindingSections(n int) []Section {
	sections := make([]Section, n)
	for i := range sections {
		sections[i] = Section{
			Title: fmt.Sprintf("Section %d", i),
			Bindings: []statusbar.Binding{
				{Key: fmt.Sprintf("k%d", i), Label: fmt.Sprintf("label-%d", i)},
			},
		}
	}
	return sections
}

func TestVisibleRows_FallsBackToMaxWhenHeightUnknown(t *testing.T) {
	assert.Equal(t, maxVisibleRows, VisibleRows(0))
	assert.Equal(t, maxVisibleRows, VisibleRows(-1))
}

func TestVisibleRows_ScalesWithScreenHeightWithinBounds(t *testing.T) {
	assert.Equal(t, minVisibleRows, VisibleRows(1), "a tiny terminal should still get at least minVisibleRows")
	assert.Equal(t, maxVisibleRows, VisibleRows(1000), "a huge terminal should be capped at maxVisibleRows")

	mid := VisibleRows(20)
	assert.Equal(t, 20-reservedRows, mid)
	assert.True(t, mid > minVisibleRows && mid < maxVisibleRows)
}

func TestMaxScroll_ZeroWhenContentFits(t *testing.T) {
	sections := []Section{{Title: "Global", Bindings: []statusbar.Binding{{Key: "q", Label: "quit"}}}}
	assert.Equal(t, 0, MaxScroll(sections, testScreenHeight))
}

func TestMaxScroll_PositiveWhenContentOverflows(t *testing.T) {
	sections := manyBindingSections(30)
	assert.True(t, MaxScroll(sections, testScreenHeight) > 0)
}

func TestMaxScroll_ZeroOnTallTerminalThatFitsEverything(t *testing.T) {
	// A tall enough terminal should have enough VisibleRows budget (up
	// to maxVisibleRows) to show every section without scrolling.
	sections := manyBindingSections(12)
	assert.Equal(t, 0, MaxScroll(sections, 200))
}

func TestBox_ScrollOffsetChangesVisibleContent(t *testing.T) {
	sections := manyBindingSections(30)
	top := Box(sections, 100, testScreenHeight, 0)
	scrolled := Box(sections, 100, testScreenHeight, MaxScroll(sections, testScreenHeight))

	assert.True(t, strings.Contains(top, "label-0"))
	assert.False(t, strings.Contains(top, fmt.Sprintf("label-%d", len(sections)-1)), "last section shouldn't be visible at the top of a long, unscrolled list")

	assert.True(t, strings.Contains(scrolled, fmt.Sprintf("label-%d", len(sections)-1)), "last section should be visible once scrolled to MaxScroll")
	assert.False(t, strings.Contains(scrolled, "label-0"), "first section shouldn't still be visible once scrolled past it")
}

func TestBox_ScrollOffsetClampedToValidRange(t *testing.T) {
	sections := manyBindingSections(30)
	maxScroll := MaxScroll(sections, testScreenHeight)

	negative := Box(sections, 100, testScreenHeight, -5)
	tooFar := Box(sections, 100, testScreenHeight, maxScroll+100)

	assert.Equal(t, Box(sections, 100, testScreenHeight, 0), negative, "a negative scrollOffset should clamp to 0")
	assert.Equal(t, Box(sections, 100, testScreenHeight, maxScroll), tooFar, "an overlarge scrollOffset should clamp to MaxScroll")
}

func TestBox_ShowsScrollHintOnlyWhenContentOverflows(t *testing.T) {
	short := Box([]Section{{Title: "Global", Bindings: []statusbar.Binding{{Key: "q", Label: "quit"}}}}, 100, testScreenHeight, 0)
	assert.False(t, strings.Contains(short, "scroll"))

	long := Box(manyBindingSections(30), 100, testScreenHeight, 0)
	assert.True(t, strings.Contains(long, "scroll"))
}

func TestBox_TallerTerminalShowsMoreRowsWithoutScrolling(t *testing.T) {
	// Confirms the popup actually grows with the terminal (the bug being
	// fixed here): the same overflowing content should need scrolling on
	// a short terminal but not on a tall one.
	sections := manyBindingSections(12)
	assert.True(t, strings.Contains(Box(sections, 100, testScreenHeight, 0), "scroll"))
	assert.False(t, strings.Contains(Box(sections, 100, 200, 0), "scroll"))
}
