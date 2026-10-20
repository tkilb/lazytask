package statusbar

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
)

func plain(s string) string { return ansi.Strip(s) }

func TestRenderSuggested_EmptyState(t *testing.T) {
	assert.Contains(t, RenderSuggested("", "", "", 40, false), "nothing pending")
}

func TestRenderSuggested_ProjectRightAligned(t *testing.T) {
	out := RenderSuggested("started", "Write report", "Work", 40, false)
	assert.Equal(t, 40, ansi.StringWidth(out))
	p := plain(out)
	assert.True(t, strings.HasPrefix(p, "started: Write report"))
	assert.True(t, strings.HasSuffix(p, "[Work]"))
}

func TestRenderSuggested_LongDescriptionTruncatedBeforeProject(t *testing.T) {
	out := RenderSuggested("urgent", strings.Repeat("long ", 20), "Work", 30, false)
	p := plain(out)
	assert.Equal(t, 30, ansi.StringWidth(out))
	assert.True(t, strings.HasSuffix(p, " [Work]"), "keeps a gap before the project: %q", p)
	assert.Contains(t, p, "…")
}

func TestRenderSuggested_NoProject(t *testing.T) {
	p := plain(RenderSuggested("urgent", "Do it", "", 40, false))
	assert.Equal(t, "urgent: Do it", p)
	assert.NotContains(t, p, "[")
}

func TestRenderSuggested_ProjectTooWideIsDropped(t *testing.T) {
	out := RenderSuggested("urgent", "Do it", "A-very-long-project-name", 20, false)
	assert.NotContains(t, plain(out), "[")
	assert.LessOrEqual(t, ansi.StringWidth(out), 20)
}

func TestRenderSuggested_SelectedFillsWidthWithBackground(t *testing.T) {
	for _, tc := range []struct{ reason, desc, project string }{
		{"", "", ""},
		{"started", "Write report", "Work"},
		{"urgent", "No project", ""},
	} {
		out := RenderSuggested(tc.reason, tc.desc, tc.project, 40, true)
		assert.Equal(t, 40, ansi.StringWidth(out), "%+v", tc)
	}
	sel := RenderSuggested("started", "Write report", "Work", 40, true)
	assert.Equal(t, plain(RenderSuggested("started", "Write report", "Work", 40, false)), plain(sel),
		"selected differs from unselected only by styling")
}
