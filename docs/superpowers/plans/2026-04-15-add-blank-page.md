# AddBlankPage / InsertBlankPage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `AddBlankPage`, `AddBlankPageFromFormat`, `InsertBlankPage`, and `InsertBlankPageFromFormat` methods to `*Document` for appending or inserting blank pages.

**Architecture:** Extract a `createBlankPage` helper from the existing `NewDocument` code, then build 4 public methods that use it. `AddBlankPage` appends to `d.pages`, `InsertBlankPage` inserts at a given index. `FromFormat` variants delegate to the base methods.

**Tech Stack:** Go standard library only

---

## File Structure

| File | Responsibility |
|------|----------------|
| `document_new.go` | Add `createBlankPage` helper, 4 new public methods |
| `document_new_test.go` | Add 4 unit tests |
| `document_new_integration_test.go` | Add 1 integration test |

---

### Task 1: Extract `createBlankPage` helper and implement all 4 methods

**Files:**
- Modify: `document_new.go`
- Modify: `document_new_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `document_new_test.go`:

```go
func TestAddBlankPage(t *testing.T) {
	doc := NewDocument(612, 792) // Letter
	if err := doc.AddBlankPage(595, 842); err != nil {
		t.Fatalf("AddBlankPage: %v", err)
	}
	if doc.PageCount() != 2 {
		t.Fatalf("PageCount() = %d, want 2", doc.PageCount())
	}
	page, _ := doc.Page(2)
	size, _ := page.Size()
	if size.Width != 595 || size.Height != 842 {
		t.Errorf("page 2 size = {%.0f, %.0f}, want {595, 842}", size.Width, size.Height)
	}
}

func TestInsertBlankPage(t *testing.T) {
	doc := NewDocument(612, 792) // page 1: Letter
	doc.AddBlankPage(595, 842)   // page 2: A4

	// Insert at position 1 — becomes new page 1, others shift.
	if err := doc.InsertBlankPage(1, 842, 1191); err != nil {
		t.Fatalf("InsertBlankPage: %v", err)
	}
	if doc.PageCount() != 3 {
		t.Fatalf("PageCount() = %d, want 3", doc.PageCount())
	}

	// New page 1 should be A3.
	page1, _ := doc.Page(1)
	size1, _ := page1.Size()
	if size1.Width != 842 || size1.Height != 1191 {
		t.Errorf("page 1 size = {%.0f, %.0f}, want {842, 1191}", size1.Width, size1.Height)
	}

	// Original page 1 (Letter) is now page 2.
	page2, _ := doc.Page(2)
	size2, _ := page2.Size()
	if size2.Width != 612 || size2.Height != 792 {
		t.Errorf("page 2 size = {%.0f, %.0f}, want {612, 792}", size2.Width, size2.Height)
	}
}

func TestInsertBlankPageEnd(t *testing.T) {
	doc := NewDocument(595, 842)
	// Insert at PageCount()+1 = append.
	if err := doc.InsertBlankPage(2, 612, 792); err != nil {
		t.Fatalf("InsertBlankPage at end: %v", err)
	}
	if doc.PageCount() != 2 {
		t.Fatalf("PageCount() = %d, want 2", doc.PageCount())
	}
	page, _ := doc.Page(2)
	size, _ := page.Size()
	if size.Width != 612 || size.Height != 792 {
		t.Errorf("page 2 size = {%.0f, %.0f}, want {612, 792}", size.Width, size.Height)
	}
}

func TestInsertBlankPageInvalidPosition(t *testing.T) {
	doc := NewDocument(595, 842) // 1 page
	if err := doc.InsertBlankPage(0, 595, 842); err == nil {
		t.Fatal("expected error for position 0")
	}
	if err := doc.InsertBlankPage(3, 595, 842); err == nil {
		t.Fatal("expected error for position > PageCount()+1")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run "TestAddBlankPage|TestInsertBlankPage" -v ./...`
Expected: compilation error — `AddBlankPage` undefined.

- [ ] **Step 3: Implement `createBlankPage` helper and all 4 methods**

Modify `document_new.go`. Refactor `NewDocument` to use a new `createBlankPage` helper, then add the 4 methods:

```go
package asposepdf

import "fmt"

// PageFormat describes a page size in points (1/72 inch).
type PageFormat struct {
	Width  float64
	Height float64
}

// Predefined page formats (portrait orientation).
var (
	PageFormatA3     = PageFormat{Width: 842, Height: 1191}
	PageFormatA4     = PageFormat{Width: 595, Height: 842}
	PageFormatLetter = PageFormat{Width: 612, Height: 792}
	PageFormatLegal  = PageFormat{Width: 612, Height: 1008}
)

// Landscape returns the format with width and height swapped.
func (f PageFormat) Landscape() PageFormat {
	return PageFormat{Width: f.Height, Height: f.Width}
}

// createBlankPage creates a blank page with associated content stream and registers
// both objects in the document. Returns the page object.
func (d *Document) createBlankPage(width, height float64) *pdfObject {
	contentID := d.nextID
	d.nextID++
	contentStream := &pdfStream{
		Dict:    pdfDict{},
		Data:    []byte{},
		Decoded: true,
	}
	d.objects[contentID] = &pdfObject{Num: contentID, Value: contentStream}

	pageID := d.nextID
	d.nextID++
	pageDict := pdfDict{
		"/Type":      pdfName("/Page"),
		"/MediaBox":  pdfArray{0.0, 0.0, width, height},
		"/Resources": pdfDict{},
		"/Contents":  pdfRef{Num: contentID},
	}
	pageObj := &pdfObject{Num: pageID, Value: pageDict}
	d.objects[pageID] = pageObj

	return pageObj
}

// NewDocument creates a single-page blank document with the given dimensions in points.
func NewDocument(width, height float64) *Document {
	doc := &Document{
		objects: make(map[int]*pdfObject),
		nextID:  1,
	}
	pageObj := doc.createBlankPage(width, height)
	doc.pages = []*pdfObject{pageObj}
	return doc
}

// NewDocumentFromFormat creates a single-page blank document using a predefined page format.
func NewDocumentFromFormat(format PageFormat) *Document {
	return NewDocument(format.Width, format.Height)
}

// AddBlankPage appends a blank page to the end of the document.
func (d *Document) AddBlankPage(width, height float64) error {
	pageObj := d.createBlankPage(width, height)
	d.pages = append(d.pages, pageObj)
	return nil
}

// AddBlankPageFromFormat appends a blank page using a predefined page format.
func (d *Document) AddBlankPageFromFormat(format PageFormat) error {
	return d.AddBlankPage(format.Width, format.Height)
}

// InsertBlankPage inserts a blank page at the given 1-based position.
// Existing pages at and after that position shift by one.
func (d *Document) InsertBlankPage(position int, width, height float64) error {
	if position < 1 || position > len(d.pages)+1 {
		return fmt.Errorf("insert blank page: position %d out of range [1, %d]", position, len(d.pages)+1)
	}
	pageObj := d.createBlankPage(width, height)
	idx := position - 1
	d.pages = append(d.pages, nil)
	copy(d.pages[idx+1:], d.pages[idx:])
	d.pages[idx] = pageObj
	return nil
}

// InsertBlankPageFromFormat inserts a blank page at the given position using a predefined page format.
func (d *Document) InsertBlankPageFromFormat(position int, format PageFormat) error {
	return d.InsertBlankPage(position, format.Width, format.Height)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run "TestAddBlankPage|TestInsertBlankPage|TestNewDocument|TestPageFormat" -v ./...`
Expected: all PASS (including existing tests — `NewDocument` was refactored but behavior unchanged).

- [ ] **Step 5: Run full suite**

Run: `go test ./...`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add document_new.go document_new_test.go
git commit -m "feat: add AddBlankPage and InsertBlankPage with format variants"
```

---

### Task 2: Integration test

**Files:**
- Modify: `document_new_integration_test.go`

- [ ] **Step 1: Write the integration test**

Add to `document_new_integration_test.go`:

```go
func TestAddBlankPageRoundTrip(t *testing.T) {
	doc, err := asposepdf.Open("testdata/PdfWithImages.pdf")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	origCount := doc.PageCount()

	err = doc.AddBlankPageFromFormat(asposepdf.PageFormatA4)
	if err != nil {
		t.Fatalf("AddBlankPageFromFormat: %v", err)
	}
	if doc.PageCount() != origCount+1 {
		t.Fatalf("PageCount() = %d, want %d", doc.PageCount(), origCount+1)
	}

	outDir := filepath.Join("result_files", "TestAddBlankPageRoundTrip")
	os.MkdirAll(outDir, 0o755)
	outPath := filepath.Join(outDir, "output.pdf")
	if err := doc.Save(outPath); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Validate.
	report, err := asposepdf.Validate(outPath)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !report.Valid {
		for _, issue := range report.Issues {
			t.Errorf("validation issue: [%s] %s", issue.Code, issue.Message)
		}
	}

	// Reopen and verify.
	reopened, err := asposepdf.Open(outPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if reopened.PageCount() != origCount+1 {
		t.Fatalf("reopened PageCount() = %d, want %d", reopened.PageCount(), origCount+1)
	}
	lastPage, _ := reopened.Page(reopened.PageCount())
	size, _ := lastPage.Size()
	if size.Width != 595 || size.Height != 842 {
		t.Errorf("last page size = {%.0f, %.0f}, want {595, 842}", size.Width, size.Height)
	}
}
```

- [ ] **Step 2: Run integration test**

Run: `go test -run TestAddBlankPageRoundTrip -v ./...`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add document_new_integration_test.go
git commit -m "test: add AddBlankPage round-trip integration test"
```

---

### Task 3: Update CLAUDE.md and README.md

**Files:**
- Modify: `CLAUDE.md`
- Modify: `README.md`

- [ ] **Step 1: Update CLAUDE.md**

Add after the `NewDocumentFromFormat` entry in the `document_new.go` section:

```markdown
- `(*Document).AddBlankPage(width, height) error` — appends a blank page with given dimensions
- `(*Document).AddBlankPageFromFormat(format) error` — appends a blank page from a page format
- `(*Document).InsertBlankPage(position, width, height) error` — inserts a blank page at a 1-based position
- `(*Document).InsertBlankPageFromFormat(position, format) error` — inserts a blank page from a page format at a position
```

- [ ] **Step 2: Update README.md**

Add to the Features list (after "Create blank documents"):

```markdown
- **Add blank pages** — append or insert blank pages into existing documents at any position
```

Add a new section after "Creating Blank Documents":

```markdown
### Adding Blank Pages

```go
doc, _ := pdf.Open("input.pdf")

// Append a blank A4 page
doc.AddBlankPageFromFormat(pdf.PageFormatA4)

// Insert a landscape Letter page at position 2
doc.InsertBlankPageFromFormat(2, pdf.PageFormatLetter.Landscape())

doc.Save("output.pdf")
```
```

- [ ] **Step 3: Run full test suite**

Run: `go test ./...`
Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md README.md
git commit -m "docs: add AddBlankPage and InsertBlankPage to CLAUDE.md and README.md"
```
