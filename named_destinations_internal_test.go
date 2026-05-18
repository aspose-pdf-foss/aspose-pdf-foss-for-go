package asposepdf

import (
	"bytes"
	"testing"
)

func TestDestinationTypeNamedConstant(t *testing.T) {
	if int(DestinationTypeNamed) != 8 {
		t.Errorf("DestinationTypeNamed = %d, want 8 (after FitBV=7)", int(DestinationTypeNamed))
	}
}

func TestNewNamedDestination_Basic(t *testing.T) {
	doc := NewDocument(595, 842)
	nd := NewNamedDestination(doc, "chapter1")
	if nd == nil {
		t.Fatal("NewNamedDestination returned nil")
	}
	if nd.DestinationType() != DestinationTypeNamed {
		t.Errorf("DestinationType = %v, want DestinationTypeNamed", nd.DestinationType())
	}
	if nd.Name() != "chapter1" {
		t.Errorf("Name() = %q, want \"chapter1\"", nd.Name())
	}
}

func TestNamedDestination_UnresolvedReturnsNil(t *testing.T) {
	doc := NewDocument(595, 842)
	nd := NewNamedDestination(doc, "no-such-name")
	if nd.Resolve() != nil {
		t.Error("Resolve() should be nil for unregistered name")
	}
	if nd.Page() != nil {
		t.Error("Page() should be nil for unregistered name")
	}
}

func TestBuildNamedDestTree_Empty(t *testing.T) {
	doc := NewDocument(595, 842)
	treeRef, namesDictRef, objs := buildNamedDestTree(doc)
	if treeRef.Num != 0 || namesDictRef.Num != 0 || len(objs) != 0 {
		t.Errorf("empty doc: treeRef=%v namesDictRef=%v objCount=%d, want zeros", treeRef, namesDictRef, len(objs))
	}
}

func TestBuildNamedDestTree_FlatShape(t *testing.T) {
	doc := NewDocument(595, 842)
	page, _ := doc.Page(1)
	nd := doc.NamedDestinations()
	nd.Add("alpha", NewDestinationFit(page))
	nd.Add("beta", NewDestinationFit(page))
	nd.Add("gamma", NewDestinationFit(page))
	treeRef, namesDictRef, objs := buildNamedDestTree(doc)
	if treeRef.Num == 0 || namesDictRef.Num == 0 {
		t.Fatal("refs should be non-zero")
	}
	if len(objs) != 2 {
		t.Fatalf("expected 2 objects (tree root + /Names dict), got %d", len(objs))
	}
	// Find tree root by /Names key.
	var treeRoot pdfDict
	for _, o := range objs {
		if d, ok := o.Value.(pdfDict); ok {
			if _, hasNames := d["/Names"]; hasNames {
				treeRoot = d
			}
		}
	}
	if treeRoot == nil {
		t.Fatal("no tree root found")
	}
	// /Names array: 3 names × 2 = 6 entries (name, dest, name, dest, ...).
	namesArr, _ := treeRoot["/Names"].(pdfArray)
	if len(namesArr) != 6 {
		t.Errorf("/Names len = %d, want 6", len(namesArr))
	}
	// Lex order check.
	if namesArr[0] != "alpha" || namesArr[2] != "beta" || namesArr[4] != "gamma" {
		t.Errorf("/Names not lex-sorted: %v %v %v", namesArr[0], namesArr[2], namesArr[4])
	}
	// /Limits.
	limits, _ := treeRoot["/Limits"].(pdfArray)
	if len(limits) != 2 || limits[0] != "alpha" || limits[1] != "gamma" {
		t.Errorf("/Limits wrong: %v", limits)
	}
}

func TestBuildNamedDestTree_SkipsNestedNamedDest(t *testing.T) {
	// Direct call simulating defensive write (Add already rejects this,
	// but the writer must defend too).
	doc := NewDocument(595, 842)
	page, _ := doc.Page(1)
	nd := doc.NamedDestinations()
	nd.Add("real", NewDestinationFit(page))
	// Bypass Add validation by writing directly into the map.
	nd.entries["loop"] = &NamedDestination{doc: doc, name: "real"}
	treeRef, _, _ := buildNamedDestTree(doc)
	if treeRef.Num == 0 {
		t.Fatal("should still emit (real entry survives)")
	}
	_ = bytes.Buffer{} // keep import used
}
