package asposepdf

import (
	"strings"
	"testing"
)

func TestFindFontExact(t *testing.T) {
	f, err := FindFont("Helvetica")
	if err != nil {
		t.Fatalf("FindFont: %v", err)
	}
	if f.BaseFont() != "Helvetica" {
		t.Errorf("FindFont(\"Helvetica\").BaseFont() = %q, want Helvetica", f.BaseFont())
	}
}

func TestFindFontCaseInsensitive(t *testing.T) {
	cases := []string{"helvetica", "HELVETICA", "HeLvEtIcA"}
	for _, name := range cases {
		f, err := FindFont(name)
		if err != nil {
			t.Fatalf("FindFont(%q): %v", name, err)
		}
		if f.BaseFont() != "Helvetica" {
			t.Errorf("FindFont(%q) = %q, want Helvetica", name, f.BaseFont())
		}
	}
}

func TestFindFontAllStandard14(t *testing.T) {
	names := []string{
		"Helvetica", "Helvetica-Bold", "Helvetica-Oblique", "Helvetica-BoldOblique",
		"Times-Roman", "Times-Bold", "Times-Italic", "Times-BoldItalic",
		"Courier", "Courier-Bold", "Courier-Oblique", "Courier-BoldOblique",
		"Symbol", "ZapfDingbats",
	}
	for _, name := range names {
		f, err := FindFont(name)
		if err != nil {
			t.Errorf("FindFont(%q): %v", name, err)
			continue
		}
		if f.BaseFont() != name {
			t.Errorf("FindFont(%q).BaseFont() = %q", name, f.BaseFont())
		}
	}
}

func TestFindFontUnknown(t *testing.T) {
	_, err := FindFont("Arial")
	if err == nil {
		t.Fatal("FindFont(\"Arial\"): expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("error message = %q, expected to contain \"unknown\"", err.Error())
	}
}
