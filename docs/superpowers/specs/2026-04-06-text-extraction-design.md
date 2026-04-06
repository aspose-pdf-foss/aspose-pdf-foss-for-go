# Phase 2: Content Stream Parsing & Text Extraction

## Summary

Add text extraction to the aspose.pdf-for-go library. Parse PDF content streams, resolve font encodings, track text positioning, and expose a simple public API returning plain text per page.

## Decisions

- **Accuracy level:** B — with positioning (spaces/newlines inferred from text matrix), without CIDFont/ToUnicode (deferred to Phase 2.5)
- **API:** `(*Page).ExtractText() (string, error)` + `(*Document).ExtractText() ([]string, error)`
- **Unknown fonts:** best-effort, replace unrecognized characters with U+FFFD
- **Multiple content streams:** concatenate per spec
- **Form XObjects:** recursive extraction via `Do` operator
- **Architecture:** monolithic — 4 new files in root package, no subpackages
- **Encodings:** WinAnsiEncoding, MacRomanEncoding, StandardEncoding, /Differences, Symbol, ZapfDingbats, 14 standard fonts

## New Files

### `content_parser.go` — Content Stream Parser

Parses decoded content stream bytes into a sequence of operators with operands.

```go
type contentOp struct {
    Operator string
    Operands []pdfValue
}

func parseContentStream(data []byte) ([]contentOp, error)
```

Algorithm:
1. Create `lexer` over stream bytes
2. Read tokens: numbers, strings, names push to operand stack
3. Keyword token → emit `contentOp` from stack + operator, clear stack
4. Special case: `BI`...`ID`...`EI` (inline images) — skip binary data until `EI`

Reuses the existing `lexer` from `lexer.go` as-is — content stream tokens are a subset of PDF object tokens.

### `encoding.go` — Encoding Tables

Static encoding tables and glyph name resolution.

```go
var winAnsiEncoding [256]rune
var macRomanEncoding [256]rune
var standardEncoding [256]rune
var symbolEncoding [256]rune
var zapfDingbatsEncoding [256]rune

var glyphToRune map[string]rune  // ~500 entries from Adobe Glyph List

func applyDifferences(base [256]rune, diffs pdfArray) [256]rune
```

### `font.go` — Font Resolver

Resolves a font dictionary into a usable encoding table.

```go
type fontInfo struct {
    name     string      // /BaseFont value
    encoding [256]rune   // code → Unicode
    known    bool        // false if encoding couldn't be determined
}

func resolveFont(objects map[int]*pdfObject, fontDict pdfDict) fontInfo
```

Encoding resolution logic:
1. `/Encoding` is a name → lookup table (WinAnsi, MacRoman, Standard)
2. `/Encoding` is a dict → lookup `/BaseEncoding` + apply `/Differences`
3. No `/Encoding` → StandardEncoding for standard 14 fonts, else `known=false`

Standard 14 fonts: Courier (4 variants), Helvetica (4), Times-Roman (4), Symbol, ZapfDingbats.

### `text.go` — Text State Machine & Public API

```go
func (p *Page) ExtractText() (string, error)
func (d *Document) ExtractText() ([]string, error)
```

Internal text state:

```go
type textState struct {
    font      fontInfo
    fontSize  float64
    charSpace float64    // Tc
    wordSpace float64    // Tw
    leading   float64    // TL
    tm        [6]float64 // text matrix
    lm        [6]float64 // line matrix
    ctm       [6]float64 // current transformation matrix
}
```

Operator handling:

| Operator | Action |
|----------|--------|
| `BT` | Reset tm, lm to identity |
| `ET` | End text block |
| `Tf` | Set font + fontSize from /Resources /Font |
| `Td` | `lm = translate(tx,ty) * lm; tm = lm` |
| `TD` | `TL = -ty; Td(tx,ty)` |
| `Tm` | Set tm and lm directly |
| `T*` | `Td(0, -TL)` |
| `Tj` | Decode string via font.encoding, emit characters |
| `TJ` | Array: strings decoded, numbers are kerning adjustments |
| `'` | `T*` then `Tj` |
| `"` | Set Tw, Tc, then `'` |
| `Tc` | Set charSpace |
| `Tw` | Set wordSpace |
| `TL` | Set leading |
| `cm` | Update CTM |
| `Do` | If Form XObject → recursive extraction with inherited CTM |

Space/newline insertion heuristic:
- Horizontal gap > fontSize * 0.25 * 0.3 → insert space
- Vertical shift > fontSize * 0.5 → insert `\n`
- Space width approximated as fontSize * 0.25 (no glyph metrics available)

Form XObject handling:
1. `Do /Fm1` → find `/Fm1` in `/Resources /XObject`
2. Verify `/Subtype /Form`
3. Parse form's content stream
4. Inherit current CTM, apply form's `/Matrix`
5. Form's `/Resources` override page resources for the recursive call

## Changes to Existing Files

### `doc.go` — `/Resources` inheritance

Add `/Resources` inheritance in `resolvePageTree`. When a page lacks `/Resources` but its parent `/Pages` node has one, copy the reference to the page dict. Same mechanism already used for `/MediaBox` inheritance.

### `page.go` — internal content stream access

### Internal helper on `Page`

```go
func (p *Page) contentStreams() ([]byte, error)
```

Returns concatenated decoded bytes of all content streams for the page. `/Contents` may be a single ref or an array of refs.

## Testing

### `content_parser_test.go`

Unit tests for content stream parser:
- Parse simple `BT ... ET` block
- Parse TJ array with kerning
- Handle inline images (BI/ID/EI skip)
- Empty stream
- Nested operators

### `text_test.go`

Unit tests (synthetic PDFs):
- `TestExtractTextMinimal` — existing `buildMinimalPDF()` contains `(Page 1) Tj`, verify extraction
- `TestExtractTextTJ` — TJ operator with kerning numbers
- `TestExtractTextMultiline` — multiple Td operators, verify `\n` insertion
- `TestExtractTextFormXObject` — text inside Form XObject
- `TestExtractTextUnknownFont` — unrecognized font → U+FFFD
- `TestDocumentExtractText` — `(*Document).ExtractText()` returns correct slice length

Integration tests (real PDFs from testdata):
- `TestExtractTextFiles` — iterate testfiles.json, extract text from every page, verify non-empty and not all U+FFFD

## Phase 2.5 — Future Work (backlog)

These items are explicitly out of scope for Phase 2:

1. **ToUnicode CMap** — parse `/ToUnicode` stream for CID → Unicode mapping. Critical for modern PDF generators (Chrome, Word, LaTeX).
2. **CIDFont / Type0** — composite fonts for CJK (Chinese, Japanese, Korean). Requires CMap + CIDToGIDMap.
3. **MacExpertEncoding** — rare encoding, for completeness.
4. **`ExtractTextWithLayout`** — structured result: `[]TextLine{Text, X, Y, Width, Height}`.
5. **ActualText / MarkedContent** — extract text from `/ActualText` in marked content sequences (accessible PDF).
6. **TrueType glyph mapping** — direct mapping via font's `cmap` table when neither `/Encoding` nor `/ToUnicode` are present.
