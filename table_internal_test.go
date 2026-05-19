package asposepdf

import "testing"

func TestMeasureText_SingleLine(t *testing.T) {
	style := TextStyle{Font: FontHelvetica, Size: 12}
	lines, lineHeight, err := measureText("Hello", style, 1000) // wide enough for one line
	if err != nil {
		t.Fatal(err)
	}
	if lines != 1 {
		t.Errorf("lines = %d, want 1", lines)
	}
	if lineHeight <= 0 {
		t.Errorf("lineHeight = %g, want > 0", lineHeight)
	}
}

func TestMeasureText_Wrap(t *testing.T) {
	style := TextStyle{Font: FontHelvetica, Size: 12}
	// 40pt is too narrow for "Hello World" (~ 60pt at 12pt Helvetica) — should wrap.
	lines, _, err := measureText("Hello World", style, 40)
	if err != nil {
		t.Fatal(err)
	}
	if lines < 2 {
		t.Errorf("expected wrap, got lines = %d", lines)
	}
}

func TestMeasureText_Empty(t *testing.T) {
	lines, _, err := measureText("", TextStyle{}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if lines != 0 {
		t.Errorf("lines = %d, want 0 for empty text", lines)
	}
}

func TestMeasureText_DefaultsApplied(t *testing.T) {
	// Font nil → Helvetica; Size 0 → 12; LineSpacing 0 → 1.2.
	lines, lh, err := measureText("Hello", TextStyle{}, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if lines != 1 {
		t.Errorf("lines = %d, want 1", lines)
	}
	// With Size=12, LineSpacing=1.2 → lineHeight = 14.4. Compute via
	// runtime float64 multiplication to match measureText's rounding (the
	// constant expression 12.0*1.2 would be folded at arbitrary precision
	// at compile time and round differently).
	size, spacing := 12.0, 1.2
	want := size * spacing
	if lh != want {
		t.Errorf("lineHeight = %g, want %g", lh, want)
	}
}

func TestComputeRowHeights_Explicit(t *testing.T) {
	table := NewTable().SetColumnWidths([]float64{50})
	table.AddRow().SetHeight(25).AddCell("x")
	table.AddRow().SetHeight(40).AddCell("y")
	heights, err := computeRowHeights(table)
	if err != nil {
		t.Fatal(err)
	}
	if len(heights) != 2 || heights[0] != 25 || heights[1] != 40 {
		t.Errorf("heights = %v, want [25 40]", heights)
	}
}

func TestComputeRowHeights_AutoFitOneLine(t *testing.T) {
	table := NewTable().SetColumnWidths([]float64{200}).
		SetDefaultCellStyle(TextStyle{Font: FontHelvetica, Size: 12}).
		SetDefaultCellMargin(MarginInfo{Top: 4, Right: 6, Bottom: 4, Left: 6})
	table.AddRow().AddCell("Hello") // single line
	heights, err := computeRowHeights(table)
	if err != nil {
		t.Fatal(err)
	}
	// Expected: lineHeight (12 * 1.2 = 14.4) + margin.Top (4) + margin.Bottom (4).
	// Compute via runtime float64 ops to match production rounding (the
	// constant expression 12.0*1.2 would be folded at arbitrary precision
	// at compile time and may round differently than runtime multiplication).
	size, spacing := 12.0, 1.2
	lineHeight := size * spacing
	want := 1.0*lineHeight + 4.0 + 4.0
	if heights[0] != want {
		t.Errorf("auto-fit single-line height = %g, want %g", heights[0], want)
	}
}

func TestComputeRowHeights_AutoFitTallestWins(t *testing.T) {
	table := NewTable().SetColumnWidths([]float64{30, 30}).
		SetDefaultCellStyle(TextStyle{Font: FontHelvetica, Size: 12}).
		SetDefaultCellMargin(MarginInfo{Top: 2, Right: 2, Bottom: 2, Left: 2})
	// First cell: 1 line. Second cell: forced wrap (narrow width).
	table.AddRow().AddCells("Hi", "Hello World Foo Bar")
	heights, err := computeRowHeights(table)
	if err != nil {
		t.Fatal(err)
	}
	// The second cell should wrap to at least 2 lines, making row taller than
	// the single-line cell would dictate.
	want := 2*12.0*1.2 + 4.0
	if heights[0] < want {
		t.Errorf("row height = %g, want >= %g (taller cell wins)", heights[0], want)
	}
}

func TestComputeRowHeights_EmptyCellTextIsZero(t *testing.T) {
	table := NewTable().SetColumnWidths([]float64{50}).
		SetDefaultCellStyle(TextStyle{Font: FontHelvetica, Size: 12}).
		SetDefaultCellMargin(MarginInfo{Top: 3, Bottom: 3})
	table.AddRow().AddCell("") // empty text
	heights, err := computeRowHeights(table)
	if err != nil {
		t.Fatal(err)
	}
	// Empty cell: 0 lines × lineHeight + margin = 6
	if heights[0] != 6 {
		t.Errorf("empty-cell row height = %g, want 6", heights[0])
	}
}
