// SPDX-License-Identifier: MIT

package asposepdf

import "math"

// resolveGradientFill returns a PDF pattern resource name (e.g. "/P0") when
// the paint value is a gradient reference that can be resolved and registered.
// Returns "" when paint is nil, is a plain color, or the ref is unknown.
//
// ctm is the cumulative SVG-to-page transform in effect at the shape's draw
// site (composed of the viewBox-fit matrix plus any group/shape `transform=`
// attributes encountered along the way). PDF Type 2 (shading) pattern /Matrix
// maps pattern coordinates to the page's *initial* user space (ISO 32000-1
// §8.7.4.5.1) — not to the user space at scn time — so we must fold the CTM
// into the matrix here. The cm operators emitted around the shape draw apply
// to the path coordinates and stroke/fill colour selection but, per spec, do
// not affect a Type 2 pattern's coordinate mapping.
func resolveGradientFill(p *Page, svg *SVG, paint *svgPaint, shape svgNode, ctm svgMatrix) string {
	if paint == nil || paint.gradRef == "" || svg == nil {
		return ""
	}
	grad, ok := svg.gradients[paint.gradRef]
	if !ok {
		return ""
	}

	// Start with the cumulative CTM. Compose bounding-box scale (when
	// gradientUnits is objectBoundingBox) and gradientTransform on top.
	matrix := ctm
	var units svgGradientUnits
	var transform *svgMatrix
	switch g := grad.(type) {
	case *svgLinearGradient:
		units, transform = g.units, g.transform
	case *svgRadialGradient:
		units, transform = g.units, g.transform
	}

	if units == svgGradientObjectBBox {
		x0, y0, x1, y1 := svgShapeBBox(shape)
		bboxMatrix := svgMatrix{x1 - x0, 0, 0, y1 - y0, x0, y0}
		matrix = matrixMul(matrix, bboxMatrix)
	}
	if transform != nil {
		matrix = matrixMul(matrix, *transform)
	}

	name, err := p.ensurePatternResource(grad, matrix)
	if err != nil {
		return ""
	}
	return name
}

// svgShapeBBox returns the axis-aligned bounding box of a shape in its local
// coordinate space. Used for objectBoundingBox gradient unit mapping.
func svgShapeBBox(n svgNode) (x0, y0, x1, y1 float64) {
	switch s := n.(type) {
	case *svgRect:
		return s.x, s.y, s.x + s.w, s.y + s.h
	case *svgCircle:
		return s.cx - s.r, s.cy - s.r, s.cx + s.r, s.cy + s.r
	case *svgEllipse:
		return s.cx - s.rx, s.cy - s.ry, s.cx + s.rx, s.cy + s.ry
	case *svgLine:
		x0, x1 = bboxMinMax(s.x1, s.x2)
		y0, y1 = bboxMinMax(s.y1, s.y2)
		return
	case *svgPolyline:
		return pointsBBox(s.points)
	case *svgPolygon:
		return pointsBBox(s.points)
	case *svgPath:
		return pathOpsBBox(s.commands)
	}
	return 0, 0, 0, 0
}

func bboxMinMax(a, b float64) (float64, float64) {
	if a < b {
		return a, b
	}
	return b, a
}

func pointsBBox(pts []Point) (x0, y0, x1, y1 float64) {
	if len(pts) == 0 {
		return
	}
	x0, y0 = pts[0].X, pts[0].Y
	x1, y1 = x0, y0
	for _, p := range pts[1:] {
		if p.X < x0 {
			x0 = p.X
		}
		if p.X > x1 {
			x1 = p.X
		}
		if p.Y < y0 {
			y0 = p.Y
		}
		if p.Y > y1 {
			y1 = p.Y
		}
	}
	return
}

func pathOpsBBox(ops []svgPathOp) (x0, y0, x1, y1 float64) {
	first := true
	track := func(x, y float64) {
		if first {
			x0, y0, x1, y1 = x, y, x, y
			first = false
			return
		}
		if x < x0 {
			x0 = x
		}
		if x > x1 {
			x1 = x
		}
		if y < y0 {
			y0 = y
		}
		if y > y1 {
			y1 = y
		}
	}
	for _, op := range ops {
		switch op.kind {
		case 'M', 'L':
			track(op.args[0], op.args[1])
		case 'C':
			track(op.args[4], op.args[5])
		case 'Q':
			track(op.args[2], op.args[3])
		}
	}
	return
}

// buildShadingFunction returns a *pdfObject containing a PDF function that maps t in [0,1]
// to an RGB color triple, suitable for use as the /Function entry of a PDF shading dictionary.
//
// SVG allows the first stop offset to be > 0 and the last < 1 — in that case the
// region before the first stop is filled with the first stop's color and the
// region after the last stop with the last stop's color (per SVG 1.1 §13.2.4).
// To replicate this in PDF stitching, we prepend a synthetic stop at offset 0
// and append one at offset 1 (cloning the colors) so the full [0,1] domain is
// covered correctly.
//
// The returned object has Num==0; the caller is responsible for assigning a real
// object number and inserting it into doc.objects before writing.
func buildShadingFunction(stops []svgGradientStop) *pdfObject {
	if len(stops) == 0 {
		stops = []svgGradientStop{
			{offset: 0, color: &Color{R: 0, G: 0, B: 0, A: 1}, opacity: 1},
		}
	}

	// Normalize: ensure the first stop is at offset 0 and the last at 1, by
	// cloning the endpoint colors into synthetic stops. The cloned regions
	// render as a solid colour because the exponential sub-function spanning
	// them has C0 == C1.
	if stops[0].offset > 0 {
		head := stops[0]
		head.offset = 0
		stops = append([]svgGradientStop{head}, stops...)
	}
	if stops[len(stops)-1].offset < 1 {
		tail := stops[len(stops)-1]
		tail.offset = 1
		stops = append(stops, tail)
	}

	if len(stops) == 1 {
		return &pdfObject{Value: exponentialFunctionDict(stops[0].color, stops[0].color)}
	}
	if len(stops) == 2 {
		return &pdfObject{Value: exponentialFunctionDict(stops[0].color, stops[1].color)}
	}

	// 3+ stops: build a Type 3 stitching function.
	// Sub-functions: one Type 2 per adjacent stop pair.
	subFunctions := make(pdfArray, 0, len(stops)-1)
	for i := 0; i < len(stops)-1; i++ {
		subFunctions = append(subFunctions, exponentialFunctionDict(stops[i].color, stops[i+1].color))
	}

	// /Bounds: internal stop offsets (all except first and last).
	// PDF spec requires strictly-increasing values. SVG allows duplicate offsets
	// (sharp color transitions), so we bump each non-increasing bound by a small
	// epsilon to satisfy the spec while preserving the visual intent.
	const minBoundGap = 1e-6
	bounds := make(pdfArray, 0, len(stops)-2)
	prev := 0.0
	for i := 1; i < len(stops)-1; i++ {
		b := stops[i].offset
		if b <= prev {
			b = prev + minBoundGap
		}
		bounds = append(bounds, b)
		prev = b
	}

	// /Encode: each sub-function maps its local [0,1] interval to [0 1].
	encode := make(pdfArray, 0, (len(stops)-1)*2)
	for i := 0; i < len(stops)-1; i++ {
		encode = append(encode, 0.0, 1.0)
	}

	dict := pdfDict{
		"/FunctionType": 3,
		"/Domain":       pdfArray{0.0, 1.0},
		"/Functions":    subFunctions,
		"/Bounds":       bounds,
		"/Encode":       encode,
	}
	return &pdfObject{Value: dict}
}

// gradientToShadingObject creates a /Shading dictionary indirect object for the gradient.
//
// The supplied matrix m (gradientUnits + gradientTransform composition) is baked
// into /Coords so the shading dictionary holds final user-space coordinates.
// The caller can then emit /Matrix as identity on the parent /Pattern dict.
//
// Why bake instead of relying on /Pattern /Matrix: real-world PDF renderers
// (Acrobat, MuPDF/PyMuPDF, …) disagree in practice on how /Matrix composes
// with the CTM for Type 3 (radial) shadings. Baking the transform into
// /Coords removes that ambiguity — the gradient renders identically across
// all spec-conformant viewers. The Aspose logo's blades exposed this: with
// /Matrix kept separate, viewers were treating /Coords as if already in user
// space, leaving the blade pixels at distance ~500+ units from the gradient
// center and so painting them all with the extended last-stop colour
// ("invisible gradient"). After baking, gradient extents land where SVG
// intends them.
//
// For non-uniform matrices (rare — e.g. anisotropic scale on highlight
// overlays), the circular shading approximates the intended ellipse using
// sqrt(|det|) as a single radius scale. PDF Type 3 cannot represent true
// ellipses via /Coords alone; this is the best fidelity available without
// reintroducing /Matrix (and its cross-viewer ambiguity).
//
// The /Function entry is stored as a pdfRef to the function object already
// registered in doc.objects.
//
// Returns nil if grad is an unsupported type.
// The returned *pdfObject's Num is 0; ensurePatternResource assigns a real number.
func gradientToShadingObject(grad svgGradient, fnRef pdfRef, m svgMatrix) *pdfObject {
	shading := pdfDict{
		"/ColorSpace": pdfName("/DeviceRGB"),
		"/Extend":     pdfArray{true, true},
		"/Function":   fnRef,
	}
	switch g := grad.(type) {
	case *svgLinearGradient:
		shading["/ShadingType"] = 2
		x1, y1 := transformPoint(m, g.x1, g.y1)
		x2, y2 := transformPoint(m, g.x2, g.y2)
		shading["/Coords"] = pdfArray{x1, y1, x2, y2}
	case *svgRadialGradient:
		shading["/ShadingType"] = 3
		fx, fy := transformPoint(m, g.fx, g.fy)
		cx, cy := transformPoint(m, g.cx, g.cy)
		rScale := math.Sqrt(math.Abs(m[0]*m[3] - m[1]*m[2]))
		// Coords: [fx fy 0 cx cy r] — focal point as inner circle with radius 0.
		shading["/Coords"] = pdfArray{fx, fy, 0.0, cx, cy, g.r * rScale}
	default:
		return nil
	}
	return &pdfObject{Value: shading}
}

// transformPoint applies a 2x3 affine matrix (a b c d e f form) to (x, y).
func transformPoint(m svgMatrix, x, y float64) (float64, float64) {
	return m[0]*x + m[2]*y + m[4], m[1]*x + m[3]*y + m[5]
}

// exponentialFunctionDict returns an inline pdfDict (not wrapped in pdfObject) for a
// PDF Type 2 exponential function with N=1 interpolating between c0 and c1 in DeviceRGB.
func exponentialFunctionDict(c0, c1 *Color) pdfDict {
	return pdfDict{
		"/FunctionType": 2,
		"/Domain":       pdfArray{0.0, 1.0},
		"/C0":           pdfArray{c0.R, c0.G, c0.B},
		"/C1":           pdfArray{c1.R, c1.G, c1.B},
		"/N":            1,
	}
}
