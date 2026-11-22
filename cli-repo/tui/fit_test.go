package tui

import (
	"strings"
	"testing"
)

func TestListWindowKeepsCursorVisible(t *testing.T) {
	for _, tc := range []struct{ total, cursor, visible int }{{50, 0, 10}, {50, 49, 10}, {50, 25, 10}, {50, 3, 3}, {5, 2, 10}, {50, 10, 1}, {50, 0, 0}} {
		start, end, above, below := listWindow(tc.total, tc.cursor, tc.visible)
		if tc.cursor < start || tc.cursor >= end {
			t.Fatalf("%+v: cursor outside [%d,%d)", tc, start, end)
		}
		if tc.visible >= 3 && above+below+(end-start) != tc.total {
			t.Fatalf("%+v: rows don't add up", tc)
		}
		drawn := end - start
		if above > 0 {
			drawn++
		}
		if below > 0 {
			drawn++
		}
		if tc.total > tc.visible && drawn > max(tc.visible, 1) {
			t.Fatalf("%+v: %d lines drawn for %d visible", tc, drawn, tc.visible)
		}
	}
}

func TestFitPanelClips(t *testing.T) {
	out := fitPanel("\x1b[31mabcdefghij\x1b[0m\nline2\nline3", 5, 2)
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		t.Fatalf("height: got %d lines: %q", len(lines), out)
	}
	if !strings.Contains(lines[0], "\x1b[31m") || strings.Contains(lines[0], "fghij") {
		t.Fatalf("width: first line not truncated with styling kept: %q", lines[0])
	}
}
