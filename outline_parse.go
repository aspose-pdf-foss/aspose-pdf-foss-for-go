package asposepdf

// parseDestinationArray decodes a destination per ISO 32000-1 §12.3.2.2.
// Returns nil if the array is malformed or the referenced page is not
// in the in-memory document.
func parseDestinationArray(doc *Document, arr pdfArray) Destination {
	if len(arr) < 2 {
		return nil
	}
	page := resolvePageFromDestRef(doc, arr[0])
	if page == nil {
		return nil
	}
	fit, ok := arr[1].(pdfName)
	if !ok {
		return nil
	}
	switch fit {
	case "/XYZ":
		return parseDestXYZ(page, arr)
	case "/Fit":
		return &DestinationFit{page: page}
	case "/FitH":
		if len(arr) < 3 {
			return &DestinationFitH{page: page, useTop: false}
		}
		top, has := destFloat(arr[2])
		return &DestinationFitH{page: page, top: top, useTop: has}
	case "/FitV":
		if len(arr) < 3 {
			return &DestinationFitV{page: page, useLeft: false}
		}
		left, has := destFloat(arr[2])
		return &DestinationFitV{page: page, left: left, useLeft: has}
	case "/FitR":
		if len(arr) < 6 {
			return nil
		}
		l, _ := destFloat(arr[2])
		b, _ := destFloat(arr[3])
		r, _ := destFloat(arr[4])
		t, _ := destFloat(arr[5])
		return &DestinationFitR{page: page, left: l, bottom: b, right: r, top: t}
	case "/FitB":
		return &DestinationFitB{page: page}
	case "/FitBH":
		if len(arr) < 3 {
			return &DestinationFitBH{page: page, useTop: false}
		}
		top, has := destFloat(arr[2])
		return &DestinationFitBH{page: page, top: top, useTop: has}
	case "/FitBV":
		if len(arr) < 3 {
			return &DestinationFitBV{page: page, useLeft: false}
		}
		left, has := destFloat(arr[2])
		return &DestinationFitBV{page: page, left: left, useLeft: has}
	}
	return nil
}

func parseDestXYZ(page *Page, arr pdfArray) *DestinationXYZ {
	out := &DestinationXYZ{page: page}
	if len(arr) >= 3 {
		out.left, out.useLeft = destFloat(arr[2])
	}
	if len(arr) >= 4 {
		out.top, out.useTop = destFloat(arr[3])
	}
	if len(arr) >= 5 {
		out.zoom, out.useZoom = destFloat(arr[4])
	}
	return out
}

// destFloat returns (value, true) if v is a numeric value, or (0, false)
// if v is pdfNull (meaning "unchanged" in destination semantics).
func destFloat(v pdfValue) (float64, bool) {
	if _, ok := v.(pdfNull); ok {
		return 0, false
	}
	f, err := toFloat(v)
	if err != nil {
		return 0, false
	}
	return f, true
}

// resolvePageFromDestRef walks doc.pages looking for a page whose
// underlying object number matches the destination's first element.
// Returns nil if no match.
func resolvePageFromDestRef(doc *Document, v pdfValue) *Page {
	ref, ok := v.(pdfRef)
	if !ok {
		return nil
	}
	for i, po := range doc.pages {
		if po != nil && po.Num == ref.Num {
			// Return the cached page (1-based index).
			p, _ := doc.Page(i + 1)
			return p
		}
	}
	return nil
}
