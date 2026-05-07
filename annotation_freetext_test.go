package asposepdf_test

import (
	"bytes"
	"testing"

	pdf "github.com/aspose/pdf-for-go"
)

func TestFreeTextAnnotationContentsRoundTrip(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	page, _ := doc.Page(1)
	ft := pdf.NewFreeTextAnnotation(page,
		pdf.Rectangle{LLX: 50, LLY: 600, URX: 300, URY: 700},
		"Hello, FreeText!",
		pdf.TextStyle{Font: pdf.FontHelvetica, Size: 12})
	if err := page.Annotations().Add(ft); err != nil {
		t.Fatalf("Add: %v", err)
	}
	var buf bytes.Buffer
	doc.WriteTo(&buf)
	doc2, _ := pdf.OpenStream(bytes.NewReader(buf.Bytes()))
	got := doc2.Pages()[0].Annotations().At(0)
	if got.AnnotationType() != pdf.AnnotationTypeFreeText {
		t.Errorf("type = %v, want AnnotationTypeFreeText", got.AnnotationType())
	}
	ft2, ok := got.(*pdf.FreeTextAnnotation)
	if !ok {
		t.Fatalf("concrete type = %T", got)
	}
	if ft2.Contents() != "Hello, FreeText!" {
		t.Errorf("Contents = %q, want \"Hello, FreeText!\"", ft2.Contents())
	}
}

func TestFreeTextAnnotationSetContentsRegenerates(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	page, _ := doc.Page(1)
	ft := pdf.NewFreeTextAnnotation(page,
		pdf.Rectangle{LLX: 50, LLY: 600, URX: 300, URY: 700},
		"initial",
		pdf.TextStyle{})
	if err := page.Annotations().Add(ft); err != nil {
		t.Fatalf("Add: %v", err)
	}
	ft.SetContents("updated")
	var buf bytes.Buffer
	doc.WriteTo(&buf)
	doc2, _ := pdf.OpenStream(bytes.NewReader(buf.Bytes()))
	ft2 := doc2.Pages()[0].Annotations().At(0).(*pdf.FreeTextAnnotation)
	if ft2.Contents() != "updated" {
		t.Errorf("Contents after SetContents = %q, want \"updated\"", ft2.Contents())
	}
}

func TestFreeTextAnnotationConstructorPanicOnNilPage(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic")
		}
	}()
	pdf.NewFreeTextAnnotation(nil, pdf.Rectangle{}, "", pdf.TextStyle{})
}

func TestFreeTextAnnotationTextStyleRoundTrip(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	page, _ := doc.Page(1)
	style := pdf.TextStyle{
		Font:       pdf.FontHelveticaBold,
		Size:       14,
		Color:      &pdf.Color{R: 1, G: 0, B: 0, A: 1},
		Background: &pdf.Color{R: 1, G: 1, B: 0, A: 1},
		HAlign:     pdf.HAlignCenter,
	}
	ft := pdf.NewFreeTextAnnotation(page,
		pdf.Rectangle{LLX: 50, LLY: 600, URX: 300, URY: 700},
		"styled text", style)
	if err := page.Annotations().Add(ft); err != nil {
		t.Fatalf("Add: %v", err)
	}
	var buf bytes.Buffer
	doc.WriteTo(&buf)
	doc2, _ := pdf.OpenStream(bytes.NewReader(buf.Bytes()))
	ft2 := doc2.Pages()[0].Annotations().At(0).(*pdf.FreeTextAnnotation)
	got := ft2.TextStyle()
	if got.Size != 14 {
		t.Errorf("Size = %v, want 14", got.Size)
	}
	if got.Color == nil || got.Color.R != 1 || got.Color.G != 0 || got.Color.B != 0 {
		t.Errorf("Color = %+v, want {1 0 0}", got.Color)
	}
	if got.Background == nil || got.Background.R != 1 || got.Background.G != 1 || got.Background.B != 0 {
		t.Errorf("Background = %+v, want {1 1 0}", got.Background)
	}
	if got.HAlign != pdf.HAlignCenter {
		t.Errorf("HAlign = %v, want HAlignCenter", got.HAlign)
	}
	// Font: standard14 round-trip should preserve PostScript name.
	if got.Font == nil {
		t.Error("Font = nil, want HelveticaBold")
	} else if name := got.Font.BaseFont(); name != "Helvetica-Bold" {
		t.Errorf("Font.BaseFont() = %q, want \"Helvetica-Bold\"", name)
	}
}

func TestFreeTextAnnotationSetTextStyleRegenerates(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	page, _ := doc.Page(1)
	ft := pdf.NewFreeTextAnnotation(page,
		pdf.Rectangle{LLX: 0, LLY: 0, URX: 100, URY: 100},
		"x", pdf.TextStyle{})
	if err := page.Annotations().Add(ft); err != nil {
		t.Fatalf("Add: %v", err)
	}
	ft.SetTextStyle(pdf.TextStyle{
		Font:   pdf.FontTimesRoman,
		Size:   18,
		Color:  &pdf.Color{R: 0, G: 0, B: 1, A: 1},
		HAlign: pdf.HAlignRight,
	})
	var buf bytes.Buffer
	doc.WriteTo(&buf)
	doc2, _ := pdf.OpenStream(bytes.NewReader(buf.Bytes()))
	ft2 := doc2.Pages()[0].Annotations().At(0).(*pdf.FreeTextAnnotation)
	got := ft2.TextStyle()
	if got.Size != 18 {
		t.Errorf("Size = %v", got.Size)
	}
	if got.HAlign != pdf.HAlignRight {
		t.Errorf("HAlign = %v", got.HAlign)
	}
}

func TestFreeTextAnnotationDefaultTextStyle(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	page, _ := doc.Page(1)
	ft := pdf.NewFreeTextAnnotation(page,
		pdf.Rectangle{LLX: 0, LLY: 0, URX: 100, URY: 100},
		"x", pdf.TextStyle{})
	got := ft.TextStyle()
	// Empty TextStyle → defaults: Helvetica 12pt black, no bg, left-align.
	if got.Size != 12 {
		t.Errorf("default Size = %v, want 12", got.Size)
	}
	if got.HAlign != pdf.HAlignLeft {
		t.Errorf("default HAlign = %v, want HAlignLeft", got.HAlign)
	}
}
