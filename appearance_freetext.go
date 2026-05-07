package asposepdf

import "math"

// generateFreeTextAppearance produces /AP/N for a FreeText annotation.
//
// Order:
//  1. Optional /BG background fill (full rect) — skipped for Typewriter.
//  2. Standard rectangle border — skipped for Typewriter.
//  3. Text rendered inside an inner rect via renderTextInBuilder, using
//     the XObject's own /Resources/Font.
//
// Typewriter intent renders bare text with no background or border and
// zero padding (text fills the full bbox), matching Acrobat behavior.
//
// Callout intent (Task 14) will add leader-line drawing here later.
// Cloudy border (BorderEffect) is wired in Tasks 15-16.
// VAlign in /AP is verified end-to-end in Task 17 (renderTextInBuilder
// already supports VAlign from Task 1).
func generateFreeTextAppearance(a *FreeTextAnnotation) *pdfStream {
	rect := a.Rect()
	width := rect.URX - rect.LLX
	height := rect.URY - rect.LLY
	style := a.TextStyle()
	intent := a.Intent()

	b := newAppearanceBuilder()

	// Reuse existing /Resources from the current /AP/N XObject so that
	// font objects already registered in doc.objects are reused rather
	// than duplicated on each regeneration call.
	resources := existingAPNResources(&a.annotationBase)
	if resources == nil {
		resources = pdfDict{}
	}

	// Typewriter intent: bare text, no background or border per Acrobat behavior.
	skipChrome := intent == FreeTextIntentTypewriter

	// 1. Background fill (skip for typewriter).
	if !skipChrome && style.Background != nil {
		b.PushState()
		b.SetFillColorRGB(*style.Background)
		b.Rect(0, 0, width, height)
		b.Fill()
		b.PopState()
	}

	// 2. Border (skip for typewriter).
	bw := a.BorderWidth()
	if !skipChrome && bw > 0 {
		drawStandardRectBorder(b, width, height, a.BorderStyle(), bw, a.DashPattern(), a.Color())
	}

	// 3. Determine text rendering rect.
	var innerLocal Rectangle
	if intent == FreeTextIntentCallout {
		// Use /RD-derived inner rect, translated to local /BBox space.
		innerPage := a.InnerRect()
		innerLocal = Rectangle{
			LLX: innerPage.LLX - rect.LLX,
			LLY: innerPage.LLY - rect.LLY,
			URX: innerPage.URX - rect.LLX,
			URY: innerPage.URY - rect.LLY,
		}
	} else {
		var pad float64
		if skipChrome {
			pad = 0 // typewriter has no border/padding chrome
		} else {
			pad = bw
			if pad < 2 {
				pad = 2 // at least 2 pt of margin even with 0-width border
			}
		}
		innerLocal = Rectangle{LLX: pad, LLY: pad, URX: width - pad, URY: height - pad}
	}

	// 4. Render text.
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

	// 5. Callout line (only for callout intent).
	if intent == FreeTextIntentCallout {
		ptsPage := a.CalloutPoints()
		if len(ptsPage) >= 2 {
			ptsLocal := make([]Point, len(ptsPage))
			for i, p := range ptsPage {
				ptsLocal[i] = Point{X: p.X - rect.LLX, Y: p.Y - rect.LLY}
			}
			startLocal := nearestInnerEdgeMidpoint(innerLocal, ptsLocal[0])
			drawCalloutLine(b, startLocal, ptsLocal, bw, a.Color(), a.EndLineEnding())
		}
	}

	return makeFormXObjectWithResources(b.Bytes(), Rectangle{URX: width, URY: height}, resources)
}

// nearestInnerEdgeMidpoint returns the midpoint of the inner rect's
// edge nearest to target. Used as the implicit "start" point for a
// callout line, per ISO 32000-1 §12.5.6.6.
func nearestInnerEdgeMidpoint(inner Rectangle, target Point) Point {
	midX := (inner.LLX + inner.URX) / 2
	midY := (inner.LLY + inner.URY) / 2
	candidates := []Point{
		{X: midX, Y: inner.LLY},  // bottom
		{X: inner.URX, Y: midY},  // right
		{X: midX, Y: inner.URY},  // top
		{X: inner.LLX, Y: midY},  // left
	}
	bestIdx := 0
	bestDist := math.Inf(1)
	for i, p := range candidates {
		dx := p.X - target.X
		dy := p.Y - target.Y
		d := dx*dx + dy*dy
		if d < bestDist {
			bestDist = d
			bestIdx = i
		}
	}
	return candidates[bestIdx]
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

// drawCalloutLine renders a FreeText callout connector line: start →
// knee(s) → endpoint, with an optional line ending at the endpoint.
//
// pts must have 2 elements (one knee + endpoint) or 3 elements (two
// knees + endpoint). With fewer than 2, this is a no-op.
//
// All coordinates are in local /BBox space (caller translates from
// page space). The start point is computed by the caller as the
// midpoint of the inner-rect edge nearest to pts[0].
//
// The endpoint is at pts[len(pts)-1]. Theta for the line ending is
// the angle of the last segment (last-knee → endpoint), pointing
// outward (matching Subepic 3 line-ending conventions).
func drawCalloutLine(b *appearanceBuilder, start Point, pts []Point, lineWidth float64, color *Color, ending LineEndingStyle) {
	if len(pts) < 2 {
		return
	}
	b.PushState()
	b.SetLineWidth(lineWidth)
	if color != nil {
		b.SetStrokeColorRGB(*color)
	}
	b.MoveTo(start.X, start.Y)
	for _, p := range pts {
		b.LineTo(p.X, p.Y)
	}
	b.Stroke()
	b.PopState()

	// Line ending at endpoint.
	if ending != LineEndingNone {
		endpoint := pts[len(pts)-1]
		prev := pts[len(pts)-2]
		theta := math.Atan2(endpoint.Y-prev.Y, endpoint.X-prev.X)
		// /IC fill is not applicable here (FreeText callout endings
		// typically use the stroke color for fill); use stroke color
		// when a fill is needed.
		var fill *Color
		if color != nil {
			fc := *color
			fill = &fc
		}
		drawLineEnding(b, ending, endpoint.X, endpoint.Y, theta, lineWidth, fill)
	}
}
