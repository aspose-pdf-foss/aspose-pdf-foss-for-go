package asposepdf

import (
	"strings"
	"testing"
)

func TestGroupFragmentsIntoLines(t *testing.T) {
	frags := []textFragment{
		{x: 100, y: 50, endX: 150, fontName: "/Helvetica", fontSize: 12},   // footer
		{x: 100, y: 700, endX: 160, fontName: "/Helvetica", fontSize: 12},  // line 1
		{x: 170, y: 700, endX: 230, fontName: "/Helvetica", fontSize: 12},  // line 1 continued
		{x: 100, y: 680, endX: 180, fontName: "/Helvetica", fontSize: 12},  // line 2
	}
	frags[0].text.WriteString("Footer")
	frags[1].text.WriteString("Hello")
	frags[2].text.WriteString("World")
	frags[3].text.WriteString("Second line")

	lines := groupFragmentsIntoLines(frags)

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	// First line should be y=700 (top of page).
	if !strings.Contains(lines[0].Text, "Hello") {
		t.Errorf("line 0: %q, expected Hello", lines[0].Text)
	}
	if !strings.Contains(lines[0].Text, "World") {
		t.Errorf("line 0: %q, expected World", lines[0].Text)
	}
	// Second line y=680.
	if !strings.Contains(lines[1].Text, "Second line") {
		t.Errorf("line 1: %q, expected 'Second line'", lines[1].Text)
	}
	// Last line is footer y=50.
	if !strings.Contains(lines[2].Text, "Footer") {
		t.Errorf("line 2: %q, expected Footer", lines[2].Text)
	}
}

func TestGroupFragmentsEmpty(t *testing.T) {
	lines := groupFragmentsIntoLines(nil)
	if len(lines) != 0 {
		t.Errorf("expected 0 lines, got %d", len(lines))
	}
}

func TestGroupFragmentsSpaceInsertion(t *testing.T) {
	// Two fragments on same line with a gap — should get a space.
	frags := []textFragment{
		{x: 100, y: 700, endX: 140, fontName: "/Helvetica", fontSize: 12},
		{x: 150, y: 700, endX: 200, fontName: "/Helvetica", fontSize: 12},
	}
	frags[0].text.WriteString("Hello")
	frags[1].text.WriteString("World")

	lines := groupFragmentsIntoLines(frags)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0].Text != "Hello World" {
		t.Errorf("text=%q, want 'Hello World'", lines[0].Text)
	}
}

func TestGroupFragmentsNoSpuriousSpace(t *testing.T) {
	// Two fragments on same line with no gap — no space.
	frags := []textFragment{
		{x: 100, y: 700, endX: 140, fontName: "/Helvetica", fontSize: 12},
		{x: 140, y: 700, endX: 180, fontName: "/Helvetica", fontSize: 12},
	}
	frags[0].text.WriteString("Hel")
	frags[1].text.WriteString("lo")

	lines := groupFragmentsIntoLines(frags)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0].Text != "Hello" {
		t.Errorf("text=%q, want 'Hello'", lines[0].Text)
	}
}

func TestCleanFontName(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"/MCEFGG+Garamond-Bold", "Garamond-Bold"},
		{"/Helvetica", "Helvetica"},
		{"/ABCDEF+Arial-BoldMT", "Arial-BoldMT"},
		{"", ""},
	}
	for _, tt := range tests {
		got := cleanFontName(tt.in)
		if got != tt.want {
			t.Errorf("cleanFontName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
