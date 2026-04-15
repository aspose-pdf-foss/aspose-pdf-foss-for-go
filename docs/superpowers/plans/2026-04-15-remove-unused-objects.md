# RemoveUnusedObjects Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `(*Document).RemoveUnusedObjects()` to delete orphaned objects from the in-memory object store, reducing output file size.

**Architecture:** A reachability walker (`collectReachableIDs`) traverses the object graph from page roots, marking visited IDs. `RemoveUnusedObjects` deletes any ID not in the visited set from `doc.objects`.

**Tech Stack:** Pure Go, no external dependencies. Reuses existing regex `reRefDoc` for stream byte scanning.

---

### Task 1: Implement `collectReachableIDs`

**Files:**
- Modify: `doc.go`
- Create: `document_internal_test.go`

- [ ] **Step 1: Write failing test**

Create `document_internal_test.go`:

```go
package asposepdf

import (
	"testing"
)

func TestCollectReachableIDs(t *testing.T) {
	// Page references object 1 (image) via /XObject dict.
	// Object 10 is orphaned (not referenced from page).
	imgStream := &pdfStream{
		Dict: pdfDict{
			"/Type":    pdfName("/XObject"),
			"/Subtype": pdfName("/Image"),
			"/Width":   10,
			"/Height":  10,
		},
		Data:    []byte{0xFF, 0xD8, 0xFF, 0xD9},
		Decoded: false,
	}
	imgObj := &pdfObject{Num: 1, Value: imgStream}

	contentStream := &pdfStream{
		Dict:    pdfDict{},
		Data:    []byte("q\n10 0 0 10 50 50 cm\n/Im0 Do\nQ\n"),
		Decoded: true,
	}
	contentObj := &pdfObject{Num: 2, Value: contentStream}

	pageDict := pdfDict{
		"/Type":     pdfName("/Page"),
		"/MediaBox": pdfArray{0.0, 0.0, 200.0, 300.0},
		"/Resources": pdfDict{
			"/XObject": pdfDict{
				"/Im0": pdfRef{Num: 1},
			},
		},
		"/Contents": pdfRef{Num: 2},
	}
	pageObj := &pdfObject{Num: 3, Value: pageDict}

	// Orphaned object — not referenced from the page.
	orphanStream := &pdfStream{
		Dict: pdfDict{
			"/Type":    pdfName("/XObject"),
			"/Subtype": pdfName("/Image"),
			"/Width":   5,
			"/Height":  5,
		},
		Data:    []byte{0xFF, 0xD8, 0xFF, 0xD9},
		Decoded: false,
	}
	orphanObj := &pdfObject{Num: 10, Value: orphanStream}

	objects := map[int]*pdfObject{
		1: imgObj, 2: contentObj, 3: pageObj, 10: orphanObj,
	}

	reachable := collectReachableIDs(objects, []*pdfObject{pageObj})

	// Page (3), image (1), content (2) should be reachable.
	if !reachable[3] {
		t.Error("page object should be reachable")
	}
	if !reachable[1] {
		t.Error("image object should be reachable")
	}
	if !reachable[2] {
		t.Error("content object should be reachable")
	}
	// Orphan (10) should NOT be reachable.
	if reachable[10] {
		t.Error("orphaned object should not be reachable")
	}
}

func TestCollectReachableIDsCyclic(t *testing.T) {
	// Two objects referencing each other, but not reachable from any page.
	dictA := pdfDict{"/Ref": pdfRef{Num: 2}}
	objA := &pdfObject{Num: 1, Value: dictA}

	dictB := pdfDict{"/Ref": pdfRef{Num: 1}}
	objB := &pdfObject{Num: 2, Value: dictB}

	// A simple page with no references to objA or objB.
	pageDict := pdfDict{
		"/Type":     pdfName("/Page"),
		"/MediaBox": pdfArray{0.0, 0.0, 100.0, 100.0},
	}
	pageObj := &pdfObject{Num: 3, Value: pageDict}

	objects := map[int]*pdfObject{1: objA, 2: objB, 3: pageObj}

	reachable := collectReachableIDs(objects, []*pdfObject{pageObj})

	if reachable[1] {
		t.Error("cyclic orphan A should not be reachable")
	}
	if reachable[2] {
		t.Error("cyclic orphan B should not be reachable")
	}
	if !reachable[3] {
		t.Error("page should be reachable")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run "TestCollectReachableIDs" -v ./...`
Expected: FAIL — `collectReachableIDs` not defined.

- [ ] **Step 3: Implement collectReachableIDs**

Add to `doc.go`, after the `collectDictDeps` function (around line 223):

```go
// collectReachableIDs returns the set of object IDs reachable from the given root objects.
// Used by RemoveUnusedObjects to identify orphaned objects.
func collectReachableIDs(objects map[int]*pdfObject, roots []*pdfObject) map[int]bool {
	visited := make(map[int]bool)
	for _, root := range roots {
		visited[root.Num] = true
		markReachable(objects, root.Value, visited)
	}
	return visited
}

func markReachable(objects map[int]*pdfObject, v pdfValue, visited map[int]bool) {
	switch val := v.(type) {
	case pdfRef:
		if visited[val.Num] {
			return
		}
		obj, ok := objects[val.Num]
		if !ok {
			return
		}
		visited[val.Num] = true
		markReachable(objects, obj.Value, visited)
	case pdfDict:
		for _, dv := range val {
			markReachable(objects, dv, visited)
		}
	case pdfArray:
		for _, av := range val {
			markReachable(objects, av, visited)
		}
	case *pdfStream:
		for _, dv := range val.Dict {
			markReachable(objects, dv, visited)
		}
		// Scan stream bytes for inline references (e.g. content streams).
		for _, m := range reRefDoc.FindAllSubmatch(val.Data, -1) {
			n := toIntBytes(m[1])
			if n > 0 && !visited[n] {
				if obj, ok := objects[n]; ok {
					visited[n] = true
					markReachable(objects, obj.Value, visited)
				}
			}
		}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run "TestCollectReachableIDs" -v ./...`
Expected: PASS.

- [ ] **Step 5: Run all tests**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add doc.go document_internal_test.go
git commit -m "feat: add collectReachableIDs for object graph traversal"
```

---

### Task 2: Implement `RemoveUnusedObjects`

**Files:**
- Modify: `document.go`
- Modify: `document_internal_test.go`

- [ ] **Step 1: Write failing tests**

Add to `document_internal_test.go`:

```go
func TestRemoveUnusedObjectsBasic(t *testing.T) {
	// Page references objects 1 and 2. Object 10 is orphaned.
	imgStream := &pdfStream{
		Dict: pdfDict{
			"/Type":    pdfName("/XObject"),
			"/Subtype": pdfName("/Image"),
			"/Width":   10,
			"/Height":  10,
		},
		Data:    []byte{0xFF, 0xD8, 0xFF, 0xD9},
		Decoded: false,
	}
	contentStream := &pdfStream{
		Dict:    pdfDict{},
		Data:    []byte("q\n10 0 0 10 50 50 cm\n/Im0 Do\nQ\n"),
		Decoded: true,
	}
	pageDict := pdfDict{
		"/Type":     pdfName("/Page"),
		"/MediaBox": pdfArray{0.0, 0.0, 200.0, 300.0},
		"/Resources": pdfDict{
			"/XObject": pdfDict{
				"/Im0": pdfRef{Num: 1},
			},
		},
		"/Contents": pdfRef{Num: 2},
	}
	pageObj := &pdfObject{Num: 3, Value: pageDict}
	orphanObj := &pdfObject{Num: 10, Value: &pdfStream{
		Dict:    pdfDict{"/Type": pdfName("/XObject")},
		Data:    []byte{0xFF, 0xD8, 0xFF, 0xD9},
		Decoded: false,
	}}

	doc := &Document{
		objects: map[int]*pdfObject{
			1: {Num: 1, Value: imgStream}, 2: {Num: 2, Value: contentStream},
			3: pageObj, 10: orphanObj,
		},
		pages:  []*pdfObject{pageObj},
		nextID: 11,
	}

	removed := doc.RemoveUnusedObjects()
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if _, exists := doc.objects[10]; exists {
		t.Error("orphaned object 10 should be deleted")
	}
	if _, exists := doc.objects[1]; !exists {
		t.Error("referenced object 1 should still exist")
	}
}

func TestRemoveUnusedObjectsNone(t *testing.T) {
	// All objects are reachable.
	contentStream := &pdfStream{
		Dict:    pdfDict{},
		Data:    []byte("q Q\n"),
		Decoded: true,
	}
	pageDict := pdfDict{
		"/Type":     pdfName("/Page"),
		"/MediaBox": pdfArray{0.0, 0.0, 100.0, 100.0},
		"/Contents": pdfRef{Num: 1},
	}
	pageObj := &pdfObject{Num: 2, Value: pageDict}

	doc := &Document{
		objects: map[int]*pdfObject{
			1: {Num: 1, Value: contentStream}, 2: pageObj,
		},
		pages:  []*pdfObject{pageObj},
		nextID: 3,
	}

	origLen := len(doc.objects)
	removed := doc.RemoveUnusedObjects()
	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
	if len(doc.objects) != origLen {
		t.Errorf("objects count changed: %d -> %d", origLen, len(doc.objects))
	}
}

func TestRemoveUnusedObjectsAfterRemoveImage(t *testing.T) {
	doc := createDocWithImage()
	page, _ := doc.Page(1)
	infos, _ := page.ImageInfos()

	// Object 1 is the image XObject.
	if _, exists := doc.objects[1]; !exists {
		t.Fatal("setup: object 1 should exist before removal")
	}

	err := infos[0].Remove()
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// Image XObject (1) is now orphaned.
	removed := doc.RemoveUnusedObjects()
	if removed < 1 {
		t.Errorf("expected at least 1 removed, got %d", removed)
	}
	if _, exists := doc.objects[1]; exists {
		t.Error("orphaned image XObject should be deleted after RemoveUnusedObjects")
	}
}

func TestRemoveUnusedObjectsSharedXObject(t *testing.T) {
	// Two pages sharing the same image XObject (object 1).
	imgStream := &pdfStream{
		Dict: pdfDict{
			"/Type":    pdfName("/XObject"),
			"/Subtype": pdfName("/Image"),
			"/Width":   10,
			"/Height":  10,
		},
		Data:    []byte{0xFF, 0xD8, 0xFF, 0xD9},
		Decoded: false,
	}
	imgObj := &pdfObject{Num: 1, Value: imgStream}

	content1 := &pdfStream{
		Dict:    pdfDict{},
		Data:    []byte("q\n10 0 0 10 50 50 cm\n/Im0 Do\nQ\n"),
		Decoded: true,
	}
	content1Obj := &pdfObject{Num: 2, Value: content1}

	content2 := &pdfStream{
		Dict:    pdfDict{},
		Data:    []byte("q\n10 0 0 10 50 50 cm\n/Im0 Do\nQ\n"),
		Decoded: true,
	}
	content2Obj := &pdfObject{Num: 5, Value: content2}

	page1Dict := pdfDict{
		"/Type":     pdfName("/Page"),
		"/MediaBox": pdfArray{0.0, 0.0, 200.0, 300.0},
		"/Resources": pdfDict{
			"/XObject": pdfDict{"/Im0": pdfRef{Num: 1}},
		},
		"/Contents": pdfRef{Num: 2},
	}
	page1Obj := &pdfObject{Num: 3, Value: page1Dict}

	page2Dict := pdfDict{
		"/Type":     pdfName("/Page"),
		"/MediaBox": pdfArray{0.0, 0.0, 200.0, 300.0},
		"/Resources": pdfDict{
			"/XObject": pdfDict{"/Im0": pdfRef{Num: 1}},
		},
		"/Contents": pdfRef{Num: 5},
	}
	page2Obj := &pdfObject{Num: 4, Value: page2Dict}

	doc := &Document{
		objects: map[int]*pdfObject{
			1: imgObj, 2: content1Obj, 3: page1Obj, 4: page2Obj, 5: content2Obj,
		},
		pages:  []*pdfObject{page1Obj, page2Obj},
		nextID: 6,
	}

	// Remove image from page 1 only.
	page, _ := doc.Page(1)
	infos, _ := page.ImageInfos()
	infos[0].Remove()

	removed := doc.RemoveUnusedObjects()
	// Image is still referenced from page 2 — should NOT be removed.
	if _, exists := doc.objects[1]; !exists {
		t.Error("shared XObject should NOT be removed (still referenced from page 2)")
	}
	// The old content stream (object 2) is now orphaned (page 1 got a new content stream).
	if removed < 1 {
		t.Errorf("expected at least 1 removed (old content stream), got %d", removed)
	}
}

func TestRemoveUnusedObjectsCyclicRefs(t *testing.T) {
	// Two orphaned objects referencing each other.
	dictA := pdfDict{"/Ref": pdfRef{Num: 2}}
	objA := &pdfObject{Num: 1, Value: dictA}
	dictB := pdfDict{"/Ref": pdfRef{Num: 1}}
	objB := &pdfObject{Num: 2, Value: dictB}

	pageDict := pdfDict{
		"/Type":     pdfName("/Page"),
		"/MediaBox": pdfArray{0.0, 0.0, 100.0, 100.0},
	}
	pageObj := &pdfObject{Num: 3, Value: pageDict}

	doc := &Document{
		objects: map[int]*pdfObject{1: objA, 2: objB, 3: pageObj},
		pages:   []*pdfObject{pageObj},
		nextID:  4,
	}

	removed := doc.RemoveUnusedObjects()
	if removed != 2 {
		t.Errorf("removed = %d, want 2", removed)
	}
	if _, exists := doc.objects[1]; exists {
		t.Error("cyclic orphan A should be removed")
	}
	if _, exists := doc.objects[2]; exists {
		t.Error("cyclic orphan B should be removed")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run "TestRemoveUnusedObjects" -v ./...`
Expected: FAIL — `RemoveUnusedObjects` not defined.

- [ ] **Step 3: Implement RemoveUnusedObjects**

Add to `document.go`, after the `Append` method:

```go
// RemoveUnusedObjects removes objects from the document that are not
// reachable from any page. Returns the number of objects removed.
func (d *Document) RemoveUnusedObjects() int {
	reachable := collectReachableIDs(d.objects, d.pages)

	removed := 0
	for id := range d.objects {
		if !reachable[id] {
			delete(d.objects, id)
			removed++
		}
	}
	return removed
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run "TestRemoveUnusedObjects" -v ./...`
Expected: PASS.

- [ ] **Step 5: Run all tests**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add document.go document_internal_test.go
git commit -m "feat: add Document.RemoveUnusedObjects"
```

---

### Task 3: Integration test and docs update

**Files:**
- Modify: `document_test.go`
- Modify: `CLAUDE.md`
- Modify: `README.md`

- [ ] **Step 1: Write integration test**

Add to `document_test.go`:

```go
func TestRemoveUnusedObjectsRoundTrip(t *testing.T) {
	doc, err := asposepdf.Open("testdata/PdfWithImages.pdf")
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	page, _ := doc.Page(1)
	infos, err := page.ImageInfos()
	if err != nil {
		t.Fatalf("ImageInfos: %v", err)
	}
	if len(infos) == 0 {
		t.Fatal("expected at least 1 image")
	}

	// Remove all images from page 1.
	for _, info := range infos {
		if err := info.Remove(); err != nil {
			t.Fatalf("Remove: %v", err)
		}
	}

	removed := doc.RemoveUnusedObjects()
	t.Logf("removed %d unused objects", removed)
	if removed < 1 {
		t.Error("expected at least 1 object removed after image removal")
	}

	outDir := filepath.Join("result_files", "TestRemoveUnusedObjectsRoundTrip")
	os.MkdirAll(outDir, 0o755)
	outPath := filepath.Join(outDir, "output.pdf")
	if err := doc.Save(outPath); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Validate the output.
	report, err := asposepdf.Validate(outPath)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !report.Valid {
		for _, issue := range report.Issues {
			t.Errorf("validation issue: [%s] %s", issue.Code, issue.Message)
		}
	}

	// Verify file size decreased compared to saving without cleanup.
	docNoCleanup, _ := asposepdf.Open("testdata/PdfWithImages.pdf")
	page2, _ := docNoCleanup.Page(1)
	infos2, _ := page2.ImageInfos()
	for _, info := range infos2 {
		info.Remove()
	}
	noCleanupPath := filepath.Join(outDir, "no_cleanup.pdf")
	docNoCleanup.Save(noCleanupPath)

	cleanupInfo, _ := os.Stat(outPath)
	noCleanupInfo, _ := os.Stat(noCleanupPath)
	t.Logf("with cleanup: %d bytes, without: %d bytes", cleanupInfo.Size(), noCleanupInfo.Size())
	if cleanupInfo.Size() >= noCleanupInfo.Size() {
		t.Error("expected smaller file size after RemoveUnusedObjects")
	}
}
```

- [ ] **Step 2: Run integration test**

Run: `go test -run "TestRemoveUnusedObjectsRoundTrip" -v ./...`
Expected: PASS.

- [ ] **Step 3: Update CLAUDE.md**

After the `(*Document).ExtractTextWithLayout()` line in the Public API section, find the `(*Document).ExtractImages()` line and add `RemoveUnusedObjects` nearby. Specifically, after the `ImageToDocumentOptions` line, add:

```
- `(*Document).RemoveUnusedObjects() int` — removes objects not reachable from any page; returns count of removed objects
```

- [ ] **Step 4: Update README.md**

In the Features list, after "Remove images", add:
```
- **Remove unused objects** — clean up orphaned objects after modifications to reduce file size
```

After the "Replacing and Removing Images" usage section, add:

```markdown
### Cleaning Up Unused Objects

```go
doc, _ := pdf.Open("input.pdf")
page, _ := doc.Page(1)
infos, _ := page.ImageInfos()
infos[0].Remove()

removed := doc.RemoveUnusedObjects()
fmt.Printf("removed %d unused objects\n", removed)
doc.Save("output.pdf") // smaller file
```
```

- [ ] **Step 5: Run all tests**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add document_test.go CLAUDE.md README.md
git commit -m "docs: add RemoveUnusedObjects integration test, update CLAUDE.md and README"
```
