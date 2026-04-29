package asposepdf_test

import (
	"bytes"
	"testing"

	pdf "github.com/aspose/pdf-for-go"
)

func TestFormAddTextFieldRoundTrip(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	tf, err := doc.Form().AddTextField(1, pdf.Rectangle{LLX: 50, LLY: 700, URX: 545, URY: 730}, "name")
	if err != nil {
		t.Fatalf("AddTextField: %v", err)
	}
	if tf == nil {
		t.Fatal("AddTextField returned nil *TextBoxField")
	}
	tf.SetValue("Jane Doe")

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	doc2, err := pdf.OpenStream(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("OpenStream: %v", err)
	}
	if doc2.Form().HasField("name") == false {
		t.Fatal("HasField('name') = false after roundtrip")
	}
	tf2 := doc2.Form().Field("name").(*pdf.TextBoxField)
	if got := tf2.Value(); got != "Jane Doe" {
		t.Errorf("Value() = %q, want %q", got, "Jane Doe")
	}
}
