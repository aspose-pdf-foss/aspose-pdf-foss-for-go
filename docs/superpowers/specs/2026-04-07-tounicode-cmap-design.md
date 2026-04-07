# ToUnicode CMap Support

**Date:** 2026-04-07
**Status:** Approved
**Branch:** phase2.5-advanced-text-extraction
**Beads:** pdf-go-vg8.1

## Problem

Fonts using `/Encoding /Identity-H` (Type0/CIDFont) produce all U+FFFD because the current code only handles single-byte encodings via `[256]rune`. These fonts use 16-bit glyph IDs and require a `/ToUnicode` CMap stream to map glyph IDs to Unicode.

Affected test PDFs:
- **PdfWithLinks.pdf** — page 1 is 88% U+FFFD (Calibri, Identity-H, has ToUnicode)
- **alfa.pdf** — 92% U+FFFD (MuseoSansCyrl, Identity-H, 14 ToUnicode CMaps, Cyrillic text)
- **marketing.pdf** — 64 U+FFFD for SymbolMT special characters (has ToUnicode)
- **Binder1.pdf** — 87% U+FFFD (no ToUnicode — will NOT be fixed by this task)

## Solution: Approach C — `toUnicode map[uint16]rune` with Type0 auto-detection

### 1. Extend `fontInfo`

```go
type fontInfo struct {
    name      string
    encoding  [256]rune        // single-byte fonts (Type1, TrueType)
    widths    [256]float64     // single-byte widths
    toUnicode map[uint16]rune  // CMap mapping (glyph ID -> Unicode)
    cidWidths map[uint16]float64 // CID widths from /W array
    defaultW  float64          // /DW default width for CIDFont (1000 if absent)
    isType0   bool             // true = two-byte codes, false = single-byte
    known     bool
}
```

When `isType0 == true`, `showString`/`showTJ` read 2 bytes per character and use `toUnicode` + `cidWidths`/`defaultW`. Otherwise, existing single-byte path is unchanged.

**File:** `font.go`

### 2. CMap parser

New file `cmap.go`:

```go
func parseCMap(data []byte) map[uint16]rune
```

Parses ToUnicode CMap stream. Line-oriented parser (not reusing the PDF lexer — CMap is PostScript-like but simple enough for line parsing).

Supported sections:
- `beginbfchar`/`endbfchar`: `<srcCode> <dstUnicode>` — individual mappings
- `beginbfrange`/`endbfrange`: `<start> <end> <dstStart>` — contiguous range mappings
- `beginbfrange`/`endbfrange`: `<start> <end> [<dst1> <dst2> ...]` — array form (each code maps to the corresponding entry)

Hex parsing: `<0003>` -> `uint16(3)`, `<0410>` -> `rune(0x0410)` = Cyrillic A.

Multi-byte Unicode targets (e.g., `<D800DC00>` for surrogate pairs) are out of scope — these are rare and can be added later.

### 3. Integration in `resolveFont`

1. Detect `isType0`: check `/Subtype` == `/Type0` in font dict
2. For ANY font with `/ToUnicode`: resolve the stream, decompress, call `parseCMap` -> `fi.toUnicode`; set `fi.known = true`
3. For Type0 fonts: descend into `/DescendantFonts` array -> get CIDFont dict -> read `/DW` (default width, default 1000) and `/W` (width array)
4. `/W` format: `[cid1 cid2 [w1 w2 ...]]` or `[cid1 cid2 w]` — parse into `cidWidths map[uint16]float64`

ToUnicode takes precedence over encoding for character mapping when present (even for single-byte fonts).

**File:** `font.go`

### 4. Changes to `showString` / `showTJ`

Split into single-byte and multi-byte paths:

```go
func (e *textExtractor) showString(operand pdfValue) {
    s, ok := operand.(string)
    if !ok { return }
    if e.font.isType0 {
        e.showStringMultiByte(s)
    } else {
        e.showStringSingleByte(s)
    }
}
```

`showStringSingleByte`: current code, unchanged. If `toUnicode` is present and has a mapping for the byte code, use it instead of `encoding[code]`.

`showStringMultiByte`: read 2 bytes at a time (`uint16(s[i])<<8 | uint16(s[i+1])`), look up in `toUnicode`, emit rune. Width from `cidWidths[code]` or `defaultW`. Advance text matrix per glyph.

Analogous split for `showTJ`.

**File:** `text.go`

### 5. CID width parsing

Parse `/W` array from CIDFont descendant dict. The `/W` array format (PDF spec 9.7.4.3):

```
[cid_start cid_end width]         -> all CIDs in range get same width
[cid_start [w1 w2 w3 ...]]        -> consecutive CIDs get individual widths
```

Multiple entries can appear in the same array. Parse into `map[uint16]float64`.

`/DW` is a single number — default width for any CID not in `/W`. Default is 1000 if absent.

**File:** `font.go`

## Files Changed

| File | Change |
|------|--------|
| `cmap.go` | **New.** `parseCMap(data []byte) map[uint16]rune` — ToUnicode CMap parser |
| `font.go` | Add `toUnicode`, `cidWidths`, `defaultW`, `isType0` to `fontInfo`; parse `/ToUnicode`, `/DescendantFonts`, `/DW`, `/W` |
| `text.go` | Split `showString`/`showTJ` into single-byte and multi-byte paths |
| `cmap_test.go` | **New.** CMap parser tests |
| `content_parser_test.go` | Tests for Type0 font resolution, CID width parsing |
| `text_test.go` | Integration tests: PdfWithLinks, alfa.pdf readable text |

## Out of Scope

- Multi-byte Unicode targets in CMap (surrogate pairs like `<D800DC00>`) — rare
- Predefined CMaps (Adobe-Japan1-UCS2 etc.) — only embedded ToUnicode streams
- Binder1.pdf recovery — has no ToUnicode, needs different approach
- `beginbfrange` with array form where entries are multi-byte — rare edge case
