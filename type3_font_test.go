// SPDX-License-Identifier: MIT

package asposepdf_test

import (
	"image"
	"strings"
	"testing"

	pdf "github.com/aspose-pdf-foss/aspose-pdf-foss-for-go"
)

// buildStarFont defines a two-glyph Type3 font: a filled star for '★' and a
// square outline for '□', both drawn with the vector API in glyph space.
func buildStarFont(t *testing.T, doc *pdf.Document) *pdf.Type3Font {
	t.Helper()
	t3 := doc.CreateType3Font()

	star, err := t3.AddGlyph('★', 900)
	if err != nil {
		t.Fatal(err)
	}
	fill := pdf.Color{R: 0.9, G: 0.6, B: 0.1, A: 1}
	pts := []pdf.Point{
		{X: 450, Y: 900}, {X: 561, Y: 555}, {X: 900, Y: 555}, {X: 629, Y: 342},
		{X: 733, Y: 0}, {X: 450, Y: 210}, {X: 167, Y: 0}, {X: 271, Y: 342},
		{X: 0, Y: 555}, {X: 339, Y: 555},
	}
	if err := star.DrawPolygon(pts, pdf.ShapeStyle{FillColor: &fill}); err != nil {
		t.Fatal(err)
	}

	square, err := t3.AddGlyph('□', 800)
	if err != nil {
		t.Fatal(err)
	}
	if err := square.DrawRectangle(pdf.Rectangle{LLX: 60, LLY: 60, URX: 740, URY: 740},
		pdf.ShapeStyle{LineStyle: pdf.LineStyle{Width: 60}}); err != nil {
		t.Fatal(err)
	}
	return t3
}

// The authored font draws, survives Save+Open, extracts its text back, and
// renders visible glyph pixels.
func TestType3FontAuthoring(t *testing.T) {
	doc := pdf.NewDocumentFromFormat(pdf.PageFormatA4)
	p, _ := doc.Page(1)
	t3 := buildStarFont(t, doc)

	style := pdf.TextStyle{Font: t3, Size: 36}
	if err := p.AddText("★□★", style, pdf.Rectangle{LLX: 100, LLY: 600, URX: 500, URY: 700}); err != nil {
		t.Fatal(err)
	}
	// Mixing with a standard font on the same page must coexist.
	if err := p.AddText("rating:", pdf.TextStyle{Font: pdf.FontHelvetica, Size: 14},
		pdf.Rectangle{LLX: 100, LLY: 710, URX: 300, URY: 730}); err != nil {
		t.Fatal(err)
	}

	// Round-trip.
	path := "result_files/type3_font_test.pdf"
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}
	re, err := pdf.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	rp, _ := re.Page(1)
	txt, err := rp.ExtractText()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(txt, "★□★") {
		t.Errorf("extraction lost the type3 glyph runes: %q", txt)
	}

	// Render: the star area must carry non-white pixels of the fill colour.
	img, err := rp.RenderImage(pdf.RenderOptions{DPI: 96})
	if err != nil {
		t.Fatal(err)
	}
	rgba := img.(*image.RGBA)
	coloured := 0
	b := rgba.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := rgba.At(x, y).RGBA()
			if r>>8 > 180 && g>>8 > 100 && g>>8 < 220 && bl>>8 < 120 {
				coloured++ // the orange star fill
			}
		}
	}
	if coloured < 100 {
		t.Errorf("rendered star pixels = %d; want >= 100 (glyph did not draw)", coloured)
	}
}

// Guard-rails: duplicate runes, bad width, post-freeze AddGlyph, empty font.
func TestType3FontErrors(t *testing.T) {
	doc := pdf.NewDocumentFromFormat(pdf.PageFormatA4)
	p, _ := doc.Page(1)

	empty := doc.CreateType3Font()
	if err := p.AddText("x", pdf.TextStyle{Font: empty, Size: 12},
		pdf.Rectangle{LLX: 10, LLY: 10, URX: 100, URY: 30}); err == nil {
		t.Error("empty type3 font must error on use")
	}

	t3 := doc.CreateType3Font()
	if _, err := t3.AddGlyph('a', 500); err != nil {
		t.Fatal(err)
	}
	if _, err := t3.AddGlyph('a', 500); err == nil {
		t.Error("duplicate rune accepted")
	}
	if _, err := t3.AddGlyph('b', 0); err == nil {
		t.Error("zero width accepted")
	}
	if err := p.AddText("a", pdf.TextStyle{Font: t3, Size: 12},
		pdf.Rectangle{LLX: 10, LLY: 40, URX: 100, URY: 60}); err != nil {
		t.Fatal(err)
	}
	if _, err := t3.AddGlyph('c', 500); err == nil {
		t.Error("AddGlyph after freeze accepted")
	}
}
