package asposepdf

import "testing"

func TestBeveledColorPair(t *testing.T) {
	base := Color{R: 0.5, G: 0.5, B: 0.5, A: 1}
	light, dark := beveledColorPair(base, false)
	// Light = 50% blend with white → all channels 0.75
	if light.R != 0.75 || light.G != 0.75 || light.B != 0.75 {
		t.Errorf("light = %+v, want {0.75 0.75 0.75 1}", light)
	}
	// Dark = base * 0.5 → all channels 0.25
	if dark.R != 0.25 || dark.G != 0.25 || dark.B != 0.25 {
		t.Errorf("dark = %+v, want {0.25 0.25 0.25 1}", dark)
	}
}

func TestBeveledColorPairInverted(t *testing.T) {
	// Inverted = Inset style — light/dark swapped.
	base := Color{R: 0.5, G: 0.5, B: 0.5, A: 1}
	light, dark := beveledColorPair(base, true)
	if light.R != 0.25 {
		t.Errorf("inverted light.R = %v, want 0.25 (Inset swaps)", light.R)
	}
	if dark.R != 0.75 {
		t.Errorf("inverted dark.R = %v, want 0.75 (Inset swaps)", dark.R)
	}
}
