package asposepdf

import "fmt"

// ValidateRedactions performs a pre-flight dry-run parseability check on
// every page that has at least one RedactAnnotation. Returns the first
// content-stream parse error encountered, or nil if every redact-bearing
// page's content stream parses cleanly. Pages without redact annotations
// are not inspected.
//
// Callers should invoke this before ApplyRedactions to surface malformed
// content streams up-front; ApplyRedactions has best-effort semantics
// and may leave partial state on failure.
func (d *Document) ValidateRedactions() error {
	for _, page := range d.Pages() {
		if !pageHasRedact(page) {
			continue
		}
		data, err := page.contentStreams()
		if err != nil {
			return fmt.Errorf("ValidateRedactions: page %d: %w", page.Number(), err)
		}
		if data == nil {
			continue
		}
		if _, err := parseContentStream(data); err != nil {
			return fmt.Errorf("ValidateRedactions: page %d content stream: %w", page.Number(), err)
		}
	}
	return nil
}

// ApplyRedactions destructively removes content (text glyphs, image
// XObjects, paths) inside every /Redact annotation's /QuadPoints (or
// /Rect if /QuadPoints is empty). Renders /OverlayText if set, then
// removes the redact annotations from the page.
//
// Best-effort semantics: partial state on failure. Call
// ValidateRedactions first to surface parse errors before mutating.
func (d *Document) ApplyRedactions() error {
	for _, page := range d.Pages() {
		if !pageHasRedact(page) {
			continue
		}
		if err := applyRedactionsToPage(page); err != nil {
			return fmt.Errorf("ApplyRedactions: page %d: %w", page.Number(), err)
		}
	}
	return nil
}

// applyRedactionsToPage collects regions from every RedactAnnotation on the
// page, runs the three content-stream rewriters, replaces the page content,
// and removes the redact annotations from the collection.
func applyRedactionsToPage(p *Page) error {
	// 1. Collect regions and redact annotation pointers.
	var regions []QuadPoint
	coll := p.Annotations()
	var redacts []*RedactAnnotation
	for i := 0; i < coll.Count(); i++ {
		ra, ok := coll.At(i).(*RedactAnnotation)
		if !ok {
			continue
		}
		redacts = append(redacts, ra)
		qps := ra.QuadPoints()
		if len(qps) == 0 {
			// Fallback: treat /Rect as a single quad.
			qps = []QuadPoint{rectAsQuadPoint(ra.Rect())}
		}
		regions = append(regions, qps...)
	}
	if len(regions) == 0 {
		return nil // no regions to apply
	}

	// 2. Get decoded page content stream bytes.
	data, err := p.contentStreams()
	if err != nil {
		return err
	}
	if data == nil {
		// Empty page — still remove the annotations below.
		data = []byte{}
	}

	// 3. Build font map from page resources.
	resources := p.pageResources()
	fontMap := resolveFontResources(p.doc.objects, resources)

	// 4. Run rewriters sequentially: text → image → path.
	data, err = rewriteTextOperatorsInStream(data, regions, fontMap)
	if err != nil {
		return fmt.Errorf("text rewriter: %w", err)
	}
	data, err = rewriteImageOperatorsInStream(data, regions)
	if err != nil {
		return fmt.Errorf("image rewriter: %w", err)
	}
	data, err = rewritePathOperatorsInStream(data, regions)
	if err != nil {
		return fmt.Errorf("path rewriter: %w", err)
	}

	// 5. Replace the page's /Contents with a single new stream.
	if err := replacePageContents(p, data); err != nil {
		return fmt.Errorf("replace content: %w", err)
	}

	// 6. Remove every redact annotation from the collection.
	for _, ra := range redacts {
		coll.Delete(ra)
	}

	return nil
}

// replacePageContents allocates a new stream object holding data and sets
// the page dict's /Contents to a single pdfRef pointing at it. Any prior
// /Contents entries (single ref or array) are abandoned; the writer's
// reachability pass will drop them on save.
func replacePageContents(p *Page, data []byte) error {
	pageDict := p.pageDict()
	if pageDict == nil {
		return fmt.Errorf("no page dict")
	}

	newID := p.doc.nextID
	p.doc.nextID++
	p.doc.objects[newID] = &pdfObject{
		Num: newID,
		Value: &pdfStream{
			Dict:    pdfDict{"/Length": len(data)},
			Data:    data,
			Decoded: true, // raw bytes; writer will FlateDecode on output
		},
	}

	pageDict["/Contents"] = pdfRef{Num: newID}
	return nil
}

// pageHasRedact reports whether the given page has at least one
// RedactAnnotation in its annotation collection.
func pageHasRedact(p *Page) bool {
	coll := p.Annotations()
	for i := 0; i < coll.Count(); i++ {
		if _, ok := coll.At(i).(*RedactAnnotation); ok {
			return true
		}
	}
	return false
}
