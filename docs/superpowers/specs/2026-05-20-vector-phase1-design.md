# Vector Graphics Phase 1 — Native Drawing Primitives

**Beads:** [pdf-go-5pq](bd show pdf-go-5pq) (Phase 1) under umbrella [pdf-go-ybu](bd show pdf-go-ybu) (Vector support)
**Date:** 2026-05-20
**Status:** Design proposed

---

## Future roadmap (context for Phase 1 decisions)

| Phase | Scope | Estimate |
|---|---|---|
| **1 (this spec)** | First-class drawing primitives on `(*Page)`: lines, rectangles, circles, ellipses, polylines, polygons, paths. Line/shape styling (color, width, dash, caps, joins, alpha). Path builder. | ~12-15 tasks |
| **2** | SVG-lite embedding — `(*Page).AddSVG(path, rect)`. Parser + shapes + path commands + transforms + presentation attrs + viewBox. No text, no gradients, no raster `<image>`. | ~18-22 tasks |
| **3** | SVG full — `<text>` with font matching, embedded raster via data-uri, `<defs>`/`<use>`, gradients (PDF Type 2/3 shading), patterns, masks/clipPath, CSS subset, basic filters. | ~20-25 tasks |

Phase 1 is the foundation for Phase 2 — every SVG primitive ultimately maps to one of the Phase 1 drawing methods. Without Phase 1 (or an equivalent internal API), Phase 2 can't be written cleanly.

Phase 1 also has standalone value: it exposes the operators that table borders / annotation drawing / redact paths already use internally, but currently are not callable from outside.

---

## Phase 1 goals

Provide a first-class API for drawing 2D vector content on PDF pages.

Specifically:
- Aspose-style fluent / strongly-typed API that mirrors what one would expect from a "graphics" library.
- Pure additive — no changes to existing types or method signatures.
- All output emitted via the existing `(*Page).appendToContentStream` path, so encryption, multi-page, and content-stream coalescing all "just work."
- Reuses existing types where they fit (`Color`, `Point`, `Rectangle`).
- Reuses internal infrastructure where possible (alpha via `ensureExtGState`, formatting via `formatFloat`).

### Non-goals (Phase 2+ candidates)

- **Transforms on individual draw calls** — no `DrawLineTransformed(...)`. Users pre-compute coordinates. Transforms become first-class with Phase 2 because SVG needs them.
- **Gradients** — Phase 3 (PDF shading patterns are complex).
- **Patterns / textures** — Phase 3.
- **Clipping paths** as a public API — internal use only (already used by `AddText` etc.).
- **Vector text rendering** — text already has `(*Page).AddText`. No need to duplicate.
- **Curve-fitting / smoothing** — caller decides control points.
- **Picture/SVG embedding** — Phase 2.
- **Reading/extracting existing vector graphics from a PDF** — separate feature.

---

## Public API

### Existing types reused

- `Point` (in `annotation_drawing.go`) — `X, Y float64`. Reuse.
- `Color` (in `color.go`) — `R, G, B, A float64`. Reuse.
- `Rectangle` (in `page.go`) — `LLX, LLY, URX, URY float64`. Reuse.

### New types

```go
// LineCap controls the shape at the endpoints of an open stroked path.
// Per ISO 32000-1 §8.4.3.3 / PDF operator J.
type LineCap int

const (
    LineCapButt    LineCap = 0 // default — flat end at the endpoint
    LineCapRound   LineCap = 1 // semicircle centered on endpoint
    LineCapSquare  LineCap = 2 // square extending half-width beyond endpoint
)

// LineJoin controls the shape at the corners of stroked paths.
// Per ISO 32000-1 §8.4.3.4 / PDF operator j.
type LineJoin int

const (
    LineJoinMiter LineJoin = 0 // sharp corner (clipped at miter limit)
    LineJoinRound LineJoin = 1
    LineJoinBevel LineJoin = 2
)

// LineStyle describes how a stroked path is drawn.
//
// Zero value: black, 1pt wide, solid, butt cap, miter join.
type LineStyle struct {
    Color       *Color    // nil → black {0,0,0,1}
    Width       float64   // 0 → 1pt default; negative ignored (treated as 0)
    DashPattern []float64 // [on, off, on, off, ...]; nil → solid
    DashPhase   float64   // offset into the dash pattern, default 0
    Cap         LineCap   // default Butt
    Join        LineJoin  // default Miter
    MiterLimit  float64   // 0 → PDF default (10)
}

// ShapeStyle combines stroke (LineStyle) and optional fill.
//
// FillColor nil → no fill (stroke-only). Width 0 in LineStyle → no stroke.
// At least one of stroke or fill must be configured; otherwise the shape is
// invisible (no error, just a no-op).
type ShapeStyle struct {
    LineStyle
    FillColor *Color // nil = no fill
}
```

### Path builder

```go
// Path is a sequence of moveto/lineto/curveto/close operations that defines
// an arbitrary 2D path. Construct via NewPath() and chain mutator methods.
//
// Coordinates are in PDF user space (origin at page bottom-left).
type Path struct {
    ops []pathOp // unexported; opaque
}

// NewPath returns an empty path.
func NewPath() *Path

// MoveTo begins a new subpath at (x, y). Returns p for chaining.
func (p *Path) MoveTo(x, y float64) *Path

// LineTo adds a straight line segment from the current point to (x, y).
func (p *Path) LineTo(x, y float64) *Path

// CurveTo adds a cubic Bezier curve from the current point to (x, y) with
// control points (c1x, c1y) and (c2x, c2y). PDF operator c.
func (p *Path) CurveTo(c1x, c1y, c2x, c2y, x, y float64) *Path

// QuadTo adds a quadratic Bezier curve (one control point) from the current
// point to (x, y), automatically converted to an equivalent cubic.
func (p *Path) QuadTo(cx, cy, x, y float64) *Path

// Arc adds an arc to the path, approximated by cubic Bezier curves.
// (cx, cy) is the center; r is the radius; startAngle and sweepAngle are in
// radians, counter-clockwise. The path's current point becomes the arc's
// endpoint after the call. If the path has no current point (no prior MoveTo),
// MoveTo to the arc start is implicit.
//
// Implementation: subdivide the sweep into ≤90° quadrants, approximate each
// with one cubic Bezier per Stanislaw Goldapp's formula
// (https://pomax.github.io/bezierinfo/#circles_cubic).
func (p *Path) Arc(cx, cy, r, startAngle, sweepAngle float64) *Path

// Close closes the current subpath by drawing a line from the current point
// back to the most recent MoveTo. PDF operator h.
func (p *Path) Close() *Path
```

### Page drawing methods

```go
// DrawLine strokes a single line segment from→to with the given style.
// If style.Width == 0, no stroking occurs (no-op).
func (p *Page) DrawLine(from, to Point, style LineStyle) error

// DrawRectangle strokes and/or fills an axis-aligned rectangle.
// At least one of style.Width > 0 (stroke) or style.FillColor != nil (fill)
// should be set; otherwise the call is a no-op.
//
// Emits the compact `re` operator + paint op.
func (p *Page) DrawRectangle(rect Rectangle, style ShapeStyle) error

// DrawRoundedRectangle strokes and/or fills an axis-aligned rectangle with
// rounded corners of the given radius. The radius is clamped to half the
// shorter side. Implemented as a Path internally.
func (p *Page) DrawRoundedRectangle(rect Rectangle, radius float64, style ShapeStyle) error

// DrawCircle strokes and/or fills a circle centered at the given point.
// Implemented via four cubic Beziers (one per quadrant) — visually
// indistinguishable from a true circle for all sizes.
func (p *Page) DrawCircle(center Point, radius float64, style ShapeStyle) error

// DrawEllipse strokes and/or fills an axis-aligned ellipse with semi-axes
// rx (horizontal) and ry (vertical). Implemented via four cubic Beziers.
func (p *Page) DrawEllipse(center Point, rx, ry float64, style ShapeStyle) error

// DrawPolyline strokes a polyline (open — first and last points not connected)
// through the given points. No fill. Errors if len(points) < 2.
func (p *Page) DrawPolyline(points []Point, style LineStyle) error

// DrawPolygon strokes and/or fills a closed polygon through the given points
// (last point connects to first). Errors if len(points) < 3.
func (p *Page) DrawPolygon(points []Point, style ShapeStyle) error

// DrawPath emits a previously-built Path with the given style. If path has
// no operations, the call is a no-op.
func (p *Page) DrawPath(path *Path, style ShapeStyle) error
```

---

## Rendering / output strategy

### Content-stream emission

Each draw method:
1. Builds a single `q ... Q` block.
2. Inside, emits graphics state ops (color, width, dash, etc.).
3. Emits the path-construction ops (`m`, `l`, `c`, `re`, `h`).
4. Emits the paint op (`S`, `f`, `B`).
5. Appends to the page content stream via `appendToContentStream`.

This isolates each draw call from anything else on the page — no graphics state leaks.

### Coalescing (future optimization)

For large numbers of draw calls, each `appendToContentStream` creates a new content-stream object. A future optimization could batch them. Phase 1 emits one append per draw call (acceptable; matches current `AddText` behavior).

### Operator sequence for a typical call

For `DrawLine({100, 100}, {200, 150}, LineStyle{Color: red, Width: 2, DashPattern: []float64{4, 2}})`:

```
q
2 w
[4 2] 0 d
1 0 0 RG
100 100 m
200 150 l
S
Q
```

### Operator sequence for `DrawRectangle` with stroke + fill

```
q
1 w
0 0 0 RG
1 1 0.5 rg
50 50 100 75 re
B
Q
```

### Operator sequence for `DrawCircle`

Circle approximation constant: `k = 0.5522847498` (4 × (√2 − 1) / 3). For radius r centered at (cx, cy):

```
q
... style ops ...
cx+r cy m
cx+r cy+r*k cx+r*k cy+r cx cy+r c    # upper-right quadrant
cx-r*k cy+r cx-r cy+r*k cx-r cy c    # upper-left
cx-r cy-r*k cx-r*k cy-r cx cy-r c    # lower-left
cx+r*k cy-r cx+r cy-r*k cx+r cy c    # lower-right
h
... paint op ...
Q
```

`DrawEllipse(cx, cy, rx, ry, ...)` substitutes `rx` for horizontal radius and `ry` for vertical.

### Alpha (transparency)

When any color's `A < 1`:
- For stroke: emit an `ExtGState` resource with `/CA <alpha>` and reference via `<name> gs`.
- For fill: emit an `ExtGState` resource with `/ca <alpha>` and reference via `<name> gs`.

The existing `ensureExtGState(alpha)` helper from `text_add.go` already handles this. We reuse it.

### Color operators

- Stroke: `R G B RG`
- Fill: `R G B rg`

Defaults to black `(0 0 0)` when `Color` is nil.

### DashPattern encoding

`[on off on off ...] phase d` — e.g. `[4 2] 0 d` for "4pt dashes, 2pt gaps, no offset". Empty pattern (nil or `[]`) → solid line, emitted as `[] 0 d`.

### Validation

- `DrawPolyline` with fewer than 2 points → error.
- `DrawPolygon` with fewer than 3 points → error.
- `DrawRoundedRectangle` with negative radius → error.
- `DrawCircle` / `DrawEllipse` with negative radius / rx / ry → error.
- `DrawPath` with nil path → error.

Style fields are not strictly validated — `Width < 0` is treated as 0 (no stroke), `MiterLimit < 1` is silently ignored (PDF default 10). Permissive intent: don't fail on user-typo `-1` width when intention is clear.

---

## Aspose .NET parity table

Aspose.PDF for .NET has `Aspose.Pdf.Drawing.Graph`, `Aspose.Pdf.Drawing.Line`, `Aspose.Pdf.Drawing.Circle`, etc., living under a `Graph` container that gets added to the page. We adopt a different, more idiomatic Go approach: methods directly on `*Page`.

| Aspose .NET | This library |
|---|---|
| `Graph graph = new Graph(width, height); page.Paragraphs.Add(graph);` (flow-layout container) | Direct: `page.DrawX(...)` with explicit coordinates. No container needed. |
| `Line line = new Line(new float[] {x1, y1, x2, y2}); line.GraphInfo.Color = ...; graph.Shapes.Add(line);` | `page.DrawLine(Point{x1, y1}, Point{x2, y2}, LineStyle{Color: ..., Width: ...})` |
| `Rectangle rect = new Rectangle(x, y, w, h); rect.GraphInfo.FillColor = ...; ...` | `page.DrawRectangle(Rectangle{LLX, LLY, URX, URY}, ShapeStyle{FillColor: ...})` |
| `Circle circle = new Circle(...); circle.GraphInfo.LineWidth = ...; ...` | `page.DrawCircle(Point{cx, cy}, r, ShapeStyle{LineStyle: LineStyle{Width: ...}})` |
| `circle.GraphInfo.DashArray = new int[] {4, 2}` | `LineStyle{DashPattern: []float64{4, 2}}` |
| `circle.GraphInfo.LineCap = LineCapStyle.Round` | `LineStyle{Cap: LineCapRound}` |

Conceptual mapping faithful; ergonomic API more Go-idiomatic.

---

## Edge cases and error semantics

| Case | Behavior |
|---|---|
| `LineStyle{}.Width == 0` | No stroke emitted. If `ShapeStyle.FillColor == nil` too → no-op, no error. |
| Path with no `MoveTo` before `LineTo` | Treat as starting from `(0,0)`. (Matches PDF spec — but document the surprise.) |
| `Path` with only `MoveTo`, no path-construction ops | No-op (no path to paint). No error. |
| `DrawCircle` with `radius <= 0` | Error. |
| Color components outside [0,1] | Pass through to PDF as-is (clamping is the consumer's job — this matches existing AddText behavior). |
| Empty `DashPattern []float64{}` (zero-length slice) | Treated as solid line — equivalent to nil. |
| `DashPattern` with negative values | PDF spec rejects negatives. We could validate or pass through. For MVP: pass through; document. |
| Coordinates outside the page MediaBox | PDF allows drawing outside; gets clipped by viewers. No error. |

---

## Internal refactor / reuse considerations

Several internal callers currently emit similar operators ad-hoc:

| File | What it emits | Phase 1 reuse potential |
|---|---|---|
| `table_render.go` (drawBorderSides) | Stroke ops for cell borders | Could be refactored to use `DrawLine` internally — defer to Phase 2 to avoid scope creep |
| `appearance_builder.go` | Mixed graphics state via builder pattern | Already a builder — unchanged |
| `annotation_drawing.go` (square/circle/line/ink) | Builds shapes into `/AP/N` Form XObjects, not page content | Cannot reuse Page methods (different output target); shared helpers possible |
| `redact_apply_path.go` | Path-construction parsing/rewriting | Read-side concern; no reuse |
| `text_add.go` | Text rendering only; no shape primitives | Already uses `ensureExtGState` which we reuse |

**For Phase 1:** introduce shared private helpers (`formatLineStyle`, `formatShapeStyle`, `pathOpsToOperators`) in a new file `vector.go`. **Don't** refactor existing internal callers in Phase 1 — too much risk, defer to a separate "consolidation" task in Phase 2.

---

## File structure

| File | Purpose |
|---|---|
| `vector.go` (new) | `LineCap` / `LineJoin` enums, `LineStyle` / `ShapeStyle` types, `Path` builder + methods, private `formatLineStyle` / `formatShapeStyle` helpers. |
| `vector_draw.go` (new) | `(*Page).DrawLine`, `DrawRectangle`, `DrawRoundedRectangle`, `DrawCircle`, `DrawEllipse`, `DrawPolyline`, `DrawPolygon`, `DrawPath` — all the Page methods. |
| `vector_test.go` (new) | External tests (`package asposepdf_test`). |
| `vector_internal_test.go` (new) | Internal tests for `Path` builder + `formatLineStyle` / arc decomposition. |
| `CLAUDE.md` (modify, last task) | New section under existing graphics blocks. |
| `README.md` (modify, last task) | Features bullet + usage snippet. |

Total estimate: ~700 LOC production code + ~700 LOC tests, in 12-15 TDD tasks.

---

## Testing strategy

### Unit tests (`vector_test.go`)

1. `TestDrawLine_BasicSolidStroke` — output contains `m`/`l`/`S` operators and color.
2. `TestDrawLine_DashedStroke` — output contains `d` op with the pattern.
3. `TestDrawLine_ZeroWidthNoStroke` — no `S` operator emitted; no error.
4. `TestDrawRectangle_StrokeOnly` — output uses `re` + `S`.
5. `TestDrawRectangle_FillOnly` — output uses `re` + `f`.
6. `TestDrawRectangle_StrokeAndFill` — output uses `re` + `B`.
7. `TestDrawRectangle_NoStyle_NoOp` — empty `ShapeStyle{}` → no stroke or fill emitted.
8. `TestDrawCircle_StrokeOnly_OutputHasBezier` — verifies 4 `c` operators (one per quadrant).
9. `TestDrawCircle_NegativeRadiusErrors`.
10. `TestDrawEllipse_AspectPreserved` — rx ≠ ry produces correct path.
11. `TestDrawPolyline_TwoPoints_Pass` — 2 points OK.
12. `TestDrawPolyline_OnePointErrors`.
13. `TestDrawPolygon_ThreePoints_Pass` — triangle.
14. `TestDrawPolygon_TwoPointsErrors`.
15. `TestDrawPath_EmptyPath_NoOp` — path with no ops, no error.
16. `TestDrawPath_LineTo_MoveTo_CurveTo_Close` — full roundtrip.
17. `TestDrawRoundedRectangle_Basic` — output has expected curve count + line count.
18. `TestDrawRoundedRectangle_NegativeRadiusErrors`.
19. `TestVector_AlphaTransparency_UsesExtGState` — `Color.A < 1` triggers `/CA`/`/ca` in ExtGState.
20. `TestVector_AES128Roundtrip` — drawing primitives survive encryption.

### Path builder unit tests (`vector_internal_test.go`)

21. `TestPath_BuilderChain` — chained calls produce expected internal op count.
22. `TestPath_ArcDecomposition_90Degrees` — arc(0, 90°) → 1 cubic Bezier.
23. `TestPath_ArcDecomposition_270Degrees` — arc(0, 270°) → 3 cubic Beziers.
24. `TestPath_ArcDecomposition_FullCircle` — arc(0, 360°) → 4 cubic Beziers.
25. `TestPath_QuadToConvertsToCubic` — quadratic Bezier emitted as cubic equivalent.

### Aspose parity tests (`vector_test.go`)

26. `TestAsposeParity_DrawLineWithDashAndCap` — mimics Aspose's Line + GraphInfo.DashArray + LineCap.
27. `TestAsposeParity_DrawCircleWithFill` — mimics Aspose's Circle + FillColor.

### Cross-cutting

28. `TestVector_DrawingOnMultiplePages` — draw on each of 3 pages; verify each has its own content.

---

## Risks & mitigations

| Risk | Mitigation |
|---|---|
| Arc decomposition has off-by-one quadrant edge cases (e.g., exactly 90° sweep) | Test all 4 quadrant boundaries explicitly (`TestPath_ArcDecomposition_90Degrees`). |
| Coordinate-system confusion (PDF Y-up vs SVG/screen Y-down) | Document that all coordinates are PDF user-space (Y-up). Phase 2 SVG parser handles the SVG→PDF Y flip. |
| `DashPattern` with negative numbers crashes some viewers | Document; defer validation to Phase 2 if real-world bugs emerge. |
| Existing PDF examples reading the page may see new XObject patterns | None — drawings go in the content stream, not as XObjects. Image extraction / text extraction unaffected. |
| Phase 2 will want transforms — Phase 1 API has no place for them | Acceptable. Phase 2 can add `*Path.Transform(matrix)` or `(*Page).DrawPathTransformed(...)`. Phase 1 keeps the surface tight. |
| Float-precision in arc / bezier approximations | Use double-precision throughout (Go's float64 default). Arc decomposition tolerance is much finer than PDF rendering resolution. |

---

## Self-review

**Placeholder scan:** every type and method signature is concrete. Operators specified with exact PDF syntax. Error messages templated where applicable.

**Internal consistency:** `LineStyle`/`ShapeStyle` follow the same pattern as existing `TextStyle`/`MarginInfo`/`BorderInfo`. `Path` is a builder type analogous to `*Table`. `Point` reused from `annotation_drawing.go`. `Color` reused from `color.go`. `Rectangle` reused from `page.go`.

**Scope check:** explicitly defers transforms, gradients, SVG, vector text, picture embedding. Three included features (primitives + Path builder + styling) are independent of any future SVG work but lay the foundation for it.

**Ambiguity check:** circle approximation constant is explicit (`0.5522847498`). Arc decomposition is bezier-based with explicit reference. Color encoding maps to existing conventions. Coordinate system is PDF user-space throughout (documented).
