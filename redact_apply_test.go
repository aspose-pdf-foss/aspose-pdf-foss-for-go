package asposepdf_test

import (
	"bytes"
	"testing"

	pdf "github.com/aspose/pdf-for-go"
)

func TestValidateRedactionsEmpty(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	if err := doc.ValidateRedactions(); err != nil {
		t.Errorf("ValidateRedactions on empty doc: %v", err)
	}
}

func TestValidateRedactionsWithRedact(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	page, _ := doc.Page(1)
	page.AddText("Confidential", pdf.TextStyle{Font: pdf.FontHelvetica, Size: 12},
		pdf.Rectangle{LLX: 50, LLY: 700, URX: 300, URY: 720})
	ra := pdf.NewRedactAnnotation(page, pdf.Rectangle{LLX: 100, LLY: 700, URX: 250, URY: 720})
	if err := page.Annotations().Add(ra); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := doc.ValidateRedactions(); err != nil {
		t.Errorf("ValidateRedactions: %v", err)
	}
}

func TestValidateRedactionsNoPagesWithRedact(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	page, _ := doc.Page(1)
	page.AddText("Hello", pdf.TextStyle{Font: pdf.FontHelvetica, Size: 12},
		pdf.Rectangle{LLX: 50, LLY: 700, URX: 200, URY: 720})
	// No redact annotations — Validate should still succeed (no-op).
	if err := doc.ValidateRedactions(); err != nil {
		t.Errorf("ValidateRedactions: %v", err)
	}
}

func TestApplyRedactionsStubReturnsNil(t *testing.T) {
	// Stub for Task 8 — full implementation in Task 12.
	doc := pdf.NewDocument(595, 842)
	if err := doc.ApplyRedactions(); err != nil {
		t.Errorf("ApplyRedactions stub: %v", err)
	}
}

func TestValidateRedactionsRoundTripParseability(t *testing.T) {
	// After writing + reading, Validate should still pass on the parsed doc.
	doc := pdf.NewDocument(595, 842)
	page, _ := doc.Page(1)
	page.AddText("Test", pdf.TextStyle{Font: pdf.FontHelvetica, Size: 12},
		pdf.Rectangle{LLX: 50, LLY: 700, URX: 200, URY: 720})
	ra := pdf.NewRedactAnnotation(page, pdf.Rectangle{LLX: 50, LLY: 700, URX: 200, URY: 720})
	page.Annotations().Add(ra)
	var buf bytes.Buffer
	doc.WriteTo(&buf)
	doc2, _ := pdf.OpenStream(bytes.NewReader(buf.Bytes()))
	if err := doc2.ValidateRedactions(); err != nil {
		t.Errorf("ValidateRedactions after roundtrip: %v", err)
	}
}
