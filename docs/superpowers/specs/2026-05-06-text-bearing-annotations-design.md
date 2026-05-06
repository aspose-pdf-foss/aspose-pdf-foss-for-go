# Text-bearing Annotations — Design Spec

**Date:** 2026-05-06
**Epic:** `pdf-go-37n` (Annotations umbrella). Third tranche under that epic — Subepic 2 (text-bearing): Text/sticky-note + FreeText + Stamp.
**Previous tranches:**
- Subepic 1 (Link + Highlight family + Actions) shipped 2026-05-05.
- /AP infrastructure + Subepic 3 (Square/Circle/Line/Ink drawing primitives) shipped 2026-05-06.

## Goal

Ship the three text-bearing annotation types of the annotations epic: `TextAnnotation` (sticky note with named icons), `FreeTextAnnotation` (text drawn on the page), and `StampAnnotation` (predefined or custom image stamps). FreeText supports the full Acrobat-equivalent feature set: `/IT` intents (FreeText/Callout/Typewriter), `/BE` border effects (Cloudy), `/CL` callout lines with arrow tips, `/RD` inner text rectangles. Stamp supports all 14 ISO-defined predefined names with auto-generated `/AP` visuals AND optional custom image override.

Users get markup annotations that render correctly in any spec-conforming viewer without `/NeedAppearances=true`, identically to what Acrobat produces.

## Non-goals

- Subepic 4 types (FileAttachment, Redact, JavaScript-construct).
- Rich-text default style (`/DS` + `/RC`) — optional in spec, niche feature.
- `/AP/R` (rollover) and `/AP/D` (down) variants — only `/AP/N` (normal).
- Transparency / blend modes / patterns / gradients (same as prior subepics).
- Migration of existing annotations through Split/Extract/Append (no new behavior — existing recursive object collection in `doc.go` already handles it).

## Architecture

Five-file split mirroring the prior subepic pattern. Three public type files + two `appearance` rendering files; `appearance.go` (existing) keeps shared helpers from Subepic 3.

```
┌────────────────────────────────────────────────────────────┐
│ Public API                                                 │
│   annotation_text.go     — TextAnnotation + TextIcon       │
│   annotation_freetext.go — FreeTextAnnotation + intent +   │
│                            border effect + callout         │
│   annotation_stamp.go    — StampAnnotation + StampName +   │
│                            custom-image embedding          │
└──────────┬───────────────────────────────────┬─────────────┘
           │ FreeText: regenerateAP            │ Stamp: regenerateAP
           ▼                                   ▼
┌──────────────────────────────┐  ┌────────────────────────────┐
│ appearance_freetext.go       │  │ appearance_stamp.go        │
│   generateFreeTextAppearance │  │   generateStampAppearance  │
│   drawCloudyRectBorder       │  │   generatePredefinedStamp  │
│   drawCalloutLine            │  │   generateCustomImageStamp │
│   drawStandardRectBorder     │  │   stampVisualParams (14×)  │
│                              │  │   drawRoundedRect          │
└──────────────┬───────────────┘  └─────────────┬──────────────┘
               │                                │
               ▼                                ▼
┌────────────────────────────────────────────────────────────┐
│ Existing infrastructure                                    │
│   appearance_builder.go (operators)                        │
│   appearance.go (setAppearanceN, makeFormXObject,          │
│                  drawLineEnding, etc.)                     │
│   text_add.go (refactored — extracts                       │
│                renderTextInBuilder helper)                 │
│   image_add.go (createImageXObject — reused for Stamp      │
│                 custom image)                              │
└────────────────────────────────────────────────────────────┘
```

`TextAnnotation` does **not** generate `/AP/N` — viewers render the icon. `FreeTextAnnotation` and `StampAnnotation` both generate `/AP/N` so they render visibly in any spec-conforming viewer.

## Public API

### Common new types

```go
// TextIcon names per ISO 32000-1 §12.5.6.4 Table 172.
type TextIcon int
const (
    TextIconUnknown TextIcon = iota
    TextIconComment
    TextIconKey
    TextIconNote      // default
    TextIconHelp
    TextIconNewParagraph
    TextIconParagraph
    TextIconInsert
)

// FreeTextIntent per ISO 32000-1 §12.5.6.6 /IT entry.
type FreeTextIntent int
const (
    FreeTextIntentFreeText FreeTextIntent = iota // default — plain text in box
    FreeTextIntentCallout                         // /FreeTextCallout — text + /CL pointer
    FreeTextIntentTypewriter                      // /FreeTextTypeWriter — bare text, no border/bg
)

// BorderEffect per ISO 32000-1 §12.5.4 Table 167 /BE entry.
type BorderEffect int
const (
    BorderEffectNone   BorderEffect = iota // /S = /S (default — no effect)
    BorderEffectCloudy                      // /S = /C — wavy "cloud" border
)

// StampName — 14 predefined stamp names per ISO 32000-1 §12.5.6.13 Table 184
// + Unknown for non-spec / custom names.
type StampName int
const (
    StampNameUnknown StampName = iota
    StampNameApproved
    StampNameAsIs
    StampNameConfidential
    StampNameDepartmental
    StampNameDraft         // default
    StampNameExperimental
    StampNameExpired
    StampNameFinal
    StampNameForComment
    StampNameForPublicRelease
    StampNameNotApproved
    StampNameNotForPublicRelease
    StampNameSold
    StampNameTopSecret
)
```

### `AnnotationType` enum additions

```go
AnnotationTypeText
AnnotationTypeFreeText
AnnotationTypeStamp
```

### `TextAnnotation`

```go
NewTextAnnotation(page *Page, position Point) *TextAnnotation

.Icon() TextIcon
.SetIcon(t TextIcon)         // default TextIconNote

.Open() bool
.SetOpen(open bool)          // default false

// Inherited from annotationBase (NOT drawingAnnotationBase):
.Rect()/SetRect(r)           // setter does NOT regenerate (no /AP)
.Color()/SetColor(c)         // /C — icon color
.Title()/SetTitle(s)         // /T
.Contents()/SetContents(s)   // /Contents — note body text
.PageIndex()
```

`NewTextAnnotation` accepts a `Point` (not `Rectangle`). Constructor computes `/Rect = {x, y, x+24, y+24}` per Acrobat 24-pt sticky-note convention.

### `FreeTextAnnotation`

```go
NewFreeTextAnnotation(page *Page, rect Rectangle, contents string, style TextStyle) *FreeTextAnnotation

// Text body — overrides inherited SetContents to regenerate /AP
.Contents() string
.SetContents(s string)        // → /Contents + regenerate /AP

// Reusable text formatting via existing TextStyle struct
.TextStyle() TextStyle        // reconstructed from /DA + /Q + /BG
.SetTextStyle(s TextStyle)    // → /DA (Font/Size/Color) + /Q (HAlign) + /BG + regenerate /AP

// Intent
.Intent() FreeTextIntent
.SetIntent(i FreeTextIntent)  // default FreeTextIntentFreeText

// Callout (relevant only when Intent == FreeTextIntentCallout)
.CalloutPoints() []Point      // 2 or 3 points: [knee, endpoint] or [knee1, knee2, endpoint]
.SetCalloutPoints(pts []Point) // 2-3 points; auto-sets Intent to Callout
.EndLineEnding() LineEndingStyle    // /LE — arrow at endpoint
.SetEndLineEnding(s LineEndingStyle) // default LineEndingNone
.InnerRect() Rectangle        // /RD-derived inner text rect
.SetInnerRect(r Rectangle)    // explicit; auto-computes /RD

// Border effect
.BorderEffect() BorderEffect
.SetBorderEffect(e BorderEffect)
.BorderEffectIntensity() float64    // /BE/I — cloudiness 0-2
.SetBorderEffectIntensity(i float64) // default 1.0 when BorderEffectCloudy

// Inherited from drawingAnnotationBase (regen-aware):
.BorderWidth()/SetBorderWidth(w)
.BorderStyle()/SetBorderStyle(s)    // BorderSolid/Dashed/Beveled/Inset/Underline
.DashPattern()/SetDashPattern(p)
.SetRect(r)
.SetColor(c)                  // /C — border color (NOT text color, that's in style.Color)

// Inherited from annotationBase: Title, PageIndex
```

`TextStyle` mapping: `Font/Size/Color` → `/DA` string; `Background` → `/BG`; `HAlign` → `/Q`. Honored extras: `LineSpacing`, `Underline`, `Strikethrough`, `VAlign` (rendered in `/AP`). Ignored: `Rotation`, `Behind` (not applicable to FreeText).

### `StampAnnotation`

```go
NewStampAnnotation(page *Page, rect Rectangle, name StampName) *StampAnnotation

.Name() StampName
.SetName(n StampName)         // updates /Name; regenerates /AP

.RawName() string             // /Name as raw string ("/Approved", custom values)
.SetRawName(s string)         // escape hatch; sets Name() = StampNameUnknown

// Custom image override
.SetCustomImage(path string) error                // detects format from magic bytes (JPEG, PNG)
.SetCustomImageFromStream(r io.Reader) error
.ClearCustomImage()                               // revert to predefined-template visual
.HasCustomImage() bool

// Inherited from drawingAnnotationBase:
.BorderWidth()/SetBorderWidth(w)
.BorderStyle()/SetBorderStyle(s)
.DashPattern()/SetDashPattern(p)
.SetRect(r)
.SetColor(c)                  // /C — border color override

// Inherited: Title, Contents, PageIndex
```

### Public regeneration

All three types expose `RegenerateAppearance()`. For `TextAnnotation` it is a no-op (no `/AP`), present for API symmetry.

## Internal infrastructure

### `text_add.go` refactor

The current `Page.AddText(text, style, rect)` is monolithic. The refactor extracts:

```go
// renderTextInBuilder draws wrapped/aligned text into b. Font references
// are accumulated into resources["/Font"]; caller is responsible for
// ensuring /Resources is wired into whichever container the builder's
// bytes will end up in (page or XObject).
func renderTextInBuilder(
    b *appearanceBuilder,
    resources pdfDict,
    text string,
    style TextStyle,
    rect Rectangle,
) error
```

Pure helpers (`wrapText`, `breakWord`, `measureString`, font-width tables) stay as-is. `Page.AddText` becomes a thin wrapper:
1. Get/create page `/Resources` dict.
2. Build a fresh `appearanceBuilder`.
3. Call `renderTextInBuilder(builder, resources, text, style, rect)`.
4. `appendToContentStream` (existing) appends `builder.Bytes()` to the page content stream.

`generateFreeTextAppearance` and `generatePredefinedStamp` reuse the same helper, accumulating fonts into the XObject's own `/Resources` instead of the page's.

### `appearanceBuilder` extension

New method:
```go
func (ab *appearanceBuilder) DoXObject(name pdfName)  // emits "<name> Do\n"
```
Used by `generateCustomImageStamp` to invoke an Image XObject inside the Form XObject.

### `makeFormXObjectWithResources` helper

Variant of `makeFormXObject` accepting an explicit `/Resources` dict (`makeFormXObject` always emits empty `/Resources`). Both helpers stay; FreeText/Stamp use the variant.

```go
func makeFormXObjectWithResources(content []byte, bbox Rectangle, resources pdfDict) *pdfStream
```

### `generateFreeTextAppearance`

```go
func generateFreeTextAppearance(a *FreeTextAnnotation) *pdfStream {
    rect := a.Rect()
    width := rect.URX - rect.LLX
    height := rect.URY - rect.LLY
    style := a.TextStyle()
    intent := a.Intent()

    b := newAppearanceBuilder()
    resources := pdfDict{}

    // 1. Background fill (skip for typewriter)
    if intent != FreeTextIntentTypewriter && style.Background != nil {
        b.PushState()
        b.SetFillColorRGB(*style.Background)
        b.Rect(0, 0, width, height)
        b.Fill()
        b.PopState()
    }

    // 2. Border (skip for typewriter)
    if intent != FreeTextIntentTypewriter {
        if a.BorderEffect() == BorderEffectCloudy {
            drawCloudyRectBorder(b, width, height, a.BorderWidth(), a.Color(), a.BorderEffectIntensity())
        } else {
            drawStandardRectBorder(b, width, height,
                a.BorderStyle(), a.BorderWidth(), a.DashPattern(), a.Color())
        }
    }

    // 3. Text in inner rect (callout uses /RD; otherwise inset by border width)
    inner := freeTextInnerRect(a, width, height)
    renderTextInBuilder(b, resources, a.Contents(), style, inner)

    // 4. Callout line (only for callout intent)
    if intent == FreeTextIntentCallout {
        drawCalloutLine(b, a.CalloutPoints(), rect, a.BorderWidth(), a.Color(), a.EndLineEnding())
    }

    return makeFormXObjectWithResources(b.Bytes(), Rectangle{URX: width, URY: height}, resources)
}
```

### `drawCloudyRectBorder` (`/BE = /C`)

Acrobat-equivalent wavy border:
- 4 sides (bottom, right, top, left) each subdivided into segments of length `~10 × intensity × lineWidth`.
- Each segment renders as a half-circle arc protruding outward via 2 cubic Beziers (using kappa).
- Corners closed with quarter-arcs for smooth continuity.
- Final `Stroke` for the entire perimeter as one path.

`intensity` is the `/BE/I` value; default 1.0.

### `drawCalloutLine`

```go
// pts: 2 points (knee, endpoint) or 3 points (knee1, knee2, endpoint).
// Implicit start: midpoint of the inner rect's edge nearest to pts[0].
// Endpoint is where the arrow tip lands.
func drawCalloutLine(b *appearanceBuilder, pts []Point, rect Rectangle,
    lineWidth float64, color *Color, ending LineEndingStyle)
```

Reuses `drawLineEnding` from Subepic 3 (already covers all 10 line-ending styles).

### `/RD` and inner rect

`/RD = [left top right bottom]` — distances (in points) from the outer `/Rect` to the inner text rect. Default for callout: scaled by callout knee position (text rect on the side opposite the endpoint, taking ~2/3 of /Rect). For non-callout: defaults to `BorderWidth × 4` padding on all sides.

`SetInnerRect(r Rectangle)` accepts page-space coordinates; `/RD` is computed automatically.

### `generateStampAppearance`

```go
func generateStampAppearance(a *StampAnnotation) *pdfStream {
    if a.HasCustomImage() {
        return generateCustomImageStamp(a)
    }
    return generatePredefinedStamp(a)
}
```

**Custom image path** (`generateCustomImageStamp`): the image XObject was already embedded into `doc.objects` when `SetCustomImage` was called (via `createImageXObject` from `image_add.go`). The Form XObject `/AP/N` references it through `/Resources/XObject/Im0` and emits `<width> 0 0 <height> 0 0 cm /Im0 Do`.

**Predefined path** (`generatePredefinedStamp`): switch on `Name()` → `stampVisualParams(name)` returns `(primaryColor, fillColor, label string)`. Then:
1. Filled rounded rect (background): `drawRoundedRect` + `FillStroke`.
2. Centered uppercase label in `FontHelveticaBoldOblique`, scaled to fit `width - 12` margin, color = `primaryColor`.

### `stampVisualParams`

14-case switch:
```go
func stampVisualParams(n StampName) (primary, fill Color, label string)
```

Color scheme (all colors expressed as `Color{R, G, B, A: 1}`):
- **Green** (positive): `Approved`, `Final`, `ForPublicRelease` → primary `(0.13, 0.52, 0.13)`, fill `(0.85, 0.95, 0.85)`.
- **Red** (warning): `Confidential`, `Expired`, `NotApproved`, `NotForPublicRelease`, `TopSecret` → primary `(0.78, 0.13, 0.13)`, fill `(0.99, 0.85, 0.85)`.
- **Orange** (informational): `AsIs`, `Draft`, `Experimental`, `ForComment`, `Sold` → primary `(0.85, 0.55, 0.13)`, fill `(0.99, 0.92, 0.78)`.
- **Gray** (neutral): `Departmental` → primary `(0.40, 0.40, 0.40)`, fill `(0.92, 0.92, 0.92)`.

Labels are the spec name in uppercase: `Approved → "APPROVED"`, `ForPublicRelease → "FOR PUBLIC RELEASE"`, etc.

### `drawRoundedRect`

```go
func drawRoundedRect(b *appearanceBuilder, x, y, w, h, radius float64)
```

Standard 4-corner construction: `m + 4 quarter-arcs (cubic Beziers) + 4 line segments + h`. Corner radius clamped to `min(w/2, h/2)`.

### Custom-image embedding

`SetCustomImage(path)` flow:
1. Open file, read first 4 bytes for magic (FFD8FF for JPEG, 89504E47 for PNG).
2. Reuse `createImageXObject(path, ...)` from `image_add.go` — already handles JPEG/PNG decoding and XObject construction.
3. Allocate `objID`, store XObject in `doc.objects`.
4. Save `objID` on `StampAnnotation` (private field `customImageObjID int`; default 0 means "no custom image").
5. Call `regenerateAP`.

`SetCustomImageFromStream(r)` is the streaming variant — reads bytes via `io.ReadAll`, then same flow.

`ClearCustomImage()`: zeroes `customImageObjID`. The orphaned XObject becomes garbage for `RemoveUnusedObjects`. Calls `regenerateAP`.

`HasCustomImage()`: `customImageObjID != 0`.

### Pre-Add behavior

Constructors set `doc: page.doc`, so `regenerateAP` runs on first setter call before `Add`. For `FreeTextAnnotation` and `StampAnnotation`, `/AP/N` is always present from construction. For `TextAnnotation`, no `/AP` is generated (no-op).

## Property → dict mapping

| Property | PDF dict location | Notes |
|---|---|---|
| TextIcon | `/Name` | name code per Table 172 |
| Open (TextAnnotation) | `/Open` | bool |
| FreeText Contents | `/Contents` | text string |
| FreeText DA (font + size + text color) | `/DA` | string like `/Helv 12 Tf 0 0 0 rg` |
| FreeText alignment (HAlign) | `/Q` | int 0/1/2 (Left/Center/Right) |
| FreeText background | `/BG` | 3-elem RGB array |
| FreeText intent | `/IT` | name `/FreeText`, `/FreeTextCallout`, `/FreeTextTypeWriter` |
| FreeText callout points | `/CL` | array of 4 or 6 numbers |
| FreeText callout end line ending | `/LE` | name |
| FreeText inner rect | `/RD` | array of 4 numbers (left top right bottom offsets) |
| FreeText border effect | `/BE/S` | name `/S` or `/C` |
| FreeText border effect intensity | `/BE/I` | number 0-2 |
| Stamp name | `/Name` | name like `/Approved`, `/Confidential` |
| Stamp custom image | XObject in `/AP/N`'s `/Resources/XObject` | Image XObject ref |

Border-related entries (`/BS`, `/Border`, `/C`) inherited from `drawingAnnotationBase` for `FreeTextAnnotation` and `StampAnnotation` (Square/Circle/Line/Ink already use the same).

## Setter regeneration

`FreeTextAnnotation` and `StampAnnotation` follow the existing pattern from Subepic 3:
- Every property setter mutates the dict and calls `regenerateAP()`.
- `regenerateAP` calls `setAppearanceN(&a.annotationBase, generateXxxAppearance(a))`.
- Mutate-in-place semantics (no XObject leaks from repeated setters).

`TextAnnotation` setters skip regenerate (no `/AP`). `RegenerateAppearance()` exists as no-op for API symmetry across all three types.

## Error handling

- Constructors panic on `nil page` (matches existing pattern).
- `SetCustomImage` / `SetCustomImageFromStream` return `error` on file-open / read / format-detect failures.
- All other setters are infallible.
- `Add` propagates the existing `AnnotationCollection.Add` error (cross-page re-attach error).
- `parseAnnotation` falls through to `GenericAnnotation` for malformed dicts.

## Testing strategy

Four levels.

### Level 1: Internal helpers (`appearance_text_internal_test.go`)

- `TestStampVisualParams` — table-driven over all 14 names, verify primary/fill/label triples.
- `TestDrawRoundedRect` — golden-byte for a 100×50 rect, radius 5.
- `TestDrawCloudyRectBorder` — operator-counts (lots of `c`'s, no straight `l`'s on edges).
- `TestDrawCalloutLine` — 2-pt and 3-pt cases produce m + l + l/l + S + ending shape.
- `TestRenderTextInBuilderBasic` — "Hello" in 100×50 produces BT/Tf/Tj/ET + populates `resources["/Font"]`.
- `TestRenderTextInBuilderVAlignTop`, `TestRenderTextInBuilderVAlignMiddle`, `TestRenderTextInBuilderVAlignBottom` — different baseline Y positions.

### Level 2: Generators (in same internal test file)

- `TestGenerateFreeTextBasic` — plain FreeText with /BG and /BS: fill + border + text in proper sequence.
- `TestGenerateFreeTextTypewriter` — no border, no background, just text.
- `TestGenerateFreeTextCallout` — text inside inner rect + callout line.
- `TestGenerateFreeTextCloudyBorder` — border has many `c`'s instead of plain rect.
- `TestGenerateStampPredefinedApproved` — green primary color in stroke ops.
- `TestGenerateStampCustomImage` — `/Im0 Do` operator + XObject in resources.

### Level 3: End-to-end round-trip (`annotation_text_test.go`, `annotation_freetext_test.go`, `annotation_stamp_test.go`)

**TextAnnotation:**
- `TestTextAnnotationRoundTrip` — Icon + Open + Title + Contents.
- `TestTextAnnotationAllIcons` — table-driven over 7 icon values.
- `TestTextAnnotationOpenStateRoundTrip`.
- `TestTextAnnotationConstructorPanicOnNilPage`.

**FreeTextAnnotation:**
- `TestFreeTextAnnotationBasicRoundTrip` — Contents + TextStyle + /BS + /BG.
- `TestFreeTextAnnotationTypewriterIntent` — /IT round-trip.
- `TestFreeTextAnnotationCalloutRoundTrip` — 2-pt with end ending.
- `TestFreeTextAnnotationCallout3Point` — 3-pt callout.
- `TestFreeTextAnnotationCloudyBorder` — /BE round-trip.
- `TestFreeTextAnnotationVAlignRoundTrip` — table-driven Top/Middle/Bottom.
- `TestFreeTextAnnotationSetterRegenerate` — multi-mutation; last value wins.
- `TestFreeTextAnnotationNoXObjectLeak` — `RemoveUnusedObjects` returns 0 after 5 setters.
- `TestFreeTextAnnotationParseExisting` — open existing FreeText fixture.

**StampAnnotation:**
- `TestStampAnnotationAllPredefinedNames` — table-driven over 14 names.
- `TestStampAnnotationCustomImageJPEG` and `TestStampAnnotationCustomImagePNG`.
- `TestStampAnnotationCustomImageFromStream`.
- `TestStampAnnotationClearCustomImage` — reverts to predefined visual.
- `TestStampAnnotationHasCustomImage`.
- `TestStampAnnotationRawNameEscape` — non-spec name round-trip.
- `TestStampAnnotationCustomImageInvalidFormat` — error on bogus file.

### Level 4: Cross-cutting integration (`annotation_text_integration_test.go`)

- `TestSubepic2FilterByType` — 3 new types + filter loop.
- `TestSubepic2RegenerateAppearance` — public API works on all three.
- `TestSubepic2CoexistsWithSubepic1And3` — full mix on one page → save → reopen → all classify correctly.
- `TestSubepic2DoNotBreakRemoveUnusedObjects` — Stamp custom image + delete annotation → image XObject becomes orphan → `RemoveUnusedObjects` removes it.

### Level 5: External viewer cross-check (final task, manual)

pypdf script verifies 4 fixture annotations (Text, FreeText basic, Stamp predefined, Stamp custom-image): `/Subtype` correctness + `/AP` presence + (for custom Stamp) `/XObject` in `/AP/N`'s `/Resources`. Visual confirmation in Adobe Reader, SumatraPDF, Foxit, Firefox/Chrome PDF viewer.

### Test fixtures

- All production tests build documents from `NewDocument` — no testdata PDFs needed.
- Stamp custom-image tests need a tiny JPEG and PNG. We can generate them programmatically in test setup (`bytes.Buffer` + `image/jpeg` / `image/png` from Go stdlib) to avoid fixture files.

## Dependencies / impact on existing code

- `annotation.go`: `parseAnnotation` switch +3 cases; `AnnotationType` enum +3 constants. ~10 lines.
- `text_add.go`: extract `renderTextInBuilder`; `Page.AddText` becomes a thin wrapper. Net ~20 lines added; ~80 lines refactored.
- `appearance_builder.go`: add `DoXObject` method (~5 lines).
- `appearance.go`: add `makeFormXObjectWithResources` helper (~15 lines).
- `image_add.go`: no behavior changes; existing `createImageXObject` reused by Stamp.
- `CLAUDE.md`, `README.md`: updated in final task.
- No changes to writer, parser, font subsystem, or any non-annotation subsystem.
- No changes to existing 11 annotation types from prior subepics (Link/Highlight/Underline/StrikeOut/Squiggly/Widget/Generic + Square/Circle/Line/Ink).

## Risks

1. **`text_add.go` refactor changes byte-level output of `Page.AddText`.** Wrapper assembles in a buffer first, then writes. Functionally equivalent; byte-level may differ in operator order or whitespace. Existing AddText tests pass currently — refactor must keep them passing. If a test depends on exact bytes (rare), update the golden expectation. Mitigation: refactor early in the plan, run full suite, fix any breakage before adding new types.

2. **Cloudy border math is sensitive.** Acrobat's algorithm isn't published precisely; close approximation is what we ship. Visual fidelity is "close to Acrobat", not pixel-identical. Documented in code.

3. **Predefined stamp visuals don't match Acrobat exactly.** Documented as "library-default visuals"; users wanting Acrobat-exact appearance use `SetCustomImage` with their own PNG.

4. **Callout `/RD` semantics.** Spec: `/RD = [left top right bottom]` distances from `/Rect` to inner rect. `SetInnerRect` computes this correctly assuming inner ⊂ outer. If caller passes `inner` outside `outer`, math produces negative `/RD` entries which some viewers may reject. Validate or clamp at `SetInnerRect` time.

5. **Custom image JPEG with embedded color profiles or unusual chroma subsampling.** `image_add.go` already handles common JPEG forms; oddball variants may produce viewer-rendering glitches. Same risk as existing `Page.AddImage` — accepted.

6. **VAlign in FreeText is a deviation from spec convention.** Spec says FreeText is top-aligned; we honor user's `VAlign`. Documented; round-trip preserves the visual via the rendered `/AP`. Other viewers re-rendering `/NeedAppearances` would default to top-aligned — but our `/AP` is always present, so viewers use it.

## Out of scope (deferred)

- Polygon (`/Polygon`) and PolyLine (`/PolyLine`) subtypes — share callout-line geometry with FreeText callout; future subepic.
- Rich-text annotations (`/RC` + `/DS`) — niche, deferred.
- Stamp `/Name` round-trip preservation when value is non-spec custom string AND `RawName()` was used — works (`SetRawName(s)` writes the literal name); but pypdf may not surface non-spec stamps in its UI. Spec-correct behavior, viewer limitation.
- `/AP/R` and `/AP/D` — niche.
- FreeText with embedded images — out of scope; user can compose Page.AddImage + FreeText separately.

## Aspose.PDF for .NET fidelity

API names and shapes mirror Aspose.PDF for .NET:
- `TextAnnotation`, `FreeTextAnnotation`, `StampAnnotation` match the .NET class names.
- `TextIcon` enum values match the .NET `TextIcon` enumeration members.
- `BorderEffect` enum (None/Cloudy) matches .NET `BorderEffectStyle`.
- `StampName` enum members match .NET predefined names.
- `SetCustomImage(path)` matches .NET `Image` property semantics.
- `Intent`, `CalloutPoints`, `EndLineEnding` match .NET `Intent`, `CalloutLine`, `EndingStyle` properties.
