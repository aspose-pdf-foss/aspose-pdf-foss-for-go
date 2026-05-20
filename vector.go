package asposepdf

// LineCap and LineJoin enums (LineCapButt/Round/Square, LineJoinMiter/Round/Bevel)
// are declared in appearance_builder.go. They are reused here for the public
// vector graphics surface; values match PDF operators J (§8.4.3.3) and j (§8.4.3.4).

// LineStyle describes how a stroked path is drawn.
//
// Zero value: black, 0pt wide (no stroke), solid, butt cap, miter join.
// Mirrors Aspose.PDF for .NET's GraphInfo stroke fields.
type LineStyle struct {
	Color       *Color    // nil → black {0,0,0,1}
	Width       float64   // ≤ 0 → no stroke (the draw call becomes a no-op for stroke)
	DashPattern []float64 // [on, off, on, off, ...]; nil or empty → solid
	DashPhase   float64   // offset into the dash pattern, default 0
	Cap         LineCap   // default LineCapButt (see appearance_builder.go); per ISO 32000-1 §8.4.3.3
	Join        LineJoin  // default LineJoinMiter (see appearance_builder.go); per ISO 32000-1 §8.4.3.4
	MiterLimit  float64   // ≤ 0 → PDF default (10)
}

// ShapeStyle combines a stroke (LineStyle) with an optional fill color.
//
// FillColor nil → no fill (stroke-only). Width ≤ 0 in the embedded LineStyle
// → no stroke (fill-only). If both are unset, the draw call is a no-op.
//
// Mirrors Aspose.PDF for .NET's GraphInfo (stroke + fill).
type ShapeStyle struct {
	LineStyle
	FillColor *Color // nil = no fill
}

// pathOpKind enumerates the kinds of operations a Path can contain.
type pathOpKind int

const (
	pathOpMoveTo pathOpKind = iota
	pathOpLineTo
	pathOpCurveTo // cubic Bezier — uses x/y as endpoint plus c1x/c1y/c2x/c2y
	pathOpClose
)

// pathOp is a single operation in a Path. Unused fields for the kind are zero.
type pathOp struct {
	kind               pathOpKind
	x, y               float64 // endpoint (MoveTo, LineTo, CurveTo)
	c1x, c1y, c2x, c2y float64 // bezier control points (CurveTo only)
}

// Path is a sequence of MoveTo/LineTo/CurveTo/Close operations defining an
// arbitrary 2D path in PDF user space (origin at page bottom-left, Y up).
//
// Construct via NewPath() and chain mutator methods, then pass to
// (*Page).DrawPath. Path is a builder type — it accumulates operations and
// holds no rendering state.
type Path struct {
	ops []pathOp
}

// NewPath returns an empty path.
func NewPath() *Path { return &Path{} }

// MoveTo begins a new subpath at (x, y). Returns p for chaining.
func (p *Path) MoveTo(x, y float64) *Path {
	p.ops = append(p.ops, pathOp{kind: pathOpMoveTo, x: x, y: y})
	return p
}

// LineTo adds a straight line segment from the current point to (x, y).
// If there is no current point, equivalent to MoveTo per PDF spec semantics.
func (p *Path) LineTo(x, y float64) *Path {
	p.ops = append(p.ops, pathOp{kind: pathOpLineTo, x: x, y: y})
	return p
}

// Close closes the current subpath with a line back to the most recent MoveTo.
// PDF operator h.
func (p *Path) Close() *Path {
	p.ops = append(p.ops, pathOp{kind: pathOpClose})
	return p
}
