// SPDX-License-Identifier: MIT

package asposepdf

import (
	"bytes"
	"os"
	"testing"
)

func TestParseSVG_FilterDropShadow(t *testing.T) {
	data, _ := os.ReadFile("testdata/svg/filter_dropshadow.svg")
	svg, err := parseSVGBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	f, ok := svg.defs["ds"].(*svgFilter)
	if !ok {
		t.Fatalf("defs[ds] = %T", svg.defs["ds"])
	}
	if len(f.primitives) != 1 {
		t.Errorf("expected 1 primitive, got %d", len(f.primitives))
	}
	ds := f.findDropShadow()
	if ds == nil {
		t.Fatal("no feDropShadow")
	}
	if ds.dx != 2 || ds.dy != 3 {
		t.Errorf("dx=%g dy=%g, want 2/3", ds.dx, ds.dy)
	}
	if ds.floodColor == nil || ds.floodColor.R != 0 || ds.floodColor.G != 0 || ds.floodColor.B != 0 {
		t.Errorf("flood-color = %+v, want black", ds.floodColor)
	}
	if ds.floodOpacity != 0.5 {
		t.Errorf("flood-opacity = %g, want 0.5", ds.floodOpacity)
	}
}

func TestParseSVG_FilterUnsupportedPrimitiveStored(t *testing.T) {
	svg, _ := parseSVGBytes([]byte(`<svg xmlns="http://www.w3.org/2000/svg">
		<defs><filter id="blur"><feGaussianBlur stdDeviation="5"/></filter></defs>
	</svg>`))
	f, _ := svg.defs["blur"].(*svgFilter)
	if f == nil {
		t.Fatal("filter not stored")
	}
	if len(f.primitives) != 1 {
		t.Errorf("expected 1, got %d", len(f.primitives))
	}
	if f.primitives[0].kind != "feGaussianBlur" {
		t.Errorf("kind = %q", f.primitives[0].kind)
	}
	if f.findDropShadow() != nil {
		t.Error("findDropShadow should return nil when no drop shadow present")
	}
}

func TestApplyStyle_Filter(t *testing.T) {
	s := defaultSVGStyle()
	applySingleSVGStyleProp(&s, "filter", "url(#ds)")
	if s.filter != "ds" {
		t.Errorf("filter = %q", s.filter)
	}
	applySingleSVGStyleProp(&s, "filter", "none")
	if s.filter != "" {
		t.Errorf("filter should be cleared, got %q", s.filter)
	}
}

func TestRenderSVG_FilterDropShadow(t *testing.T) {
	data, _ := os.ReadFile("testdata/svg/filter_dropshadow.svg")
	svg, _ := parseSVGBytes(data)
	doc := NewDocumentFromFormat(PageFormatA4)
	page, _ := doc.Page(1)
	_ = renderSVG(page, svg, Rectangle{LLX: 0, LLY: 0, URX: 200, URY: 200})
	stream, _ := page.contentStreams()
	// Expect at least 2 "re" operators: shadow rect + original rect
	count := bytes.Count(stream, []byte(" re"))
	if count < 2 {
		t.Errorf("expected ≥2 re ops (shadow + original), got %d:\n%s", count, stream)
	}
}
