# Tables — Design Specification

**Beads:** [pdf-go-0d8](bd show pdf-go-0d8)
**Date:** 2026-05-19
**Status:** Design proposed

---

## Goals

Add public API for **drawing tables on a page** with full per-cell control, mirroring Aspose.PDF for .NET's `Table` / `Row` / `Cell` / `BorderInfo` / `MarginInfo` types as closely as Go allows. Tables are positioned via a `Rectangle` (consistent with `AddText` / `AddImage`), not flow-layout.

### Non-goals (out of MVP)

- Multi-page overflow (tables that don't fit are clipped at the bounding rect, like `AddText`).
- Cell merging (rowspan / colspan).
- Image content in cells (text-only).
- Repeating header rows across pages.
- Column auto-width (fit-to-content) — all column widths are explicit in points.
- Per-row default text style (only table-level default + per-cell override).
- Cell borders with mixed widths per side or per-side colors (one width, one color per cell — but sides are independently selectable via bitmask).

These can be follow-ups; the design leaves room without baking assumptions that block them.

---

## Architecture

### Data model

Three pure-Go structs (no PDF dict backing). Built up by the caller before rendering, then handed to `(*Page).AddTable(t, rect)` which writes graphics operators to the page's content stream and returns. After rendering, the `*Table` is no longer referenced by the document — it's a transient builder.

```
Table
├── columnWidths      []float64   // pt
├── border            BorderInfo  // outer table border
├── defaultCellBorder BorderInfo  // applied to every cell unless cell.SetBorder overrides
├── defaultCellMargin MarginInfo  // applied to every cell unless cell.SetMargin overrides
├── defaultCellStyle  *TextStyle  // applied to every cell unless cell.SetTextStyle overrides
└── rows              []*Row

Row
├── table       *Table
├── height      float64  // 0 = auto-fit (compute from content)
└── cells       []*Cell

Cell
├── row             *Row
├── text            string
├── style           *TextStyle  // nil = use Row's table's defaultCellStyle
├── background      *Color      // nil = transparent
├── border          *BorderInfo // nil = use table's defaultCellBorder
├── margin          *MarginInfo // nil = use table's defaultCellMargin
├── hAlign / vAlign HAlign/VAlign
└── hAlignSet / vAlignSet bool  // distinguishes "set to zero value" from "not set"
```

### Rendering pipeline

`(*Page).AddTable(t *Table, rect Rectangle)`:

1. **Validate** — non-empty column widths; every row's cell count == `len(columnWidths)`; non-zero rectangle.
2. **Compute row heights** — for each row, if `row.height > 0` use it; otherwise compute auto-fit:
   - For each cell, compute available width = `columnWidths[col] - margin.Left - margin.Right`.
   - Compute content height = number of wrapped lines × line height (font size × line spacing). Uses the existing word-wrap logic from `text_add.go`.
   - Cell height = `content height + margin.Top + margin.Bottom`.
   - Row height = max(cell heights in the row).
3. **Draw** in this order to ensure correct layering:
   - For each cell, fill its background (if set).
   - For each cell, render its text content via the existing `AddText` machinery, with a sub-rectangle that respects per-cell margin.
   - Draw cell borders: per-side stroking using `BorderInfo.Sides` bitmask. The table outer border is drawn last to ensure it appears on top of any cell-edge overlaps.
4. **Clipping** — if computed total table height > `rect` height, draw what fits and stop. Each cell's text is also clipped to its own cell rect by reusing `AddText`'s clipping (q/Q + clip).

### Why reuse AddText for cell content

The existing `AddText` already handles: font resolution (standard 14 + embedded TTF + CID), text encoding (UTF-16 / WinAnsi / Identity-H), word wrap at glyph-width boundaries, alignment (H + V), line spacing, color, background, rotation, underline/strikethrough, clipping. Reimplementing any subset for cells would be technical debt and a maintenance trap. The Cell renderer composes a `TextStyle` (cell override > table default > library default) and a cell-interior `Rectangle` (cell rect minus margins), then calls `p.AddText(text, style, cellInteriorRect)`.

This means cell text inherits everything `AddText` supports for free — including font embedding, Unicode, and existing alignment semantics — at zero implementation cost.

### Border drawing

Borders are drawn via direct content-stream operators:

- Color via `r g b RG` (stroke RGB).
- Width via `w w`.
- Line caps via default (butt). No dash pattern in MVP.
- Per-side stroking: for each side requested by the bitmask, emit `m x1 y1` + `l x2 y2` + `S`.

The cell border is drawn after the cell text to ensure borders appear on top of (clip-truncated) cell content. The table outer border is drawn last.

### Page content stream

Reuses `(*Page).appendToContentStream(data []byte)` (already used by `AddText` / `AddImage`). One `AddTable` call produces one append (concatenated borders + backgrounds + per-cell `AddText` calls in sequence). Each per-cell `AddText` call appends separately to the page's content stream — acceptable bloat for MVP; can be coalesced later if it becomes a performance issue.

---

## Public API

### Types

```go
// BorderSide is a bitmask selecting which sides of a rectangular border are
// drawn. Combine with bitwise OR.
type BorderSide int

const (
    BorderSideNone   BorderSide = 0
    BorderSideTop    BorderSide = 1 << iota
    BorderSideRight
    BorderSideBottom
    BorderSideLeft
    BorderSideAll = BorderSideTop | BorderSideRight | BorderSideBottom | BorderSideLeft
)

// BorderInfo describes a border drawn around a table or cell.
// Mirrors Aspose.PDF for .NET's BorderInfo class.
//
// Zero value means "no border" (Sides == BorderSideNone).
type BorderInfo struct {
    Sides BorderSide
    Width float64 // in points; 0 means no border regardless of Sides
    Color *Color  // nil → black (R:0 G:0 B:0 A:1)
}

// MarginInfo describes margins or padding in points: Top / Right / Bottom / Left.
// Inside a Cell, MarginInfo represents the padding between the cell's border
// and its text content. Mirrors Aspose.PDF for .NET's MarginInfo class.
type MarginInfo struct {
    Top    float64
    Right  float64
    Bottom float64
    Left   float64
}

// Table is a transient builder for a tabular layout drawn onto a Page.
// Mirrors Aspose.PDF for .NET's Table class. After (*Page).AddTable renders
// the table, the *Table is not held by the document.
type Table struct { /* unexported */ }

// Row is a single row within a Table.
type Row struct { /* unexported */ }

// Cell is a single cell within a Row.
type Cell struct { /* unexported */ }
```

### Constructors

```go
func NewTable() *Table
```

### Table methods (all `Set*` return `*Table` for chaining)

```go
func (t *Table) SetColumnWidths(widths []float64) *Table
func (t *Table) ColumnWidths() []float64
func (t *Table) SetBorder(b BorderInfo) *Table
func (t *Table) Border() BorderInfo
func (t *Table) SetDefaultCellBorder(b BorderInfo) *Table
func (t *Table) DefaultCellBorder() BorderInfo
func (t *Table) SetDefaultCellMargin(m MarginInfo) *Table
func (t *Table) DefaultCellMargin() MarginInfo
func (t *Table) SetDefaultCellStyle(s TextStyle) *Table
func (t *Table) DefaultCellStyle() TextStyle
func (t *Table) AddRow() *Row
func (t *Table) Rows() []*Row
func (t *Table) RowCount() int
```

### Row methods

```go
func (r *Row) AddCell(text string) *Cell
func (r *Row) AddCells(texts ...string) []*Cell // convenience; equivalent to N×AddCell
func (r *Row) Cells() []*Cell
func (r *Row) CellCount() int
func (r *Row) SetHeight(h float64) *Row          // 0 = auto-fit (default)
func (r *Row) Height() float64
func (r *Row) Table() *Table
```

### Cell methods (all `Set*` return `*Cell` for chaining)

```go
func (c *Cell) SetText(text string) *Cell
func (c *Cell) Text() string
func (c *Cell) SetTextStyle(s TextStyle) *Cell
func (c *Cell) TextStyle() *TextStyle             // nil = inherits table default
func (c *Cell) SetBackground(col *Color) *Cell    // nil = transparent (default)
func (c *Cell) Background() *Color
func (c *Cell) SetBorder(b BorderInfo) *Cell      // overrides table.DefaultCellBorder
func (c *Cell) Border() *BorderInfo               // nil = inherits table default
func (c *Cell) SetMargin(m MarginInfo) *Cell      // overrides table.DefaultCellMargin
func (c *Cell) Margin() *MarginInfo               // nil = inherits table default
func (c *Cell) SetHAlign(h HAlign) *Cell
func (c *Cell) SetVAlign(v VAlign) *Cell
func (c *Cell) Row() *Row
```

### Rendering — Page method

```go
// AddTable renders the table inside the given rectangle. Cell content is drawn
// using the per-cell or table-default TextStyle, with per-cell padding (margin)
// applied. The table is clipped to the bounding rectangle; rows that don't fit
// are not drawn.
//
// Errors:
//   - rect.validate fails (LLX >= URX or LLY >= URY)
//   - len(t.columnWidths) == 0
//   - any row has CellCount() != len(t.columnWidths)
//   - any column width is non-positive
//   - the total of column widths is < 0 (not validated against rect width — over-wide tables clip on the right)
func (p *Page) AddTable(t *Table, rect Rectangle) error
```

---

## Aspose .PDF for .NET parity table

| Aspose .NET | This library |
|---|---|
| `new Table()` | `pdf.NewTable()` |
| `table.ColumnWidths = "100 200 100"` (string) | `table.SetColumnWidths([]float64{100, 200, 100})` |
| `table.Border = new BorderInfo(BorderSide.All, 1f)` | `table.SetBorder(pdf.BorderInfo{Sides: pdf.BorderSideAll, Width: 1})` |
| `table.DefaultCellBorder = new BorderInfo(...)` | `table.SetDefaultCellBorder(pdf.BorderInfo{...})` |
| `table.DefaultCellPadding = new MarginInfo(...)` | `table.SetDefaultCellMargin(pdf.MarginInfo{...})` — Aspose conflates "Margin" with "Padding"; we keep `MarginInfo` for type-name parity but the method is `SetDefaultCellMargin` (mirrors Aspose's `DefaultCellPadding` naming since both terms appear in the .NET surface) |
| `table.DefaultCellTextState = new TextState{...}` | `table.SetDefaultCellStyle(pdf.TextStyle{...})` — we use `TextStyle` (existing type) instead of inventing `TextState` |
| `Row row = table.Rows.Add()` | `row := table.AddRow()` |
| `Cell cell = row.Cells.Add("text")` | `cell := row.AddCell("text")` |
| `cell.BackgroundColor = Color.Yellow` | `cell.SetBackground(&pdf.Color{R: 1, G: 1, B: 0, A: 1})` |
| `cell.Alignment = HorizontalAlignment.Center` | `cell.SetHAlign(pdf.HAlignCenter)` |
| `cell.VerticalAlignment = VerticalAlignment.Center` | `cell.SetVAlign(pdf.VAlignMiddle)` |
| `cell.Border = new BorderInfo(...)` | `cell.SetBorder(pdf.BorderInfo{...})` |
| `cell.Margin = new MarginInfo(...)` | `cell.SetMargin(pdf.MarginInfo{...})` |
| `cell.TextState = new TextState{...}` | `cell.SetTextStyle(pdf.TextStyle{...})` |
| `page.Paragraphs.Add(table)` — flow layout | `page.AddTable(table, rect)` — explicit Rectangle (project convention) |

**One deliberate divergence:** Aspose uses `BackgroundColor` (`Color` value type) for cells; we use `SetBackground(*Color)` (pointer to nullable Color) so the zero-value distinguishes "no background" from "explicit black". This matches the `TextStyle.Background *Color` convention already in this codebase.

---

## Rendering details

### Cell rect computation

Given:
- Table top-left = `(rect.LLX, rect.URY)` in PDF user-space (origin at bottom-left of page).
- Column widths = `[w0, w1, w2, ...]`.
- Row heights = `[h0, h1, h2, ...]` (after auto-fit pass).

Cell at row `i`, column `j`:
- `cellLLX = rect.LLX + sum(w[0..j-1])`
- `cellURX = cellLLX + w[j]`
- `cellURY = rect.URY - sum(h[0..i-1])`
- `cellLLY = cellURY - h[i]`

### Cell interior rect (text)

After applying cell padding (margin):
- `textLLX = cellLLX + margin.Left`
- `textURX = cellURX - margin.Right`
- `textURY = cellURY - margin.Top`
- `textLLY = cellLLY + margin.Bottom`

### Auto-fit row height — content height calculation

The auto-fit pass must measure how tall the cell text would be **before** rendering, since the row height feeds into the cell rect, which feeds into AddText. We can't just call AddText and read back — we need a measurement-only helper.

Two options:
1. **Extract measurement from `text_add.go`** — refactor word-wrap logic out of `AddText` into a `measureText(text, style, maxWidth) (lines int, lineHeight float64)` helper that both `AddText` and the table renderer use.
2. **Pre-compute** by inlining the word-wrap algorithm in `table_render.go`.

**Decision: Option 1.** Word-wrap is the kind of logic that drifts if duplicated. The refactor in `text_add.go` is small (extract the wrap loop), preserves the existing public API, and gives us a documented internal helper.

### Border drawing — operator sequence

For each cell, after the text is drawn, emit:

```
q                              % save graphics state
0.5 w                          % border width
0 0 0 RG                       % stroke color (black or from BorderInfo.Color)
m x1 y1                        % move to start of top side
l x2 y1                        % top side
S                              % stroke
... repeat for other sides selected by bitmask
Q                              % restore graphics state
```

The "draw cell text first, then cell border" order means borders appear on top of (clipped) text — small but visible difference at glyph edges.

The table outer border is drawn last, after all cells. It is independent of the cell borders — the same edge may be drawn twice (once as the right-side cell border of column N-1 cells, once as the right side of the table border). Acceptable visual artifact for MVP; over-stroking on shared edges is invisible.

### Color encoding

Reuse the existing `Color` struct (R/G/B/A floats 0..1). For stroking and filling, emit `r g b RG` (stroke) or `r g b rg` (fill). The alpha component is ignored for borders in MVP (no semi-transparent borders); semi-transparent backgrounds reuse the existing `ensureExtGState` mechanism from `text_add.go`.

### TextStyle inheritance for cells

When rendering cell `c`:
1. Start with `TextStyle{}` zero value.
2. If `table.defaultCellStyle != nil`, overlay it on zero.
3. If `cell.style != nil`, overlay it on the result.
4. If `cell.hAlignSet`, override `style.HAlign`.
5. If `cell.vAlignSet`, override `style.VAlign`.

Overlay means: every non-zero field from the source replaces the destination field. For the `Font` field (interface), nil means "not set". For numeric fields like `Size` and `LineSpacing`, zero means "not set" (consistent with `AddText`'s own defaulting behavior).

---

## Edge cases & errors

| Case | Behavior |
|---|---|
| Empty table (no rows) | `AddTable` returns nil; no drawing. |
| Empty row (0 cells) | Skipped (row.height = 0). |
| Mismatched cell count | `error: "add table: row N has K cells, want %d"` |
| Zero column width | `error: "add table: column N has non-positive width"` |
| Negative column width | Same as zero. |
| `cell.SetText("")` | Cell is drawn with background + borders but no text. |
| Cell text overflows cell height | Clipped by `AddText`'s built-in clipping. |
| Total table height > rect height | Rows are drawn until one doesn't fit; remaining rows are not drawn. (Like `AddText` clipping the bottom.) |
| Total column width > rect width | Cells are drawn past the right edge of the bounding rect; **not** clipped at table level for MVP (no global clip rect is set). Document this as a known limitation. |
| `cell.SetBackground(nil)` | Transparent cell (no fill). |
| `cell.SetBorder(BorderInfo{Sides: BorderSideNone})` | No border drawn for this cell, even if `table.defaultCellBorder` has sides. |
| Border width = 0 | No border (even if Sides != None). |

---

## Testing strategy

### Unit tests (table_test.go, external `asposepdf_test` package)

1. `TestTable_EmptyRendersWithoutError` — empty table writes nothing, no error.
2. `TestTable_BasicShapeRoundTrip` — 2-row × 3-col table; open the output PDF and verify the content stream contains expected operators (line counts, fill counts).
3. `TestTable_CellTextAppears` — render a table with known cell text; extract text from the page; verify all cell texts are present.
4. `TestTable_AlignmentRespected` — render cells with H/V alignment override; check fragment X/Y positions via `ExtractTextWithLayout`.
5. `TestTable_BackgroundFillEmitted` — cell with background; output bytes contain a fill operator at expected coords (`re ... f`).
6. `TestTable_BorderSidesMask` — table with `BorderSideTop|BorderSideBottom` only; output contains 2 strokes per cell + table border, not 4.
7. `TestTable_CellOverridesDefault` — table.defaultCellBorder is set; one cell overrides via SetBorder; verify the override is used (different width detectable in output).
8. `TestTable_MismatchedCellCountErrors` — row has 2 cells but ColumnWidths has 3 entries; AddTable returns the expected error.
9. `TestTable_NonPositiveColumnWidthErrors` — width 0 or -1 → error.
10. `TestTable_AutoHeightFitsContent` — cell with multi-line text; row height auto-fits; manual check via ExtractTextWithLayout that all lines appear.
11. `TestTable_ExplicitRowHeight` — `row.SetHeight(20)`; verify row is exactly 20pt tall in the output.
12. `TestTable_Clipping` — table with total height > rect; rows past the bottom aren't drawn (last row's text not present in extraction).
13. `TestTable_Chaining` — `NewTable().SetColumnWidths(...).SetBorder(...).AddRow().AddCell("x")...` compiles and produces expected output.

### Cross-cutting

14. `TestTable_RoundTripUnderAES128` — write a doc with a table + AES-128; reopen; extract text; cell content present.
15. `TestTable_WithCustomFontEmbedded` — load a TTF, use it in `DefaultCellStyle`; verify cell text uses the embedded font (font name appears in the PDF output's font resources).

### Aspose .NET parity tests (named to match `TestAsposeParity_*` family)

16. `TestAsposeParity_TableBasic` — minimal Aspose-style table construction.
17. `TestAsposeParity_CellOverrides` — per-cell background/alignment/border.

---

## Risks & mitigations

| Risk | Mitigation |
|---|---|
| Auto-fit row height drifts from AddText's actual rendering | Refactor `AddText`'s word-wrap into a shared `measureText` helper used by both. Then row height is exactly what AddText will draw. |
| Multiple AddText calls bloat content streams | Acceptable for MVP. Document as a follow-up; can later coalesce by building one buffer and calling `appendToContentStream` once. |
| Border over-stroking on shared edges produces visible artifacts at high zoom | None in MVP. Document as known. Future fix: emit each edge exactly once via a `seen` map keyed by `(side, x1, y1, x2, y2)`. |
| Encryption-time content stream encryption double-encrypts when AddTable is called after SetEncryption | Same as existing AddText behavior. Encryption applies on `WriteTo` to the final content stream, regardless of how many appends preceded it. No new risk. |
| Cell text in non-ASCII / Unicode | Inherits from `AddText` (which supports UTF-16BE-with-BOM and Identity-H via embedded fonts). No new code needed. |
| Forward-compat for future cell content types (image) | Cell stores `text string` directly. Adding an `image` field later is non-breaking; renderer dispatches on what's set. |

---

## File structure

| File | Purpose |
|---|---|
| `table.go` (new) | `BorderSide` enum, `BorderInfo` / `MarginInfo` / `Table` / `Row` / `Cell` types + all method receivers (no rendering logic). |
| `table_render.go` (new) | `(*Page).AddTable` + private helpers: `computeRowHeights`, `drawCellBackground`, `drawCellBorder`, `drawTableBorder`. |
| `text_add.go` (modify) | Extract `measureText(text, style, maxWidth) (lines int, lineHeight, width float64, err error)` from the existing word-wrap path. `AddText` itself unchanged signature. |
| `table_test.go` (new) | External tests covering rendering, errors, alignment, encryption roundtrip, font embedding, and Aspose parity. |
| `CLAUDE.md` (modify) | New section after the annotations block documenting Table API. |
| `README.md` (modify) | Features bullet + usage snippet. |

Estimated total: ~600 lines of production code + ~400 lines of tests, in 10–12 TDD tasks.

---

## Self-review

**Placeholder scan:** every type, method, error message, and operator sequence is concrete. No TBDs.

**Internal consistency:** `MarginInfo` is consistent across `defaultCellMargin`, `SetDefaultCellMargin`, `cell.margin`, `cell.SetMargin`. `BorderInfo` is consistent across all four uses. `TextStyle` reuse matches the existing AddText/AddTextWatermark surface.

**Scope check:** MVP only, ~600 LOC of production code. Out-of-scope features (overflow, merge, image-cells) are deferred without leaving stubs or hooks that constrain future work.

**Ambiguity check:** "MarginInfo means padding inside cells" is documented explicitly in the type comment. The table-level inheritance chain (zero → defaultCellStyle → cell.style → explicit hAlign/vAlign) is written out. Over-wide tables not clipping at right edge is documented as a known MVP limitation.
