package asposepdf_test

import (
	"testing"

	pdf "github.com/aspose/pdf-for-go"
)

func TestOutlines_EmptyDocReturnsRoot(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	root := doc.Outlines()
	if root == nil {
		t.Fatal("Outlines() returned nil; want non-nil empty root")
	}
	if root.Count() != 0 {
		t.Errorf("empty doc root Count = %d, want 0", root.Count())
	}
	if root.Document() != doc {
		t.Error("Document() should return original doc")
	}
	if root.Parent() != nil {
		t.Error("root.Parent() should be nil")
	}
}

func TestOutlines_RootIsStable(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	r1 := doc.Outlines()
	r2 := doc.Outlines()
	if r1 != r2 {
		t.Error("Outlines() should return the same instance on repeated calls")
	}
}

func TestNewOutlineItemCollection_Standalone(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	oic := pdf.NewOutlineItemCollection(doc)
	if oic == nil {
		t.Fatal("constructor returned nil")
	}
	if oic.Document() != doc {
		t.Error("Document() should bind to provided doc")
	}
	if oic.Parent() != nil {
		t.Error("unattached item should have nil parent")
	}
	if oic.Count() != 0 {
		t.Errorf("fresh item Count = %d, want 0", oic.Count())
	}
}
