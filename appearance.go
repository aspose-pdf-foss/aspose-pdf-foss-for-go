package asposepdf

// makeFormXObject builds a Form XObject stream wrapping the given content
// bytes and bbox. The returned stream is ready for storage in
// doc.objects and reference from /AP/N.
//
// /Resources is empty — drawing annotations (Square/Circle/Line/Ink)
// don't use fonts or images. Future subepics (FreeText, Stamp) will
// extend this helper or supply their own.
func makeFormXObject(content []byte, bbox Rectangle) *pdfStream {
	return &pdfStream{
		Dict: pdfDict{
			"/Type":      pdfName("/XObject"),
			"/Subtype":   pdfName("/Form"),
			"/BBox":      pdfArray{bbox.LLX, bbox.LLY, bbox.URX, bbox.URY},
			"/Resources": pdfDict{},
		},
		Data:    content,
		Decoded: true,
	}
}

// generateSquareAppearance produces /AP/N for a Square annotation.
// Supports all five border styles: Solid, Dashed, Beveled, Inset, Underline.
// InteriorColor (/IC) is applied as a fill for Solid/Dashed styles and as a
// background rectangle for Beveled/Inset; it is ignored for Underline per
// spec convention.
func generateSquareAppearance(a *SquareAnnotation) *pdfStream {
	rect := a.Rect()
	width := rect.URX - rect.LLX
	height := rect.URY - rect.LLY

	bw := a.BorderWidth()
	style := a.BorderStyle()

	b := newAppearanceBuilder()

	switch style {
	case BorderBeveled, BorderInset:
		// Two-pass color render. Fill first if /IC is set.
		if ic := a.InteriorColor(); ic != nil {
			b.PushState()
			b.SetFillColorRGB(*ic)
			inset := bw
			b.Rect(inset, inset, width-2*bw, height-2*bw)
			b.Fill()
			b.PopState()
		}
		drawBeveledRectBorder(b, width, height, bw, a.Color(), style == BorderInset)

	case BorderUnderline:
		b.PushState()
		b.SetLineWidth(bw)
		if c := a.Color(); c != nil {
			b.SetStrokeColorRGB(*c)
		}
		// Bottom edge only.
		b.MoveTo(0, bw/2)
		b.LineTo(width, bw/2)
		b.Stroke()
		b.PopState()
		// Underline ignores /IC by spec convention.

	default:
		b.PushState()
		b.SetLineWidth(bw)
		if c := a.Color(); c != nil {
			b.SetStrokeColorRGB(*c)
		}
		if style == BorderDashed {
			dp := a.DashPattern()
			if len(dp) == 0 {
				dp = []float64{3, 3}
			}
			b.SetDashPattern(dp, 0)
		}
		inset := bw / 2
		b.Rect(inset, inset, width-bw, height-bw)
		hasFill := false
		if ic := a.InteriorColor(); ic != nil {
			b.SetFillColorRGB(*ic)
			hasFill = true
		}
		if hasFill {
			b.FillStroke()
		} else {
			b.Stroke()
		}
		b.PopState()
	}

	return makeFormXObject(b.Bytes(), Rectangle{URX: width, URY: height})
}

// drawBeveledRectBorder emits a two-pass beveled (or inset) border on a
// rectangle of size (width, height). Top + left edges use the light
// color; bottom + right edges use the dark color (inverted for Inset).
func drawBeveledRectBorder(b *appearanceBuilder, width, height, bw float64, baseColor *Color, inverted bool) {
	base := Color{R: 0, G: 0, B: 0, A: 1}
	if baseColor != nil {
		base = *baseColor
	}
	light, dark := beveledColorPair(base, inverted)

	// Light pass: top + left edges as filled trapezoids.
	b.PushState()
	b.SetFillColorRGB(light)
	// Outer top-left corner → outer top-right → inner top-right → inner top-left.
	b.MoveTo(0, height)
	b.LineTo(width, height)
	b.LineTo(width-bw, height-bw)
	b.LineTo(bw, height-bw)
	b.ClosePath()
	b.Fill()
	// Outer top-left → outer bottom-left → inner bottom-left → inner top-left.
	b.MoveTo(0, height)
	b.LineTo(0, 0)
	b.LineTo(bw, bw)
	b.LineTo(bw, height-bw)
	b.ClosePath()
	b.Fill()
	b.PopState()

	// Dark pass: bottom + right edges.
	b.PushState()
	b.SetFillColorRGB(dark)
	// Outer bottom-left → outer bottom-right → inner bottom-right → inner bottom-left.
	b.MoveTo(0, 0)
	b.LineTo(width, 0)
	b.LineTo(width-bw, bw)
	b.LineTo(bw, bw)
	b.ClosePath()
	b.Fill()
	// Outer bottom-right → outer top-right → inner top-right → inner bottom-right.
	b.MoveTo(width, 0)
	b.LineTo(width, height)
	b.LineTo(width-bw, height-bw)
	b.LineTo(width-bw, bw)
	b.ClosePath()
	b.Fill()
	b.PopState()
}

// beveledColorPair returns a (light, dark) color pair for Beveled and
// Inset border rendering. Light = base × 0.5 + white × 0.5; Dark =
// base × 0.5. When inverted is true (Inset style) the pair is swapped.
//
// PDF spec doesn't precisely fix the algorithm; this matches Acrobat
// output for the same input.
func beveledColorPair(base Color, inverted bool) (light, dark Color) {
	light = Color{
		R: base.R*0.5 + 0.5,
		G: base.G*0.5 + 0.5,
		B: base.B*0.5 + 0.5,
		A: 1,
	}
	dark = Color{
		R: base.R * 0.5,
		G: base.G * 0.5,
		B: base.B * 0.5,
		A: 1,
	}
	if inverted {
		return dark, light
	}
	return light, dark
}

// setAppearanceN replaces /AP/N on the annotation. If /AP/N already
// references an XObject in doc.objects, that object is mutated in place
// (no new objID allocated, no orphans). Otherwise a fresh XObject is
// allocated and /AP/N updated to reference it.
//
// No-op when base.doc is nil (annotation not yet doc-linked — should
// not normally happen because constructors set base.doc immediately).
func setAppearanceN(base *annotationBase, stream *pdfStream) {
	if base.doc == nil {
		return
	}
	apDict, _ := base.dict["/AP"].(pdfDict)
	if ref, ok := apDict["/N"].(pdfRef); ok {
		if obj, exists := base.doc.objects[ref.Num]; exists {
			obj.Value = stream
			return
		}
	}
	objID := base.doc.nextID
	base.doc.nextID++
	base.doc.objects[objID] = &pdfObject{Num: objID, Value: stream}
	if apDict == nil {
		apDict = pdfDict{}
	}
	apDict["/N"] = pdfRef{Num: objID}
	base.dict["/AP"] = apDict
}
