package asposepdf

import "fmt"

// AddTable renders the table inside the given rectangle. Per the package
// design, cell content is drawn using each cell's TextStyle override (or the
// table-level DefaultCellStyle), with per-cell padding (margin) applied.
// The table is clipped to the bounding rectangle; rows that don't fit are
// not drawn.
//
// Mirrors Aspose.PDF for .NET's flow-layout Table rendering, but uses
// explicit Rectangle positioning (consistent with AddText / AddImage).
func (p *Page) AddTable(t *Table, rect Rectangle) error {
	if t == nil {
		return fmt.Errorf("add table: nil table")
	}
	if err := rect.validate(); err != nil {
		return fmt.Errorf("add table: %w", err)
	}
	if len(t.columnWidths) == 0 {
		// Empty table — nothing to draw.
		return nil
	}
	for i, w := range t.columnWidths {
		if w <= 0 {
			return fmt.Errorf("add table: column %d has non-positive width %g", i, w)
		}
	}
	for i, row := range t.rows {
		if len(row.cells) != len(t.columnWidths) {
			return fmt.Errorf("add table: row %d has %d cells, want %d", i, len(row.cells), len(t.columnWidths))
		}
	}
	if len(t.rows) == 0 {
		return nil
	}
	heights, err := computeRowHeights(t)
	if err != nil {
		return fmt.Errorf("add table: %w", err)
	}

	// Render cells. For each cell, compute its rect and interior, then call
	// AddText (which handles font resolution, encoding, wrap, alignment, clipping).
	y := rect.URY
	for i, row := range t.rows {
		if y-heights[i] < rect.LLY {
			// Row doesn't fit — stop drawing further rows (clipping; Task 9
			// adds a regression test for this).
			break
		}
		x := rect.LLX
		for col, cell := range row.cells {
			colWidth := t.columnWidths[col]
			cellLLX := x
			cellURX := x + colWidth
			cellURY := y
			cellLLY := y - heights[i]

			margin := effectiveCellMargin(t, cell)
			style := effectiveCellStyle(t, cell)

			interior := Rectangle{
				LLX: cellLLX + margin.Left,
				LLY: cellLLY + margin.Bottom,
				URX: cellURX - margin.Right,
				URY: cellURY - margin.Top,
			}
			if interior.URX > interior.LLX && interior.URY > interior.LLY && cell.text != "" {
				if err := p.AddText(cell.text, style, interior); err != nil {
					return fmt.Errorf("add table: row %d col %d text: %w", i, col, err)
				}
			}
			x += colWidth
		}
		y -= heights[i]
	}

	return nil
}

// computeRowHeights returns the drawn height of each row in t.
//
// For rows with an explicit SetHeight > 0, the explicit value is returned.
// For rows with auto-fit (height == 0), the height is the max of cell content
// heights in the row, where each cell's content height is:
//
//	lines * (fontSize * lineSpacing) + margin.Top + margin.Bottom
//
// Lines come from measureText against the column's interior width
// (column width - margin.Left - margin.Right).
func computeRowHeights(t *Table) ([]float64, error) {
	heights := make([]float64, len(t.rows))
	for i, row := range t.rows {
		if row.height > 0 {
			heights[i] = row.height
			continue
		}
		maxH := 0.0
		for col, cell := range row.cells {
			colWidth := t.columnWidths[col]
			margin := effectiveCellMargin(t, cell)
			style := effectiveCellStyle(t, cell)
			interiorWidth := colWidth - margin.Left - margin.Right
			if interiorWidth < 0 {
				interiorWidth = 0
			}
			lines, lineHeight, err := measureText(cell.text, style, interiorWidth)
			if err != nil {
				return nil, fmt.Errorf("row %d col %d: %w", i, col, err)
			}
			cellH := float64(lines)*lineHeight + margin.Top + margin.Bottom
			if cellH > maxH {
				maxH = cellH
			}
		}
		heights[i] = maxH
	}
	return heights, nil
}

// effectiveCellMargin returns the per-cell margin, falling back to the table
// default if the cell has no override.
func effectiveCellMargin(t *Table, c *Cell) MarginInfo {
	if c.margin != nil {
		return *c.margin
	}
	return t.defaultCellMargin
}

// effectiveCellStyle returns the resolved TextStyle for a cell, layering:
// table.defaultCellStyle ← cell.style overlay ← cell H/V align overrides.
func effectiveCellStyle(t *Table, c *Cell) TextStyle {
	style := t.defaultCellStyle
	if c.style != nil {
		style = overlayTextStyle(style, *c.style)
	}
	if c.hAlignSet {
		style.HAlign = c.hAlign
	}
	if c.vAlignSet {
		style.VAlign = c.vAlign
	}
	return style
}

// overlayTextStyle returns base with every non-zero field of overlay applied
// on top. Zero-value fields in overlay leave base unchanged.
//
// Field list mirrors the TextStyle declared in color.go (Font, Size, Color,
// Background, HAlign, VAlign, LineSpacing, Underline, Strikethrough, Rotation, Behind).
func overlayTextStyle(base, overlay TextStyle) TextStyle {
	out := base
	if overlay.Font != nil {
		out.Font = overlay.Font
	}
	if overlay.Size != 0 {
		out.Size = overlay.Size
	}
	if overlay.Color != nil {
		out.Color = overlay.Color
	}
	if overlay.Background != nil {
		out.Background = overlay.Background
	}
	if overlay.HAlign != 0 {
		out.HAlign = overlay.HAlign
	}
	if overlay.VAlign != 0 {
		out.VAlign = overlay.VAlign
	}
	if overlay.LineSpacing != 0 {
		out.LineSpacing = overlay.LineSpacing
	}
	if overlay.Underline {
		out.Underline = true
	}
	if overlay.Strikethrough {
		out.Strikethrough = true
	}
	if overlay.Rotation != 0 {
		out.Rotation = overlay.Rotation
	}
	if overlay.Behind {
		out.Behind = true
	}
	return out
}
