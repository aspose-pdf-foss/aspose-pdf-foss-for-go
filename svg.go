// SPDX-License-Identifier: MIT

package asposepdf

import (
	"io"
	"os"
)

// AddSVG reads an SVG file and renders it into the given rectangle on the page.
// Unsupported elements (text, image, gradients, masks) are skipped silently per
// Phase 2 scope.
//
// Returns error only on XML parse failure, invalid numeric attributes, or I/O errors.
func (p *Page) AddSVG(path string, rect Rectangle) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return p.AddSVGFromStream(f, rect)
}

// AddSVGFromStream renders an SVG from any io.Reader into the given rectangle on
// the page. Best-effort: unsupported elements are silently skipped.
func (p *Page) AddSVGFromStream(r io.Reader, rect Rectangle) error {
	svg, err := parseSVGReader(r)
	if err != nil {
		return err
	}
	return p.AddSVGObject(svg, rect)
}

// AddSVGObject renders a pre-parsed SVG into the given rectangle on the page.
// Useful when the same SVG is rendered on multiple pages without re-parsing.
func (p *Page) AddSVGObject(svg *SVG, rect Rectangle) error {
	return renderSVG(p, svg, rect)
}

// ViewBox returns the viewBox attribute as (x, y, width, height).
// If no viewBox is set, returns (0, 0, intrinsicWidth, intrinsicHeight).
func (s *SVG) ViewBox() (x, y, w, h float64) {
	if s.viewBox != nil {
		return s.viewBox.x, s.viewBox.y, s.viewBox.w, s.viewBox.h
	}
	return 0, 0, s.width, s.height
}

// Size returns the intrinsic width and height as parsed from the <svg> root
// element's width and height attributes. Returns (0, 0) if neither is present.
func (s *SVG) Size() (width, height float64) {
	return s.width, s.height
}
