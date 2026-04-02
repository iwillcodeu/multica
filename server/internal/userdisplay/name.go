package userdisplay

import (
	"errors"
	"strings"
	"unicode"
)

var (
	ErrEmptyDisplayName = errors.New("name is required")
	ErrDisplayNameLong  = errors.New("name is too long (max 12 units; Han counts as 2)")
)

// DisplayNameLengthUnits returns a size score: Han ideographs count as 2, other runes as 1.
// This matches "12 ASCII letters or 6 Han characters" style limits.
func DisplayNameLengthUnits(s string) int {
	units := 0
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			units += 2
		} else {
			units += 1
		}
	}
	return units
}

// ValidateDisplayName checks trim, non-empty, and max 12 units (Han = 2).
func ValidateDisplayName(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return ErrEmptyDisplayName
	}
	if DisplayNameLengthUnits(s) > 12 {
		return ErrDisplayNameLong
	}
	return nil
}

// TruncateToMaxUnits shortens s to fit within 12 display units (Han counts as 2).
func TruncateToMaxUnits(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	units := 0
	for _, r := range s {
		cost := 1
		if unicode.Is(unicode.Han, r) {
			cost = 2
		}
		if units+cost > 12 {
			break
		}
		b.WriteRune(r)
		units += cost
	}
	return strings.TrimSpace(b.String())
}
