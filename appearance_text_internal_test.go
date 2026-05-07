package asposepdf

import (
	"strings"
	"testing"
)

func TestDrawRoundedRect(t *testing.T) {
	b := newAppearanceBuilder()
	drawRoundedRect(b, 0, 0, 100, 50, 5)
	out := string(b.Bytes())
	// Should contain: 1 m + 4 c (corner arcs) + 4 l (sides) + 1 h.
	if strings.Count(out, " m\n") != 1 {
		t.Errorf("expected 1 m op, got %d in %q", strings.Count(out, " m\n"), out)
	}
	if strings.Count(out, " c\n") != 4 {
		t.Errorf("expected 4 c ops, got %d in %q", strings.Count(out, " c\n"), out)
	}
	if strings.Count(out, " l\n") != 4 {
		t.Errorf("expected 4 l ops, got %d in %q", strings.Count(out, " l\n"), out)
	}
	if !strings.HasSuffix(out, "h\n") {
		t.Errorf("expected h close, got %q", out)
	}
}

func TestDrawRoundedRectClampsRadius(t *testing.T) {
	// Radius larger than half-dimension should clamp.
	b := newAppearanceBuilder()
	drawRoundedRect(b, 0, 0, 10, 10, 100)
	out := string(b.Bytes())
	if strings.Count(out, " c\n") != 4 {
		t.Errorf("expected 4 c ops even with clamped radius, got %d", strings.Count(out, " c\n"))
	}
}
