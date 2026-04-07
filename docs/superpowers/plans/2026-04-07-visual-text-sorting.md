# Visual Text Sorting + ExtractTextWithLayout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Sort extracted text in visual reading order (top-to-bottom, left-to-right) and add `ExtractTextWithLayout()` returning structured `[]TextLine` with coordinates and font info.

**Architecture:** Refactor `textExtractor.emitRune` to collect `[]textFragment` instead of writing to `strings.Builder`. New file `text_layout.go` groups fragments into `TextLine`s sorted by visual position. `ExtractText()` delegates to `ExtractTextWithLayout()` for correct ordering.

**Tech Stack:** Pure Go, no dependencies.

**Spec:** `docs/superpowers/specs/2026-04-07-visual-text-sorting-design.md`

**Beads:** `pdf-go-vg8.7`, `pdf-go-vg8.3`

---

## File Structure

| File | Responsibility |
|------|---------------|
| `text.go` | Refactor `textExtractor`: replace `buf`/`lastX`/`lastY`/`hasPos` with `[]textFragment` + `curFrag`; update `emitRune` to collect fragments; update `ExtractText` to delegate to layout pipeline |
| `text_layout.go` | **New.** Public types `TextFragment`, `TextLine`; `groupFragmentsIntoLines()` sorting/grouping; `ExtractTextWithLayout` methods on `Page` and `Document` |
| `text_layout_test.go` | **New.** Unit tests for grouping/sorting logic; integration test for `ExtractTextWithLayout` |
| `text_test.go` | Update existing tests where output order changes; add visual sorting test |

---

### Task 1: Refactor `textExtractor` to collect fragments

**Files:**
- Modify: `text.go`

- [ ] **Step 1: Write the failing test for visual ordering**

In `text_test.go`, add a test where content stream has footer text before body text:

```go
func TestExtractTextVisualOrder(t *testing.T) {
	// Content stream draws footer first (y=50), then body (y=700).
	// ExtractText should output body first (top-to-bottom).
	content := []byte("BT /F1 12 Tf 100 50 Td (Footer) Tj ET BT /F1 12 Tf 100 700 Td (Body) Tj ET")
	pdf := buildPDFWithContent(content)
	doc, err := asposepdf.OpenStream(bytes.NewReader(pdf))
	if err != nil {
		t.Fatalf("OpenStream: %v", err)
	}
	page, _ := doc.Page(1)
	text, err := page.ExtractText()
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	// Body (y=700) should come before Footer (y=50).
	bodyIdx := strings.Index(text, "Body")
	footerIdx := strings.Index(text, "Footer")
	if bodyIdx < 0 || footerIdx < 0 {
		t.Fatalf("text=%q, missing Body or Footer", text)
	}
	if bodyIdx > footerIdx {
		t.Errorf("expected Body before Footer in visual order, got text=%q", text)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run TestExtractTextVisualOrder ./...`
Expected: FAIL — currently text outputs "Footer" before "Body" (content stream order).

- [ ] **Step 3: Refactor `textExtractor` internals**

In `text.go`, replace the output fields and `emitRune` method. The `textExtractor` struct changes:

Replace:
```go
	// Output.
	buf    strings.Builder
	lastX  float64
	lastY  float64
	hasPos bool
```

With:
```go
	// Output: collected text fragments.
	fragments []textFragment
	curFrag   *textFragment // current fragment being built
	lastX     float64       // x after last glyph advance
	lastY     float64       // y after last glyph advance
	hasPos    bool
```

Add the `textFragment` type (above the `textExtractor` struct):

```go
// textFragment is a contiguous run of text at a single position.
type textFragment struct {
	text     strings.Builder
	x, y     float64 // device-space position of first rune
	endX     float64 // device-space x after last glyph advance
	fontName string
	fontSize float64 // effective font size (fontSize * textScaleX)
}
```

Replace the `emitRune` method with:

```go
func (e *textExtractor) emitRune(r rune) {
	x, y := e.currentPos()
	effectiveFontSize := e.fontSize * e.textScaleX()
	fontName := e.font.name

	needNew := e.curFrag == nil ||
		fontName != e.curFrag.fontName ||
		math.Abs(effectiveFontSize-e.curFrag.fontSize) > 0.01

	if !needNew && e.hasPos {
		dy := e.lastY - y
		if math.Abs(dy) > effectiveFontSize*0.5 {
			needNew = true
		}
		dx := x - e.lastX
		spaceWidth := e.computeSpaceWidth()
		scale := e.textScaleX()
		if dx > spaceWidth*scale*0.3 {
			needNew = true
		}
	}

	if needNew {
		e.flushFragment()
		frag := textFragment{
			x:        x,
			y:        y,
			fontName: fontName,
			fontSize: effectiveFontSize,
		}
		e.fragments = append(e.fragments, frag)
		e.curFrag = &e.fragments[len(e.fragments)-1]
	}

	e.curFrag.text.WriteRune(r)
	e.lastX = x
	e.lastY = y
	e.hasPos = true
}

func (e *textExtractor) flushFragment() {
	if e.curFrag != nil {
		e.curFrag.endX = e.lastX
		e.curFrag = nil
	}
}

// computeSpaceWidth returns the space character width in text space units.
func (e *textExtractor) computeSpaceWidth() float64 {
	var spaceWidth float64
	if e.font.isType0 {
		if sw, ok := e.font.cidWidths[0x0020]; ok {
			spaceWidth = sw / 1000.0 * e.fontSize
		} else {
			spaceWidth = e.font.defaultW / 1000.0 * e.fontSize
		}
	} else {
		spaceWidth = e.font.widths[32] / 1000.0 * e.fontSize
	}
	if spaceWidth < 1 {
		spaceWidth = e.fontSize * 0.25
	}
	if spaceWidth < 1 {
		spaceWidth = 1
	}
	return spaceWidth
}
```

Update `advanceGlyph` and `advanceGlyphCID` — they already update `e.lastX, e.lastY` which is correct (the fragment's `endX` is set on flush from `e.lastX`).

Remove the `text()` method (it will be replaced by the layout pipeline). Replace with a temporary `text()` that uses basic fragment joining so existing tests don't break during refactor:

```go
func (e *textExtractor) text() string {
	e.flushFragment()
	return buildTextFromFragments(e.fragments)
}
```

Remove the old `buf` field references. The `cleanExtractedText` function stays — it will be called by `ExtractText`.

- [ ] **Step 4: Run all tests**

Run: `go test ./...`
Expected: some tests may need adjustment — the `text()` temporary method should preserve basic behavior. The visual ordering test should now PASS if `buildTextFromFragments` sorts by Y then X.

- [ ] **Step 5: Commit**

```bash
git add text.go text_test.go
git commit -m "refactor: collect text fragments instead of writing to buffer"
```

---

### Task 2: Fragment grouping and line assembly (`text_layout.go`)

**Files:**
- Create: `text_layout.go`
- Create: `text_layout_test.go`

- [ ] **Step 1: Write the failing test for grouping**

Create `text_layout_test.go`:

```go
package asposepdf

import (
	"strings"
	"testing"
)

func TestGroupFragmentsIntoLines(t *testing.T) {
	frags := []textFragment{
		{x: 100, y: 50, endX: 150, fontName: "/Helvetica", fontSize: 12},   // footer
		{x: 100, y: 700, endX: 160, fontName: "/Helvetica", fontSize: 12},  // line 1
		{x: 170, y: 700, endX: 230, fontName: "/Helvetica", fontSize: 12},  // line 1 continued
		{x: 100, y: 680, endX: 180, fontName: "/Helvetica", fontSize: 12},  // line 2
	}
	frags[0].text.WriteString("Footer")
	frags[1].text.WriteString("Hello")
	frags[2].text.WriteString("World")
	frags[3].text.WriteString("Second line")

	lines := groupFragmentsIntoLines(frags)

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	// First line should be y=700 (top of page).
	if !strings.Contains(lines[0].Text, "Hello") {
		t.Errorf("line 0: %q, expected Hello", lines[0].Text)
	}
	if !strings.Contains(lines[0].Text, "World") {
		t.Errorf("line 0: %q, expected World", lines[0].Text)
	}
	// Second line y=680.
	if !strings.Contains(lines[1].Text, "Second line") {
		t.Errorf("line 1: %q, expected 'Second line'", lines[1].Text)
	}
	// Last line is footer y=50.
	if !strings.Contains(lines[2].Text, "Footer") {
		t.Errorf("line 2: %q, expected Footer", lines[2].Text)
	}
}

func TestGroupFragmentsEmpty(t *testing.T) {
	lines := groupFragmentsIntoLines(nil)
	if len(lines) != 0 {
		t.Errorf("expected 0 lines, got %d", len(lines))
	}
}

func TestGroupFragmentsSpaceInsertion(t *testing.T) {
	// Two fragments on same line with a gap — should get a space.
	frags := []textFragment{
		{x: 100, y: 700, endX: 140, fontName: "/Helvetica", fontSize: 12},
		{x: 150, y: 700, endX: 200, fontName: "/Helvetica", fontSize: 12},
	}
	frags[0].text.WriteString("Hello")
	frags[1].text.WriteString("World")

	lines := groupFragmentsIntoLines(frags)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0].Text != "Hello World" {
		t.Errorf("text=%q, want 'Hello World'", lines[0].Text)
	}
}

func TestGroupFragmentsNoSpuriousSpace(t *testing.T) {
	// Two fragments on same line with no gap — no space.
	frags := []textFragment{
		{x: 100, y: 700, endX: 140, fontName: "/Helvetica", fontSize: 12},
		{x: 140, y: 700, endX: 180, fontName: "/Helvetica", fontSize: 12},
	}
	frags[0].text.WriteString("Hel")
	frags[1].text.WriteString("lo")

	lines := groupFragmentsIntoLines(frags)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0].Text != "Hello" {
		t.Errorf("text=%q, want 'Hello'", lines[0].Text)
	}
}

func TestCleanFontName(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"/MCEFGG+Garamond-Bold", "Garamond-Bold"},
		{"/Helvetica", "Helvetica"},
		{"/ABCDEF+Arial-BoldMT", "Arial-BoldMT"},
		{"", ""},
	}
	for _, tt := range tests {
		got := cleanFontName(tt.in)
		if got != tt.want {
			t.Errorf("cleanFontName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run "TestGroupFragments|TestCleanFontName" ./...`
Expected: FAIL — `groupFragmentsIntoLines` and `cleanFontName` undefined.

- [ ] **Step 3: Implement `text_layout.go`**

Create `text_layout.go`:

```go
package asposepdf

import (
	"math"
	"sort"
	"strings"
)

// TextFragment represents a contiguous run of text with uniform font.
type TextFragment struct {
	Text     string
	X        float64 // horizontal position in points (from left edge)
	FontName string  // e.g. "Helvetica", "Arial-BoldMT"
	FontSize float64 // effective size in points
}

// TextLine represents a horizontal line of text fragments at a common Y position.
type TextLine struct {
	Text      string         // concatenated text of all fragments (with spaces)
	Y         float64        // vertical position in points (from bottom edge)
	Fragments []TextFragment
}

// groupFragmentsIntoLines groups text fragments into lines sorted in visual
// reading order (top-to-bottom, left-to-right).
func groupFragmentsIntoLines(frags []textFragment) []TextLine {
	if len(frags) == 0 {
		return nil
	}

	// Sort by Y descending (top first), then X ascending (left first).
	sort.Slice(frags, func(i, j int) bool {
		if math.Abs(frags[i].y-frags[j].y) > 0.5 {
			return frags[i].y > frags[j].y
		}
		return frags[i].x < frags[j].x
	})

	// Group into lines by Y proximity.
	var lines []TextLine
	var curFrags []textFragment
	curY := frags[0].y

	for _, f := range frags {
		if f.text.Len() == 0 {
			continue
		}
		threshold := f.fontSize * 0.3
		if threshold < 1 {
			threshold = 1
		}
		if len(curFrags) > 0 && math.Abs(f.y-curY) > threshold {
			lines = append(lines, assembleLine(curFrags))
			curFrags = curFrags[:0]
		}
		curFrags = append(curFrags, f)
		curY = f.y
	}
	if len(curFrags) > 0 {
		lines = append(lines, assembleLine(curFrags))
	}

	return lines
}

// assembleLine builds a TextLine from fragments on the same line.
// Fragments must already be sorted by X ascending.
func assembleLine(frags []textFragment) TextLine {
	// Sort by X ascending within the line.
	sort.Slice(frags, func(i, j int) bool {
		return frags[i].x < frags[j].x
	})

	line := TextLine{
		Y: frags[0].y,
	}

	var buf strings.Builder
	for i, f := range frags {
		text := f.text.String()
		if text == "" {
			continue
		}

		// Insert space between fragments if there's a gap.
		if i > 0 {
			gap := f.x - frags[i-1].endX
			spaceThreshold := f.fontSize * 0.3
			if spaceThreshold < 1 {
				spaceThreshold = 1
			}
			if gap > spaceThreshold {
				buf.WriteByte(' ')
			}
		}

		buf.WriteString(text)
		line.Fragments = append(line.Fragments, TextFragment{
			Text:     text,
			X:        f.x,
			FontName: cleanFontName(f.fontName),
			FontSize: f.fontSize,
		})
	}

	line.Text = buf.String()
	return line
}

// buildTextFromFragments groups fragments into lines and joins them as plain text.
func buildTextFromFragments(frags []textFragment) string {
	lines := groupFragmentsIntoLines(frags)
	if len(lines) == 0 {
		return ""
	}

	var buf strings.Builder
	for i, line := range lines {
		if i > 0 {
			buf.WriteByte('\n')
			// Extra blank line for paragraph-sized gaps.
			if i < len(lines) {
				prevY := lines[i-1].Y
				gap := prevY - line.Y
				avgFontSize := 12.0
				if len(line.Fragments) > 0 {
					avgFontSize = line.Fragments[0].FontSize
				}
				if gap > avgFontSize*1.5 {
					buf.WriteByte('\n')
				}
			}
		}
		buf.WriteString(line.Text)
	}
	return buf.String()
}

// cleanFontName strips the PDF name prefix "/" and subset prefix "ABCDEF+".
func cleanFontName(name string) string {
	if name == "" {
		return ""
	}
	// Strip leading "/".
	if name[0] == '/' {
		name = name[1:]
	}
	// Strip subset prefix: 6 uppercase letters + "+".
	if len(name) > 7 && name[6] == '+' {
		allUpper := true
		for i := 0; i < 6; i++ {
			if name[i] < 'A' || name[i] > 'Z' {
				allUpper = false
				break
			}
		}
		if allUpper {
			name = name[7:]
		}
	}
	return name
}

// ExtractTextWithLayout returns structured text lines sorted in visual
// (top-to-bottom, left-to-right) reading order. Each line contains
// its concatenated text and individual fragments with positions.
func (p *Page) ExtractTextWithLayout() ([]TextLine, error) {
	data, err := p.contentStreams()
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}

	ops, err := parseContentStream(data)
	if err != nil {
		return nil, err
	}

	resources := p.pageResources()
	fonts := resolveFontResources(p.doc.objects, resources)

	ext := newTextExtractor(p.doc.objects, fonts)
	ext.process(ops, resources)
	ext.flushFragment()

	return groupFragmentsIntoLines(ext.fragments), nil
}

// ExtractTextWithLayout returns structured text lines for each page.
// The returned slice has one entry per page (0-indexed).
func (d *Document) ExtractTextWithLayout() ([][]TextLine, error) {
	pages := d.Pages()
	result := make([][]TextLine, len(pages))
	for i, p := range pages {
		lines, err := p.ExtractTextWithLayout()
		if err != nil {
			return nil, err
		}
		result[i] = lines
	}
	return result, nil
}
```

- [ ] **Step 4: Run grouping tests**

Run: `go test -run "TestGroupFragments|TestCleanFontName" ./...`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add text_layout.go text_layout_test.go
git commit -m "feat: add text fragment grouping and ExtractTextWithLayout"
```

---

### Task 3: Wire up `ExtractText` to use visual sorting

**Files:**
- Modify: `text.go`
- Modify: `text_test.go`

- [ ] **Step 1: Update `text()` method and `ExtractText`**

In `text.go`, update the `text()` method to use `buildTextFromFragments`:

```go
func (e *textExtractor) text() string {
	e.flushFragment()
	return cleanExtractedText(buildTextFromFragments(e.fragments))
}
```

The `ExtractText` method on `Page` stays as-is (it calls `ext.text()`). Alternatively, update it to delegate to `ExtractTextWithLayout`:

```go
func (p *Page) ExtractText() (string, error) {
	lines, err := p.ExtractTextWithLayout()
	if err != nil {
		return "", err
	}
	if len(lines) == 0 {
		return "", nil
	}
	return cleanExtractedText(buildTextFromFragments2(lines)), nil
}
```

Actually, keep it simpler — `ExtractText` continues to use the extractor directly, and `text()` uses `buildTextFromFragments` which already sorts. This avoids double work. The existing code path stays:

```go
func (p *Page) ExtractText() (string, error) {
	data, err := p.contentStreams()
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", nil
	}

	ops, err := parseContentStream(data)
	if err != nil {
		return "", err
	}

	resources := p.pageResources()
	fonts := resolveFontResources(p.doc.objects, resources)

	ext := newTextExtractor(p.doc.objects, fonts)
	ext.process(ops, resources)
	return ext.text(), nil
}
```

No change needed — `ext.text()` already calls `buildTextFromFragments`.

- [ ] **Step 2: Run the visual ordering test**

Run: `go test -run TestExtractTextVisualOrder ./...`
Expected: PASS — "Body" (y=700) now appears before "Footer" (y=50).

- [ ] **Step 3: Run ALL existing tests**

Run: `go test ./...`
Expected: all PASS. If any test fails due to changed ordering, update the test to match the correct visual order.

- [ ] **Step 4: Commit**

```bash
git add text.go text_test.go
git commit -m "feat: ExtractText now outputs text in visual reading order"
```

---

### Task 4: Integration tests with `ExtractTextWithLayout`

**Files:**
- Modify: `text_layout_test.go` (append)
- Modify: `text_test.go` (append)

- [ ] **Step 1: Write `ExtractTextWithLayout` unit test**

In `text_layout_test.go`, add:

```go
func TestExtractTextWithLayoutSynthetic(t *testing.T) {
	// Two BT/ET blocks: footer at y=50, body at y=700.
	content := []byte("BT /F1 12 Tf 100 50 Td (Footer) Tj ET BT /F1 12 Tf 100 700 Td (Body Text) Tj ET")
	pdf := buildPDFWithContent(content)
	doc, err := OpenStream(bytes.NewReader(pdf))
	if err != nil {
		t.Fatalf("OpenStream: %v", err)
	}
	page, _ := doc.Page(1)
	lines, err := page.ExtractTextWithLayout()
	if err != nil {
		t.Fatalf("ExtractTextWithLayout: %v", err)
	}

	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}

	// First line (top) should be "Body Text" at y=700.
	if lines[0].Text != "Body Text" {
		t.Errorf("line 0 text=%q, want 'Body Text'", lines[0].Text)
	}
	if lines[0].Y != 700 {
		t.Errorf("line 0 Y=%v, want 700", lines[0].Y)
	}
	if len(lines[0].Fragments) < 1 {
		t.Fatal("expected at least 1 fragment in line 0")
	}
	if lines[0].Fragments[0].FontName != "Helvetica" {
		t.Errorf("fragment font=%q, want 'Helvetica'", lines[0].Fragments[0].FontName)
	}

	// Last line should be "Footer" at y=50.
	last := lines[len(lines)-1]
	if last.Text != "Footer" {
		t.Errorf("last line text=%q, want 'Footer'", last.Text)
	}
	if last.Y != 50 {
		t.Errorf("last line Y=%v, want 50", last.Y)
	}
}
```

Note: This test is in `text_layout_test.go` which is `package asposepdf` (internal), so it can use `OpenStream` and `buildPDFWithContent` directly. However, `buildPDFWithContent` is defined in `text_test.go` which is `package asposepdf_test`. So this test needs to be in `text_test.go` instead, using the external test package.

Move this to `text_test.go`:

```go
func TestExtractTextWithLayoutSynthetic(t *testing.T) {
	// Two BT/ET blocks: footer at y=50, body at y=700.
	content := []byte("BT /F1 12 Tf 100 50 Td (Footer) Tj ET BT /F1 12 Tf 100 700 Td (Body Text) Tj ET")
	pdf := buildPDFWithContent(content)
	doc, err := asposepdf.OpenStream(bytes.NewReader(pdf))
	if err != nil {
		t.Fatalf("OpenStream: %v", err)
	}
	page, _ := doc.Page(1)
	lines, err := page.ExtractTextWithLayout()
	if err != nil {
		t.Fatalf("ExtractTextWithLayout: %v", err)
	}

	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}

	// First line (top) should be "Body Text" at y=700.
	if lines[0].Text != "Body Text" {
		t.Errorf("line 0 text=%q, want 'Body Text'", lines[0].Text)
	}
	if lines[0].Y != 700 {
		t.Errorf("line 0 Y=%v, want 700", lines[0].Y)
	}
	if len(lines[0].Fragments) < 1 {
		t.Fatal("expected at least 1 fragment in line 0")
	}
	if lines[0].Fragments[0].FontName != "Helvetica" {
		t.Errorf("fragment font=%q, want 'Helvetica'", lines[0].Fragments[0].FontName)
	}

	// Last line should be "Footer" at y=50.
	last := lines[len(lines)-1]
	if last.Text != "Footer" {
		t.Errorf("last line text=%q, want 'Footer'", last.Text)
	}
	if last.Y != 50 {
		t.Errorf("last line Y=%v, want 50", last.Y)
	}
}
```

- [ ] **Step 2: Run the test**

Run: `go test -run TestExtractTextWithLayoutSynthetic ./...`
Expected: PASS.

- [ ] **Step 3: Run `TestExtractTextFiles` and verify marketing.pdf page 2**

Run: `go test -run TestExtractTextFiles -v ./...`

Then inspect `result_files/TestExtractTextFiles/marketing/page_2.txt`:
- "11/06" should now be at the BOTTOM, not the top
- "Marketing Continued" should be near the top
- Verify other PDFs have no regressions

- [ ] **Step 4: Commit**

```bash
git add text_test.go text_layout_test.go
git commit -m "test: integration tests for ExtractTextWithLayout and visual ordering"
```

---

### Task 5: Update CLAUDE.md and close beads

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update public API docs**

In `CLAUDE.md`, add to the `**page.go**` section (or the appropriate Public API section):

After the `ExtractText` line, add:
```
- `(*Page).ExtractTextWithLayout() ([]TextLine, error)` — returns structured text lines in visual reading order with coordinates and font info
```

After the `(*Document).ExtractText` line, add:
```
- `(*Document).ExtractTextWithLayout() ([][]TextLine, error)` — returns structured text lines for each page
```

Add `TextLine` and `TextFragment` to the `PageSize` types area:
```
- `TextLine` struct — Text, Y, Fragments []TextFragment
- `TextFragment` struct — Text, X, FontName, FontSize
```

Update the text extraction section header to include `text_layout.go`:
```
### Text extraction (`text.go`, `text_layout.go`, `content_parser.go`, `font.go`, `font_metrics.go`, `encoding.go`, `cmap.go`)
```

Add a note about visual ordering:
```
6. Fragment collection and visual sorting (`text_layout.go`): `emitRune` collects `textFragment` structs with (x, y, endX, fontName, fontSize); `groupFragmentsIntoLines` sorts by Y descending then X ascending, groups into `TextLine` structs; `ExtractTextWithLayout` returns the structured result
```

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update architecture for visual sorting and ExtractTextWithLayout"
```

- [ ] **Step 3: Close beads**

```bash
bd update pdf-go-vg8.7 --status closed
bd update pdf-go-vg8.3 --status closed
```
