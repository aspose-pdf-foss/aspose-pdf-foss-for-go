# Vector Graphics Phase 2 — SVG-lite Embedding

**Beads:** [pdf-go-bu0](bd show pdf-go-bu0) (Phase 2) under umbrella [pdf-go-ybu](bd show pdf-go-ybu) (Vector support)
**Date:** 2026-05-22
**Status:** Design proposed

---

## Roadmap context

| Phase | Scope | Status |
|---|---|---|
| **1** | Native drawing primitives on `(*Page)`: lines, rectangles, circles, ellipses, polylines, polygons, paths. Line/shape styling. Path builder. | ✅ Shipped (v0.1.0) |
| **2 (this spec)** | SVG-lite embedding via `(*Page).AddSVG(path, rect)`. Parser + shapes + path commands + transforms + presentation attrs + viewBox. No text, no gradients, no raster. | Designing |
| **3** | SVG full — `<text>` with font matching, embedded raster via data-uri, `<defs>`/`<use>`, gradients (PDF Type 2/3 shading), patterns, masks/clipPath, CSS subset, basic filters. | Future |

Phase 2 is the SVG-lite layer. It targets the 70% of real-world SVG content that consists of basic shapes, paths, and group transforms — icons, simple logos, schematic diagrams, decorative vector elements. SVG files containing only-text or relying heavily on gradients fall into Phase 3.

---

## Phase 2 goals

Provide a first-class API for embedding external SVG content into PDF pages.

Specifically:

- `(*Page).AddSVG(path, rect)` as the primary entry point, with stream and pre-parsed variants
- Two-pass architecture: SVG → parsed IR (`*SVG`) → rendered PDF content stream
- Reuses Phase 1 drawing primitives — output is byte-identical to hand-written `DrawPath`/`DrawRectangle`/etc.
- Watermark variant for whole-document overlays
- Best-effort rendering: unsupported elements are silently skipped, only XML/parse errors are surfaced

### Non-goals (Phase 3 candidates)

- **`<text>` rendering** — text already has `(*Page).AddText`. SVG `<text>` integration with font matching belongs in Phase 3
- **Gradients** — `<linearGradient>` / `<radialGradient>` require PDF Type 2/3 shading patterns. Gradient `fill="url(#id)"` references in Phase 2 fall back to black
- **Raster within SVG** — `<image>` with `href="data:..."` or external `href`
- **`<defs>` / `<use>` / `<symbol>`** — reusable element definitions
- **Masks / clipPath** — `<mask>`, `<clipPath>`
- **CSS** — `<style>` blocks, CSS selectors, `class` matching. Only inline `style="..."` attribute supported in Phase 2
- **Filters** — `<filter>` and `feGaussianBlur`, `feDropShadow`, etc.
- **Markers** — `<marker>` (line endings beyond Phase 1's `LineEnding*` enum)
- **SMIL animation, scripts, `<foreignObject>`** — out of scope for static PDF
- **`em`, `ex`, `%` length units** — require font / parent context not modeled in Phase 2
- **`currentColor` with real CSS cascade** — resolved to black in Phase 2
- **Hyperlinks** — `<a>` element. Could map to PDF link annotations in a separate feature

---

## Public API

### New types

```go
// SVG is a pre-parsed SVG document. Returned by Document.LoadSVG /
// Document.LoadSVGFromStream and consumed by Page.AddSVGObject /
// Document.AddSVGObjectWatermark. Useful when the same SVG is rendered
// to multiple pages — parse once, render N times.
//
// Internal IR (parsed shape tree); has no exported fields. Inspector
// methods ViewBox and Size are provided for layout planning.
type SVG struct {
    // unexported
}

// ViewBox returns the viewBox attribute as (x, y, width, height).
// If no viewBox is set, returns (0, 0, intrinsicWidth, intrinsicHeight).
func (s *SVG) ViewBox() (x, y, w, h float64)

// Size returns the intrinsic width and height as parsed from <svg width=...
// height=...>. Returns (0, 0) if neither attribute is present.
func (s *SVG) Size() (width, height float64)
```

### Page rendering methods

```go
// AddSVG reads an SVG file and renders it into the given rectangle on the
// page. Unsupported elements (text, image, gradients, masks) are skipped
// silently per Phase 2 scope.
//
// Returns error only on XML parse failure, invalid numeric attributes
// (NaN/non-finite values), or I/O errors.
func (p *Page) AddSVG(path string, rect Rectangle) error

// AddSVGFromStream renders an SVG from any io.Reader.
func (p *Page) AddSVGFromStream(r io.Reader, rect Rectangle) error

// AddSVGObject renders a pre-parsed SVG (returned by Document.LoadSVG)
// into the given rectangle.
func (p *Page) AddSVGObject(svg *SVG, rect Rectangle) error
```

### Document load / watermark methods

```go
// LoadSVG reads and parses an SVG file once, returning a *SVG that can
// be passed to Page.AddSVGObject or Document.AddSVGObjectWatermark
// multiple times without re-parsing.
func (d *Document) LoadSVG(path string) (*SVG, error)

// LoadSVGFromStream is the io.Reader variant of LoadSVG.
func (d *Document) LoadSVGFromStream(r io.Reader) (*SVG, error)

// AddSVGWatermark applies an SVG watermark to all pages (when pageNums
// is empty) or to the specified 1-based page numbers. The SVG is
// positioned to fill each page's MediaBox honoring its own
// preserveAspectRatio attribute.
func (d *Document) AddSVGWatermark(path string, pageNums ...int) error

// AddSVGWatermarkFromStream is the io.Reader variant.
func (d *Document) AddSVGWatermarkFromStream(r io.Reader, pageNums ...int) error

// AddSVGObjectWatermark uses a pre-parsed *SVG for the watermark content.
func (d *Document) AddSVGObjectWatermark(svg *SVG, pageNums ...int) error
```

### API notes

1. `Rectangle` is in PDF user-space (Y up, origin bottom-left), consistent with all other library APIs. SVG coordinates are internally converted: SVG has Y down, the renderer inverts via CTM.
2. `preserveAspectRatio` is taken from the SVG document itself (default `xMidYMid meet`). If aspect ratios don't match, letterboxing appears on the short axis. To stretch without preserving aspect, the SVG must declare `preserveAspectRatio="none"`.
3. `AddSVGWatermark` renders into full MediaBox. For positioned watermarks, callers iterate pages and call `AddSVG` directly.
4. `*SVG.ViewBox()` / `*SVG.Size()` exist so callers can pre-compute Rectangle dimensions matching the SVG aspect ratio.
5. No options struct in Phase 2. All knobs default. Phase 3 may add `SVGOptions` for strict mode, gradient substitution, etc.

---

## Scope summary

| Category | Supported in Phase 2 |
|---|---|
| **Root element** | `<svg>` with `width`/`height`/`viewBox`/`preserveAspectRatio` |
| **Basic shapes** | `<rect>` (with `rx`/`ry`), `<circle>`, `<ellipse>`, `<line>`, `<polyline>`, `<polygon>`, `<path>` |
| **Path commands** | `M m L l H h V v C c S s Q q T t A a Z z` — full SVG 1.1 path syntax including elliptical arc |
| **Grouping** | `<g>` with attribute inheritance (fill, stroke, opacity, transform, all presentation attrs) |
| **Transforms** | `translate(x,y)`, `rotate(angle [,cx,cy])`, `scale(s)`/`scale(sx,sy)`, `matrix(a,b,c,d,e,f)`, `skewX(angle)`, `skewY(angle)` |
| **Presentation attrs** | `fill`, `stroke`, `stroke-width`, `stroke-dasharray`, `stroke-dashoffset`, `stroke-linecap`, `stroke-linejoin`, `stroke-miterlimit`, `opacity`, `fill-opacity`, `stroke-opacity`, `fill-rule`, `display`, `visibility` |
| **Style attr** | Inline `style="fill:red;stroke:blue"` — CSS-style property list. No `<style>` blocks, no selectors |
| **Colors** | `#RGB` / `#RRGGBB` / `#RRGGBBAA` / `rgb()` / `rgba()` / 147 CSS named colors / `none` / `transparent` / `currentColor` (→black) |
| **Length units** | `px`, `pt`, `pc`, `mm`, `cm`, `in`, unitless (= user units). Out: `em`, `ex`, `%` |
| **viewBox + aspect** | All 10 `preserveAspectRatio` values (`xMin/xMid/xMax` × `yMin/yMid/yMax` × `meet/slice`, plus `none`) |

---

## Internal architecture

Two-pass design: **parse → render**.

```
SVG file/stream
      ↓
  [Parser]  (encoding/xml + attribute parsing)
      ↓
  *SVG (internal IR — shape tree with resolved styles)
      ↓
  [Renderer]  (walks IR, emits PDF content stream)
      ↓
  PDF page content
```

Decoupling parser from renderer makes `*SVG` a cacheable intermediate result — the natural backing for `LoadSVG` + `AddSVGObject`.

### Files

| File | Responsibility |
|---|---|
| `svg.go` | Public types (`SVG`), methods `AddSVG*`, `LoadSVG*`, `AddSVGWatermark*`, `(*SVG).ViewBox` / `Size` |
| `svg_parse.go` | XML walker → internal IR. Uses `encoding/xml` from the standard library |
| `svg_attrs.go` | Per-attribute-type parsers: color, length-with-unit, transform list, dash array, fill-rule |
| `svg_path.go` | Path data parser (M/L/C/Q/A/Z + lowercase variants). Decomposes elliptical arcs into ≤4 cubic Béziers (extends Phase 1 `Path.Arc` logic to elliptical case) |
| `svg_render.go` | IR walker + PDF emission. Reuses Phase 1 primitives via private helpers that bypass public `q`/`Q` wrapping to avoid double-pushed gstate |
| `svg_transform.go` | Matrix composition for `<g transform>` inheritance; helper that emits PDF `cm` operator |
| `svg_viewbox.go` | viewBox + preserveAspectRatio → CTM that maps SVG content into the user-supplied Rectangle |

Tests:

| File | Coverage |
|---|---|
| `svg_test.go` | End-to-end AddSVG round-trips, error cases, watermark variants |
| `svg_parse_test.go` | XML → IR for all element types, attribute inheritance, edge cases |
| `svg_attrs_test.go` | Color parsing (all formats), length parsing (all units), transform string parsing |
| `svg_path_test.go` | Path data tokenizer + all command types, lowercase relative, smooth-curve reflection, elliptical arc decomposition |
| `svg_viewbox_test.go` | All 10 preserveAspectRatio modes, with matching and non-matching aspect ratios |

### Internal types (unexported)

```go
// svgNode is the IR interface.
type svgNode interface {
    svgNodeKind() string
}

type svgGroup struct {
    transform *svgMatrix      // optional CTM (nil = identity)
    style     svgStyle        // resolved presentation attrs
    children  []svgNode
}

type svgPath struct {
    commands  []svgPathOp     // parsed path data
    style     svgStyle
    transform *svgMatrix
}

type svgRect struct {
    x, y, w, h, rx, ry float64
    style              svgStyle
    transform          *svgMatrix
}

type svgCircle struct {
    cx, cy, r float64
    style     svgStyle
    transform *svgMatrix
}

type svgEllipse struct {
    cx, cy, rx, ry float64
    style          svgStyle
    transform      *svgMatrix
}

type svgLine struct {
    x1, y1, x2, y2 float64
    style          svgStyle
    transform      *svgMatrix
}

type svgPolyline struct {
    points    []Point
    style     svgStyle
    transform *svgMatrix
}

type svgPolygon struct {
    points    []Point
    style     svgStyle
    transform *svgMatrix
}

// svgStyle holds the resolved cascade after inheriting from parent <g>.
type svgStyle struct {
    fill          *Color    // nil = no fill
    stroke        *Color    // nil = no stroke
    strokeWidth   float64
    dashArray     []float64
    dashOffset    float64
    lineCap       LineCap   // reuse Phase 1 enum
    lineJoin      LineJoin  // reuse Phase 1 enum
    miterLimit    float64
    opacity       float64   // 1.0 default
    fillOpacity   float64   // 1.0 default
    strokeOpacity float64   // 1.0 default
    fillRule      string    // "nonzero" (default) or "evenodd"
    display       bool      // true (default) or false (skip rendering)
}

// SVG is the exported document type, returned by LoadSVG.
type SVG struct {
    viewBox *svgViewBox       // nil if no viewBox attribute
    width   float64           // intrinsic width (0 if not declared)
    height  float64           // intrinsic height (0 if not declared)
    par     svgPreserveAspect // preserveAspectRatio, default xMidYMid meet
    root    *svgGroup         // top-level shape tree
}

type svgViewBox struct {
    x, y, w, h float64
}

type svgPreserveAspect struct {
    align       string // "xMidYMid" / "none" / etc.
    meetOrSlice string // "meet" / "slice"
}

// svgMatrix is a 2D affine transform in column-major order:
//   [a c e]
//   [b d f]
//   [0 0 1]
// stored as [a b c d e f].
type svgMatrix [6]float64

// svgPathOp is one parsed path command after normalization to absolute coords.
type svgPathOp struct {
    kind      byte       // 'M', 'L', 'C', 'Q', 'A', 'Z'
    args      [7]float64 // command-specific
}
```

### Rendering algorithm

1. Renderer receives `*SVG` + `Rectangle`.
2. Computes **outer CTM** = `viewBox + preserveAspectRatio + Y-flip` → mapping into `Rectangle`.
3. Emits `q` (gsave) + outer CTM (PDF `cm` operator).
4. Recursive walk of IR:
   - For `<g>`: emit `q` + transform (if non-identity) → recurse children → emit `Q`
   - For shapes: apply style, emit `q` + transform (if present) on this element, then call Phase 1 internal helper for the shape, then `Q`
5. Final `Q` (grestore) to close outer CTM.

**Reuse of Phase 1:** Each shape is rendered via the same helpers that back `DrawRectangle` / `DrawCircle` / etc. The generated PDF byte sequence is identical to hand-written Phase 1 calls. Alpha, dash patterns, line caps/joins all work automatically.

### CTM management

PDF uses stack-based graphics state (`q`/`Q` = push/pop). SVG `<g transform>` maps naturally: each group emits `q` + `cm` + children + `Q`. Per-element `transform` attribute (on non-group) wraps that single element in `q` + `cm` + draw + `Q`.

Style inheritance is resolved at **parse time** (cascade is folded into `svgStyle` on each node), not at render time. This keeps the walker stateless and simplifies testing.

---

## Key behaviors

### viewBox + preserveAspectRatio → Rectangle mapping

The most nuanced part of the rendering pipeline.

1. If the `<svg>` element has no `viewBox`: use intrinsic dimensions (`width`/`height` attributes), or fall back to `Rectangle` directly (1:1 mapping) if those are absent too.
2. If `viewBox="x y w h"` is present:
   - `scaleX = Rectangle.width / viewBoxW`, `scaleY = Rectangle.height / viewBoxH`
   - `preserveAspectRatio="none"` → use different scaleX / scaleY (stretch, no aspect preservation)
   - `preserveAspectRatio="<align> meet"` (default `xMidYMid meet`) → use `min(scaleX, scaleY)`, align positioning per `<align>`
   - `preserveAspectRatio="<align> slice"` → use `max(scaleX, scaleY)`, clip to Rectangle (introduces a clip path)
3. Y-flip is always applied: SVG y-down → PDF y-up. Concatenated into the outer CTM as `[1, 0, 0, -1, 0, viewBoxH]`.

**Example.** Aspose logo (`viewBox="0 0 314 100"`) into `Rectangle{50, 700, 250, 750}` (200×50pt in header area):

- viewBox aspect ratio = 314/100 = 3.14
- Rectangle aspect ratio = 200/50 = 4.0
- Default `xMidYMid meet` → scale = min(200/314, 50/100) = min(0.637, 0.5) = **0.5**
- Effective render size: 314 × 0.5 = 157pt wide, 100 × 0.5 = 50pt tall
- Horizontal centering: pad = (200 - 157) / 2 = 21.5pt on each side
- Final placement: logo occupies `(50+21.5, 700)..(50+178.5, 750)` on the page

### Color parsing

| Input | Output |
|---|---|
| `#RGB` (3 chars) | Each digit × 17 (e.g., `#f00` → `(255, 0, 0)`) |
| `#RRGGBB` (6 chars) | 8-bit per channel |
| `#RRGGBBAA` (8 chars) | 8-bit per channel + alpha |
| `rgb(r,g,b)` | Integer (0-255) or percentage (`50%`) |
| `rgba(r,g,b,a)` | Same as rgb, alpha as float 0..1 or percentage |
| Named colors | 147 CSS Level-3 names, case-insensitive map |
| `none` / `transparent` | nil pointer (no fill/stroke) |
| `currentColor` | Black `(0, 0, 0, 1)` in Phase 2 (no CSS context) |
| Unrecognized | Fallback to black `(0, 0, 0, 1)` |

### Length parsing

Numeric value followed by optional unit suffix. Conversion to PDF points:

| Unit | Multiplier |
|---|---|
| (no unit), `px` | 1 (user units; 1 = 1 point after viewBox mapping) |
| `pt` | 1 |
| `pc` | 12 |
| `in` | 72 |
| `mm` | 72 / 25.4 ≈ 2.835 |
| `cm` | 72 / 2.54 ≈ 28.35 |
| `em`, `ex`, `%` | Returns 0 (Phase 3) |

### Style inheritance cascade

Each IR node stores an **already-resolved `svgStyle`**. During parsing, when creating a child node:

1. Start with the parent's `svgStyle` (copy by value)
2. Apply this element's `presentation attrs` (`fill="..."`, `stroke="..."`) on top
3. Apply this element's `style="..."` attribute on top (highest priority per SVG spec §6.4)
4. Store the result in the node

This eliminates the need to recompute the cascade during rendering and keeps the walker stateless.

### Path data parser

```
M 10,20 L 30,40 C 1 2 3 4 5 6 Z
m10,20l30,40c1 2 3 4 5 6z
```

- Commands separated by whitespace or commas (both equivalent)
- Lowercase = relative (from current point), uppercase = absolute
- `S` / `s` (smooth cubic) → reflect previous `C2` control through current point to form new `C1`
- `T` / `t` (smooth quadratic) → analogous reflection for quadratic control
- `A` / `a` (elliptical arc) → decomposes into 1-4 cubic Béziers via the Goldapp/Stanislaw formula. Extends Phase 1's `Path.Arc` (circular) to elliptical case via additional matrix transform for `rx ≠ ry` and `x-axis-rotation`
- Implicit subsequent commands: after `M x y`, additional coordinate pairs are treated as `L` (SVG spec §9.3.1)
- **Normalization during parsing**: parser converts shorthand commands to their canonical forms — `H`/`h` → `L` with current Y, `V`/`v` → `L` with current X, `S`/`s` → `C` with reflected control point, `T`/`t` → `Q` with reflected control point. Lowercase relative commands resolved against current point. The output `[]svgPathOp` thus contains only `M`, `L`, `C`, `Q`, `A`, `Z` kinds with absolute coordinates

### Transform composition

Multiple transforms in one attribute: `transform="translate(10,20) rotate(45) scale(2)"`.

- Applied **left to right** per SVG spec §7.6 — a point passes through `translate`, then `rotate`, then `scale`
- Composite matrix = `T × R × S` (matrix multiplication for column-vector convention)
- `rotate(angle, cx, cy)` is equivalent to `translate(cx, cy) rotate(angle) translate(-cx, -cy)`
- `skewX(angle)` = matrix `[1, 0, tan(angle), 1, 0, 0]`
- `skewY(angle)` = matrix `[1, tan(angle), 0, 1, 0, 0]`
- Identity matrix (no transform) → omitted from PDF `cm` emission

### Empty / degenerate input

- Empty `<svg></svg>` → no-op (no error)
- `width="0"` or `height="0"` on root → no-op
- Invalid XML → error
- Invalid path data → that single path is skipped (best-effort), other elements continue
- Missing `<svg>` root → error: "not an SVG document"
- XML namespaces: accept `xmlns="http://www.w3.org/2000/svg"` or no namespace. Foreign namespace attributes (e.g., `xmlns:inkscape="..."`) are ignored

### Cached `*SVG` size

The IR is a tree of value-typed structs. For a typical logo (10-50 elements) the in-memory size is a few kilobytes. For complex illustrations (1000+ paths) it's tens to hundreds of kilobytes — still much smaller than the original SVG text.

---

## Testing strategy

**Unit tests** (per parser / helper):

- `svg_parse_test.go` — XML → IR for all element types, attribute inheritance through nested `<g>`, namespace handling, malformed XML
- `svg_attrs_test.go` — Color parsing (all 6 formats), length parsing (all 7 units), transform string parsing (all 6 transform types and compositions)
- `svg_path_test.go` — All command types, lowercase relative, smooth-curve reflection, elliptical arc decomposition, mid-path `M` (subpath), `Z` closing
- `svg_viewbox_test.go` — All 10 preserveAspectRatio modes, matching and non-matching aspect ratios, Y-flip correctness

**Integration tests:**

- `svg_test.go` — End-to-end `AddSVG` round-trip with content-stream inspection, watermark variants, `LoadSVG` → `AddSVGObject` reuse, error paths
- AES-128 and AES-256 encryption round-trip with SVG content
- Multi-page AddSVGWatermark across all pages
- Aspose logo from `testdata/aspose-logo.svg` — verify black text path renders, gradient-filled shapes skipped silently

**Aspose .NET parity tests** (where applicable):

- `(*Page).AddSVG` matches Aspose.PDF for .NET's equivalent (which uses an SVG-to-XPS pipeline, but the public API shape and Rectangle-based positioning should be familiar to .NET migrants)

**Cross-tool validation:**

- The PDF page rendered by AddSVG should re-parse cleanly via the library's own `Validate(path)` (structural integrity)
- pypdf or qpdf round-trip: open the saved PDF, save again, the SVG-derived content stream survives intact
