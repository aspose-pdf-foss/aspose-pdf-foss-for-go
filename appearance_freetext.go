package asposepdf

// generateFreeTextAppearance produces /AP/N for a FreeText annotation.
//
// Order:
//  1. Optional /BG background fill (full rect).
//  2. Standard rectangle border (Solid by default; Beveled/Inset/Underline
//     follow Subepic 3's drawingAnnotationBase semantics; Dashed via /BS/D).
//  3. Text rendered inside an inner rect (rect minus border-width padding)
//     via renderTextInBuilder, using the XObject's own /Resources/Font.
//
// Intent dispatch (Typewriter, Callout) is wired in Tasks 12 and 14.
// Cloudy border (BorderEffect) is wired in Tasks 15-16.
// VAlign in /AP is verified end-to-end in Task 17 (renderTextInBuilder
// already supports VAlign from Task 1).
func generateFreeTextAppearance(a *FreeTextAnnotation) *pdfStream {
	rect := a.Rect()
	width := rect.URX - rect.LLX
	height := rect.URY - rect.LLY
	style := a.TextStyle()

	b := newAppearanceBuilder()

	// Reuse existing /Resources from the current /AP/N XObject so that
	// font objects already registered in doc.objects are reused rather
	// than duplicated on each regeneration call.
	resources := existingAPNResources(&a.annotationBase)
	if resources == nil {
		resources = pdfDict{}
	}

	// 1. Background fill.
	if style.Background != nil {
		b.PushState()
		b.SetFillColorRGB(*style.Background)
		b.Rect(0, 0, width, height)
		b.Fill()
		b.PopState()
	}

	// 2. Border.
	bw := a.BorderWidth()
	if bw > 0 {
		drawStandardRectBorder(b, width, height, a.BorderStyle(), bw, a.DashPattern(), a.Color())
	}

	// 3. Text in inner rect (inset by border width).
	pad := bw
	if pad < 2 {
		pad = 2 // at least 2 pt of margin even with 0-width border
	}
	innerLocal := Rectangle{LLX: pad, LLY: pad, URX: width - pad, URY: height - pad}
	contents := a.Contents()
	if contents != "" {
		// renderTextInBuilder uses style.Color for text color (separate
		// from a.Color() which is the BORDER color).
		// The second arg (pdfDict) to the resolver is ignored by
		// resolveFontForXObject — it writes to the captured `resources`
		// via closure instead.
		resolve := func(font Font, _ pdfDict) (resName string, w widthFn, e encodeFn, asc, desc float64, err error) {
			return resolveFontForXObject(font, style.Size, a.doc, resources)
		}
		// Empty ExtGState names — opaque text/bg.
		_ = renderTextInBuilder(b, resources, contents, style, innerLocal, resolve, "", "")
	}

	return makeFormXObjectWithResources(b.Bytes(), Rectangle{URX: width, URY: height}, resources)
}

// drawStandardRectBorder renders a rectangular border using the given
// BorderStyle. Dispatches:
//   - Solid: simple stroked rect.
//   - Dashed: same with dash pattern.
//   - Beveled / Inset: two-pass color render (uses Subepic 3's
//     drawBeveledRectBorder).
//   - Underline: just the bottom edge.
func drawStandardRectBorder(b *appearanceBuilder, width, height float64, style BorderStyle, lineWidth float64, dashPattern []float64, strokeColor *Color) {
	switch style {
	case BorderBeveled, BorderInset:
		drawBeveledRectBorder(b, width, height, lineWidth, strokeColor, style == BorderInset)
	case BorderUnderline:
		b.PushState()
		b.SetLineWidth(lineWidth)
		if strokeColor != nil {
			b.SetStrokeColorRGB(*strokeColor)
		}
		b.MoveTo(0, lineWidth/2)
		b.LineTo(width, lineWidth/2)
		b.Stroke()
		b.PopState()
	default: // BorderSolid, BorderDashed
		b.PushState()
		b.SetLineWidth(lineWidth)
		if strokeColor != nil {
			b.SetStrokeColorRGB(*strokeColor)
		}
		if style == BorderDashed {
			dp := dashPattern
			if len(dp) == 0 {
				dp = []float64{3, 3}
			}
			b.SetDashPattern(dp, 0)
		}
		inset := lineWidth / 2
		b.Rect(inset, inset, width-lineWidth, height-lineWidth)
		b.Stroke()
		b.PopState()
	}
}
