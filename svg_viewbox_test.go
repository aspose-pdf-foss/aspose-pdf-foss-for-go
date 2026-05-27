// SPDX-License-Identifier: MIT

package asposepdf

import (
	"math"
	"testing"
)

func TestParseViewBox(t *testing.T) {
	vb, ok := parseViewBox("0 0 100 50")
	if !ok || vb.x != 0 || vb.y != 0 || vb.w != 100 || vb.h != 50 {
		t.Errorf("got %+v ok=%v", vb, ok)
	}
}

func TestParseViewBox_NegativeMin(t *testing.T) {
	vb, _ := parseViewBox("-10 -20 100 50")
	if vb.x != -10 || vb.y != -20 {
		t.Errorf("got %+v", vb)
	}
}

func TestParseViewBox_Malformed(t *testing.T) {
	_, ok := parseViewBox("0 0 100")
	if ok {
		t.Error("expected failure for 3 numbers")
	}
}

func TestParsePreserveAspect_Default(t *testing.T) {
	p := parsePreserveAspect("")
	if p.align != "xMidYMid" || p.meetOrSlice != "meet" {
		t.Errorf("default = %+v", p)
	}
}

func TestParsePreserveAspect_None(t *testing.T) {
	p := parsePreserveAspect("none")
	if p.align != "none" {
		t.Errorf("got %+v", p)
	}
}

func TestParsePreserveAspect_Slice(t *testing.T) {
	p := parsePreserveAspect("xMinYMin slice")
	if p.align != "xMinYMin" || p.meetOrSlice != "slice" {
		t.Errorf("got %+v", p)
	}
}

func TestComputeViewBoxMatrix_NoViewBox_IdentityWithYFlip(t *testing.T) {
	// No viewBox + no intrinsic size: uses rect's own dims (1:1 with Y-flip).
	// Rect 100×50: SVG (0,0) → PDF (0,50); SVG (100,50) → PDF (100,0).
	m := computeViewBoxMatrix(nil, 100, 50, svgPreserveAspect{}, Rectangle{LLX: 0, LLY: 0, URX: 100, URY: 50})
	want := svgMatrix{1, 0, 0, -1, 0, 50}
	for i := range m {
		if math.Abs(m[i]-want[i]) > 1e-9 {
			t.Errorf("matrix[%d] = %g, want %g (full got=%v want=%v)", i, m[i], want[i], m, want)
		}
	}
}

func TestComputeViewBoxMatrix_Meet_LetterboxX(t *testing.T) {
	// viewBox 0 0 100 50 (ratio 2:1), rect 200×200 (ratio 1:1)
	// meet → scale = min(2, 4) = 2; render = 200×100, centered vertically.
	// xMidYMid → top pad = (200 - 100) / 2 = 50; SVG (0,0) → PDF (0, 150).
	vb := &svgViewBox{0, 0, 100, 50}
	rect := Rectangle{LLX: 0, LLY: 0, URX: 200, URY: 200}
	m := computeViewBoxMatrix(vb, 0, 0, svgPreserveAspect{align: "xMidYMid", meetOrSlice: "meet"}, rect)
	x0, y0 := transformPoint(m, 0, 0)
	x1, y1 := transformPoint(m, 100, 50)
	if math.Abs(x0) > 1e-9 || math.Abs(y0-150) > 1e-9 || math.Abs(x1-200) > 1e-9 || math.Abs(y1-50) > 1e-9 {
		t.Errorf("(0,0) → (%g, %g) want (0, 150); (100,50) → (%g, %g) want (200, 50)", x0, y0, x1, y1)
	}
}

func TestComputeViewBoxMatrix_None_Stretch(t *testing.T) {
	vb := &svgViewBox{0, 0, 100, 50}
	rect := Rectangle{LLX: 0, LLY: 0, URX: 200, URY: 200}
	m := computeViewBoxMatrix(vb, 0, 0, svgPreserveAspect{align: "none"}, rect)
	x1, y1 := transformPoint(m, 100, 50)
	if math.Abs(x1-200) > 1e-9 || math.Abs(y1) > 1e-9 {
		t.Errorf("none: (100,50) → (%g, %g) want (200, 0)", x1, y1)
	}
}
