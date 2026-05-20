package asposepdf_test

import (
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
