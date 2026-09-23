// Package dateparse implements the "cord" shorthand used by lazytask's
// quick due-date picker: short strings like "2d", "1w", or "2b" that
// resolve to a concrete date/time relative to a reference instant (usually
// time.Now()).
package dateparse

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Unit identifies the trailing letter of a cord.
type Unit byte

const (
	// Days advances by N calendar days.
	Days Unit = 'd'
	// Weeks advances by N*7 calendar days.
	Weeks Unit = 'w'
	// BusinessDays advances by N weekdays, skipping Saturdays and Sundays.
	BusinessDays Unit = 'b'
)

// Cord is a parsed "<N><unit>" shorthand, e.g. "2d", "1w", "2b".
type Cord struct {
	N    int
	Unit Unit
}

// Parse parses s (case-insensitive, surrounding whitespace ignored) into a
// Cord. s must be one or more digits followed by exactly one of 'd', 'w',
// or 'b'. N must be a positive integer.
func Parse(s string) (Cord, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Cord{}, fmt.Errorf("dateparse: empty cord")
	}

	unitByte := s[len(s)-1] | 0x20 // lowercase the trailing ASCII letter
	digits := s[:len(s)-1]

	n, err := strconv.Atoi(digits)
	if err != nil {
		return Cord{}, fmt.Errorf("dateparse: %q: expected digits followed by d/w/b", s)
	}
	if n <= 0 {
		return Cord{}, fmt.Errorf("dateparse: %q: N must be positive", s)
	}

	switch Unit(unitByte) {
	case Days, Weeks, BusinessDays:
		return Cord{N: n, Unit: Unit(unitByte)}, nil
	default:
		return Cord{}, fmt.Errorf("dateparse: %q: unknown unit %q, expected d/w/b", s, string(rune(unitByte)))
	}
}

// Resolve resolves the cord to a concrete time relative to now.
func (c Cord) Resolve(now time.Time) time.Time {
	switch c.Unit {
	case Weeks:
		return now.AddDate(0, 0, 7*c.N)
	case BusinessDays:
		t := now
		remaining := c.N
		for remaining > 0 {
			t = t.AddDate(0, 0, 1)
			if isBusinessDay(t) {
				remaining--
			}
		}
		return t
	default: // Days
		return now.AddDate(0, 0, c.N)
	}
}

// ResolveString parses s and resolves it relative to now in one step.
func ResolveString(s string, now time.Time) (time.Time, error) {
	c, err := Parse(s)
	if err != nil {
		return time.Time{}, err
	}
	return c.Resolve(now), nil
}

func isBusinessDay(t time.Time) bool {
	wd := t.Weekday()
	return wd != time.Saturday && wd != time.Sunday
}
