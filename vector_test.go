package asposepdf_test

import (
	"strings"
	"testing"

	pdf "github.com/aspose/pdf-for-go"
)

func TestVector_LineCapConstants(t *testing.T) {
	if pdf.LineCapButt != 0 {
		t.Errorf("LineCapButt = %d, want 0", pdf.LineCapButt)
	}
	if pdf.LineCapRound != 1 {
		t.Errorf("LineCapRound = %d, want 1", pdf.LineCapRound)
	}
	if pdf.LineCapSquare != 2 {
		t.Errorf("LineCapSquare = %d, want 2", pdf.LineCapSquare)
	}
}

func TestVector_LineJoinConstants(t *testing.T) {
	if pdf.LineJoinMiter != 0 || pdf.LineJoinRound != 1 || pdf.LineJoinBevel != 2 {
		t.Errorf("LineJoin enum mismatch: Miter=%d Round=%d Bevel=%d",
			pdf.LineJoinMiter, pdf.LineJoinRound, pdf.LineJoinBevel)
	}
}

func TestVector_LineStyleZeroValue(t *testing.T) {
	var s pdf.LineStyle
	if s.Color != nil || s.Width != 0 || s.DashPattern != nil ||
		s.Cap != pdf.LineCapButt || s.Join != pdf.LineJoinMiter {
		t.Errorf("LineStyle zero value mismatch: %+v", s)
	}
}

func TestVector_ShapeStyleEmbedsLineStyle(t *testing.T) {
	s := pdf.ShapeStyle{
		LineStyle: pdf.LineStyle{Width: 2},
		FillColor: &pdf.Color{R: 1, G: 0, B: 0, A: 1},
	}
	if s.Width != 2 {
		t.Errorf("embedded LineStyle.Width = %g, want 2", s.Width)
	}
	if s.FillColor == nil || s.FillColor.R != 1 {
		t.Error("FillColor not preserved")
	}
}

func TestDrawLine_BasicSolidStroke(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	page, _ := doc.Page(1)
	err := page.DrawLine(
		pdf.Point{X: 100, Y: 100},
		pdf.Point{X: 200, Y: 150},
		pdf.LineStyle{Color: &pdf.Color{R: 1, G: 0, B: 0, A: 1}, Width: 2},
	)
	if err != nil {
		t.Fatal(err)
	}
	s := renderedContent(t, doc)
	for _, want := range []string{"100 100 m", "200 150 l", "S", "1 0 0 RG", "2 w"} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestDrawLine_DashPattern(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	page, _ := doc.Page(1)
	err := page.DrawLine(
		pdf.Point{X: 0, Y: 0}, pdf.Point{X: 100, Y: 0},
		pdf.LineStyle{Width: 1, DashPattern: []float64{4, 2}},
	)
	if err != nil {
		t.Fatal(err)
	}
	s := renderedContent(t, doc)
	if !strings.Contains(s, "[4 2] 0 d") {
		t.Errorf("output missing dash pattern: %s", s)
	}
}

func TestDrawLine_WidthZero_NoStroke(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	page, _ := doc.Page(1)
	err := page.DrawLine(pdf.Point{}, pdf.Point{X: 100}, pdf.LineStyle{Width: 0})
	if err != nil {
		t.Fatal(err)
	}
	s := renderedContent(t, doc)
	if strings.Contains(s, " S\n") {
		t.Error("width=0 should not emit stroke op")
	}
}

func TestDrawLine_LineCapRound(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	page, _ := doc.Page(1)
	_ = page.DrawLine(pdf.Point{}, pdf.Point{X: 50}, pdf.LineStyle{
		Width: 4, Cap: pdf.LineCapRound,
	})
	s := renderedContent(t, doc)
	if !strings.Contains(s, "1 J") {
		t.Error("LineCapRound should emit `1 J`")
	}
}
