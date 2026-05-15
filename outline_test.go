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

func TestOutlines_TitleGetSet(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	oic := pdf.NewOutlineItemCollection(doc)
	if oic.Title() != "" {
		t.Errorf("default Title = %q, want \"\"", oic.Title())
	}
	oic.SetTitle("Chapter 1")
	if oic.Title() != "Chapter 1" {
		t.Errorf("Title = %q", oic.Title())
	}
}

func TestOutlines_BoldItalic(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	oic := pdf.NewOutlineItemCollection(doc)
	if oic.Bold() || oic.Italic() {
		t.Error("default Bold/Italic should be false")
	}
	oic.SetBold(true)
	oic.SetItalic(true)
	if !oic.Bold() || !oic.Italic() {
		t.Error("Set* should flip")
	}
	oic.SetBold(false)
	if oic.Bold() || !oic.Italic() {
		t.Errorf("after SetBold(false): Bold=%v Italic=%v", oic.Bold(), oic.Italic())
	}
}

func TestOutlines_Color(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	oic := pdf.NewOutlineItemCollection(doc)
	if oic.Color() != nil {
		t.Errorf("default Color should be nil")
	}
	red := &pdf.Color{R: 1, G: 0, B: 0, A: 1}
	oic.SetColor(red)
	got := oic.Color()
	if got == nil || got.R != 1 {
		t.Errorf("Color = %+v", got)
	}
	oic.SetColor(nil)
	if oic.Color() != nil {
		t.Error("SetColor(nil) should clear")
	}
}

func TestOutlines_IsExpandedDefaultsTrue(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	oic := pdf.NewOutlineItemCollection(doc)
	if !oic.IsExpanded() {
		t.Error("default IsExpanded should be true (matches Aspose .NET)")
	}
	oic.SetIsExpanded(false)
	if oic.IsExpanded() {
		t.Error("after SetIsExpanded(false), should be false")
	}
}

func TestOutlines_DestinationGetSet(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	page, _ := doc.Page(1)
	oic := pdf.NewOutlineItemCollection(doc)
	if oic.Destination() != nil {
		t.Error("default Destination should be nil")
	}
	d := pdf.NewDestinationXYZ(page, 100, 800, 1)
	oic.SetDestination(d)
	if oic.Destination() != d {
		t.Error("Destination should round-trip via pointer identity")
	}
	oic.SetDestination(nil)
	if oic.Destination() != nil {
		t.Error("SetDestination(nil) should clear")
	}
}

func TestOutlines_ActionGetSet(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	oic := pdf.NewOutlineItemCollection(doc)
	if oic.Action() != nil {
		t.Error("default Action should be nil")
	}
	a := pdf.NewGoToURIAction("https://example.com")
	oic.SetAction(a)
	if oic.Action() != a {
		t.Error("Action should round-trip via pointer identity")
	}
}

func TestOutlines_DestinationAndActionCoexist(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	page, _ := doc.Page(1)
	oic := pdf.NewOutlineItemCollection(doc)
	oic.SetDestination(pdf.NewDestinationFit(page))
	oic.SetAction(pdf.NewGoToURIAction("https://example.com"))
	if oic.Destination() == nil {
		t.Error("Destination should remain after SetAction")
	}
	if oic.Action() == nil {
		t.Error("Action should remain after SetDestination")
	}
}
