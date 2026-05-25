// SPDX-License-Identifier: MIT

package asposepdf_test

import (
	"bytes"
	"os"
	"testing"

	pdf "github.com/aspose-pdf-foss/aspose-pdf-foss-for-go"
)

func TestPage_AddSVG_FromPath(t *testing.T) {
	doc := pdf.NewDocumentFromFormat(pdf.PageFormatA4)
	page, _ := doc.Page(1)
	if err := page.AddSVG("testdata/svg/all_shapes.svg", pdf.Rectangle{LLX: 50, LLY: 600, URX: 250, URY: 800}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("result_files", 0755); err != nil {
		t.Fatal(err)
	}
	if err := doc.Save("result_files/TestPage_AddSVG_FromPath.pdf"); err != nil {
		t.Fatal(err)
	}
}

func TestPage_AddSVGFromStream(t *testing.T) {
	data, _ := os.ReadFile("testdata/svg/all_shapes.svg")
	doc := pdf.NewDocumentFromFormat(pdf.PageFormatA4)
	page, _ := doc.Page(1)
	if err := page.AddSVGFromStream(bytes.NewReader(data), pdf.Rectangle{LLX: 50, LLY: 600, URX: 250, URY: 800}); err != nil {
		t.Fatal(err)
	}
}

func TestPage_AddSVG_InvalidXMLReturnsError(t *testing.T) {
	doc := pdf.NewDocumentFromFormat(pdf.PageFormatA4)
	page, _ := doc.Page(1)
	if err := os.MkdirAll("result_files", 0755); err != nil {
		t.Fatal(err)
	}
	tmp := "result_files/_invalid.svg"
	os.WriteFile(tmp, []byte("<svg><not-closed"), 0644)
	defer os.Remove(tmp)
	if err := page.AddSVG(tmp, pdf.Rectangle{LLX: 0, LLY: 0, URX: 100, URY: 100}); err == nil {
		t.Error("expected error for malformed XML")
	}
}
