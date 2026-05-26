// SPDX-License-Identifier: MIT

package asposepdf

import (
	"os"
	"testing"
)

func TestParseSVG_ImageInlinePNG(t *testing.T) {
	data, _ := os.ReadFile("testdata/svg/image_inline_png.svg")
	svg, err := parseSVGBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(svg.root.children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(svg.root.children))
	}
	im, ok := svg.root.children[0].(*svgImage)
	if !ok {
		t.Fatalf("expected *svgImage, got %T", svg.root.children[0])
	}
	if im.x != 10 || im.y != 20 || im.w != 80 || im.h != 60 {
		t.Errorf("dims = (%g,%g) %gx%g", im.x, im.y, im.w, im.h)
	}
	if im.format != ImageFormatPNG {
		t.Errorf("format = %v", im.format)
	}
	if len(im.data) == 0 {
		t.Error("data is empty")
	}
}

func TestParseSVG_ImageWithoutHrefIsSkipped(t *testing.T) {
	svg, _ := parseSVGBytes([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><image x="0" y="0" width="10" height="10"/></svg>`))
	for _, c := range svg.root.children {
		if _, ok := c.(*svgImage); ok {
			t.Errorf("expected no <image> node when href is missing, got %+v", c)
		}
	}
}

func TestParseSVG_ImageWithExternalHrefIsSkipped(t *testing.T) {
	svg, _ := parseSVGBytes([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><image x="0" y="0" width="10" height="10" href="https://example.com/foo.png"/></svg>`))
	for _, c := range svg.root.children {
		if _, ok := c.(*svgImage); ok {
			t.Errorf("expected no <image> node for external URL, got %+v", c)
		}
	}
}
