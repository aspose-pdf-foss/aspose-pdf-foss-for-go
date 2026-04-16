package asposepdf

import "testing"

func TestFontPDFName(t *testing.T) {
	cases := []struct {
		font Font
		want string
	}{
		{FontHelvetica, "/Helvetica"},
		{FontHelveticaBold, "/Helvetica-Bold"},
		{FontHelveticaOblique, "/Helvetica-Oblique"},
		{FontHelveticaBoldOblique, "/Helvetica-BoldOblique"},
		{FontTimesRoman, "/Times-Roman"},
		{FontTimesBold, "/Times-Bold"},
		{FontTimesItalic, "/Times-Italic"},
		{FontTimesBoldItalic, "/Times-BoldItalic"},
		{FontCourier, "/Courier"},
		{FontCourierBold, "/Courier-Bold"},
		{FontCourierOblique, "/Courier-Oblique"},
		{FontCourierBoldOblique, "/Courier-BoldOblique"},
		{FontSymbol, "/Symbol"},
		{FontZapfDingbats, "/ZapfDingbats"},
	}
	for _, tc := range cases {
		got := fontPDFName(tc.font)
		if got != tc.want {
			t.Errorf("fontPDFName(%d) = %q, want %q", tc.font, got, tc.want)
		}
	}
}

func TestFontPDFNameInvalid(t *testing.T) {
	got := fontPDFName(Font(999))
	if got != "/Helvetica" {
		t.Errorf("fontPDFName(999) = %q, want /Helvetica (fallback)", got)
	}
}
