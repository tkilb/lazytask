package dateparse

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParseTime(t *testing.T, layout, value string) time.Time {
	t.Helper()
	tm, err := time.Parse(layout, value)
	require.NoError(t, err)
	return tm
}

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    Cord
		wantErr bool
	}{
		{name: "days", in: "2d", want: Cord{N: 2, Unit: Days}},
		{name: "weeks", in: "1w", want: Cord{N: 1, Unit: Weeks}},
		{name: "business days", in: "2b", want: Cord{N: 2, Unit: BusinessDays}},
		{name: "uppercase unit", in: "3D", want: Cord{N: 3, Unit: Days}},
		{name: "whitespace", in: "  4w  ", want: Cord{N: 4, Unit: Weeks}},
		{name: "multi-digit", in: "10d", want: Cord{N: 10, Unit: Days}},
		{name: "empty", in: "", wantErr: true},
		{name: "unit only", in: "d", wantErr: true},
		{name: "unknown unit", in: "2x", wantErr: true},
		{name: "zero", in: "0d", wantErr: true},
		{name: "negative", in: "-2d", wantErr: true},
		{name: "non numeric", in: "abcd", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.in)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCordResolve(t *testing.T) {
	// Wed 2024-01-10.
	wed := mustParseTime(t, "2006-01-02 15:04", "2024-01-10 09:00")
	// Fri 2024-01-12.
	fri := mustParseTime(t, "2006-01-02 15:04", "2024-01-12 09:00")
	// Sat 2024-01-13.
	sat := mustParseTime(t, "2006-01-02 15:04", "2024-01-13 09:00")
	// Sun 2024-01-14.
	sun := mustParseTime(t, "2006-01-02 15:04", "2024-01-14 09:00")

	tests := []struct {
		name string
		cord Cord
		now  time.Time
		want time.Time
	}{
		{
			name: "days from wednesday",
			cord: Cord{N: 2, Unit: Days},
			now:  wed,
			want: wed.AddDate(0, 0, 2),
		},
		{
			name: "week from wednesday",
			cord: Cord{N: 1, Unit: Weeks},
			now:  wed,
			want: wed.AddDate(0, 0, 7),
		},
		{
			name: "two weeks",
			cord: Cord{N: 2, Unit: Weeks},
			now:  wed,
			want: wed.AddDate(0, 0, 14),
		},
		{
			name: "one business day from friday rolls to monday",
			cord: Cord{N: 1, Unit: BusinessDays},
			now:  fri,
			want: fri.AddDate(0, 0, 3),
		},
		{
			name: "two business days from wednesday stays within week",
			cord: Cord{N: 2, Unit: BusinessDays},
			now:  wed,
			want: wed.AddDate(0, 0, 2),
		},
		{
			name: "one business day from saturday skips weekend entirely",
			cord: Cord{N: 1, Unit: BusinessDays},
			now:  sat,
			want: sat.AddDate(0, 0, 2),
		},
		{
			name: "one business day from sunday skips to monday",
			cord: Cord{N: 1, Unit: BusinessDays},
			now:  sun,
			want: sun.AddDate(0, 0, 1),
		},
		{
			name: "five business days from wednesday spans a weekend",
			cord: Cord{N: 5, Unit: BusinessDays},
			now:  wed,
			// Thu, Fri, (skip Sat/Sun), Mon, Tue, Wed -> +7 calendar days.
			want: wed.AddDate(0, 0, 7),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.cord.Resolve(tc.now)
			assert.True(t, got.Equal(tc.want), "got %v, want %v", got, tc.want)
		})
	}
}

func TestResolveString(t *testing.T) {
	now := mustParseTime(t, "2006-01-02 15:04", "2024-01-10 09:00")

	got, err := ResolveString("2d", now)
	require.NoError(t, err)
	assert.True(t, got.Equal(now.AddDate(0, 0, 2)))

	_, err = ResolveString("bogus", now)
	assert.Error(t, err)
}
