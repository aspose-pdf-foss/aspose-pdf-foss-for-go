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
//
// Stub for Task 8 — full implementation in Task 12.
func (d *Document) ApplyRedactions() error {
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
