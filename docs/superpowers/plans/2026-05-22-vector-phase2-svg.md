# Vector Graphics Phase 2 Implementation Plan — SVG-lite Embedding

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Embed external SVG content into PDF pages via `(*Page).AddSVG(path, rect)` and friends. Pure additive API — no breaking changes. Reuses Phase 1 drawing primitives so generated PDF output is byte-identical to hand-written `DrawPath`/`DrawRectangle`/etc.

**Architecture:** Two-pass design — `(SVG bytes → parsed IR *SVG)` then `(*SVG + Rectangle → PDF content stream)`. Parser uses `encoding/xml` from stdlib; renderer reuses Phase 1 internal helpers. Style inheritance cascade is resolved at parse time and stored on each IR node, so the renderer walker is stateless.

**Tech Stack:** Go 1.24, standard library only.

**Reference:** [docs/superpowers/specs/2026-05-22-vector-phase2-svg-design.md](../specs/2026-05-22-vector-phase2-svg-design.md)

**Beads:** [pdf-go-bu0](bd show pdf-go-bu0) (Phase 2) under umbrella [pdf-go-ybu](bd show pdf-go-ybu).

---

## File Map

| File | Purpose |
|---|---|
| `svg.go` (new) | Public types (`SVG`), public methods: `(*Page).AddSVG` / `AddSVGFromStream` / `AddSVGObject`, `(*Document).LoadSVG` / `LoadSVGFromStream` / `AddSVGWatermark` / `AddSVGWatermarkFromStream` / `AddSVGObjectWatermark`, `(*SVG).ViewBox` / `Size`. |
| `svg_types.go` (new) | Unexported IR types: `svgNode`, `svgGroup`, `svgPath`, `svgRect`, `svgCircle`, `svgEllipse`, `svgLine`, `svgPolyline`, `svgPolygon`, `svgStyle`, `svgMatrix`, `svgViewBox`, `svgPreserveAspect`, `svgPathOp`. |
| `svg_attrs.go` (new) | Attribute parsers: `parseSVGColor`, `parseSVGLength`, `parseSVGNumber`, `parseSVGFillRule`, `parseSVGLineCap`, `parseSVGLineJoin`. |
| `svg_transform.go` (new) | `parseSVGTransform`, matrix composition helpers (`matrixMul`, `matrixTranslate`, `matrixRotate`, `matrixScale`, `matrixSkew`, `matrixIdentity`, `matrixEquals`). |
| `svg_path.go` (new) | `parseSVGPathData` — tokenizer + normalizer. `decomposeArcToBeziers` (extends Phase 1 `Path.Arc` for elliptical case + x-axis-rotation). |
| `svg_viewbox.go` (new) | `parseViewBox`, `parsePreserveAspect`, `computeViewBoxMatrix` (10-mode preserveAspectRatio + Y-flip). |
| `svg_parse.go` (new) | XML walker `parseSVG`. Handles `<svg>`/`<g>`/shapes/`<path>` recursively with style cascade resolution. Skips unsupported elements silently. |
| `svg_render.go` (new) | `renderSVG` — walks IR, emits PDF content stream via Phase 1 internal helpers (`emitPath`, `emitRect`, `emitCircle`, etc.). |
| `svg_named_colors.go` (new) | 147-entry static map of CSS named colors. |
| `svg_attrs_test.go` (new) | Unit tests for attribute parsers. |
| `svg_transform_test.go` (new) | Unit tests for transform parsing + matrix composition. |
| `svg_path_test.go` (new) | Unit tests for path data parser. |
| `svg_viewbox_test.go` (new) | Unit tests for viewBox + preserveAspectRatio matrix calculation. |
| `svg_parse_test.go` (new) | Unit tests for XML walker (parser → IR). |
| `svg_test.go` (new) | External integration tests (`package asposepdf_test`): end-to-end AddSVG, watermark, encryption round-trip, Aspose logo. |
| `testdata/svg/` (new dir) | Test SVG fixtures: minimal valid, with-viewBox, with-transforms, with-gradient-ref, with-text-unsupported, malformed. |
| `CLAUDE.md` (modify, Task 20) | New "SVG embedding" section under Vector graphics. |
| `README.md` (modify, Task 20) | Features bullet + Quick Start snippet. |

---

## Task 1: Internal IR types skeleton

**Files:**
- Create: `svg_types.go`

- [ ] **Step 1: Create `svg_types.go` with all IR types**

```go
// SPDX-License-Identifier: MIT

package asposepdf

// svgNode is the interface implemented by every IR node.
type svgNode interface {
	svgNodeKind() string
}

// svgMatrix is a 2D affine transform in column-major order:
//   [a c e]
//   [b d f]
//   [0 0 1]
// stored as [a, b, c, d, e, f].
type svgMatrix [6]float64

// svgViewBox holds the four numbers of <svg viewBox="x y w h">.
type svgViewBox struct {
	x, y, w, h float64
}

// svgPreserveAspect holds the parsed preserveAspectRatio attribute.
// align is one of "none" / "xMinYMin" / "xMidYMin" / "xMaxYMin" / "xMinYMid"
// / "xMidYMid" / "xMaxYMid" / "xMinYMax" / "xMidYMax" / "xMaxYMax".
// meetOrSlice is "meet" (default) or "slice"; ignored when align == "none".
type svgPreserveAspect struct {
	align       string
	meetOrSlice string
}

// svgStyle holds resolved presentation attributes after parent cascade.
type svgStyle struct {
	fill          *Color
	stroke        *Color
	strokeWidth   float64
	dashArray     []float64
	dashOffset    float64
	lineCap       LineCap
	lineJoin      LineJoin
	miterLimit    float64
	opacity       float64
	fillOpacity   float64
	strokeOpacity float64
	fillRule      string
	display       bool
}

// defaultSVGStyle is the SVG initial value (per SVG spec §6.2 table).
// fill = black, no stroke, opacity = 1, fillRule = nonzero, display = true.
func defaultSVGStyle() svgStyle {
	return svgStyle{
		fill:          &Color{R: 0, G: 0, B: 0, A: 1},
		stroke:        nil,
		strokeWidth:   1,
		lineCap:       LineCapButt,
		lineJoin:      LineJoinMiter,
		miterLimit:    4,
		opacity:       1,
		fillOpacity:   1,
		strokeOpacity: 1,
		fillRule:      "nonzero",
		display:       true,
	}
}

// svgPathOp is one normalized path command (absolute coords, expanded shortcut).
type svgPathOp struct {
	kind byte       // 'M', 'L', 'C', 'Q', 'A', 'Z'
	args [7]float64 // command-specific; A uses all 7, M/L use [0..1], C uses [0..5], Q uses [0..3], Z uses none
}

type svgGroup struct {
	transform *svgMatrix
	style     svgStyle
	children  []svgNode
}

func (*svgGroup) svgNodeKind() string { return "g" }

type svgPath struct {
	commands  []svgPathOp
	style     svgStyle
	transform *svgMatrix
}

func (*svgPath) svgNodeKind() string { return "path" }

type svgRect struct {
	x, y, w, h, rx, ry float64
	style              svgStyle
	transform          *svgMatrix
}

func (*svgRect) svgNodeKind() string { return "rect" }

type svgCircle struct {
	cx, cy, r float64
	style     svgStyle
	transform *svgMatrix
}

func (*svgCircle) svgNodeKind() string { return "circle" }

type svgEllipse struct {
	cx, cy, rx, ry float64
	style          svgStyle
	transform      *svgMatrix
}

func (*svgEllipse) svgNodeKind() string { return "ellipse" }

type svgLine struct {
	x1, y1, x2, y2 float64
	style          svgStyle
	transform      *svgMatrix
}

func (*svgLine) svgNodeKind() string { return "line" }

type svgPolyline struct {
	points    []Point
	style     svgStyle
	transform *svgMatrix
}

func (*svgPolyline) svgNodeKind() string { return "polyline" }

type svgPolygon struct {
	points    []Point
	style     svgStyle
	transform *svgMatrix
}

func (*svgPolygon) svgNodeKind() string { return "polygon" }

// SVG is the pre-parsed SVG document.
type SVG struct {
	viewBox *svgViewBox
	width   float64
	height  float64
	par     svgPreserveAspect
	root    *svgGroup
}
```

- [ ] **Step 2: Run `go build ./...`**

Run: `go build ./...`
Expected: clean build (no test code yet — types compile against existing `Color`, `LineCap`, `LineJoin`, `Point` from Phase 1).

- [ ] **Step 3: Commit**

```bash
git add svg_types.go
git commit -m "feat: svg — internal IR types (svgNode + 7 shape types + svgStyle/svgMatrix/svgViewBox)"
```

---

## Task 2: Color parser (parseSVGColor)

**Files:**
- Create: `svg_named_colors.go`
- Create: `svg_attrs.go`
- Create: `svg_attrs_test.go`

- [ ] **Step 1: Write failing test in `svg_attrs_test.go`**

```go
// SPDX-License-Identifier: MIT

package asposepdf

import (
	"math"
	"testing"
)

func TestParseSVGColor_Hex3(t *testing.T) {
	c, ok := parseSVGColor("#f00")
	if !ok || c == nil {
		t.Fatalf("parseSVGColor(#f00) returned ok=%v c=%v", ok, c)
	}
	if math.Abs(c.R-1) > 1e-9 || c.G != 0 || c.B != 0 || c.A != 1 {
		t.Errorf("#f00 → %+v, want R=1 G=0 B=0 A=1", c)
	}
}

func TestParseSVGColor_Hex6(t *testing.T) {
	c, _ := parseSVGColor("#80c0FF")
	wantR, wantG, wantB := 128.0/255, 192.0/255, 1.0
	if math.Abs(c.R-wantR) > 1e-9 || math.Abs(c.G-wantG) > 1e-9 || math.Abs(c.B-wantB) > 1e-9 {
		t.Errorf("#80c0FF → %+v, want R=%g G=%g B=%g", c, wantR, wantG, wantB)
	}
}

func TestParseSVGColor_Hex8WithAlpha(t *testing.T) {
	c, _ := parseSVGColor("#ff000080")
	if c.R != 1 || c.G != 0 || c.B != 0 || math.Abs(c.A-128.0/255) > 1e-9 {
		t.Errorf("#ff000080 → %+v, want A=%g", c, 128.0/255)
	}
}

func TestParseSVGColor_RGB(t *testing.T) {
	c, _ := parseSVGColor("rgb(255, 128, 0)")
	if c.R != 1 || math.Abs(c.G-128.0/255) > 1e-9 || c.B != 0 {
		t.Errorf("rgb(255,128,0) → %+v", c)
	}
}

func TestParseSVGColor_RGBPercent(t *testing.T) {
	c, _ := parseSVGColor("rgb(100%, 50%, 0%)")
	if c.R != 1 || math.Abs(c.G-0.5) > 1e-9 || c.B != 0 {
		t.Errorf("rgb(100%%,50%%,0%%) → %+v", c)
	}
}

func TestParseSVGColor_RGBA(t *testing.T) {
	c, _ := parseSVGColor("rgba(0, 255, 0, 0.5)")
	if c.R != 0 || c.G != 1 || c.B != 0 || math.Abs(c.A-0.5) > 1e-9 {
		t.Errorf("rgba(0,255,0,0.5) → %+v", c)
	}
}

func TestParseSVGColor_NamedRed(t *testing.T) {
	c, _ := parseSVGColor("red")
	if c.R != 1 || c.G != 0 || c.B != 0 {
		t.Errorf("red → %+v", c)
	}
}

func TestParseSVGColor_NamedCaseInsensitive(t *testing.T) {
	c, _ := parseSVGColor("SlateBlue")
	wantR, wantG, wantB := 106.0/255, 90.0/255, 205.0/255
	if math.Abs(c.R-wantR) > 1e-9 || math.Abs(c.G-wantG) > 1e-9 || math.Abs(c.B-wantB) > 1e-9 {
		t.Errorf("SlateBlue → %+v", c)
	}
}

func TestParseSVGColor_None(t *testing.T) {
	c, ok := parseSVGColor("none")
	if !ok || c != nil {
		t.Errorf("none → ok=%v c=%v, want ok=true c=nil", ok, c)
	}
}

func TestParseSVGColor_Transparent(t *testing.T) {
	c, ok := parseSVGColor("transparent")
	if !ok || c != nil {
		t.Errorf("transparent → ok=%v c=%v, want ok=true c=nil", ok, c)
	}
}

func TestParseSVGColor_CurrentColor(t *testing.T) {
	c, ok := parseSVGColor("currentColor")
	if !ok || c == nil || c.R != 0 || c.G != 0 || c.B != 0 || c.A != 1 {
		t.Errorf("currentColor → ok=%v c=%+v, want ok=true c=black", ok, c)
	}
}

func TestParseSVGColor_Unrecognized(t *testing.T) {
	c, ok := parseSVGColor("not-a-color")
	if ok || c != nil {
		t.Errorf("garbage → ok=%v c=%v, want ok=false c=nil", ok, c)
	}
}
```

- [ ] **Step 2: Run tests, observe failures**

```powershell
go test -run TestParseSVGColor -v ./...
```

- [ ] **Step 3: Create `svg_named_colors.go` with the 147-color map**

Generate with full SVG named color table (red, green, blue, slateblue, etc.). Map keys are lowercase. Skeleton:

```go
// SPDX-License-Identifier: MIT

package asposepdf

// svgNamedColors maps CSS Level-3 named colors to RGB triples (8-bit each).
// Keys are lowercase ASCII. Lookup is case-insensitive via strings.ToLower.
// See https://www.w3.org/TR/css-color-3/#svg-color (147 entries).
var svgNamedColors = map[string][3]uint8{
	"aliceblue":            {240, 248, 255},
	"antiquewhite":         {250, 235, 215},
	"aqua":                 {0, 255, 255},
	"aquamarine":           {127, 255, 212},
	"azure":                {240, 255, 255},
	"beige":                {245, 245, 220},
	"bisque":               {255, 228, 196},
	"black":                {0, 0, 0},
	// ... (full 147 entries — see https://www.w3.org/TR/css-color-3/#svg-color)
	"slateblue":            {106, 90, 205},
	// ... continue
	"yellow":               {255, 255, 0},
	"yellowgreen":          {154, 205, 50},
}
```

*Note for implementer:* paste the full 147 entries from the CSS3 spec. Don't abbreviate. Use a tool to generate if needed (`curl https://www.w3.org/TR/css-color-3/named.html | sed ...`), or transcribe from the linked spec.

- [ ] **Step 4: Create `svg_attrs.go` with `parseSVGColor`**

```go
// SPDX-License-Identifier: MIT

package asposepdf

import (
	"strconv"
	"strings"
)

// parseSVGColor returns the parsed color and ok=true on success.
// For "none"/"transparent" returns (nil, true). For unrecognized input returns (nil, false).
func parseSVGColor(s string) (*Color, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false
	}
	switch strings.ToLower(s) {
	case "none", "transparent":
		return nil, true
	case "currentcolor":
		return &Color{R: 0, G: 0, B: 0, A: 1}, true
	}
	if strings.HasPrefix(s, "#") {
		return parseSVGColorHex(s[1:])
	}
	if strings.HasPrefix(s, "rgb(") || strings.HasPrefix(s, "rgba(") {
		return parseSVGColorRGB(s)
	}
	if rgb, ok := svgNamedColors[strings.ToLower(s)]; ok {
		return &Color{
			R: float64(rgb[0]) / 255,
			G: float64(rgb[1]) / 255,
			B: float64(rgb[2]) / 255,
			A: 1,
		}, true
	}
	return nil, false
}

func parseSVGColorHex(h string) (*Color, bool) {
	switch len(h) {
	case 3:
		// #RGB → each digit ×17 (== 0xN * 0x11)
		r, ok1 := hexNibble(h[0])
		g, ok2 := hexNibble(h[1])
		b, ok3 := hexNibble(h[2])
		if !ok1 || !ok2 || !ok3 {
			return nil, false
		}
		return &Color{R: float64(r*17) / 255, G: float64(g*17) / 255, B: float64(b*17) / 255, A: 1}, true
	case 6:
		r, ok1 := hexByte(h[0:2])
		g, ok2 := hexByte(h[2:4])
		b, ok3 := hexByte(h[4:6])
		if !ok1 || !ok2 || !ok3 {
			return nil, false
		}
		return &Color{R: float64(r) / 255, G: float64(g) / 255, B: float64(b) / 255, A: 1}, true
	case 8:
		r, ok1 := hexByte(h[0:2])
		g, ok2 := hexByte(h[2:4])
		b, ok3 := hexByte(h[4:6])
		a, ok4 := hexByte(h[6:8])
		if !ok1 || !ok2 || !ok3 || !ok4 {
			return nil, false
		}
		return &Color{R: float64(r) / 255, G: float64(g) / 255, B: float64(b) / 255, A: float64(a) / 255}, true
	}
	return nil, false
}

func hexNibble(b byte) (uint8, bool) {
	switch {
	case b >= '0' && b <= '9':
		return b - '0', true
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10, true
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10, true
	}
	return 0, false
}

func hexByte(s string) (uint8, bool) {
	hi, ok1 := hexNibble(s[0])
	lo, ok2 := hexNibble(s[1])
	if !ok1 || !ok2 {
		return 0, false
	}
	return hi*16 + lo, true
}

func parseSVGColorRGB(s string) (*Color, bool) {
	hasAlpha := strings.HasPrefix(s, "rgba(")
	open := strings.IndexByte(s, '(')
	close := strings.IndexByte(s, ')')
	if open < 0 || close < 0 || close < open {
		return nil, false
	}
	body := s[open+1 : close]
	parts := strings.Split(body, ",")
	if (hasAlpha && len(parts) != 4) || (!hasAlpha && len(parts) != 3) {
		return nil, false
	}
	chan_ := func(s string) (float64, bool) {
		s = strings.TrimSpace(s)
		if strings.HasSuffix(s, "%") {
			n, err := strconv.ParseFloat(s[:len(s)-1], 64)
			if err != nil {
				return 0, false
			}
			return n / 100, true
		}
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
		return n / 255, true
	}
	r, ok1 := chan_(parts[0])
	g, ok2 := chan_(parts[1])
	b, ok3 := chan_(parts[2])
	if !ok1 || !ok2 || !ok3 {
		return nil, false
	}
	a := 1.0
	if hasAlpha {
		as := strings.TrimSpace(parts[3])
		if strings.HasSuffix(as, "%") {
			n, err := strconv.ParseFloat(as[:len(as)-1], 64)
			if err != nil {
				return nil, false
			}
			a = n / 100
		} else {
			n, err := strconv.ParseFloat(as, 64)
			if err != nil {
				return nil, false
			}
			a = n
		}
	}
	return &Color{R: clamp01(r), G: clamp01(g), B: clamp01(b), A: clamp01(a)}, true
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}
```

- [ ] **Step 5: Run color tests, ensure all pass**

```powershell
go test -run TestParseSVGColor -v ./...
```

- [ ] **Step 6: Commit**

```bash
git add svg_named_colors.go svg_attrs.go svg_attrs_test.go
git commit -m "feat: svg — parseSVGColor (hex/rgb/rgba/named/none/currentColor)"
```

---

## Task 3: Length parser (parseSVGLength)

**Files:**
- Modify: `svg_attrs.go`
- Modify: `svg_attrs_test.go`

- [ ] **Step 1: Append failing tests**

```go
func TestParseSVGLength_Unitless(t *testing.T) {
	v, _ := parseSVGLength("42")
	if v != 42 { t.Errorf("42 → %g", v) }
}
func TestParseSVGLength_Px(t *testing.T) {
	v, _ := parseSVGLength("100px")
	if v != 100 { t.Errorf("100px → %g", v) }
}
func TestParseSVGLength_Pt(t *testing.T) {
	v, _ := parseSVGLength("10pt")
	if v != 10 { t.Errorf("10pt → %g", v) }
}
func TestParseSVGLength_Pc(t *testing.T) {
	v, _ := parseSVGLength("1pc")
	if v != 12 { t.Errorf("1pc → %g, want 12", v) }
}
func TestParseSVGLength_In(t *testing.T) {
	v, _ := parseSVGLength("1in")
	if v != 72 { t.Errorf("1in → %g, want 72", v) }
}
func TestParseSVGLength_Mm(t *testing.T) {
	v, _ := parseSVGLength("10mm")
	want := 10 * 72 / 25.4
	if math.Abs(v-want) > 1e-9 { t.Errorf("10mm → %g, want %g", v, want) }
}
func TestParseSVGLength_Cm(t *testing.T) {
	v, _ := parseSVGLength("1cm")
	want := 72 / 2.54
	if math.Abs(v-want) > 1e-9 { t.Errorf("1cm → %g, want %g", v, want) }
}
func TestParseSVGLength_Decimal(t *testing.T) {
	v, _ := parseSVGLength("3.14")
	if math.Abs(v-3.14) > 1e-9 { t.Errorf("3.14 → %g", v) }
}
func TestParseSVGLength_Negative(t *testing.T) {
	v, _ := parseSVGLength("-5")
	if v != -5 { t.Errorf("-5 → %g", v) }
}
func TestParseSVGLength_ScientificNotation(t *testing.T) {
	v, _ := parseSVGLength("1e2")
	if v != 100 { t.Errorf("1e2 → %g", v) }
}
func TestParseSVGLength_UnsupportedUnitFallsBackToZero(t *testing.T) {
	v, ok := parseSVGLength("10em")
	if ok || v != 0 { t.Errorf("10em → v=%g ok=%v, want 0/false", v, ok) }
}
func TestParseSVGLength_Garbage(t *testing.T) {
	v, ok := parseSVGLength("not-a-number")
	if ok || v != 0 { t.Errorf("garbage → v=%g ok=%v", v, ok) }
}
```

- [ ] **Step 2: Run, observe failures**

```powershell
go test -run TestParseSVGLength -v ./...
```

- [ ] **Step 3: Append `parseSVGLength` to `svg_attrs.go`**

```go
// parseSVGLength parses an SVG length value into PDF points.
// Supports unitless (= px = pt = user units), pt, pc, in, mm, cm, px.
// Returns (0, false) for em/ex/% (Phase 3) and unrecognized input.
func parseSVGLength(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	// Find unit suffix (trailing alpha chars or '%')
	i := len(s)
	for i > 0 {
		c := s[i-1]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '%' {
			i--
		} else {
			break
		}
	}
	numStr, unit := s[:i], strings.ToLower(s[i:])
	n, err := strconv.ParseFloat(strings.TrimSpace(numStr), 64)
	if err != nil {
		return 0, false
	}
	switch unit {
	case "", "px", "pt":
		return n, true
	case "pc":
		return n * 12, true
	case "in":
		return n * 72, true
	case "mm":
		return n * 72 / 25.4, true
	case "cm":
		return n * 72 / 2.54, true
	case "em", "ex", "%":
		return 0, false
	}
	return 0, false
}

// parseSVGNumber is like parseSVGLength but expects no unit suffix.
// Used for fill-opacity, stroke-width without units, etc.
func parseSVGNumber(s string) (float64, bool) {
	n, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, false
	}
	return n, true
}
```

- [ ] **Step 4: Run, ensure all length tests pass**

```powershell
go test -run "TestParseSVG(Length|Number)" -v ./...
```

- [ ] **Step 5: Commit**

```bash
git add svg_attrs.go svg_attrs_test.go
git commit -m "feat: svg — parseSVGLength (px/pt/pc/in/mm/cm) + parseSVGNumber"
```

---

## Task 4: Transform parser + matrix composition

**Files:**
- Create: `svg_transform.go`
- Create: `svg_transform_test.go`

- [ ] **Step 1: Write failing tests in `svg_transform_test.go`**

```go
// SPDX-License-Identifier: MIT

package asposepdf

import (
	"math"
	"testing"
)

func almostEqualM(t *testing.T, got, want svgMatrix, tol float64) {
	t.Helper()
	for i := range got {
		if math.Abs(got[i]-want[i]) > tol {
			t.Errorf("matrix mismatch at [%d]: got %g want %g\n  full got=%v want=%v", i, got[i], want[i], got, want)
			return
		}
	}
}

func TestMatrixIdentity(t *testing.T) {
	almostEqualM(t, matrixIdentity(), svgMatrix{1, 0, 0, 1, 0, 0}, 0)
}

func TestParseSVGTransform_Translate(t *testing.T) {
	m, ok := parseSVGTransform("translate(10, 20)")
	if !ok { t.Fatal("parse failed") }
	almostEqualM(t, m, svgMatrix{1, 0, 0, 1, 10, 20}, 1e-9)
}

func TestParseSVGTransform_TranslateSingleArg(t *testing.T) {
	m, _ := parseSVGTransform("translate(15)")
	almostEqualM(t, m, svgMatrix{1, 0, 0, 1, 15, 0}, 1e-9)
}

func TestParseSVGTransform_Scale(t *testing.T) {
	m, _ := parseSVGTransform("scale(2)")
	almostEqualM(t, m, svgMatrix{2, 0, 0, 2, 0, 0}, 1e-9)
}

func TestParseSVGTransform_ScaleXY(t *testing.T) {
	m, _ := parseSVGTransform("scale(2, 3)")
	almostEqualM(t, m, svgMatrix{2, 0, 0, 3, 0, 0}, 1e-9)
}

func TestParseSVGTransform_Rotate(t *testing.T) {
	m, _ := parseSVGTransform("rotate(90)")
	// cos90=0 sin90=1
	almostEqualM(t, m, svgMatrix{0, 1, -1, 0, 0, 0}, 1e-9)
}

func TestParseSVGTransform_RotateAroundPoint(t *testing.T) {
	m, _ := parseSVGTransform("rotate(90, 10, 20)")
	// equivalent to translate(10,20) rotate(90) translate(-10,-20):
	//   [0 1 -1 0  10+20=30  20-10=10]
	almostEqualM(t, m, svgMatrix{0, 1, -1, 0, 30, 10}, 1e-9)
}

func TestParseSVGTransform_Matrix(t *testing.T) {
	m, _ := parseSVGTransform("matrix(1, 2, 3, 4, 5, 6)")
	almostEqualM(t, m, svgMatrix{1, 2, 3, 4, 5, 6}, 1e-9)
}

func TestParseSVGTransform_Composite(t *testing.T) {
	// translate(10,20) scale(2) — point (1,1) → first scale → (2,2) → translate → (12, 22)
	m, _ := parseSVGTransform("translate(10, 20) scale(2)")
	// Composite: [2 0 0 2 10 20]
	almostEqualM(t, m, svgMatrix{2, 0, 0, 2, 10, 20}, 1e-9)
}

func TestParseSVGTransform_SkewX(t *testing.T) {
	m, _ := parseSVGTransform("skewX(45)")
	// matrix [1 0 tan(45) 1 0 0] = [1 0 1 1 0 0]
	almostEqualM(t, m, svgMatrix{1, 0, 1, 1, 0, 0}, 1e-9)
}

func TestParseSVGTransform_Empty(t *testing.T) {
	m, ok := parseSVGTransform("")
	if !ok { t.Fatal("empty should be identity, not failure") }
	almostEqualM(t, m, matrixIdentity(), 0)
}

func TestParseSVGTransform_Garbage(t *testing.T) {
	_, ok := parseSVGTransform("foo(1,2)")
	if ok { t.Error("expected garbage to fail") }
}
```

- [ ] **Step 2: Run, observe failures**

```powershell
go test -run "TestMatrix|TestParseSVGTransform" -v ./...
```

- [ ] **Step 3: Create `svg_transform.go`**

```go
// SPDX-License-Identifier: MIT

package asposepdf

import (
	"math"
	"strings"
)

func matrixIdentity() svgMatrix {
	return svgMatrix{1, 0, 0, 1, 0, 0}
}

// matrixMul returns A × B (column-vector convention).
// If point p = (x, y, 1), then (A × B) p means "first apply B, then A".
// For SVG composite transforms left-to-right, we accumulate result = result × M for each new M.
func matrixMul(a, b svgMatrix) svgMatrix {
	// [a0 a2 a4]   [b0 b2 b4]
	// [a1 a3 a5] × [b1 b3 b5]
	// [0  0  1 ]   [0  0  1 ]
	return svgMatrix{
		a[0]*b[0] + a[2]*b[1],
		a[1]*b[0] + a[3]*b[1],
		a[0]*b[2] + a[2]*b[3],
		a[1]*b[2] + a[3]*b[3],
		a[0]*b[4] + a[2]*b[5] + a[4],
		a[1]*b[4] + a[3]*b[5] + a[5],
	}
}

func matrixTranslate(tx, ty float64) svgMatrix {
	return svgMatrix{1, 0, 0, 1, tx, ty}
}

func matrixScale(sx, sy float64) svgMatrix {
	return svgMatrix{sx, 0, 0, sy, 0, 0}
}

func matrixRotate(deg float64) svgMatrix {
	r := deg * math.Pi / 180
	c, s := math.Cos(r), math.Sin(r)
	return svgMatrix{c, s, -s, c, 0, 0}
}

func matrixSkewX(deg float64) svgMatrix {
	return svgMatrix{1, 0, math.Tan(deg * math.Pi / 180), 1, 0, 0}
}

func matrixSkewY(deg float64) svgMatrix {
	return svgMatrix{1, math.Tan(deg * math.Pi / 180), 0, 1, 0, 0}
}

// parseSVGTransform parses one or more SVG transform functions joined by whitespace/commas.
// Returns identity for empty input. Returns ok=false if any function is malformed.
func parseSVGTransform(s string) (svgMatrix, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return matrixIdentity(), true
	}
	result := matrixIdentity()
	for len(s) > 0 {
		s = strings.TrimLeft(s, " \t\n\r,")
		if len(s) == 0 {
			break
		}
		open := strings.IndexByte(s, '(')
		if open < 0 {
			return matrixIdentity(), false
		}
		name := strings.TrimSpace(s[:open])
		close := strings.IndexByte(s[open:], ')')
		if close < 0 {
			return matrixIdentity(), false
		}
		body := s[open+1 : open+close]
		args, ok := parseSVGNumberList(body)
		if !ok {
			return matrixIdentity(), false
		}
		var m svgMatrix
		switch name {
		case "translate":
			switch len(args) {
			case 1:
				m = matrixTranslate(args[0], 0)
			case 2:
				m = matrixTranslate(args[0], args[1])
			default:
				return matrixIdentity(), false
			}
		case "scale":
			switch len(args) {
			case 1:
				m = matrixScale(args[0], args[0])
			case 2:
				m = matrixScale(args[0], args[1])
			default:
				return matrixIdentity(), false
			}
		case "rotate":
			switch len(args) {
			case 1:
				m = matrixRotate(args[0])
			case 3:
				m = matrixMul(matrixTranslate(args[1], args[2]),
					matrixMul(matrixRotate(args[0]), matrixTranslate(-args[1], -args[2])))
			default:
				return matrixIdentity(), false
			}
		case "matrix":
			if len(args) != 6 {
				return matrixIdentity(), false
			}
			m = svgMatrix{args[0], args[1], args[2], args[3], args[4], args[5]}
		case "skewX":
			if len(args) != 1 {
				return matrixIdentity(), false
			}
			m = matrixSkewX(args[0])
		case "skewY":
			if len(args) != 1 {
				return matrixIdentity(), false
			}
			m = matrixSkewY(args[0])
		default:
			return matrixIdentity(), false
		}
		result = matrixMul(result, m)
		s = s[open+close+1:]
	}
	return result, true
}

// parseSVGNumberList parses a comma/space-separated list of floats.
func parseSVGNumberList(s string) ([]float64, bool) {
	// Replace commas with spaces for uniform splitting
	s = strings.ReplaceAll(s, ",", " ")
	fields := strings.Fields(s)
	out := make([]float64, 0, len(fields))
	for _, f := range fields {
		n, ok := parseSVGNumber(f)
		if !ok {
			return nil, false
		}
		out = append(out, n)
	}
	return out, true
}
```

- [ ] **Step 4: Run, ensure all transform tests pass**

```powershell
go test -run "TestMatrix|TestParseSVGTransform" -v ./...
```

- [ ] **Step 5: Commit**

```bash
git add svg_transform.go svg_transform_test.go
git commit -m "feat: svg — parseSVGTransform (translate/rotate/scale/matrix/skew) + matrix helpers"
```

---

## Task 5: Path data parser (svg_path.go)

**Files:**
- Create: `svg_path.go`
- Create: `svg_path_test.go`

- [ ] **Step 1: Write failing tests covering all command types**

```go
// SPDX-License-Identifier: MIT

package asposepdf

import (
	"math"
	"testing"
)

func TestParseSVGPathData_MoveToLineTo(t *testing.T) {
	ops, err := parseSVGPathData("M 10 20 L 30 40")
	if err != nil { t.Fatal(err) }
	if len(ops) != 2 {
		t.Fatalf("len=%d, want 2; ops=%+v", len(ops), ops)
	}
	if ops[0].kind != 'M' || ops[0].args[0] != 10 || ops[0].args[1] != 20 {
		t.Errorf("op[0] = %+v", ops[0])
	}
	if ops[1].kind != 'L' || ops[1].args[0] != 30 || ops[1].args[1] != 40 {
		t.Errorf("op[1] = %+v", ops[1])
	}
}

func TestParseSVGPathData_ImplicitLineTo(t *testing.T) {
	// "M 10 20 30 40" → M 10,20 then L 30,40 (implicit)
	ops, _ := parseSVGPathData("M 10 20 30 40")
	if len(ops) != 2 || ops[1].kind != 'L' {
		t.Errorf("expected M then implicit L, got %+v", ops)
	}
}

func TestParseSVGPathData_RelativeMovingPoint(t *testing.T) {
	// "M 10 10 l 5 5" → M 10,10 then L 15,15 (absolute after relative)
	ops, _ := parseSVGPathData("M 10 10 l 5 5")
	if ops[1].kind != 'L' || ops[1].args[0] != 15 || ops[1].args[1] != 15 {
		t.Errorf("relative L not resolved to absolute: %+v", ops)
	}
}

func TestParseSVGPathData_HorizontalLine(t *testing.T) {
	// H normalizes to L with current Y
	ops, _ := parseSVGPathData("M 0 5 H 10")
	if ops[1].kind != 'L' || ops[1].args[0] != 10 || ops[1].args[1] != 5 {
		t.Errorf("H not normalized to L: %+v", ops[1])
	}
}

func TestParseSVGPathData_VerticalLine(t *testing.T) {
	ops, _ := parseSVGPathData("M 5 0 V 10")
	if ops[1].kind != 'L' || ops[1].args[0] != 5 || ops[1].args[1] != 10 {
		t.Errorf("V not normalized to L: %+v", ops[1])
	}
}

func TestParseSVGPathData_CubicBezier(t *testing.T) {
	ops, _ := parseSVGPathData("M 0 0 C 1 2 3 4 5 6")
	if ops[1].kind != 'C' {
		t.Fatalf("kind=%c", ops[1].kind)
	}
	if ops[1].args[0] != 1 || ops[1].args[5] != 6 {
		t.Errorf("C args = %v", ops[1].args[:6])
	}
}

func TestParseSVGPathData_SmoothCubic(t *testing.T) {
	// M0,0 C1,2,3,4,5,6 S 9 10 11 12
	// S becomes C with reflected C2 from previous C as new C1.
	// previous C2 = (3,4), current point = (5,6), reflect: (5*2-3, 6*2-4) = (7, 8)
	ops, _ := parseSVGPathData("M 0 0 C 1 2 3 4 5 6 S 9 10 11 12")
	if ops[2].kind != 'C' {
		t.Fatalf("S not normalized to C, kind=%c", ops[2].kind)
	}
	if ops[2].args[0] != 7 || ops[2].args[1] != 8 {
		t.Errorf("S reflection wrong: c1=%g,%g, want 7,8", ops[2].args[0], ops[2].args[1])
	}
}

func TestParseSVGPathData_QuadBezier(t *testing.T) {
	ops, _ := parseSVGPathData("M 0 0 Q 1 2 3 4")
	if ops[1].kind != 'Q' {
		t.Fatalf("kind=%c", ops[1].kind)
	}
}

func TestParseSVGPathData_Close(t *testing.T) {
	ops, _ := parseSVGPathData("M 0 0 L 10 10 Z")
	if ops[2].kind != 'Z' {
		t.Errorf("Z not parsed, ops=%+v", ops)
	}
}

func TestParseSVGPathData_NoCommas(t *testing.T) {
	ops1, _ := parseSVGPathData("M0,0L10,10")
	ops2, _ := parseSVGPathData("M 0 0 L 10 10")
	if len(ops1) != len(ops2) || ops1[1].args[0] != ops2[1].args[0] {
		t.Errorf("comma vs space parsing differs")
	}
}

func TestParseSVGPathData_Arc(t *testing.T) {
	// Just verify it parses; decomposition to Beziers tested separately.
	ops, err := parseSVGPathData("M 0 0 A 5 5 0 1 0 10 0")
	if err != nil { t.Fatal(err) }
	// Should decompose into 1-4 C ops; M plus at least 1 C
	if len(ops) < 2 {
		t.Fatalf("expected M plus at least 1 C from arc decomposition, got %d ops", len(ops))
	}
	if ops[0].kind != 'M' || ops[1].kind != 'C' {
		t.Errorf("arc should decompose to C operators, got %c %c", ops[0].kind, ops[1].kind)
	}
	// End point of the decomposed arc must reach (10, 0)
	last := ops[len(ops)-1]
	if math.Abs(last.args[4]-10) > 1e-6 || math.Abs(last.args[5]) > 1e-6 {
		t.Errorf("arc endpoint = (%g, %g), want (10, 0)", last.args[4], last.args[5])
	}
}

func TestParseSVGPathData_Malformed(t *testing.T) {
	_, err := parseSVGPathData("M 0")
	if err == nil { t.Error("expected error for incomplete M") }
}
```

- [ ] **Step 2: Run, observe failures**

```powershell
go test -run TestParseSVGPathData -v ./...
```

- [ ] **Step 3: Create `svg_path.go` (skeleton + tokenizer)**

```go
// SPDX-License-Identifier: MIT

package asposepdf

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// parseSVGPathData parses an SVG path data string into a normalized []svgPathOp.
// After normalization, only M, L, C, Q, A, Z kinds remain, all with absolute coords.
// (Note: A is further decomposed into C operators during parsing — final output has no A.)
func parseSVGPathData(d string) ([]svgPathOp, error) {
	tokens, err := tokenizeSVGPath(d)
	if err != nil {
		return nil, err
	}
	return normalizeSVGPath(tokens)
}

// svgPathToken is a raw token: either a command letter or a number.
type svgPathToken struct {
	isCmd bool
	cmd   byte
	num   float64
}

func tokenizeSVGPath(d string) ([]svgPathToken, error) {
	out := make([]svgPathToken, 0, 32)
	i := 0
	for i < len(d) {
		c := d[i]
		// Skip whitespace and commas
		if c == ' ' || c == ',' || c == '\t' || c == '\n' || c == '\r' {
			i++
			continue
		}
		// Command letter
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			out = append(out, svgPathToken{isCmd: true, cmd: c})
			i++
			continue
		}
		// Number (may start with sign, digit, or '.')
		if c == '+' || c == '-' || c == '.' || (c >= '0' && c <= '9') {
			j := i + 1
			seenDot := c == '.'
			seenE := false
			for j < len(d) {
				ch := d[j]
				if ch >= '0' && ch <= '9' {
					j++
				} else if ch == '.' && !seenDot && !seenE {
					seenDot = true
					j++
				} else if (ch == 'e' || ch == 'E') && !seenE {
					seenE = true
					j++
					if j < len(d) && (d[j] == '+' || d[j] == '-') {
						j++
					}
				} else {
					break
				}
			}
			n, err := strconv.ParseFloat(d[i:j], 64)
			if err != nil {
				return nil, fmt.Errorf("invalid number %q in path data: %w", d[i:j], err)
			}
			out = append(out, svgPathToken{num: n})
			i = j
			continue
		}
		// Unicode whitespace fallback
		if unicode.IsSpace(rune(c)) {
			i++
			continue
		}
		return nil, fmt.Errorf("unexpected character %q in path data at %d", c, i)
	}
	return out, nil
}
```

- [ ] **Step 4: Continue `svg_path.go` — normalizer**

```go
// normalizeSVGPath consumes raw tokens and emits normalized svgPathOps.
// Tracks current point (cx, cy), last C2 control (for S reflection),
// last Q control (for T reflection), and subpath start (for Z).
func normalizeSVGPath(tokens []svgPathToken) ([]svgPathOp, error) {
	ops := make([]svgPathOp, 0, len(tokens)/4)
	var cx, cy float64
	var startX, startY float64
	var lastCubicC2X, lastCubicC2Y float64
	var hasLastCubic bool
	var lastQuadCX, lastQuadCY float64
	var hasLastQuad bool

	i := 0
	if len(tokens) == 0 {
		return nil, nil
	}
	if !tokens[0].isCmd {
		return nil, errors.New("path data must start with a command")
	}
	curCmd := tokens[0].cmd
	i++

	num := func() (float64, error) {
		if i >= len(tokens) || tokens[i].isCmd {
			return 0, fmt.Errorf("expected number for command %c at token %d", curCmd, i)
		}
		v := tokens[i].num
		i++
		return v, nil
	}

	for i < len(tokens) || curCmd == 0 {
		// Resolve command for new operand group.
		// Note: in SVG, repeated args after a command continue with that command,
		// except after M/m which switches to L/l.
		if i < len(tokens) && tokens[i].isCmd {
			curCmd = tokens[i].cmd
			i++
		}
		switch curCmd {
		case 'M', 'm':
			x, err := num()
			if err != nil { return nil, err }
			y, err := num()
			if err != nil { return nil, err }
			if curCmd == 'm' {
				x += cx
				y += cy
			}
			cx, cy = x, y
			startX, startY = x, y
			ops = append(ops, svgPathOp{kind: 'M', args: [7]float64{x, y}})
			hasLastCubic, hasLastQuad = false, false
			// Implicit L/l after M/m
			if curCmd == 'M' {
				curCmd = 'L'
			} else {
				curCmd = 'l'
			}
		case 'L', 'l':
			x, err := num()
			if err != nil { return nil, err }
			y, err := num()
			if err != nil { return nil, err }
			if curCmd == 'l' { x += cx; y += cy }
			cx, cy = x, y
			ops = append(ops, svgPathOp{kind: 'L', args: [7]float64{x, y}})
			hasLastCubic, hasLastQuad = false, false
		case 'H', 'h':
			x, err := num()
			if err != nil { return nil, err }
			if curCmd == 'h' { x += cx }
			ops = append(ops, svgPathOp{kind: 'L', args: [7]float64{x, cy}})
			cx = x
			hasLastCubic, hasLastQuad = false, false
		case 'V', 'v':
			y, err := num()
			if err != nil { return nil, err }
			if curCmd == 'v' { y += cy }
			ops = append(ops, svgPathOp{kind: 'L', args: [7]float64{cx, y}})
			cy = y
			hasLastCubic, hasLastQuad = false, false
		case 'C', 'c':
			x1, _ := num(); y1, _ := num()
			x2, _ := num(); y2, _ := num()
			x, err := num(); if err != nil { return nil, err }
			y, _ := num()
			if curCmd == 'c' {
				x1 += cx; y1 += cy
				x2 += cx; y2 += cy
				x += cx; y += cy
			}
			ops = append(ops, svgPathOp{kind: 'C', args: [7]float64{x1, y1, x2, y2, x, y}})
			cx, cy = x, y
			lastCubicC2X, lastCubicC2Y = x2, y2
			hasLastCubic, hasLastQuad = true, false
		case 'S', 's':
			x2, _ := num(); y2, _ := num()
			x, err := num(); if err != nil { return nil, err }
			y, _ := num()
			if curCmd == 's' {
				x2 += cx; y2 += cy
				x += cx; y += cy
			}
			var x1, y1 float64
			if hasLastCubic {
				x1 = 2*cx - lastCubicC2X
				y1 = 2*cy - lastCubicC2Y
			} else {
				x1, y1 = cx, cy
			}
			ops = append(ops, svgPathOp{kind: 'C', args: [7]float64{x1, y1, x2, y2, x, y}})
			cx, cy = x, y
			lastCubicC2X, lastCubicC2Y = x2, y2
			hasLastCubic, hasLastQuad = true, false
		case 'Q', 'q':
			x1, _ := num(); y1, _ := num()
			x, err := num(); if err != nil { return nil, err }
			y, _ := num()
			if curCmd == 'q' { x1 += cx; y1 += cy; x += cx; y += cy }
			ops = append(ops, svgPathOp{kind: 'Q', args: [7]float64{x1, y1, x, y}})
			cx, cy = x, y
			lastQuadCX, lastQuadCY = x1, y1
			hasLastQuad, hasLastCubic = true, false
		case 'T', 't':
			x, err := num(); if err != nil { return nil, err }
			y, _ := num()
			if curCmd == 't' { x += cx; y += cy }
			var x1, y1 float64
			if hasLastQuad {
				x1 = 2*cx - lastQuadCX
				y1 = 2*cy - lastQuadCY
			} else {
				x1, y1 = cx, cy
			}
			ops = append(ops, svgPathOp{kind: 'Q', args: [7]float64{x1, y1, x, y}})
			cx, cy = x, y
			lastQuadCX, lastQuadCY = x1, y1
			hasLastQuad, hasLastCubic = true, false
		case 'A', 'a':
			rx, _ := num(); ry, _ := num()
			xRot, _ := num()
			large, _ := num()
			sweep, _ := num()
			x, err := num(); if err != nil { return nil, err }
			y, _ := num()
			if curCmd == 'a' { x += cx; y += cy }
			beziers := decomposeArcToBeziers(cx, cy, x, y, rx, ry, xRot, large != 0, sweep != 0)
			ops = append(ops, beziers...)
			cx, cy = x, y
			hasLastCubic, hasLastQuad = false, false
		case 'Z', 'z':
			ops = append(ops, svgPathOp{kind: 'Z'})
			cx, cy = startX, startY
			hasLastCubic, hasLastQuad = false, false
		default:
			return nil, fmt.Errorf("unknown path command %c", curCmd)
		}
		if i >= len(tokens) { break }
	}
	return ops, nil
}
```

- [ ] **Step 5: Continue `svg_path.go` — arc decomposition**

```go
// decomposeArcToBeziers converts an SVG elliptical arc to 1-4 cubic Beziers
// per ISO/IEC 9075 Appendix F.6 / SVG implementation notes.
func decomposeArcToBeziers(x1, y1, x2, y2, rx, ry, xRotDeg float64, large, sweep bool) []svgPathOp {
	// Endpoint-to-center conversion per SVG impl notes.
	if rx == 0 || ry == 0 {
		return []svgPathOp{{kind: 'L', args: [7]float64{x2, y2}}}
	}
	rx = math.Abs(rx); ry = math.Abs(ry)
	xRot := xRotDeg * math.Pi / 180
	cosR, sinR := math.Cos(xRot), math.Sin(xRot)

	// Step 1: compute (x1', y1') — midpoint frame
	dx := (x1 - x2) / 2
	dy := (y1 - y2) / 2
	x1p := cosR*dx + sinR*dy
	y1p := -sinR*dx + cosR*dy

	// Step 2: ensure radii are large enough
	rxSq, rySq := rx*rx, ry*ry
	x1pSq, y1pSq := x1p*x1p, y1p*y1p
	lambda := x1pSq/rxSq + y1pSq/rySq
	if lambda > 1 {
		s := math.Sqrt(lambda)
		rx *= s; ry *= s
		rxSq, rySq = rx*rx, ry*ry
	}

	// Step 3: compute (cx', cy')
	sign := 1.0
	if large == sweep { sign = -1 }
	num := rxSq*rySq - rxSq*y1pSq - rySq*x1pSq
	if num < 0 { num = 0 }
	den := rxSq*y1pSq + rySq*x1pSq
	coef := sign * math.Sqrt(num/den)
	cxp := coef * (rx * y1p / ry)
	cyp := coef * (-ry * x1p / rx)

	// Step 4: compute (cx, cy)
	cx := cosR*cxp - sinR*cyp + (x1+x2)/2
	cy := sinR*cxp + cosR*cyp + (y1+y2)/2

	// Step 5: compute start and sweep angles
	angle := func(ux, uy, vx, vy float64) float64 {
		dot := ux*vx + uy*vy
		len := math.Hypot(ux, uy) * math.Hypot(vx, vy)
		a := math.Acos(math.Max(-1, math.Min(1, dot/len)))
		if ux*vy-uy*vx < 0 { a = -a }
		return a
	}
	startA := angle(1, 0, (x1p-cxp)/rx, (y1p-cyp)/ry)
	sweepA := angle((x1p-cxp)/rx, (y1p-cyp)/ry, (-x1p-cxp)/rx, (-y1p-cyp)/ry)
	if !sweep && sweepA > 0 { sweepA -= 2 * math.Pi }
	if sweep && sweepA < 0 { sweepA += 2 * math.Pi }

	// Step 6: decompose into segments ≤ π/2 each
	segs := int(math.Ceil(math.Abs(sweepA) / (math.Pi / 2)))
	if segs == 0 { segs = 1 }
	step := sweepA / float64(segs)
	out := make([]svgPathOp, 0, segs)
	a := startA
	for i := 0; i < segs; i++ {
		b := a + step
		// Cubic approximation of arc on unit circle
		t := math.Tan(step / 4)
		alpha := math.Sin(step) * (math.Sqrt(4+3*t*t) - 1) / 3
		ax, ay := math.Cos(a), math.Sin(a)
		bx, by := math.Cos(b), math.Sin(b)
		// Control points in unit circle
		c1x := ax - alpha*ay
		c1y := ay + alpha*ax
		c2x := bx + alpha*by
		c2y := by - alpha*bx
		// Transform back: scale by (rx, ry), rotate by xRot, translate by (cx, cy)
		toPDF := func(x, y float64) (float64, float64) {
			x *= rx; y *= ry
			rx2 := cosR*x - sinR*y
			ry2 := sinR*x + cosR*y
			return rx2 + cx, ry2 + cy
		}
		c1xt, c1yt := toPDF(c1x, c1y)
		c2xt, c2yt := toPDF(c2x, c2y)
		bxt, byt := toPDF(bx, by)
		out = append(out, svgPathOp{kind: 'C', args: [7]float64{c1xt, c1yt, c2xt, c2yt, bxt, byt}})
		a = b
	}
	return out
}
```

- [ ] **Step 6: Run all path tests**

```powershell
go test -run TestParseSVGPathData -v ./...
```

- [ ] **Step 7: Commit**

```bash
git add svg_path.go svg_path_test.go
git commit -m "feat: svg — parseSVGPathData (full SVG 1.1 syntax; H/V/S/T normalized; arc → cubic Bezier decomposition)"
```

---

## Task 6: viewBox + preserveAspectRatio matrix

**Files:**
- Create: `svg_viewbox.go`
- Create: `svg_viewbox_test.go`

- [ ] **Step 1: Write failing tests**

```go
// SPDX-License-Identifier: MIT

package asposepdf

import (
	"math"
	"testing"
)

func TestParseViewBox(t *testing.T) {
	vb, ok := parseViewBox("0 0 100 50")
	if !ok || vb.x != 0 || vb.y != 0 || vb.w != 100 || vb.h != 50 {
		t.Errorf("got %+v ok=%v", vb, ok)
	}
}

func TestParseViewBox_NegativeMin(t *testing.T) {
	vb, _ := parseViewBox("-10 -20 100 50")
	if vb.x != -10 || vb.y != -20 { t.Errorf("got %+v", vb) }
}

func TestParseViewBox_Malformed(t *testing.T) {
	_, ok := parseViewBox("0 0 100")
	if ok { t.Error("expected failure for 3 numbers") }
}

func TestParsePreserveAspect_Default(t *testing.T) {
	p := parsePreserveAspect("")
	if p.align != "xMidYMid" || p.meetOrSlice != "meet" {
		t.Errorf("default = %+v", p)
	}
}

func TestParsePreserveAspect_None(t *testing.T) {
	p := parsePreserveAspect("none")
	if p.align != "none" { t.Errorf("got %+v", p) }
}

func TestParsePreserveAspect_Slice(t *testing.T) {
	p := parsePreserveAspect("xMinYMin slice")
	if p.align != "xMinYMin" || p.meetOrSlice != "slice" {
		t.Errorf("got %+v", p)
	}
}

func TestComputeViewBoxMatrix_NoViewBox_IdentityWithYFlip(t *testing.T) {
	// No viewBox + no intrinsic size: just maps to rect with Y-flip.
	// Rectangle (0,0)-(100,50) (50 wide rect, 50pt tall).
	m := computeViewBoxMatrix(nil, 100, 50, svgPreserveAspect{}, Rectangle{LLX: 0, LLY: 0, URX: 100, URY: 50})
	// Identity with Y-flip and translate to top of rect:
	// SVG (0, 0) → PDF (0, 50); SVG (100, 50) → PDF (100, 0)
	// = matrix [1, 0, 0, -1, 0, 50]
	want := svgMatrix{1, 0, 0, -1, 0, 50}
	if !almostEqMatrix(m, want) {
		t.Errorf("got %v want %v", m, want)
	}
}

func TestComputeViewBoxMatrix_Meet_LetterboxX(t *testing.T) {
	// viewBox 0 0 100 50 (ratio 2:1), rect 200×200 (ratio 1:1)
	// meet → scale = min(2, 4) = 2; effective render = 200×100, centered vertically.
	// Pad on top = (200 - 100) / 2 = 50.
	// Y-flip then maps SVG (0,0) to PDF (0, top_pad + 100) = (0, 150).
	vb := &svgViewBox{0, 0, 100, 50}
	rect := Rectangle{LLX: 0, LLY: 0, URX: 200, URY: 200}
	m := computeViewBoxMatrix(vb, 0, 0, svgPreserveAspect{align: "xMidYMid", meetOrSlice: "meet"}, rect)
	// Expected: SVG point (0,0) → PDF (0, 150); SVG (100, 50) → PDF (200, 50)
	x0, y0 := transformPoint(m, 0, 0)
	x1, y1 := transformPoint(m, 100, 50)
	if math.Abs(x0) > 1e-9 || math.Abs(y0-150) > 1e-9 || math.Abs(x1-200) > 1e-9 || math.Abs(y1-50) > 1e-9 {
		t.Errorf("(0,0) → (%g, %g) want (0, 150); (100,50) → (%g, %g) want (200, 50)", x0, y0, x1, y1)
	}
}

func TestComputeViewBoxMatrix_None_Stretch(t *testing.T) {
	vb := &svgViewBox{0, 0, 100, 50}
	rect := Rectangle{LLX: 0, LLY: 0, URX: 200, URY: 200}
	m := computeViewBoxMatrix(vb, 0, 0, svgPreserveAspect{align: "none"}, rect)
	x1, y1 := transformPoint(m, 100, 50)
	// none → fills exactly: (100,50) → (200, 0)
	if math.Abs(x1-200) > 1e-9 || math.Abs(y1) > 1e-9 {
		t.Errorf("none: (100,50) → (%g, %g) want (200, 0)", x1, y1)
	}
}

func transformPoint(m svgMatrix, x, y float64) (float64, float64) {
	return m[0]*x + m[2]*y + m[4], m[1]*x + m[3]*y + m[5]
}

func almostEqMatrix(a, b svgMatrix) bool {
	for i := range a {
		if math.Abs(a[i]-b[i]) > 1e-9 { return false }
	}
	return true
}
```

- [ ] **Step 2: Run, observe failures**

```powershell
go test -run "TestParseViewBox|TestParsePreserveAspect|TestComputeViewBoxMatrix" -v ./...
```

- [ ] **Step 3: Create `svg_viewbox.go`**

```go
// SPDX-License-Identifier: MIT

package asposepdf

import (
	"strings"
)

func parseViewBox(s string) (svgViewBox, bool) {
	nums, ok := parseSVGNumberList(s)
	if !ok || len(nums) != 4 {
		return svgViewBox{}, false
	}
	return svgViewBox{nums[0], nums[1], nums[2], nums[3]}, true
}

func parsePreserveAspect(s string) svgPreserveAspect {
	s = strings.TrimSpace(s)
	if s == "" {
		return svgPreserveAspect{align: "xMidYMid", meetOrSlice: "meet"}
	}
	parts := strings.Fields(s)
	p := svgPreserveAspect{align: "xMidYMid", meetOrSlice: "meet"}
	if len(parts) >= 1 {
		p.align = parts[0]
	}
	if len(parts) >= 2 {
		p.meetOrSlice = parts[1]
	}
	return p
}

// computeViewBoxMatrix returns the CTM that maps SVG content (in viewBox units, Y-down)
// into the user-supplied Rectangle (PDF user space, Y-up).
//
// If viewBox is nil: uses intrinsicW × intrinsicH as the source space; if both are 0,
// uses the rectangle's own dimensions (1:1 mapping with Y-flip only).
func computeViewBoxMatrix(viewBox *svgViewBox, intrinsicW, intrinsicH float64, par svgPreserveAspect, rect Rectangle) svgMatrix {
	var srcX, srcY, srcW, srcH float64
	if viewBox != nil {
		srcX, srcY, srcW, srcH = viewBox.x, viewBox.y, viewBox.w, viewBox.h
	} else if intrinsicW > 0 && intrinsicH > 0 {
		srcW, srcH = intrinsicW, intrinsicH
	} else {
		srcW = rect.URX - rect.LLX
		srcH = rect.URY - rect.LLY
	}
	if srcW <= 0 || srcH <= 0 {
		return matrixIdentity()
	}

	dstW := rect.URX - rect.LLX
	dstH := rect.URY - rect.LLY

	var scaleX, scaleY float64
	if par.align == "none" {
		scaleX = dstW / srcW
		scaleY = dstH / srcH
	} else {
		sx := dstW / srcW
		sy := dstH / srcH
		var s float64
		if par.meetOrSlice == "slice" {
			s = math.Max(sx, sy)
		} else {
			s = math.Min(sx, sy)
		}
		scaleX, scaleY = s, s
	}

	// Effective rendered size in PDF points
	renderW := srcW * scaleX
	renderH := srcH * scaleY

	// Alignment offsets within rect
	var alignX, alignY float64
	switch {
	case strings.HasPrefix(par.align, "xMin"):
		alignX = 0
	case strings.HasPrefix(par.align, "xMax"):
		alignX = dstW - renderW
	default:
		alignX = (dstW - renderW) / 2
	}
	switch {
	case strings.HasSuffix(par.align, "YMin"):
		// "YMin" = top in SVG (y=0 at top in SVG; after Y-flip, that's URY in PDF).
		alignY = dstH - renderH
	case strings.HasSuffix(par.align, "YMax"):
		alignY = 0
	default:
		alignY = (dstH - renderH) / 2
	}

	// Composite matrix:
	//   1) translate by -srcX, -srcY (move viewBox origin to (0,0))
	//   2) scale by (scaleX, -scaleY) (Y-flip)
	//   3) translate to rect.LLX + alignX, rect.LLY + alignY + renderH (top-of-flipped)
	t1 := matrixTranslate(-srcX, -srcY)
	s := svgMatrix{scaleX, 0, 0, -scaleY, 0, 0}
	t2 := matrixTranslate(rect.LLX+alignX, rect.LLY+alignY+renderH)
	return matrixMul(t2, matrixMul(s, t1))
}
```

Wait — fix the import: this file needs `math`. Add the import.

```go
import (
	"math"
	"strings"
)
```

- [ ] **Step 4: Run viewBox tests**

```powershell
go test -run "TestParseViewBox|TestParsePreserveAspect|TestComputeViewBoxMatrix" -v ./...
```

- [ ] **Step 5: Commit**

```bash
git add svg_viewbox.go svg_viewbox_test.go
git commit -m "feat: svg — viewBox + preserveAspectRatio matrix mapping (10 modes + Y-flip)"
```

---

## Task 7: XML parser skeleton + basic shapes

**Files:**
- Create: `svg_parse.go`
- Create: `svg_parse_test.go`
- Create: `testdata/svg/rect.svg`, `testdata/svg/circle.svg`, `testdata/svg/all_shapes.svg`

- [ ] **Step 1: Create test fixtures**

`testdata/svg/rect.svg`:
```xml
<?xml version="1.0"?>
<svg xmlns="http://www.w3.org/2000/svg" width="100" height="50" viewBox="0 0 100 50">
  <rect x="10" y="10" width="80" height="30" fill="red"/>
</svg>
```

`testdata/svg/all_shapes.svg`:
```xml
<?xml version="1.0"?>
<svg xmlns="http://www.w3.org/2000/svg" width="200" height="200" viewBox="0 0 200 200">
  <rect x="10" y="10" width="50" height="40" fill="red"/>
  <circle cx="100" cy="50" r="30" fill="blue"/>
  <ellipse cx="150" cy="50" rx="20" ry="15" fill="green"/>
  <line x1="10" y1="100" x2="190" y2="100" stroke="black" stroke-width="2"/>
  <polyline points="10,150 30,130 50,150 70,130" stroke="purple" fill="none"/>
  <polygon points="100,150 130,200 70,200" fill="orange"/>
</svg>
```

- [ ] **Step 2: Write failing tests**

```go
// SPDX-License-Identifier: MIT

package asposepdf

import (
	"os"
	"testing"
)

func TestParseSVG_MinimalRect(t *testing.T) {
	data, err := os.ReadFile("testdata/svg/rect.svg")
	if err != nil { t.Fatal(err) }
	svg, err := parseSVGBytes(data)
	if err != nil { t.Fatal(err) }
	if svg.viewBox == nil || svg.viewBox.w != 100 || svg.viewBox.h != 50 {
		t.Errorf("viewBox = %+v", svg.viewBox)
	}
	if svg.width != 100 || svg.height != 50 {
		t.Errorf("intrinsic = %g × %g", svg.width, svg.height)
	}
	if svg.root == nil || len(svg.root.children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(svg.root.children))
	}
	r, ok := svg.root.children[0].(*svgRect)
	if !ok { t.Fatalf("expected *svgRect, got %T", svg.root.children[0]) }
	if r.x != 10 || r.y != 10 || r.w != 80 || r.h != 30 {
		t.Errorf("rect dims = %g,%g %g×%g", r.x, r.y, r.w, r.h)
	}
	if r.style.fill == nil || r.style.fill.R != 1 || r.style.fill.G != 0 || r.style.fill.B != 0 {
		t.Errorf("fill = %+v", r.style.fill)
	}
}

func TestParseSVG_AllShapes(t *testing.T) {
	data, _ := os.ReadFile("testdata/svg/all_shapes.svg")
	svg, err := parseSVGBytes(data)
	if err != nil { t.Fatal(err) }
	got := len(svg.root.children)
	if got != 6 {
		t.Fatalf("expected 6 shape children, got %d", got)
	}
	// Just verify types are right
	kinds := map[string]int{}
	for _, c := range svg.root.children {
		kinds[c.svgNodeKind()]++
	}
	for _, k := range []string{"rect", "circle", "ellipse", "line", "polyline", "polygon"} {
		if kinds[k] != 1 {
			t.Errorf("expected 1 %s, got %d", k, kinds[k])
		}
	}
}

func TestParseSVG_InvalidXML(t *testing.T) {
	_, err := parseSVGBytes([]byte("<svg><not-closed"))
	if err == nil { t.Error("expected error for malformed XML") }
}

func TestParseSVG_NotSVGRoot(t *testing.T) {
	_, err := parseSVGBytes([]byte("<html><body></body></html>"))
	if err == nil { t.Error("expected error for non-svg root") }
}

func TestParseSVG_NoViewBox(t *testing.T) {
	svg, err := parseSVGBytes([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="50" height="30"/>`))
	if err != nil { t.Fatal(err) }
	if svg.viewBox != nil { t.Errorf("viewBox should be nil") }
	if svg.width != 50 || svg.height != 30 { t.Errorf("intrinsic = %g × %g", svg.width, svg.height) }
}
```

- [ ] **Step 3: Create `svg_parse.go` — root + shape handlers**

```go
// SPDX-License-Identifier: MIT

package asposepdf

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

func parseSVGBytes(data []byte) (*SVG, error) {
	return parseSVGReader(strings.NewReader(string(data)))
}

func parseSVGReader(r io.Reader) (*SVG, error) {
	decoder := xml.NewDecoder(r)
	decoder.Strict = false // tolerate non-strict SVG
	for {
		tok, err := decoder.Token()
		if err == io.EOF { return nil, errors.New("svg: no <svg> root element") }
		if err != nil { return nil, fmt.Errorf("svg: xml parse: %w", err) }
		start, ok := tok.(xml.StartElement)
		if !ok { continue }
		if start.Name.Local != "svg" {
			return nil, fmt.Errorf("svg: expected <svg> root, got <%s>", start.Name.Local)
		}
		return parseSVGRoot(decoder, start)
	}
}

func parseSVGRoot(d *xml.Decoder, start xml.StartElement) (*SVG, error) {
	svg := &SVG{root: &svgGroup{style: defaultSVGStyle()}}
	for _, a := range start.Attr {
		switch a.Name.Local {
		case "width":
			if v, ok := parseSVGLength(a.Value); ok { svg.width = v }
		case "height":
			if v, ok := parseSVGLength(a.Value); ok { svg.height = v }
		case "viewBox":
			if vb, ok := parseViewBox(a.Value); ok { svg.viewBox = &vb }
		case "preserveAspectRatio":
			svg.par = parsePreserveAspect(a.Value)
		}
	}
	if svg.par == (svgPreserveAspect{}) {
		svg.par = parsePreserveAspect("")
	}
	// Apply presentation attrs from root to root group's style
	applySVGStyleAttrs(&svg.root.style, start.Attr)
	if err := parseSVGChildren(d, svg.root); err != nil {
		return nil, err
	}
	return svg, nil
}

// parseSVGChildren consumes children until matching </parent>.
func parseSVGChildren(d *xml.Decoder, parent *svgGroup) error {
	for {
		tok, err := d.Token()
		if err == io.EOF { return errors.New("svg: unexpected EOF") }
		if err != nil { return err }
		switch t := tok.(type) {
		case xml.EndElement:
			return nil
		case xml.StartElement:
			child, err := parseSVGElement(d, parent, t)
			if err != nil { return err }
			if child != nil {
				parent.children = append(parent.children, child)
			}
		}
	}
}

func parseSVGElement(d *xml.Decoder, parent *svgGroup, start xml.StartElement) (svgNode, error) {
	switch start.Name.Local {
	case "g":
		g := &svgGroup{style: parent.style}
		applySVGStyleAttrs(&g.style, start.Attr)
		for _, a := range start.Attr {
			if a.Name.Local == "transform" {
				if m, ok := parseSVGTransform(a.Value); ok && m != matrixIdentity() {
					g.transform = &m
				}
			}
		}
		if err := parseSVGChildren(d, g); err != nil { return nil, err }
		return g, nil
	case "rect":
		return parseSVGRect(d, parent, start)
	case "circle":
		return parseSVGCircle(d, parent, start)
	case "ellipse":
		return parseSVGEllipse(d, parent, start)
	case "line":
		return parseSVGLine(d, parent, start)
	case "polyline":
		return parseSVGPolyline(d, parent, start, false)
	case "polygon":
		return parseSVGPolyline(d, parent, start, true)
	case "path":
		return parseSVGPath(d, parent, start)
	default:
		// Skip unsupported element (text, image, linearGradient, etc.)
		if err := d.Skip(); err != nil { return nil, err }
		return nil, nil
	}
}

// applySVGStyleAttrs reads presentation attrs from xml attributes (and style="..." attr) and merges into s.
func applySVGStyleAttrs(s *svgStyle, attrs []xml.Attr) {
	for _, a := range attrs {
		applySingleSVGStyleProp(s, a.Name.Local, a.Value)
	}
	for _, a := range attrs {
		if a.Name.Local == "style" {
			for _, decl := range strings.Split(a.Value, ";") {
				kv := strings.SplitN(decl, ":", 2)
				if len(kv) != 2 { continue }
				applySingleSVGStyleProp(s, strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1]))
			}
		}
	}
}

func applySingleSVGStyleProp(s *svgStyle, prop, val string) {
	switch prop {
	case "fill":
		if c, ok := parseSVGColor(val); ok { s.fill = c }
	case "stroke":
		if c, ok := parseSVGColor(val); ok { s.stroke = c }
	case "stroke-width":
		if v, ok := parseSVGLength(val); ok { s.strokeWidth = v }
	case "opacity":
		if v, ok := parseSVGNumber(val); ok { s.opacity = clamp01(v) }
	case "fill-opacity":
		if v, ok := parseSVGNumber(val); ok { s.fillOpacity = clamp01(v) }
	case "stroke-opacity":
		if v, ok := parseSVGNumber(val); ok { s.strokeOpacity = clamp01(v) }
	case "fill-rule":
		v := strings.ToLower(strings.TrimSpace(val))
		if v == "nonzero" || v == "evenodd" { s.fillRule = v }
	case "display":
		s.display = !(strings.TrimSpace(val) == "none")
	case "visibility":
		s.display = !(strings.TrimSpace(val) == "hidden")
	case "stroke-linecap":
		switch strings.TrimSpace(val) {
		case "round": s.lineCap = LineCapRound
		case "square": s.lineCap = LineCapSquare
		default: s.lineCap = LineCapButt
		}
	case "stroke-linejoin":
		switch strings.TrimSpace(val) {
		case "round": s.lineJoin = LineJoinRound
		case "bevel": s.lineJoin = LineJoinBevel
		default: s.lineJoin = LineJoinMiter
		}
	case "stroke-dasharray":
		if val == "none" || val == "" { s.dashArray = nil; return }
		if nums, ok := parseSVGNumberList(val); ok { s.dashArray = nums }
	case "stroke-dashoffset":
		if v, ok := parseSVGLength(val); ok { s.dashOffset = v }
	case "stroke-miterlimit":
		if v, ok := parseSVGNumber(val); ok { s.miterLimit = v }
	}
}

func parseSVGRect(d *xml.Decoder, parent *svgGroup, start xml.StartElement) (svgNode, error) {
	r := &svgRect{style: parent.style}
	for _, a := range start.Attr {
		switch a.Name.Local {
		case "x": r.x, _ = parseSVGLength(a.Value)
		case "y": r.y, _ = parseSVGLength(a.Value)
		case "width": r.w, _ = parseSVGLength(a.Value)
		case "height": r.h, _ = parseSVGLength(a.Value)
		case "rx": r.rx, _ = parseSVGLength(a.Value)
		case "ry": r.ry, _ = parseSVGLength(a.Value)
		case "transform":
			if m, ok := parseSVGTransform(a.Value); ok && m != matrixIdentity() { r.transform = &m }
		}
	}
	applySVGStyleAttrs(&r.style, start.Attr)
	if err := d.Skip(); err != nil { return nil, err }
	return r, nil
}

func parseSVGCircle(d *xml.Decoder, parent *svgGroup, start xml.StartElement) (svgNode, error) {
	c := &svgCircle{style: parent.style}
	for _, a := range start.Attr {
		switch a.Name.Local {
		case "cx": c.cx, _ = parseSVGLength(a.Value)
		case "cy": c.cy, _ = parseSVGLength(a.Value)
		case "r": c.r, _ = parseSVGLength(a.Value)
		case "transform":
			if m, ok := parseSVGTransform(a.Value); ok && m != matrixIdentity() { c.transform = &m }
		}
	}
	applySVGStyleAttrs(&c.style, start.Attr)
	if err := d.Skip(); err != nil { return nil, err }
	return c, nil
}

func parseSVGEllipse(d *xml.Decoder, parent *svgGroup, start xml.StartElement) (svgNode, error) {
	e := &svgEllipse{style: parent.style}
	for _, a := range start.Attr {
		switch a.Name.Local {
		case "cx": e.cx, _ = parseSVGLength(a.Value)
		case "cy": e.cy, _ = parseSVGLength(a.Value)
		case "rx": e.rx, _ = parseSVGLength(a.Value)
		case "ry": e.ry, _ = parseSVGLength(a.Value)
		case "transform":
			if m, ok := parseSVGTransform(a.Value); ok && m != matrixIdentity() { e.transform = &m }
		}
	}
	applySVGStyleAttrs(&e.style, start.Attr)
	if err := d.Skip(); err != nil { return nil, err }
	return e, nil
}

func parseSVGLine(d *xml.Decoder, parent *svgGroup, start xml.StartElement) (svgNode, error) {
	l := &svgLine{style: parent.style}
	for _, a := range start.Attr {
		switch a.Name.Local {
		case "x1": l.x1, _ = parseSVGLength(a.Value)
		case "y1": l.y1, _ = parseSVGLength(a.Value)
		case "x2": l.x2, _ = parseSVGLength(a.Value)
		case "y2": l.y2, _ = parseSVGLength(a.Value)
		case "transform":
			if m, ok := parseSVGTransform(a.Value); ok && m != matrixIdentity() { l.transform = &m }
		}
	}
	applySVGStyleAttrs(&l.style, start.Attr)
	if err := d.Skip(); err != nil { return nil, err }
	return l, nil
}

func parseSVGPolyline(d *xml.Decoder, parent *svgGroup, start xml.StartElement, closed bool) (svgNode, error) {
	var points []Point
	var transform *svgMatrix
	for _, a := range start.Attr {
		switch a.Name.Local {
		case "points":
			nums, _ := parseSVGNumberList(a.Value)
			for i := 0; i+1 < len(nums); i += 2 {
				points = append(points, Point{X: nums[i], Y: nums[i+1]})
			}
		case "transform":
			if m, ok := parseSVGTransform(a.Value); ok && m != matrixIdentity() { transform = &m }
		}
	}
	if err := d.Skip(); err != nil { return nil, err }
	if closed {
		p := &svgPolygon{points: points, style: parent.style, transform: transform}
		applySVGStyleAttrs(&p.style, start.Attr)
		return p, nil
	}
	p := &svgPolyline{points: points, style: parent.style, transform: transform}
	applySVGStyleAttrs(&p.style, start.Attr)
	return p, nil
}

func parseSVGPath(d *xml.Decoder, parent *svgGroup, start xml.StartElement) (svgNode, error) {
	p := &svgPath{style: parent.style}
	for _, a := range start.Attr {
		switch a.Name.Local {
		case "d":
			ops, err := parseSVGPathData(a.Value)
			if err != nil {
				// best-effort: invalid path → skip
				_ = d.Skip()
				return nil, nil
			}
			p.commands = ops
		case "transform":
			if m, ok := parseSVGTransform(a.Value); ok && m != matrixIdentity() { p.transform = &m }
		}
	}
	applySVGStyleAttrs(&p.style, start.Attr)
	if err := d.Skip(); err != nil { return nil, err }
	return p, nil
}
```

- [ ] **Step 4: Run XML parser tests**

```powershell
go test -run TestParseSVG_ -v ./...
```

- [ ] **Step 5: Commit**

```bash
git add svg_parse.go svg_parse_test.go testdata/svg/rect.svg testdata/svg/all_shapes.svg
git commit -m "feat: svg — XML parser with all 7 basic shape types + style attr"
```

---

## Task 8: XML parser — group nesting + inheritance cascade

**Files:**
- Modify: `svg_parse_test.go`
- Create: `testdata/svg/nested_groups.svg`

- [ ] **Step 1: Create test fixture with nested groups**

`testdata/svg/nested_groups.svg`:
```xml
<?xml version="1.0"?>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
  <g fill="red" stroke="blue">
    <rect x="10" y="10" width="20" height="20"/>
    <g opacity="0.5">
      <circle cx="50" cy="50" r="10"/>
      <rect x="60" y="60" width="20" height="20" fill="green"/>
    </g>
  </g>
</svg>
```

- [ ] **Step 2: Add tests**

```go
func TestParseSVG_GroupInheritance(t *testing.T) {
	data, _ := os.ReadFile("testdata/svg/nested_groups.svg")
	svg, _ := parseSVGBytes(data)
	if len(svg.root.children) != 1 {
		t.Fatalf("expected 1 top-level group, got %d", len(svg.root.children))
	}
	outer, _ := svg.root.children[0].(*svgGroup)
	if outer.style.fill == nil || outer.style.fill.R != 1 {
		t.Errorf("outer fill = %+v, want red", outer.style.fill)
	}
	// First child of outer group: rect inherits fill=red stroke=blue
	r, _ := outer.children[0].(*svgRect)
	if r.style.fill.R != 1 || r.style.stroke.B != 1 {
		t.Errorf("rect inheritance failed: fill=%+v stroke=%+v", r.style.fill, r.style.stroke)
	}
	// Second child: nested group with opacity 0.5; inherits fill=red stroke=blue
	inner, _ := outer.children[1].(*svgGroup)
	if inner.style.opacity != 0.5 {
		t.Errorf("inner opacity = %g", inner.style.opacity)
	}
	if inner.style.fill.R != 1 {
		t.Errorf("inner inherited fill should be red")
	}
	// Third-level rect overrides fill to green but keeps stroke from grandparent
	innerRect, _ := inner.children[1].(*svgRect)
	if innerRect.style.fill.G != 1 || innerRect.style.fill.R != 0 {
		t.Errorf("rect override fill should be green, got %+v", innerRect.style.fill)
	}
	if innerRect.style.stroke == nil || innerRect.style.stroke.B != 1 {
		t.Errorf("rect should inherit stroke=blue, got %+v", innerRect.style.stroke)
	}
}
```

- [ ] **Step 3: Verify — already implemented in Task 7**

The cascade in Task 7's code already does:
1. Start with `parent.style` (copy of parent's resolved style)
2. Apply presentation attrs
3. Apply style="..." attr

Run test to confirm:
```powershell
go test -run TestParseSVG_GroupInheritance -v ./...
```

- [ ] **Step 4: Commit (only fixture + test added)**

```bash
git add svg_parse_test.go testdata/svg/nested_groups.svg
git commit -m "test: svg — group inheritance cascade across nested <g>"
```

---

## Task 9: XML parser — skip unsupported elements + namespace handling

**Files:**
- Modify: `svg_parse_test.go`
- Create: `testdata/svg/with_unsupported.svg`, `testdata/svg/with_namespaces.svg`

- [ ] **Step 1: Create fixtures**

`testdata/svg/with_unsupported.svg`:
```xml
<?xml version="1.0"?>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
  <defs>
    <linearGradient id="grad1"><stop offset="0" stop-color="red"/></linearGradient>
  </defs>
  <rect x="10" y="10" width="80" height="20" fill="url(#grad1)"/>
  <text x="10" y="60">Should be skipped</text>
  <image x="10" y="70" width="20" height="20" href="data:image/png;base64,..."/>
  <rect x="10" y="80" width="80" height="10" fill="blue"/>
</svg>
```

`testdata/svg/with_namespaces.svg`:
```xml
<?xml version="1.0"?>
<svg xmlns="http://www.w3.org/2000/svg" xmlns:inkscape="http://www.inkscape.org/namespaces/inkscape" viewBox="0 0 100 100">
  <rect x="10" y="10" width="80" height="80" fill="red" inkscape:label="my-rect"/>
</svg>
```

- [ ] **Step 2: Add tests**

```go
func TestParseSVG_SkipsUnsupportedElements(t *testing.T) {
	data, _ := os.ReadFile("testdata/svg/with_unsupported.svg")
	svg, err := parseSVGBytes(data)
	if err != nil { t.Fatal(err) }
	// defs, text, image skipped. Two rects remain.
	rects := 0
	for _, c := range svg.root.children {
		if c.svgNodeKind() == "rect" { rects++ }
	}
	if rects != 2 {
		t.Errorf("expected 2 rects, got %d (total children: %d)", rects, len(svg.root.children))
	}
}

func TestParseSVG_GradientRefFallbacksToFill(t *testing.T) {
	// fill="url(#grad1)" is unrecognized → parseSVGColor returns false →
	// rect keeps inherited (default) fill = black.
	data, _ := os.ReadFile("testdata/svg/with_unsupported.svg")
	svg, _ := parseSVGBytes(data)
	r0, _ := svg.root.children[0].(*svgRect)
	if r0.style.fill == nil || (r0.style.fill.R != 0 || r0.style.fill.G != 0 || r0.style.fill.B != 0) {
		t.Errorf("gradient-ref rect fill = %+v, want black fallback", r0.style.fill)
	}
}

func TestParseSVG_IgnoresForeignNamespaceAttrs(t *testing.T) {
	data, _ := os.ReadFile("testdata/svg/with_namespaces.svg")
	svg, err := parseSVGBytes(data)
	if err != nil { t.Fatal(err) }
	r, _ := svg.root.children[0].(*svgRect)
	if r == nil || r.style.fill == nil || r.style.fill.R != 1 {
		t.Errorf("inkscape namespace shouldn't break red fill: %+v", r)
	}
}
```

- [ ] **Step 3: No code changes needed**

Task 7's `parseSVGElement` already calls `d.Skip()` for unknown element names. Foreign namespace attrs are simply not matched by `applySingleSVGStyleProp`'s switch. Run tests to verify.

```powershell
go test -run "TestParseSVG_(SkipsUnsupportedElements|GradientRefFallbacksToFill|IgnoresForeignNamespaceAttrs)" -v ./...
```

- [ ] **Step 4: Commit**

```bash
git add svg_parse_test.go testdata/svg/with_unsupported.svg testdata/svg/with_namespaces.svg
git commit -m "test: svg — skip unsupported elements (defs/text/image), fallback fill on gradient ref, ignore foreign-namespace attrs"
```

---

## Task 10: XML parser — transform attribute on shapes + paths

**Files:**
- Modify: `svg_parse_test.go`
- Create: `testdata/svg/with_transforms.svg`

- [ ] **Step 1: Fixture**

`testdata/svg/with_transforms.svg`:
```xml
<?xml version="1.0"?>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
  <g transform="translate(10, 20)">
    <rect x="0" y="0" width="50" height="30" transform="rotate(45)"/>
  </g>
  <path d="M 0 0 L 100 100" transform="scale(2)"/>
</svg>
```

- [ ] **Step 2: Test**

```go
func TestParseSVG_TransformOnGroup(t *testing.T) {
	data, _ := os.ReadFile("testdata/svg/with_transforms.svg")
	svg, _ := parseSVGBytes(data)
	g, _ := svg.root.children[0].(*svgGroup)
	if g.transform == nil {
		t.Fatal("expected group to have transform")
	}
	if g.transform[4] != 10 || g.transform[5] != 20 {
		t.Errorf("translate(10,20) → %v", *g.transform)
	}
}

func TestParseSVG_TransformOnShape(t *testing.T) {
	data, _ := os.ReadFile("testdata/svg/with_transforms.svg")
	svg, _ := parseSVGBytes(data)
	g, _ := svg.root.children[0].(*svgGroup)
	r, _ := g.children[0].(*svgRect)
	if r.transform == nil {
		t.Fatal("expected rect to have own transform")
	}
}

func TestParseSVG_TransformOnPath(t *testing.T) {
	data, _ := os.ReadFile("testdata/svg/with_transforms.svg")
	svg, _ := parseSVGBytes(data)
	// path is 2nd child of root
	p, _ := svg.root.children[1].(*svgPath)
	if p.transform == nil {
		t.Fatal("expected path to have transform")
	}
	if p.transform[0] != 2 || p.transform[3] != 2 {
		t.Errorf("scale(2) → %v", *p.transform)
	}
}
```

- [ ] **Step 3: No code changes — transform handling already in Task 7**

Run:
```powershell
go test -run "TestParseSVG_TransformOn" -v ./...
```

- [ ] **Step 4: Commit**

```bash
git add svg_parse_test.go testdata/svg/with_transforms.svg
git commit -m "test: svg — transform attribute on <g>, shapes, and <path>"
```

---

## Task 11: Renderer skeleton — outer CTM + walker

**Files:**
- Create: `svg_render.go`
- Modify: `svg_parse_test.go` (or new `svg_render_test.go`)

This task focuses on the rendering scaffold. We don't yet emit shape operators (that's Task 12); we verify the outer `q ... Q` is emitted with the right CTM and the walker visits each child.

- [ ] **Step 1: Create `svg_render.go`**

```go
// SPDX-License-Identifier: MIT

package asposepdf

import (
	"bytes"
	"fmt"
)

// renderSVG renders a parsed *SVG into the given Page within rect.
// Best-effort: unsupported nodes are silently skipped (already filtered at parse time).
func renderSVG(p *Page, svg *SVG, rect Rectangle) error {
	if svg == nil || svg.root == nil {
		return nil
	}
	if rect.URX-rect.LLX <= 0 || rect.URY-rect.LLY <= 0 {
		return nil
	}

	var buf bytes.Buffer
	outer := computeViewBoxMatrix(svg.viewBox, svg.width, svg.height, svg.par, rect)

	buf.WriteString("q\n")
	writeCMOperator(&buf, outer)
	renderSVGNodes(&buf, p, svg.root.children, svg.root.style)
	buf.WriteString("Q\n")

	return p.appendToContentStream(buf.Bytes())
}

func writeCMOperator(buf *bytes.Buffer, m svgMatrix) {
	// PDF cm operator: a b c d e f cm
	fmt.Fprintf(buf, "%s %s %s %s %s %s cm\n",
		formatFloat(m[0]), formatFloat(m[1]),
		formatFloat(m[2]), formatFloat(m[3]),
		formatFloat(m[4]), formatFloat(m[5]))
}

func renderSVGNodes(buf *bytes.Buffer, p *Page, nodes []svgNode, parentStyle svgStyle) {
	for _, n := range nodes {
		renderSVGNode(buf, p, n, parentStyle)
	}
}

func renderSVGNode(buf *bytes.Buffer, p *Page, n svgNode, parentStyle svgStyle) {
	switch node := n.(type) {
	case *svgGroup:
		renderSVGGroup(buf, p, node)
	case *svgRect:
		renderSVGRect(buf, p, node)
	case *svgCircle:
		renderSVGCircle(buf, p, node)
	case *svgEllipse:
		renderSVGEllipse(buf, p, node)
	case *svgLine:
		renderSVGLine(buf, p, node)
	case *svgPolyline:
		renderSVGPolyline(buf, p, node)
	case *svgPolygon:
		renderSVGPolygon(buf, p, node)
	case *svgPath:
		renderSVGPath(buf, p, node)
	}
}

func renderSVGGroup(buf *bytes.Buffer, p *Page, g *svgGroup) {
	if !g.style.display { return }
	buf.WriteString("q\n")
	if g.transform != nil {
		writeCMOperator(buf, *g.transform)
	}
	// Group-level opacity becomes ExtGState for children
	applyGroupOpacity(buf, p, g.style)
	renderSVGNodes(buf, p, g.children, g.style)
	buf.WriteString("Q\n")
}

// Stubs — filled in by Task 12
func renderSVGRect(buf *bytes.Buffer, p *Page, r *svgRect)         {}
func renderSVGCircle(buf *bytes.Buffer, p *Page, c *svgCircle)     {}
func renderSVGEllipse(buf *bytes.Buffer, p *Page, e *svgEllipse)   {}
func renderSVGLine(buf *bytes.Buffer, p *Page, l *svgLine)         {}
func renderSVGPolyline(buf *bytes.Buffer, p *Page, pl *svgPolyline){}
func renderSVGPolygon(buf *bytes.Buffer, p *Page, pg *svgPolygon)  {}
func renderSVGPath(buf *bytes.Buffer, p *Page, sp *svgPath)        {}

func applyGroupOpacity(buf *bytes.Buffer, p *Page, s svgStyle) {
	if s.opacity < 1 {
		// Reuse Phase 1's ExtGState mechanism
		gsName := p.ensureExtGState(s.opacity, s.opacity)
		fmt.Fprintf(buf, "/%s gs\n", gsName)
	}
}
```

- [ ] **Step 2: Build to confirm scaffold compiles**

```powershell
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add svg_render.go
git commit -m "feat: svg — renderer scaffold (outer q/cm/Q, group walker, opacity via ExtGState)"
```

---

## Task 12: Renderer — basic shapes (rect, circle, ellipse, line, polyline, polygon)

**Files:**
- Modify: `svg_render.go`
- Create: `svg_render_test.go`

These render via Phase 1 internal helpers. We bypass the public `(*Page).DrawRectangle` etc. because they emit their own `q/Q` and we already pushed our own at the group level.

- [ ] **Step 1: Refactor Phase 1 to expose internal emit helpers**

The Phase 1 implementation has private `formatLineStyle` / `formatShapeStyle` / `pathOpsToOperators`. We need helpers that emit shape operators into a caller-provided buffer (not directly into content stream). Inspect `vector_draw.go` for the patterns; add internal `emitRectangle`, `emitCircle`, etc. helpers if needed.

If Phase 1 helpers already take an `*bytes.Buffer`, reuse them. Otherwise, extract the core operator generation into private functions like:

```go
// In vector_draw.go (or new vector_emit.go):
func emitRectangleToBuf(buf *bytes.Buffer, rect Rectangle, style ShapeStyle, page *Page) {
	// Emit graphics state + path + paint operators into buf (no q/Q wrapping)
}
```

Note for implementer: study `vector_draw.go` first; if existing functions already write to a `bytes.Buffer`, just call them. The point is that we emit into our SVG-renderer's buffer, not the page's content stream directly (so we can sandwich them between our own q/Q).

- [ ] **Step 2: Implement shape renderers**

Replace stubs in `svg_render.go`:

```go
func renderSVGRect(buf *bytes.Buffer, p *Page, r *svgRect) {
	if !r.style.display || r.w <= 0 || r.h <= 0 { return }
	buf.WriteString("q\n")
	if r.transform != nil { writeCMOperator(buf, *r.transform) }
	style := svgStyleToShapeStyle(r.style)
	rect := Rectangle{LLX: r.x, LLY: r.y, URX: r.x + r.w, URY: r.y + r.h}
	if r.rx > 0 || r.ry > 0 {
		// Rounded rect — reuse Phase 1 internal
		rr := r.rx
		if rr == 0 { rr = r.ry }
		emitRoundedRectangleToBuf(buf, p, rect, rr, style)
	} else {
		emitRectangleToBuf(buf, p, rect, style)
	}
	buf.WriteString("Q\n")
}

func renderSVGCircle(buf *bytes.Buffer, p *Page, c *svgCircle) {
	if !c.style.display || c.r <= 0 { return }
	buf.WriteString("q\n")
	if c.transform != nil { writeCMOperator(buf, *c.transform) }
	emitCircleToBuf(buf, p, Point{X: c.cx, Y: c.cy}, c.r, svgStyleToShapeStyle(c.style))
	buf.WriteString("Q\n")
}

func renderSVGEllipse(buf *bytes.Buffer, p *Page, e *svgEllipse) {
	if !e.style.display || e.rx <= 0 || e.ry <= 0 { return }
	buf.WriteString("q\n")
	if e.transform != nil { writeCMOperator(buf, *e.transform) }
	emitEllipseToBuf(buf, p, Point{X: e.cx, Y: e.cy}, e.rx, e.ry, svgStyleToShapeStyle(e.style))
	buf.WriteString("Q\n")
}

func renderSVGLine(buf *bytes.Buffer, p *Page, l *svgLine) {
	if !l.style.display { return }
	buf.WriteString("q\n")
	if l.transform != nil { writeCMOperator(buf, *l.transform) }
	emitLineToBuf(buf, p, Point{X: l.x1, Y: l.y1}, Point{X: l.x2, Y: l.y2}, svgStyleToLineStyle(l.style))
	buf.WriteString("Q\n")
}

func renderSVGPolyline(buf *bytes.Buffer, p *Page, pl *svgPolyline) {
	if !pl.style.display || len(pl.points) < 2 { return }
	buf.WriteString("q\n")
	if pl.transform != nil { writeCMOperator(buf, *pl.transform) }
	emitPolylineToBuf(buf, p, pl.points, svgStyleToLineStyle(pl.style))
	buf.WriteString("Q\n")
}

func renderSVGPolygon(buf *bytes.Buffer, p *Page, pg *svgPolygon) {
	if !pg.style.display || len(pg.points) < 3 { return }
	buf.WriteString("q\n")
	if pg.transform != nil { writeCMOperator(buf, *pg.transform) }
	emitPolygonToBuf(buf, p, pg.points, svgStyleToShapeStyle(pg.style))
	buf.WriteString("Q\n")
}

// svgStyleToShapeStyle maps the resolved SVG cascade into Phase 1's ShapeStyle.
func svgStyleToShapeStyle(s svgStyle) ShapeStyle {
	ss := ShapeStyle{LineStyle: svgStyleToLineStyle(s)}
	if s.fill != nil {
		c := *s.fill
		c.A *= s.fillOpacity
		ss.FillColor = &c
	}
	return ss
}

func svgStyleToLineStyle(s svgStyle) LineStyle {
	ls := LineStyle{
		Width:       s.strokeWidth,
		DashPattern: s.dashArray,
		DashPhase:   s.dashOffset,
		Cap:         s.lineCap,
		Join:        s.lineJoin,
		MiterLimit:  s.miterLimit,
	}
	if s.stroke != nil {
		c := *s.stroke
		c.A *= s.strokeOpacity
		ls.Color = &c
	} else {
		ls.Width = 0 // signal "no stroke"
	}
	return ls
}
```

- [ ] **Step 3: Implement `emitXxxToBuf` helpers**

Inspect Phase 1 implementations and extract the operator-emission code into buffer-accepting helpers. Either:
- (a) Refactor Phase 1 `DrawXxx` to call internal `emitXxxToBuf` helpers + the existing `q ... Q + appendToContentStream` wrapper, then SVG renderer calls the internal helper directly.
- (b) Add a parallel internal API alongside the public one.

(a) is cleaner. Touches `vector_draw.go`. Verify Phase 1 tests still pass.

- [ ] **Step 4: Write smoke test for shape rendering**

```go
// svg_render_test.go
func TestRenderSVG_BasicShapesProducesContentStream(t *testing.T) {
	data, _ := os.ReadFile("testdata/svg/all_shapes.svg")
	svg, err := parseSVGBytes(data)
	if err != nil { t.Fatal(err) }
	doc := NewDocumentFromFormat(PageFormatA4)
	page := doc.Page(1)
	if err := renderSVG(page, svg, Rectangle{LLX: 100, LLY: 600, URX: 300, URY: 800}); err != nil {
		t.Fatal(err)
	}
	stream := page.contentStreamBytes()
	for _, want := range []string{"q\n", "Q\n", "cm\n", "re\n"} { // q/Q wrapping + at least one rect operator
		if !bytes.Contains(stream, []byte(want)) {
			t.Errorf("expected %q in content stream", want)
		}
	}
}
```

- [ ] **Step 5: Run + iterate**

```powershell
go test -run "TestRenderSVG|TestVector" -v ./...
```

Phase 1 vector tests must still pass after refactor.

- [ ] **Step 6: Commit**

```bash
git add svg_render.go svg_render_test.go vector_draw.go
git commit -m "feat: svg — render basic shapes (rect/circle/ellipse/line/polyline/polygon) via Phase 1 helpers"
```

---

## Task 13: Renderer — paths (M/L/C/Q/Z + Phase 1 painting)

**Files:**
- Modify: `svg_render.go`
- Modify: `svg_render_test.go`

- [ ] **Step 1: Implement `renderSVGPath`**

```go
func renderSVGPath(buf *bytes.Buffer, p *Page, sp *svgPath) {
	if !sp.style.display || len(sp.commands) == 0 { return }
	buf.WriteString("q\n")
	if sp.transform != nil { writeCMOperator(buf, *sp.transform) }
	// Build a Phase 1 Path from svgPathOps for reuse of emitPathToBuf
	path := NewPath()
	for _, op := range sp.commands {
		switch op.kind {
		case 'M':
			path.MoveTo(op.args[0], op.args[1])
		case 'L':
			path.LineTo(op.args[0], op.args[1])
		case 'C':
			path.CurveTo(op.args[0], op.args[1], op.args[2], op.args[3], op.args[4], op.args[5])
		case 'Q':
			path.QuadTo(op.args[0], op.args[1], op.args[2], op.args[3])
		case 'Z':
			path.Close()
		}
	}
	emitPathToBuf(buf, p, path, svgStyleToShapeStyle(sp.style), sp.style.fillRule)
	buf.WriteString("Q\n")
}
```

`emitPathToBuf` takes an extra `fillRule` parameter — if `"evenodd"`, use `f*` / `B*` paint operators; else use `f` / `B`. Update Phase 1's path emission to take fill-rule param (currently it defaults to nonzero).

- [ ] **Step 2: Test with Aspose-style path**

```go
func TestRenderSVG_PathWithCubicBeziers(t *testing.T) {
	svg, _ := parseSVGBytes([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<path d="M 10 10 C 20 0 80 0 90 10 Z" fill="red"/>
	</svg>`))
	doc := NewDocumentFromFormat(PageFormatA4)
	page := doc.Page(1)
	if err := renderSVG(page, svg, Rectangle{LLX: 0, LLY: 0, URX: 200, URY: 200}); err != nil {
		t.Fatal(err)
	}
	stream := page.contentStreamBytes()
	// Expect M operator (m), C operator (c), Z (h)
	for _, want := range []string{"m\n", " c\n", "h\n"} {
		if !bytes.Contains(stream, []byte(want)) {
			t.Errorf("missing operator %q", want)
		}
	}
}
```

- [ ] **Step 3: Run + iterate**

```powershell
go test -run "TestRenderSVG_Path" -v ./...
```

- [ ] **Step 4: Commit**

```bash
git add svg_render.go svg_render_test.go vector_draw.go
git commit -m "feat: svg — render <path> via Phase 1 Path builder; fill-rule support (nonzero/evenodd)"
```

---

## Task 14: Renderer — group transform composition + style application

**Files:**
- Modify: `svg_render.go`
- Modify: `svg_render_test.go`

Verify nested `<g transform>` produces correct nested `q ... cm ... Q` blocks in PDF content stream. Verify opacity inheritance through groups.

- [ ] **Step 1: Add tests**

```go
func TestRenderSVG_NestedGroupTransforms(t *testing.T) {
	svg, _ := parseSVGBytes([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<g transform="translate(10,20)">
			<g transform="rotate(45)">
				<rect x="0" y="0" width="20" height="20" fill="red"/>
			</g>
		</g>
	</svg>`))
	doc := NewDocumentFromFormat(PageFormatA4)
	page := doc.Page(1)
	if err := renderSVG(page, svg, Rectangle{LLX: 0, LLY: 0, URX: 200, URY: 200}); err != nil { t.Fatal(err) }
	stream := page.contentStreamBytes()
	// Expect at least 3 q/Q pairs (outer + 2 groups) and 3 cm operators (outer viewBox, translate, rotate).
	qCount := bytes.Count(stream, []byte("q\n"))
	cmCount := bytes.Count(stream, []byte("cm\n"))
	if qCount < 3 {
		t.Errorf("q count = %d, want >= 3", qCount)
	}
	if cmCount < 3 {
		t.Errorf("cm count = %d, want >= 3", cmCount)
	}
}

func TestRenderSVG_GroupOpacity(t *testing.T) {
	svg, _ := parseSVGBytes([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
		<g opacity="0.5">
			<rect x="0" y="0" width="50" height="50" fill="red"/>
		</g>
	</svg>`))
	doc := NewDocumentFromFormat(PageFormatA4)
	page := doc.Page(1)
	_ = renderSVG(page, svg, Rectangle{LLX: 0, LLY: 0, URX: 200, URY: 200})
	stream := page.contentStreamBytes()
	if !bytes.Contains(stream, []byte("gs\n")) {
		t.Error("expected /GSx gs operator for group opacity")
	}
}
```

- [ ] **Step 2: Run + iterate (handlers from Task 11 already cover the structure)**

```powershell
go test -run "TestRenderSVG_(NestedGroupTransforms|GroupOpacity)" -v ./...
```

If failures, debug the writeCMOperator + applyGroupOpacity invocations.

- [ ] **Step 3: Commit**

```bash
git add svg_render_test.go
git commit -m "test: svg — nested group transforms produce nested q/cm/Q; opacity emits /GSx gs"
```

---

## Task 15: Public Page API — `AddSVG` / `AddSVGFromStream` / `AddSVGObject` + `SVG` inspectors

**Files:**
- Create: `svg.go`
- Modify: `svg_test.go` (external integration test file)

- [ ] **Step 1: Write failing external tests in `svg_test.go`**

```go
// SPDX-License-Identifier: MIT

package asposepdf_test

import (
	"bytes"
	"os"
	"testing"

	pdf "github.com/aspose-pdf-foss/aspose-pdf-foss-for-go"
)

func TestPage_AddSVG_FromPath(t *testing.T) {
	doc := pdf.NewDocumentFromFormat(pdf.PageFormatA4)
	page := doc.Page(1)
	if err := page.AddSVG("testdata/svg/all_shapes.svg", pdf.Rectangle{LLX: 50, LLY: 600, URX: 250, URY: 800}); err != nil {
		t.Fatal(err)
	}
	if err := doc.Save("result_files/TestPage_AddSVG_FromPath.pdf"); err != nil { t.Fatal(err) }
}

func TestPage_AddSVGFromStream(t *testing.T) {
	data, _ := os.ReadFile("testdata/svg/all_shapes.svg")
	doc := pdf.NewDocumentFromFormat(pdf.PageFormatA4)
	page := doc.Page(1)
	if err := page.AddSVGFromStream(bytes.NewReader(data), pdf.Rectangle{LLX: 50, LLY: 600, URX: 250, URY: 800}); err != nil {
		t.Fatal(err)
	}
}

func TestPage_AddSVGObject_PreParsedReuse(t *testing.T) {
	doc := pdf.NewDocumentFromFormat(pdf.PageFormatA4)
	svg, err := doc.LoadSVG("testdata/svg/all_shapes.svg")
	if err != nil { t.Fatal(err) }
	// Add to two pages — should not re-parse
	doc.AddBlankPageFromFormat(pdf.PageFormatA4)
	for i := 1; i <= 2; i++ {
		if err := doc.Page(i).AddSVGObject(svg, pdf.Rectangle{LLX: 50, LLY: 600, URX: 250, URY: 800}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSVG_Inspectors(t *testing.T) {
	doc := pdf.NewDocumentFromFormat(pdf.PageFormatA4)
	svg, _ := doc.LoadSVG("testdata/svg/rect.svg")
	w, h := svg.Size()
	if w != 100 || h != 50 {
		t.Errorf("Size() = (%g, %g), want (100, 50)", w, h)
	}
	vx, vy, vw, vh := svg.ViewBox()
	if vx != 0 || vy != 0 || vw != 100 || vh != 50 {
		t.Errorf("ViewBox() = (%g, %g, %g, %g)", vx, vy, vw, vh)
	}
}

func TestPage_AddSVG_InvalidXMLReturnsError(t *testing.T) {
	doc := pdf.NewDocumentFromFormat(pdf.PageFormatA4)
	page := doc.Page(1)
	tmp := "result_files/_invalid.svg"
	os.WriteFile(tmp, []byte("<svg><not-closed"), 0644)
	defer os.Remove(tmp)
	if err := page.AddSVG(tmp, pdf.Rectangle{LLX: 0, LLY: 0, URX: 100, URY: 100}); err == nil {
		t.Error("expected error for malformed XML")
	}
}
```

- [ ] **Step 2: Run, observe failures**

```powershell
go test -run "TestPage_AddSVG|TestSVG_Inspectors" -v ./...
```

- [ ] **Step 3: Create `svg.go`**

```go
// SPDX-License-Identifier: MIT

package asposepdf

import (
	"io"
	"os"
)

// AddSVG reads an SVG file and renders it into the given rectangle on the page.
// Unsupported elements (text, image, gradients, masks) are skipped silently per
// Phase 2 scope.
//
// Returns error only on XML parse failure, invalid numeric attributes, or I/O errors.
func (p *Page) AddSVG(path string, rect Rectangle) error {
	f, err := os.Open(path)
	if err != nil { return err }
	defer f.Close()
	return p.AddSVGFromStream(f, rect)
}

// AddSVGFromStream renders an SVG from any io.Reader.
func (p *Page) AddSVGFromStream(r io.Reader, rect Rectangle) error {
	svg, err := parseSVGReader(r)
	if err != nil { return err }
	return p.AddSVGObject(svg, rect)
}

// AddSVGObject renders a pre-parsed SVG into the given rectangle.
func (p *Page) AddSVGObject(svg *SVG, rect Rectangle) error {
	return renderSVG(p, svg, rect)
}

// ViewBox returns the viewBox attribute as (x, y, width, height).
// If no viewBox is set, returns (0, 0, intrinsicWidth, intrinsicHeight).
func (s *SVG) ViewBox() (x, y, w, h float64) {
	if s.viewBox != nil {
		return s.viewBox.x, s.viewBox.y, s.viewBox.w, s.viewBox.h
	}
	return 0, 0, s.width, s.height
}

// Size returns intrinsic width and height as parsed from <svg width=... height=...>.
// Returns (0, 0) if neither attribute is present.
func (s *SVG) Size() (width, height float64) {
	return s.width, s.height
}
```

- [ ] **Step 4: Run tests + iterate**

```powershell
go test -run "TestPage_AddSVG|TestSVG_Inspectors" -v ./...
```

- [ ] **Step 5: Commit**

```bash
git add svg.go svg_test.go
git commit -m "feat: svg — public (*Page).AddSVG/AddSVGFromStream/AddSVGObject + SVG.ViewBox/Size"
```

---

## Task 16: Public Document API — `LoadSVG`, `LoadSVGFromStream`, `AddSVGWatermark` variants

**Files:**
- Modify: `svg.go`
- Modify: `svg_test.go`

- [ ] **Step 1: Tests**

```go
func TestDocument_LoadSVG(t *testing.T) {
	doc := pdf.NewDocumentFromFormat(pdf.PageFormatA4)
	svg, err := doc.LoadSVG("testdata/svg/all_shapes.svg")
	if err != nil { t.Fatal(err) }
	if svg == nil { t.Fatal("nil SVG") }
}

func TestDocument_LoadSVGFromStream(t *testing.T) {
	data, _ := os.ReadFile("testdata/svg/all_shapes.svg")
	doc := pdf.NewDocumentFromFormat(pdf.PageFormatA4)
	svg, err := doc.LoadSVGFromStream(bytes.NewReader(data))
	if err != nil { t.Fatal(err) }
	if svg == nil { t.Fatal("nil") }
}

func TestDocument_AddSVGWatermark_AllPages(t *testing.T) {
	doc := pdf.NewDocumentFromFormat(pdf.PageFormatA4)
	doc.AddBlankPageFromFormat(pdf.PageFormatA4)
	doc.AddBlankPageFromFormat(pdf.PageFormatA4)
	if err := doc.AddSVGWatermark("testdata/aspose-logo.svg"); err != nil { t.Fatal(err) }
	// All 3 pages should have content streams
	for i := 1; i <= 3; i++ {
		if doc.Page(i) == nil { t.Errorf("page %d nil", i) }
	}
	doc.Save("result_files/TestDocument_AddSVGWatermark_AllPages.pdf")
}

func TestDocument_AddSVGWatermark_SelectPages(t *testing.T) {
	doc := pdf.NewDocumentFromFormat(pdf.PageFormatA4)
	doc.AddBlankPageFromFormat(pdf.PageFormatA4)
	doc.AddBlankPageFromFormat(pdf.PageFormatA4)
	if err := doc.AddSVGWatermark("testdata/aspose-logo.svg", 1, 3); err != nil { t.Fatal(err) }
}

func TestDocument_AddSVGObjectWatermark(t *testing.T) {
	doc := pdf.NewDocumentFromFormat(pdf.PageFormatA4)
	svg, _ := doc.LoadSVG("testdata/aspose-logo.svg")
	doc.AddBlankPageFromFormat(pdf.PageFormatA4)
	if err := doc.AddSVGObjectWatermark(svg); err != nil { t.Fatal(err) }
}
```

- [ ] **Step 2: Append to `svg.go`**

```go
// LoadSVG reads and parses an SVG file once.
func (d *Document) LoadSVG(path string) (*SVG, error) {
	f, err := os.Open(path)
	if err != nil { return nil, err }
	defer f.Close()
	return d.LoadSVGFromStream(f)
}

// LoadSVGFromStream is the io.Reader variant.
func (d *Document) LoadSVGFromStream(r io.Reader) (*SVG, error) {
	return parseSVGReader(r)
}

// AddSVGWatermark applies an SVG watermark to all pages (when pageNums is empty)
// or to the specified 1-based page numbers.
func (d *Document) AddSVGWatermark(path string, pageNums ...int) error {
	svg, err := d.LoadSVG(path)
	if err != nil { return err }
	return d.AddSVGObjectWatermark(svg, pageNums...)
}

// AddSVGWatermarkFromStream is the io.Reader variant.
func (d *Document) AddSVGWatermarkFromStream(r io.Reader, pageNums ...int) error {
	svg, err := d.LoadSVGFromStream(r)
	if err != nil { return err }
	return d.AddSVGObjectWatermark(svg, pageNums...)
}

// AddSVGObjectWatermark uses a pre-parsed *SVG.
func (d *Document) AddSVGObjectWatermark(svg *SVG, pageNums ...int) error {
	targets := pageNums
	if len(targets) == 0 {
		targets = make([]int, d.PageCount())
		for i := range targets { targets[i] = i + 1 }
	}
	for _, n := range targets {
		page := d.Page(n)
		if page == nil { continue }
		size := page.Size()
		rect := Rectangle{LLX: 0, LLY: 0, URX: size.Width, URY: size.Height}
		if err := page.AddSVGObject(svg, rect); err != nil { return err }
	}
	return nil
}
```

- [ ] **Step 3: Run + iterate**

```powershell
go test -run "TestDocument_(LoadSVG|AddSVGWatermark|AddSVGObjectWatermark)" -v ./...
```

- [ ] **Step 4: Commit**

```bash
git add svg.go svg_test.go
git commit -m "feat: svg — (*Document) LoadSVG/LoadSVGFromStream + AddSVGWatermark/AddSVGWatermarkFromStream/AddSVGObjectWatermark"
```

---

## Task 17: End-to-end Aspose logo test

**Files:**
- Modify: `svg_test.go`

- [ ] **Step 1: Test using `testdata/aspose-logo.svg` (already in repo from prior commit)**

```go
func TestAddSVG_AsposeLogoBlackTextRenders(t *testing.T) {
	doc := pdf.NewDocumentFromFormat(pdf.PageFormatA4)
	page := doc.Page(1)
	// White background already (blank A4); place logo near top-left
	if err := page.AddSVG("testdata/aspose-logo.svg", pdf.Rectangle{LLX: 50, LLY: 750, URX: 250, URY: 800}); err != nil {
		t.Fatal(err)
	}
	out := "result_files/TestAddSVG_AsposeLogoBlackTextRenders.pdf"
	if err := doc.Save(out); err != nil { t.Fatal(err) }

	// Sanity check: re-open and verify page count, structural integrity
	reopened, err := pdf.Open(out)
	if err != nil { t.Fatal(err) }
	if reopened.PageCount() != 1 { t.Errorf("page count = %d", reopened.PageCount()) }
	report := pdf.Validate(out)
	if !report.Valid {
		t.Errorf("validation failed: %+v", report.Issues)
	}
}

func TestAddSVG_AsposeLogoGradientShapesSkippedSilently(t *testing.T) {
	doc := pdf.NewDocumentFromFormat(pdf.PageFormatA4)
	page := doc.Page(1)
	// Must not error — gradient refs are silently skipped per Phase 2 scope
	if err := page.AddSVG("testdata/aspose-logo.svg", pdf.Rectangle{LLX: 0, LLY: 0, URX: 595, URY: 100}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddSVGWatermark_AsposeLogoOnEveryPage(t *testing.T) {
	doc := pdf.NewDocumentFromFormat(pdf.PageFormatA4)
	for i := 0; i < 4; i++ { doc.AddBlankPageFromFormat(pdf.PageFormatA4) }
	// Pre-load once, watermark all 5 pages
	svg, _ := doc.LoadSVG("testdata/aspose-logo.svg")
	if err := doc.AddSVGObjectWatermark(svg); err != nil { t.Fatal(err) }
	out := "result_files/TestAddSVGWatermark_AsposeLogoOnEveryPage.pdf"
	doc.Save(out)
	report := pdf.Validate(out)
	if !report.Valid {
		t.Errorf("validation failed: %+v", report.Issues)
	}
}
```

- [ ] **Step 2: Run**

```powershell
go test -run "TestAddSVG_(AsposeLogo|Watermark)" -v ./...
```

Open generated PDFs in a viewer to manually inspect: black "ASPOSE" should render, colored circles should not.

- [ ] **Step 3: Commit**

```bash
git add svg_test.go
git commit -m "test: svg — end-to-end Aspose logo (black text renders; gradient-filled shapes silently skipped) + watermark"
```

---

## Task 18: Encryption round-trip — AES-128 / AES-256 / RC4-128 with SVG

**Files:**
- Modify: `svg_test.go`

- [ ] **Step 1: Tests**

```go
func TestAddSVG_AES128Roundtrip(t *testing.T) {
	doc := pdf.NewDocumentFromFormat(pdf.PageFormatA4)
	doc.Page(1).AddSVG("testdata/svg/all_shapes.svg", pdf.Rectangle{LLX: 50, LLY: 600, URX: 250, URY: 800})
	doc.SetEncryption(pdf.EncryptionOptions{
		UserPassword: "u", OwnerPassword: "o",
		Algorithm: pdf.EncryptionAlgAES128,
	})
	out := "result_files/TestAddSVG_AES128Roundtrip.pdf"
	if err := doc.Save(out); err != nil { t.Fatal(err) }
	reopened, err := pdf.OpenWithPassword(out, "u")
	if err != nil { t.Fatal(err) }
	if reopened.PageCount() != 1 { t.Errorf("page count after roundtrip = %d", reopened.PageCount()) }
}

func TestAddSVG_AES256Roundtrip(t *testing.T) {
	doc := pdf.NewDocumentFromFormat(pdf.PageFormatA4)
	doc.Page(1).AddSVG("testdata/aspose-logo.svg", pdf.Rectangle{LLX: 50, LLY: 700, URX: 250, URY: 800})
	doc.SetEncryption(pdf.EncryptionOptions{
		UserPassword: "u", OwnerPassword: "o",
		Algorithm: pdf.EncryptionAlgAES256,
	})
	out := "result_files/TestAddSVG_AES256Roundtrip.pdf"
	doc.Save(out)
	if _, err := pdf.OpenWithPassword(out, "u"); err != nil { t.Fatal(err) }
}

func TestAddSVG_RC4Roundtrip(t *testing.T) {
	doc := pdf.NewDocumentFromFormat(pdf.PageFormatA4)
	doc.Page(1).AddSVG("testdata/svg/all_shapes.svg", pdf.Rectangle{LLX: 50, LLY: 600, URX: 250, URY: 800})
	doc.SetEncryption(pdf.EncryptionOptions{
		UserPassword: "u", OwnerPassword: "o",
		Algorithm: pdf.EncryptionAlgRC4_128,
	})
	out := "result_files/TestAddSVG_RC4Roundtrip.pdf"
	doc.Save(out)
	if _, err := pdf.OpenWithPassword(out, "u"); err != nil { t.Fatal(err) }
}
```

- [ ] **Step 2: Run**

```powershell
go test -run "TestAddSVG_(AES128|AES256|RC4)Roundtrip" -v ./...
```

- [ ] **Step 3: Commit**

```bash
git add svg_test.go
git commit -m "test: svg — AES-128/256 + RC4-128 encryption round-trip with SVG content"
```

---

## Task 19: Update `testdata/testfiles.json` (if needed) + register tests

**Files:**
- Modify: `testdata/testfiles.json`

Per project convention (CLAUDE.md), test files that load real PDF/SVG resources should be declared in `testdata/testfiles.json`. Our SVG fixtures are loaded via direct paths (`testdata/svg/...`), not the `testFile(t)` helper, so this task may be a no-op — verify by checking how the helpers work in `helpers_test.go`.

- [ ] **Step 1: Inspect existing `testdata/testfiles.json` and `helpers_test.go`**

```powershell
cat testdata/testfiles.json | head -30
```

If the `testFile(t)` helper is required, add entries; if direct paths are acceptable for our SVG fixtures, this task is empty (skip).

- [ ] **Step 2: Run full test suite**

```powershell
go test ./...
```

All tests must pass.

- [ ] **Step 3: Commit if any registration needed**

```bash
git add testdata/testfiles.json
git commit -m "test: register SVG fixtures in testfiles.json"
```

---

## Task 20: Documentation — CLAUDE.md + README.md updates

**Files:**
- Modify: `CLAUDE.md`
- Modify: `README.md`

- [ ] **Step 1: Update `CLAUDE.md`**

Under the existing "Public API" section, after the "**`vector.go` / `vector_draw.go`**" block, add a new block:

```markdown
**`svg.go` / `svg_parse.go` / `svg_render.go` / `svg_path.go` / `svg_transform.go` / `svg_viewbox.go` / `svg_attrs.go` / `svg_types.go` / `svg_named_colors.go`**
- `(*Page).AddSVG(path, rect)` — reads an SVG file and renders it into the given rectangle on the page
- `(*Page).AddSVGFromStream(r, rect)` — io.Reader variant
- `(*Page).AddSVGObject(svg *SVG, rect)` — renders a pre-parsed `*SVG`
- `(*Document).LoadSVG(path) (*SVG, error)` — parse once, reuse on many pages
- `(*Document).LoadSVGFromStream(r io.Reader) (*SVG, error)` — io.Reader variant
- `(*Document).AddSVGWatermark(path string, pageNums ...int) error` — watermark on all or selected pages (uses full MediaBox honoring SVG preserveAspectRatio)
- `(*Document).AddSVGWatermarkFromStream(r io.Reader, pageNums ...int) error` — io.Reader variant
- `(*Document).AddSVGObjectWatermark(svg *SVG, pageNums ...int) error` — pre-parsed watermark
- `SVG` — opaque pre-parsed type
- `(*SVG).ViewBox() (x, y, w, h float64)` — viewBox attribute or (0,0,intrinsicW,intrinsicH)
- `(*SVG).Size() (width, height float64)` — intrinsic dimensions
- Supported in Phase 2: basic shapes (rect/circle/ellipse/line/polyline/polygon/path), full SVG 1.1 path syntax including arcs, transforms (translate/rotate/scale/matrix/skewX/skewY), viewBox + all 10 preserveAspectRatio modes, presentation attrs + inline `style="..."`, hex/rgb/147 CSS named colors, absolute length units (px/pt/pc/mm/cm/in)
- Out of scope (Phase 3): `<text>`, `<image>`, gradients (`fill="url(#id)"` falls back to inherited color), `<defs>`/`<use>`, masks/clipPath, CSS `<style>` blocks, filters, em/ex/% units
- Best-effort rendering: unsupported elements are silently skipped; only XML/parse failures return errors
```

- [ ] **Step 2: Update `README.md`**

Find the "Features" / "Vector graphics" section. Add an "SVG embedding" sub-section:

```markdown
### SVG embedding

```go
// Embed an external SVG file into a page
doc, _ := pdf.Open("input.pdf")
page := doc.Page(1)
page.AddSVG("logo.svg", pdf.Rectangle{LLX: 50, LLY: 700, URX: 250, URY: 800})

// Pre-parse for reuse on many pages
svg, _ := doc.LoadSVG("watermark.svg")
for i := 1; i <= doc.PageCount(); i++ {
    doc.Page(i).AddSVGObject(svg, pdf.Rectangle{LLX: 0, LLY: 0, URX: 595, URY: 842})
}

// Or use the watermark helper (covers all pages with full-MediaBox)
doc.AddSVGWatermark("watermark.svg")
```

Supports: basic shapes, full SVG 1.1 path syntax (with arc decomposition), transforms,
viewBox + preserveAspectRatio (all modes), 147 CSS named colors, hex/rgb/rgba, absolute
length units. Unsupported in Phase 2 (skipped silently): `<text>`, `<image>`, gradients,
masks, CSS style blocks.
```

- [ ] **Step 3: Run full test suite + verify build**

```powershell
go build ./... && go test ./...
```

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md README.md
git commit -m "docs: vector graphics Phase 2 (SVG embedding) in CLAUDE.md and README"
```

---

## Task 21: Close beads ticket + final verification

**Files:** (none)

- [ ] **Step 1: Close beads ticket**

```powershell
bd update pdf-go-bu0 --status closed
```

- [ ] **Step 2: Run full test suite one final time**

```powershell
go test ./...
```

- [ ] **Step 3: Run `gofmt -s -l .` to keep Go Report Card grade**

```powershell
gofmt -s -l .
```

If anything listed, run `gofmt -s -w .` and commit:

```bash
git add -u
git commit -m "style: apply gofmt -s after Phase 2 implementation"
```

- [ ] **Step 4: Done — Phase 2 complete**

Phase 3 candidate items (deferred): `<text>` with font matching, gradients via PDF Type 2/3 shading patterns, `<image>` data-uri raster, `<defs>`/`<use>`, masks/clipPath, CSS `<style>` blocks + selectors, filters, em/ex/% units, `currentColor` with real cascade, `<a>` → PDF link annotation mapping.


