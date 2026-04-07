# ToUnicode CMap Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Support ToUnicode CMap streams and Type0/CIDFont fonts so that PDFs using Identity-H encoding produce readable text instead of U+FFFD.

**Architecture:** Add a CMap parser (`cmap.go`) that extracts `beginbfchar`/`beginbfrange` mappings into `map[uint16]rune`. Extend `fontInfo` with `toUnicode`, `cidWidths`, `defaultW`, and `isType0` fields. Split `showString`/`showTJ` into single-byte and multi-byte paths.

**Tech Stack:** Pure Go, no dependencies.

**Spec:** `docs/superpowers/specs/2026-04-07-tounicode-cmap-design.md`

**Beads:** `pdf-go-vg8.1`

---

## File Structure

| File | Responsibility |
|------|---------------|
| `cmap.go` | **New.** `parseCMap(data []byte) map[uint16]rune` — ToUnicode CMap parser |
| `cmap_test.go` | **New.** Unit tests for CMap parser |
| `font.go` | Extend `fontInfo` struct; add ToUnicode/CIDFont resolution to `resolveFont` |
| `text.go` | Split `showString`/`showTJ` into single-byte and multi-byte paths; add `advanceGlyphCID` |
| `content_parser_test.go` | Tests for Type0 font resolution, CID width parsing |
| `text_test.go` | Integration tests for Type0 text extraction |

---

### Task 1: CMap parser

**Files:**
- Create: `cmap.go`
- Create: `cmap_test.go`

- [ ] **Step 1: Write the failing test for CMap parser**

Create `cmap_test.go`:

```go
package asposepdf

import (
	"testing"
)

func TestParseCMapBfchar(t *testing.T) {
	data := []byte(`/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CMapName /Adobe-Identity-UCS def
/CMapType 2 def
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
3 beginbfchar
<0003> <0020>
<0004> <0041>
<015E> <0410>
endbfchar
endcmap`)
	m := parseCMap(data)
	if m[0x0003] != 0x0020 {
		t.Errorf("0x0003: got %U, want U+0020", m[0x0003])
	}
	if m[0x0004] != 0x0041 {
		t.Errorf("0x0004: got %U, want U+0041 (A)", m[0x0004])
	}
	if m[0x015E] != 0x0410 {
		t.Errorf("0x015E: got %U, want U+0410 (Cyrillic A)", m[0x015E])
	}
}

func TestParseCMapBfrange(t *testing.T) {
	data := []byte(`begincmap
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
1 beginbfrange
<0041> <0043> <0061>
endbfrange
endcmap`)
	m := parseCMap(data)
	// 0x0041 -> 'a', 0x0042 -> 'b', 0x0043 -> 'c'
	if m[0x0041] != 'a' {
		t.Errorf("0x0041: got %U, want U+0061 (a)", m[0x0041])
	}
	if m[0x0042] != 'b' {
		t.Errorf("0x0042: got %U, want U+0062 (b)", m[0x0042])
	}
	if m[0x0043] != 'c' {
		t.Errorf("0x0043: got %U, want U+0063 (c)", m[0x0043])
	}
}

func TestParseCMapBfrangeArray(t *testing.T) {
	data := []byte(`begincmap
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
1 beginbfrange
<0100> <0102> [<0041> <0042> <0043>]
endbfrange
endcmap`)
	m := parseCMap(data)
	if m[0x0100] != 'A' {
		t.Errorf("0x0100: got %U, want U+0041 (A)", m[0x0100])
	}
	if m[0x0101] != 'B' {
		t.Errorf("0x0101: got %U, want U+0042 (B)", m[0x0101])
	}
	if m[0x0102] != 'C' {
		t.Errorf("0x0102: got %U, want U+0043 (C)", m[0x0102])
	}
}

func TestParseCMapEmpty(t *testing.T) {
	m := parseCMap([]byte{})
	if len(m) != 0 {
		t.Errorf("expected empty map, got %d entries", len(m))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run "TestParseCMap" ./...`
Expected: compilation error — `parseCMap` undefined.

- [ ] **Step 3: Implement `parseCMap`**

Create `cmap.go`:

```go
package asposepdf

import (
	"bytes"
	"encoding/hex"
	"strings"
)

// parseCMap parses a ToUnicode CMap stream and returns a mapping
// from character codes (glyph IDs) to Unicode runes.
// It handles beginbfchar/endbfchar and beginbfrange/endbfrange sections.
func parseCMap(data []byte) map[uint16]rune {
	m := make(map[uint16]rune)
	lines := bytes.Split(data, []byte("\n"))

	inBfchar := false
	inBfrange := false

	for _, line := range lines {
		s := strings.TrimSpace(string(line))
		if s == "" {
			continue
		}

		if strings.HasSuffix(s, "beginbfchar") {
			inBfchar = true
			continue
		}
		if s == "endbfchar" {
			inBfchar = false
			continue
		}
		if strings.HasSuffix(s, "beginbfrange") {
			inBfrange = true
			continue
		}
		if s == "endbfrange" {
			inBfrange = false
			continue
		}

		if inBfchar {
			parseBfcharLine(s, m)
		}
		if inBfrange {
			parseBfrangeLine(s, m)
		}
	}
	return m
}

// parseBfcharLine parses a line like "<0003> <0020>".
func parseBfcharLine(s string, m map[uint16]rune) {
	tokens := extractHexTokens(s)
	if len(tokens) < 2 {
		return
	}
	src := decodeHexUint16(tokens[0])
	dst := decodeHexRune(tokens[1])
	if dst != 0 {
		m[src] = dst
	}
}

// parseBfrangeLine parses a line like "<0041> <0043> <0061>"
// or "<0100> <0102> [<0041> <0042> <0043>]".
func parseBfrangeLine(s string, m map[uint16]rune) {
	// Check for array form: [...] at the end.
	if idx := strings.Index(s, "["); idx >= 0 {
		// Parse the two hex tokens before the bracket.
		prefix := s[:idx]
		tokens := extractHexTokens(prefix)
		if len(tokens) < 2 {
			return
		}
		start := decodeHexUint16(tokens[0])
		end := decodeHexUint16(tokens[1])
		// Parse array entries.
		arrayPart := s[idx:]
		arrayTokens := extractHexTokens(arrayPart)
		for i, tok := range arrayTokens {
			code := start + uint16(i)
			if code > end {
				break
			}
			r := decodeHexRune(tok)
			if r != 0 {
				m[code] = r
			}
		}
		return
	}

	tokens := extractHexTokens(s)
	if len(tokens) < 3 {
		return
	}
	start := decodeHexUint16(tokens[0])
	end := decodeHexUint16(tokens[1])
	dstStart := decodeHexRune(tokens[2])
	for code := start; code <= end; code++ {
		m[code] = dstStart + rune(code-start)
	}
}

// extractHexTokens returns all <hex> tokens from a string.
func extractHexTokens(s string) []string {
	var tokens []string
	for {
		start := strings.IndexByte(s, '<')
		if start < 0 {
			break
		}
		end := strings.IndexByte(s[start:], '>')
		if end < 0 {
			break
		}
		tokens = append(tokens, s[start+1:start+end])
		s = s[start+end+1:]
	}
	return tokens
}

// decodeHexUint16 decodes a hex string to uint16 (e.g., "0003" -> 3).
func decodeHexUint16(s string) uint16 {
	b, err := hex.DecodeString(s)
	if err != nil || len(b) == 0 {
		return 0
	}
	if len(b) == 1 {
		return uint16(b[0])
	}
	return uint16(b[0])<<8 | uint16(b[1])
}

// decodeHexRune decodes a hex string to a rune (e.g., "0041" -> 'A').
func decodeHexRune(s string) rune {
	b, err := hex.DecodeString(s)
	if err != nil || len(b) == 0 {
		return 0
	}
	if len(b) == 1 {
		return rune(b[0])
	}
	return rune(uint16(b[0])<<8 | uint16(b[1]))
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run "TestParseCMap" ./...`
Expected: all 4 PASS.

- [ ] **Step 5: Commit**

```bash
git add cmap.go cmap_test.go
git commit -m "feat: add ToUnicode CMap parser"
```

---

### Task 2: Extend `fontInfo` and resolve ToUnicode/CIDFont

**Files:**
- Modify: `font.go`
- Test: `content_parser_test.go` (append)

- [ ] **Step 1: Write the failing tests**

In `content_parser_test.go`, add:

```go
func TestResolveFontType0WithToUnicode(t *testing.T) {
	cmapData := []byte(`begincmap
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
2 beginbfchar
<0003> <0020>
<0004> <0041>
endbfchar
endcmap`)
	cmapStream := &pdfStream{
		Dict: pdfDict{"/Length": len(cmapData)},
		Data: cmapData,
	}
	objects := map[int]*pdfObject{
		10: {value: cmapStream},
		20: {value: pdfDict{
			"/Type":    pdfName("/Font"),
			"/Subtype": pdfName("/CIDFontType2"),
			"/DW":      1000,
			"/W":       pdfArray{3, pdfArray{250, 600}},
		}},
	}
	fontDict := pdfDict{
		"/Type":            pdfName("/Font"),
		"/Subtype":         pdfName("/Type0"),
		"/BaseFont":        pdfName("/Calibri"),
		"/Encoding":        pdfName("/Identity-H"),
		"/ToUnicode":       pdfRef{id: 10},
		"/DescendantFonts": pdfArray{pdfRef{id: 20}},
	}
	fi := resolveFont(objects, fontDict)
	if !fi.isType0 {
		t.Error("expected isType0=true")
	}
	if !fi.known {
		t.Error("expected known=true")
	}
	if fi.toUnicode == nil {
		t.Fatal("expected toUnicode to be populated")
	}
	if fi.toUnicode[0x0003] != 0x0020 {
		t.Errorf("toUnicode[0x0003]: got %U, want U+0020", fi.toUnicode[0x0003])
	}
	if fi.toUnicode[0x0004] != 0x0041 {
		t.Errorf("toUnicode[0x0004]: got %U, want U+0041", fi.toUnicode[0x0004])
	}
}

func TestResolveFontCIDWidths(t *testing.T) {
	objects := map[int]*pdfObject{
		10: {value: &pdfStream{
			Dict: pdfDict{},
			Data: []byte("begincmap\n1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n1 beginbfchar\n<0003> <0041>\nendbfchar\nendcmap"),
		}},
		20: {value: pdfDict{
			"/Type":    pdfName("/Font"),
			"/Subtype": pdfName("/CIDFontType2"),
			"/DW":      500,
			"/W":       pdfArray{3, pdfArray{250, 600}},
		}},
	}
	fontDict := pdfDict{
		"/Type":            pdfName("/Font"),
		"/Subtype":         pdfName("/Type0"),
		"/BaseFont":        pdfName("/TestFont"),
		"/Encoding":        pdfName("/Identity-H"),
		"/ToUnicode":       pdfRef{id: 10},
		"/DescendantFonts": pdfArray{pdfRef{id: 20}},
	}
	fi := resolveFont(objects, fontDict)
	if fi.defaultW != 500 {
		t.Errorf("defaultW: got %v, want 500", fi.defaultW)
	}
	if fi.cidWidths[3] != 250 {
		t.Errorf("cidWidths[3]: got %v, want 250", fi.cidWidths[3])
	}
	if fi.cidWidths[4] != 600 {
		t.Errorf("cidWidths[4]: got %v, want 600", fi.cidWidths[4])
	}
}

func TestResolveFontCIDWidthsRangeForm(t *testing.T) {
	objects := map[int]*pdfObject{
		10: {value: &pdfStream{
			Dict: pdfDict{},
			Data: []byte("begincmap\n1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\nendcmap"),
		}},
		20: {value: pdfDict{
			"/Type":    pdfName("/Font"),
			"/Subtype": pdfName("/CIDFontType2"),
			"/DW":      1000,
			"/W":       pdfArray{10, 12, 400},
		}},
	}
	fontDict := pdfDict{
		"/Type":            pdfName("/Font"),
		"/Subtype":         pdfName("/Type0"),
		"/BaseFont":        pdfName("/TestFont"),
		"/Encoding":        pdfName("/Identity-H"),
		"/ToUnicode":       pdfRef{id: 10},
		"/DescendantFonts": pdfArray{pdfRef{id: 20}},
	}
	fi := resolveFont(objects, fontDict)
	if fi.cidWidths[10] != 400 {
		t.Errorf("cidWidths[10]: got %v, want 400", fi.cidWidths[10])
	}
	if fi.cidWidths[11] != 400 {
		t.Errorf("cidWidths[11]: got %v, want 400", fi.cidWidths[11])
	}
	if fi.cidWidths[12] != 400 {
		t.Errorf("cidWidths[12]: got %v, want 400", fi.cidWidths[12])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run "TestResolveFontType0|TestResolveFontCIDWidths" ./...`
Expected: FAIL — `isType0`, `toUnicode`, `cidWidths`, `defaultW` fields don't exist.

- [ ] **Step 3: Extend `fontInfo` and update `resolveFont`**

In `font.go`, update `fontInfo`:

```go
type fontInfo struct {
	name      string
	encoding  [256]rune        // single-byte fonts (Type1, TrueType)
	widths    [256]float64     // single-byte widths
	toUnicode map[uint16]rune  // ToUnicode CMap mapping (glyph ID -> Unicode)
	cidWidths map[uint16]float64 // CID widths from /W array
	defaultW  float64          // /DW default width for CIDFont (1000 if absent)
	isType0   bool             // true = two-byte character codes
	known     bool
}
```

Add at the beginning of `resolveFont`, before the encoding logic:

```go
func resolveFont(objects map[int]*pdfObject, fontDict pdfDict) fontInfo {
	name := dictGetName(fontDict, "/BaseFont")
	fi := fontInfo{name: name, defaultW: 1000}

	// Detect Type0 (composite) font.
	subtype := dictGetName(fontDict, "/Subtype")
	if subtype == "/Type0" {
		fi.isType0 = true
	}

	// Parse /ToUnicode CMap if present (works for any font type).
	if tuVal, ok := fontDict["/ToUnicode"]; ok {
		resolved := resolveRef(objects, tuVal)
		if stream, ok := resolved.(*pdfStream); ok {
			fi.toUnicode = parseCMap(stream.Data)
			if len(fi.toUnicode) > 0 {
				fi.known = true
			}
		}
	}

	// For Type0: resolve descendant CIDFont for widths.
	if fi.isType0 {
		fi.cidWidths, fi.defaultW = resolveCIDWidths(objects, fontDict)
		fi.widths = resolveWidths(objects, fontDict, name)
		return fi
	}

	// --- existing single-byte encoding logic below (unchanged) ---
	encVal, hasEncoding := fontDict["/Encoding"]
	// ... rest of existing code ...
```

Add `resolveCIDWidths`:

```go
// resolveCIDWidths extracts /DW and /W from the CIDFont descendant.
func resolveCIDWidths(objects map[int]*pdfObject, type0Dict pdfDict) (map[uint16]float64, float64) {
	widths := make(map[uint16]float64)
	defaultW := 1000.0

	// Get descendant font dict.
	descVal, ok := type0Dict["/DescendantFonts"]
	if !ok {
		return widths, defaultW
	}
	descResolved := resolveRef(objects, descVal)
	descArr, ok := descResolved.(pdfArray)
	if !ok || len(descArr) == 0 {
		return widths, defaultW
	}
	cidDict, ok := resolveRefToDict(objects, descArr[0])
	if !ok {
		return widths, defaultW
	}

	// /DW — default width.
	if dw, ok := cidDict["/DW"]; ok {
		defaultW = operandFloat(dw)
	}

	// /W — width array.
	// Format: [cid [w1 w2 ...]] or [cidStart cidEnd width]
	if wVal, ok := cidDict["/W"]; ok {
		wResolved := resolveRef(objects, wVal)
		if wArr, ok := wResolved.(pdfArray); ok {
			parseCIDWidthArray(wArr, widths)
		}
	}

	return widths, defaultW
}

// parseCIDWidthArray parses a /W array into a map.
func parseCIDWidthArray(arr pdfArray, widths map[uint16]float64) {
	i := 0
	for i < len(arr) {
		cidStart, ok := toInt(arr[i])
		if !ok {
			i++
			continue
		}
		i++
		if i >= len(arr) {
			break
		}
		// Next element: array of individual widths, or second CID for range.
		switch v := arr[i].(type) {
		case pdfArray:
			// [cidStart [w1 w2 w3 ...]]
			for j, w := range v {
				widths[uint16(cidStart+j)] = operandFloat(w)
			}
			i++
		default:
			// [cidStart cidEnd width]
			cidEnd, ok := toInt(arr[i])
			if !ok {
				i++
				continue
			}
			i++
			if i >= len(arr) {
				break
			}
			w := operandFloat(arr[i])
			i++
			for c := cidStart; c <= cidEnd; c++ {
				widths[uint16(c)] = w
			}
		}
	}
}

// toInt converts a pdfValue to int.
func toInt(v pdfValue) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case float64:
		return int(n), true
	}
	return 0, false
}
```

- [ ] **Step 4: Run all font tests**

Run: `go test -run "TestResolveFont|TestParseCMap|TestStandard14|TestApplyDifferences" ./...`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add font.go content_parser_test.go
git commit -m "feat: resolve ToUnicode CMap and CIDFont widths in resolveFont"
```

---

### Task 3: Multi-byte text extraction in showString/showTJ

**Files:**
- Modify: `text.go`
- Test: `text_test.go` (append)

- [ ] **Step 1: Write the failing test**

In `text_test.go`, add:

```go
func TestExtractTextType0(t *testing.T) {
	// Build a Type0 font PDF with ToUnicode CMap.
	cmapData := []byte("begincmap\n1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n3 beginbfchar\n<0003> <0048>\n<0004> <0069>\n<0005> <0021>\nendbfchar\nendcmap")
	cmapStream := fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(cmapData), cmapData)
	cidFontDict := "<< /Type /Font /Subtype /CIDFontType2 /BaseFont /TestFont /DW 600 /W [3 [500 400 300]] /CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> >>"
	// Content: two-byte codes: 0x0003=H, 0x0004=i, 0x0005=!
	pageContent := "BT /F1 12 Tf 100 700 Td (\x00\x03\x00\x04\x00\x05) Tj ET"
	pageStream := fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(pageContent), pageContent)

	pdf := extAssemblePDF([]extTestObj{
		{1, []byte("<< /Type /Catalog /Pages 2 0 R >>")},
		{2, []byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")},
		{3, []byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>")},
		{4, []byte(pageStream)},
		{5, []byte(fmt.Sprintf("<< /Type /Font /Subtype /Type0 /BaseFont /TestFont /Encoding /Identity-H /ToUnicode 7 0 R /DescendantFonts [6 0 R] >>"))},
		{6, []byte(cidFontDict)},
		{7, []byte(cmapStream)},
	})
	doc, err := asposepdf.OpenStream(bytes.NewReader(pdf))
	if err != nil {
		t.Fatalf("OpenStream: %v", err)
	}
	page, _ := doc.Page(1)
	text, err := page.ExtractText()
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	if !strings.Contains(text, "Hi!") {
		t.Errorf("text=%q, want it to contain 'Hi!'", text)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run TestExtractTextType0 ./...`
Expected: FAIL — Type0 font is not handled by `showString`, produces U+FFFD or wrong text.

- [ ] **Step 3: Split `showString` into single-byte and multi-byte paths**

In `text.go`, replace the existing `showString`:

```go
func (e *textExtractor) showString(operand pdfValue) {
	s, ok := operand.(string)
	if !ok {
		return
	}
	if e.font.isType0 {
		e.showStringMultiByte(s)
	} else {
		e.showStringSingleByte(s)
	}
}

func (e *textExtractor) showStringSingleByte(s string) {
	for i := 0; i < len(s); i++ {
		code := s[i]
		r := e.font.encoding[code]
		// If toUnicode is available, prefer it for single-byte fonts too.
		if e.font.toUnicode != nil {
			if tr, ok := e.font.toUnicode[uint16(code)]; ok {
				r = tr
			}
		}
		if r == 0 {
			r = '\uFFFD'
		}
		e.emitRune(r)
		e.advanceGlyph(code)
	}
}

func (e *textExtractor) showStringMultiByte(s string) {
	for i := 0; i+1 < len(s); i += 2 {
		code := uint16(s[i])<<8 | uint16(s[i+1])
		r := rune(0)
		if e.font.toUnicode != nil {
			r = e.font.toUnicode[code]
		}
		if r == 0 {
			r = '\uFFFD'
		}
		e.emitRune(r)
		e.advanceGlyphCID(code)
	}
}
```

- [ ] **Step 4: Add `advanceGlyphCID` method**

```go
func (e *textExtractor) advanceGlyphCID(code uint16) {
	w0 := e.font.defaultW
	if cw, ok := e.font.cidWidths[code]; ok {
		w0 = cw
	}
	tx := (w0/1000.0*e.fontSize + e.charSpace) * e.horizScaling
	if code == 32 {
		tx += e.wordSpace * e.horizScaling
	}
	e.tm = matMul(translateMatrix(tx, 0), e.tm)
	e.lastX, e.lastY = e.currentPos()
}
```

- [ ] **Step 5: Split `showTJ` analogously**

Replace the existing `showTJ`:

```go
func (e *textExtractor) showTJ(operand pdfValue) {
	arr, ok := operand.(pdfArray)
	if !ok {
		return
	}
	for _, elem := range arr {
		switch v := elem.(type) {
		case string:
			if e.font.isType0 {
				e.showStringMultiByte(v)
			} else {
				e.showStringSingleByte(v)
			}
		case int:
			displacement := -float64(v) / 1000.0 * e.fontSize
			e.tm = matMul(translateMatrix(displacement, 0), e.tm)
		case float64:
			displacement := -v / 1000.0 * e.fontSize
			e.tm = matMul(translateMatrix(displacement, 0), e.tm)
		}
	}
}
```

- [ ] **Step 6: Run the new test**

Run: `go test -run TestExtractTextType0 ./...`
Expected: PASS — text contains "Hi!".

- [ ] **Step 7: Run ALL existing tests**

Run: `go test ./...`
Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
git add text.go text_test.go
git commit -m "feat: add multi-byte text extraction for Type0/CIDFont"
```

---

### Task 4: Integration tests with real PDFs

**Files:**
- Test: `text_test.go` (modify `TestExtractTextFiles`)

- [ ] **Step 1: Run TestExtractTextFiles and check PdfWithLinks**

Run: `go test -run TestExtractTextFiles -v ./...`

Then inspect `result_files/TestExtractTextFiles/PdfWithLinks/page_1.txt`. Verify:
- Page 1 should now contain readable English text, not all U+FFFD
- If still all U+FFFD, debug: check if the font dict has `/ToUnicode`, if the CMap stream is decompressed, if `isType0` is set correctly

- [ ] **Step 2: Check alfa.pdf**

Inspect `result_files/TestExtractTextFiles/alfa/page_1.txt`. Verify:
- Should contain readable Cyrillic text (MuseoSansCyrl fonts)
- Significantly fewer U+FFFD characters than before

- [ ] **Step 3: Check marketing.pdf**

Inspect `result_files/TestExtractTextFiles/marketing/full_text.txt`. Verify:
- Bullet characters: `��` should now be resolved (SymbolMT has ToUnicode with diamond suit `<2666>`)
- Or if the bullet font is not the one with ToUnicode, they may remain U+FFFD

- [ ] **Step 4: Check other test PDFs for regressions**

Verify 4pages.pdf, Binder1.pdf, PdfWithTable.pdf still produce same or better results.

- [ ] **Step 5: Commit any fixes**

If any adjustments were needed:

```bash
git add text.go font.go cmap.go
git commit -m "fix: adjustments from integration testing"
```

---

### Task 5: Update CLAUDE.md and close beads

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update architecture docs**

In the "Text extraction" section of `CLAUDE.md`, add `cmap.go` to the file list and update the `resolveFont` description:

Change the line about resolveFont to:
```
2. `resolveFont(objects, fontDict)` maps font dictionaries to `fontInfo` — supports WinAnsi, MacRoman, Standard encodings, `/Differences`, standard 14 fonts, Symbol, ZapfDingbats, ToUnicode CMap, Type0/CIDFont with Identity-H encoding; resolves glyph widths from `/Widths`, Standard 14 metrics, CID `/DW`+`/W`, or fallback
```

Add to the file list in the section header:
```
### Text extraction (`text.go`, `content_parser.go`, `font.go`, `font_metrics.go`, `encoding.go`, `cmap.go`)
```

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update architecture for ToUnicode CMap support"
```

- [ ] **Step 3: Close beads**

```bash
bd update pdf-go-vg8.1 --status closed
```
