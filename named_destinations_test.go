package asposepdf_test

import (
	"bytes"
	"strings"
	"testing"

	pdf "github.com/aspose/pdf-for-go"
)

func TestNamedDestinations_EmptyDoc(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	nd := doc.NamedDestinations()
	if nd == nil {
		t.Fatal("NamedDestinations() returned nil")
	}
	if nd.Count() != 0 {
		t.Errorf("Count = %d, want 0", nd.Count())
	}
	if nd.Document() != doc {
		t.Error("Document() != original doc")
	}
}

func TestNamedDestinations_RootStable(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	if doc.NamedDestinations() != doc.NamedDestinations() {
		t.Error("repeated calls should return same instance")
	}
}

func TestNamedDestinations_AddGet(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	page, _ := doc.Page(1)
	nd := doc.NamedDestinations()
	dest := pdf.NewDestinationXYZ(page, 100, 800, 1)
	if err := nd.Add("intro", dest); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if nd.Count() != 1 {
		t.Errorf("Count = %d", nd.Count())
	}
	if got := nd.Get("intro"); got != dest {
		t.Errorf("Get returned %v, want %v", got, dest)
	}
	if !nd.Has("intro") {
		t.Error("Has should report true")
	}
}

func TestNamedDestinations_AddNilError(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	if err := doc.NamedDestinations().Add("x", nil); err == nil {
		t.Error("Add(nil) should error")
	}
}

func TestNamedDestinations_AddEmptyNameError(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	page, _ := doc.Page(1)
	if err := doc.NamedDestinations().Add("", pdf.NewDestinationFit(page)); err == nil {
		t.Error("Add with empty name should error")
	}
}

func TestNamedDestinations_AddNamedDestValueError(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	nd := doc.NamedDestinations()
	inner := pdf.NewNamedDestination(doc, "x")
	if err := nd.Add("y", inner); err == nil {
		t.Error("Add(NamedDestination value) should error (would loop)")
	}
}

func TestNamedDestinations_AddOverwrites(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	page, _ := doc.Page(1)
	nd := doc.NamedDestinations()
	d1 := pdf.NewDestinationFit(page)
	d2 := pdf.NewDestinationXYZ(page, 0, 0, 0)
	nd.Add("x", d1)
	if err := nd.Add("x", d2); err != nil {
		t.Fatalf("overwrite Add: %v", err)
	}
	if nd.Count() != 1 {
		t.Errorf("Count after overwrite = %d", nd.Count())
	}
	if nd.Get("x") != d2 {
		t.Error("overwrite should replace value")
	}
}

func TestNamedDestinations_Remove(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	page, _ := doc.Page(1)
	nd := doc.NamedDestinations()
	nd.Add("x", pdf.NewDestinationFit(page))
	if !nd.Remove("x") {
		t.Error("Remove on present should return true")
	}
	if nd.Count() != 0 {
		t.Errorf("Count after Remove = %d", nd.Count())
	}
	if nd.Remove("x") {
		t.Error("Remove on absent should return false")
	}
}

func TestNamedDestinations_NamesSorted(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	page, _ := doc.Page(1)
	nd := doc.NamedDestinations()
	for _, n := range []string{"zebra", "apple", "mango"} {
		nd.Add(n, pdf.NewDestinationFit(page))
	}
	names := nd.Names()
	if len(names) != 3 || names[0] != "apple" || names[1] != "mango" || names[2] != "zebra" {
		t.Errorf("Names() = %v, want sorted [apple mango zebra]", names)
	}
}

func TestNamedDestinations_AllSnapshot(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	page, _ := doc.Page(1)
	nd := doc.NamedDestinations()
	nd.Add("x", pdf.NewDestinationFit(page))
	snap := nd.All()
	if len(snap) != 1 {
		t.Errorf("All() len = %d", len(snap))
	}
	// Mutate snapshot → collection should be unchanged.
	delete(snap, "x")
	if nd.Count() != 1 {
		t.Error("All() should return a snapshot, not the live map")
	}
}

func TestNamedDestinations_Clear(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	page, _ := doc.Page(1)
	nd := doc.NamedDestinations()
	nd.Add("a", pdf.NewDestinationFit(page))
	nd.Add("b", pdf.NewDestinationFit(page))
	nd.Clear()
	if nd.Count() != 0 {
		t.Error("Clear should empty the collection")
	}
}

func TestNamedDestinations_WriterEmitsNamesDests(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	page, _ := doc.Page(1)
	doc.NamedDestinations().Add("intro", pdf.NewDestinationFit(page))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	if !strings.Contains(s, "/Names") {
		t.Error("output missing /Catalog/Names entry")
	}
	if !strings.Contains(s, "/Dests") {
		t.Error("output missing /Dests inside name tree")
	}
	if !strings.Contains(s, "/Limits") {
		t.Error("output missing /Limits in tree root")
	}
	if !strings.Contains(s, "intro") {
		t.Error("output missing the registered name")
	}
}

func TestNamedDestinations_WriterSkipsEmptyCollection(t *testing.T) {
	doc := pdf.NewDocument(595, 842)
	var buf bytes.Buffer
	doc.WriteTo(&buf)
	if strings.Contains(buf.String(), "/Dests") {
		t.Error("empty collection should not produce /Dests in output")
	}
}
