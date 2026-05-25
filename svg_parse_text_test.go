// SPDX-License-Identifier: MIT

package asposepdf

import (
	"os"
	"testing"
)

func TestParseSVG_TextBasic(t *testing.T) {
	data, _ := os.ReadFile("testdata/svg/text_basic.svg")
	svg, err := parseSVGBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(svg.root.children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(svg.root.children))
	}
	tn, ok := svg.root.children[0].(*svgText)
	if !ok {
		t.Fatalf("expected *svgText, got %T", svg.root.children[0])
	}
	if len(tn.runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(tn.runs))
	}
	run := tn.runs[0]
	if run.text != "Hello world" {
		t.Errorf("text = %q", run.text)
	}
	if run.x != 10 || run.y != 50 {
		t.Errorf("position = (%g, %g)", run.x, run.y)
	}
	if run.style.fontFamily != "Arial" {
		t.Errorf("fontFamily = %q", run.style.fontFamily)
	}
	if run.style.fontSize != 14 {
		t.Errorf("fontSize = %g", run.style.fontSize)
	}
}

func TestParseSVG_TextWhitespaceCollapsed(t *testing.T) {
	svg, _ := parseSVGBytes([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><text x="0" y="0">  hello   world  </text></svg>`))
	tn, _ := svg.root.children[0].(*svgText)
	if tn == nil || len(tn.runs) != 1 {
		t.Fatal("expected one run")
	}
	if tn.runs[0].text != "hello world" {
		t.Errorf("text = %q", tn.runs[0].text)
	}
}

func TestParseSVG_TextInheritsGroupFont(t *testing.T) {
	svg, _ := parseSVGBytes([]byte(`<svg xmlns="http://www.w3.org/2000/svg">
		<g font-family="Times" font-size="18">
			<text x="0" y="0">Hi</text>
		</g>
	</svg>`))
	g, _ := svg.root.children[0].(*svgGroup)
	tn, _ := g.children[0].(*svgText)
	if tn == nil {
		t.Fatal("no text node")
	}
	if tn.runs[0].style.fontFamily != "Times" {
		t.Errorf("inherited fontFamily = %q", tn.runs[0].style.fontFamily)
	}
	if tn.runs[0].style.fontSize != 18 {
		t.Errorf("inherited fontSize = %g", tn.runs[0].style.fontSize)
	}
}

func TestParseSVG_TextTSpan(t *testing.T) {
	data, _ := os.ReadFile("testdata/svg/text_tspan.svg")
	svg, _ := parseSVGBytes(data)
	tn, _ := svg.root.children[0].(*svgText)
	if tn == nil || len(tn.runs) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(tn.runs))
	}
	if tn.runs[0].text != "Hello" {
		t.Errorf("run[0] = %q", tn.runs[0].text)
	}
	if tn.runs[1].text != "world" || !tn.runs[1].style.bold {
		t.Errorf("run[1] = %q bold=%v", tn.runs[1].text, tn.runs[1].style.bold)
	}
	if tn.runs[2].text != "!" {
		t.Errorf("run[2] = %q", tn.runs[2].text)
	}
	for i, run := range tn.runs {
		if run.y != 50 {
			t.Errorf("run[%d].y = %g, want 50", i, run.y)
		}
	}
	if !(tn.runs[0].x <= tn.runs[1].x && tn.runs[1].x <= tn.runs[2].x) {
		t.Errorf("x ordering broken: %g %g %g", tn.runs[0].x, tn.runs[1].x, tn.runs[2].x)
	}
}

func TestParseSVG_TextTSpanAbsoluteXY(t *testing.T) {
	data, _ := os.ReadFile("testdata/svg/text_tspan_abs.svg")
	svg, _ := parseSVGBytes(data)
	tn, _ := svg.root.children[0].(*svgText)
	if tn == nil || len(tn.runs) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(tn.runs))
	}
	if tn.runs[0].x != 10 || tn.runs[0].y != 50 {
		t.Errorf("run[0] pos = (%g, %g)", tn.runs[0].x, tn.runs[0].y)
	}
	if tn.runs[1].x != 100 || tn.runs[1].y != 80 {
		t.Errorf("run[1] pos = (%g, %g)", tn.runs[1].x, tn.runs[1].y)
	}
	if tn.runs[2].y != 80 {
		t.Errorf("run[2].y = %g, want 80", tn.runs[2].y)
	}
	if tn.runs[2].x <= 100 {
		t.Errorf("run[2].x = %g, want > 100", tn.runs[2].x)
	}
}

func TestParseSVG_TextTSpanDxDy(t *testing.T) {
	data, _ := os.ReadFile("testdata/svg/text_tspan_dxdy.svg")
	svg, _ := parseSVGBytes(data)
	tn, _ := svg.root.children[0].(*svgText)
	if tn == nil || len(tn.runs) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(tn.runs))
	}
	// dx="20" dy="-5" applied at the start of tspan
	if tn.runs[1].y != 45 {
		t.Errorf("run[1].y = %g, want 45", tn.runs[1].y)
	}
}
