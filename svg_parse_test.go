// SPDX-License-Identifier: MIT

package asposepdf

import (
	"os"
	"testing"
)

func TestParseSVG_MinimalRect(t *testing.T) {
	data, err := os.ReadFile("testdata/svg/rect.svg")
	if err != nil { t.Fatal(err) }
	svg, err := parseSVGBytes(data)
	if err != nil { t.Fatal(err) }
	if svg.viewBox == nil || svg.viewBox.w != 100 || svg.viewBox.h != 50 {
		t.Errorf("viewBox = %+v", svg.viewBox)
	}
	if svg.width != 100 || svg.height != 50 {
		t.Errorf("intrinsic = %g × %g", svg.width, svg.height)
	}
	if svg.root == nil || len(svg.root.children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(svg.root.children))
	}
	r, ok := svg.root.children[0].(*svgRect)
	if !ok { t.Fatalf("expected *svgRect, got %T", svg.root.children[0]) }
	if r.x != 10 || r.y != 10 || r.w != 80 || r.h != 30 {
		t.Errorf("rect dims = %g,%g %g×%g", r.x, r.y, r.w, r.h)
	}
	if r.style.fill == nil || r.style.fill.R != 1 || r.style.fill.G != 0 || r.style.fill.B != 0 {
		t.Errorf("fill = %+v", r.style.fill)
	}
}

func TestParseSVG_AllShapes(t *testing.T) {
	data, _ := os.ReadFile("testdata/svg/all_shapes.svg")
	svg, err := parseSVGBytes(data)
	if err != nil { t.Fatal(err) }
	got := len(svg.root.children)
	if got != 6 {
		t.Fatalf("expected 6 shape children, got %d", got)
	}
	kinds := map[string]int{}
	for _, c := range svg.root.children {
		kinds[c.svgNodeKind()]++
	}
	for _, k := range []string{"rect", "circle", "ellipse", "line", "polyline", "polygon"} {
		if kinds[k] != 1 {
			t.Errorf("expected 1 %s, got %d", k, kinds[k])
		}
	}
}

func TestParseSVG_InvalidXML(t *testing.T) {
	_, err := parseSVGBytes([]byte("<svg><not-closed"))
	if err == nil { t.Error("expected error for malformed XML") }
}

func TestParseSVG_NotSVGRoot(t *testing.T) {
	_, err := parseSVGBytes([]byte("<html><body></body></html>"))
	if err == nil { t.Error("expected error for non-svg root") }
}

func TestParseSVG_NoViewBox(t *testing.T) {
	svg, err := parseSVGBytes([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="50" height="30"/>`))
	if err != nil { t.Fatal(err) }
	if svg.viewBox != nil { t.Errorf("viewBox should be nil") }
	if svg.width != 50 || svg.height != 30 { t.Errorf("intrinsic = %g × %g", svg.width, svg.height) }
}
