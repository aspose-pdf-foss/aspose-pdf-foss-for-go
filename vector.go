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
