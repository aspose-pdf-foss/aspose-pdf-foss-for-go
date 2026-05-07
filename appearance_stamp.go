package asposepdf

// drawRoundedRect adds a closed rounded-rectangle subpath to the
// builder. Corner radius is clamped to min(w/2, h/2). Geometry: m at
// bottom-edge (just past the bottom-left corner), then 4 cubic Beziers
// for the corners interleaved with 4 line segments for the sides,
// closed with h.
func drawRoundedRect(b *appearanceBuilder, x, y, w, h, radius float64) {
	r := radius
	if r > w/2 {
		r = w / 2
	}
	if r > h/2 {
		r = h / 2
	}
	rk := r * kappa // control-point distance for quarter-circle Bezier

	// Start at bottom-edge, just past the bottom-left corner.
	b.MoveTo(x+r, y)
	// Bottom edge to bottom-right corner start.
	b.LineTo(x+w-r, y)
	// Bottom-right corner.
	b.CurveTo(x+w-r+rk, y, x+w, y+r-rk, x+w, y+r)
	// Right edge.
	b.LineTo(x+w, y+h-r)
	// Top-right corner.
	b.CurveTo(x+w, y+h-r+rk, x+w-r+rk, y+h, x+w-r, y+h)
	// Top edge.
	b.LineTo(x+r, y+h)
	// Top-left corner.
	b.CurveTo(x+r-rk, y+h, x, y+h-r+rk, x, y+h-r)
	// Left edge.
	b.LineTo(x, y+r)
	// Bottom-left corner.
	b.CurveTo(x, y+r-rk, x+r-rk, y, x+r, y)
	b.ClosePath()
}
