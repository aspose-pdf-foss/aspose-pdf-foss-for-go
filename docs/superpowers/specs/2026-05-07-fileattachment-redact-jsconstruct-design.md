# FileAttachment + Redact + JavaScript Construct — Design Spec

**Date:** 2026-05-07
**Epic:** `pdf-go-37n` (Annotations umbrella). Final tranche under that epic — Subepic 4: specialised types.
**Previous tranches:**
- Subepic 1 (Link + Highlight family + Actions) shipped 2026-05-05.
- /AP infrastructure + Subepic 3 (Square/Circle/Line/Ink drawing primitives) shipped 2026-05-06.
- Subepic 2 (Text/sticky-note + FreeText + Stamp) shipped 2026-05-07.

After this subepic, the umbrella `pdf-go-37n` epic can be closed.

## Goal

Ship the three remaining annotation-related deliverables under the umbrella:
- `FileAttachmentAnnotation` — embedded file with icon (Paperclip / Graph / PushPin / Tag).
- `RedactAnnotation` — full mark + apply (irreversible content removal: text glyphs, images, paths). The apply step is the largest piece of new content-stream rewriting code in the project.
- `NewJavaScriptAction(script)` — public constructor (parse-only since Subepic 1).

Users get the complete annotations API surface promised by ISO 32000-1 §12.5 (excluding the deferred Polygon / PolyLine and Sound / Movie / Screen / 3D types — out of scope for the umbrella).

## Non-goals

- Polygon and PolyLine annotations (separate future epic).
- Sound, Movie, Screen, 3D, RichMedia, PrinterMark, TrapNet, Watermark, Projection annotations.
- Embedded Files top-level API (separate epic `pdf-go-p1d`). FileAttachment embeds files locally; the broader API for non-annotation file attachments and document-level navigation through embedded files is deferred.
- Form-field aware redaction. If a form widget covers redacted text, the form's value is unaffected — caller must clear the field first.
- Annotation removal during apply (only Redact annotations themselves are removed; Highlights / Comments covering redacted regions are preserved).
- Rich-text overlay (/RC) for Redact /OverlayText.
- Redact apply atomicity — best-effort semantics; partial state on failure.

## Architecture

Three-bucket split: declarative API (annotation types), /AP rendering (mark mode visuals), apply machinery (the big new piece).

```
┌──────────────────────────────────────────────────────────────┐
│ Declarative API                                              │
│   annotation_fileattachment.go                               │
│      FileAttachmentAnnotation + FileAttachmentIcon enum      │
│      File embedding helpers (path/stream → /Filespec dict)   │
│   annotation_redact.go                                       │
│      RedactAnnotation + accessors                            │
│   annotation_action.go (modify)                              │
│      NewJavaScriptAction(script) public ctor                 │
└─────────────┬────────────────────────────────────────────────┘
              │
              ▼
┌──────────────────────────────────────────────────────────────┐
│ /AP rendering                                                │
│   appearance_redact.go                                       │
│      generateRedactAppearance — mark-mode visual:            │
│        QuadPoints fills with /IC + optional /OverlayText      │
│        preview                                               │
│   FileAttachment uses no /AP — viewers render the icon.      │
└──────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────┐
│ Redact apply machinery (new — content-stream rewriting)      │
│   redact_apply.go                                            │
│      (*Document).ApplyRedactions() error                     │
│      (*Document).ValidateRedactions() error                  │
│      Per-page orchestration: walk redact annotations, dispatch│
│      to text/image/path rewriters, emit overlay text, remove │
│      annotations from /Annots                                │
│   redact_apply_text.go                                       │
│      rewriteTextOperatorsInStream — BT/ET walker, glyph      │
│      position computation, Tj/TJ/'/" rewriting               │
│   redact_apply_image.go                                      │
│      rewriteImageOperatorsInStream — Do walker with CTM      │
│      tracking, bbox intersection, clip-path wrapping         │
│   redact_apply_path.go                                       │
│      rewritePathOperatorsInStream — path-construction walker,│
│      accumulated bbox, clip-path wrapping                    │
└──────────────────────────────────────────────────────────────┘
```

`FileAttachmentAnnotation` does not generate `/AP/N` (viewers render the icon themselves).
`RedactAnnotation` generates `/AP/N` for mark-mode display; after `ApplyRedactions`, the annotation is removed entirely along with the underlying content.

## Public API

### Common new types

```go
// FileAttachmentIcon names per ISO 32000-1 §12.5.6.15 Table 178.
type FileAttachmentIcon int

const (
    FileAttachmentIconUnknown FileAttachmentIcon = iota
    FileAttachmentIconGraph
    FileAttachmentIconPaperclip   // PDF default
    FileAttachmentIconPushPin
    FileAttachmentIconTag
)
```

### `AnnotationType` enum additions

```go
AnnotationTypeFileAttachment
AnnotationTypeRedact
```

### `FileAttachmentAnnotation`

```go
NewFileAttachmentAnnotation(page *Page, position Point) *FileAttachmentAnnotation

// Icon
.Icon() FileAttachmentIcon
.SetIcon(i FileAttachmentIcon)        // default Paperclip

// Embedded file
.SetFile(path string) error           // detects MIME from extension
.SetFileFromStream(r io.Reader, name string) error
.HasFile() bool

// Read-only file metadata after embedding
.FileName() string                    // displayed filename
.FileMIMEType() string                // /Subtype on /EmbeddedFile (e.g. "application/pdf")
.FileSize() int                       // bytes embedded
.FileBytes() []byte                   // returns a defensive copy of embedded data; nil if HasFile() == false
.FileDescription() string             // /F dict's /Desc entry
.SetFileDescription(s string)

// Inherited from annotationBase:
.Rect()/SetRect(r)                    // setter does NOT regenerate (no /AP)
.Color()/SetColor(c)                  // /C — icon color tint
.Title()/SetTitle(s)                  // /T
.Contents()/SetContents(s)            // /Contents — popup body text
.PageIndex()
.RegenerateAppearance()               // no-op (no /AP)
```

`NewFileAttachmentAnnotation` accepts a `Point` (not `Rectangle`). Constructor computes `/Rect = {x, y, x+24, y+24}` (Acrobat 24pt icon convention).

MIME type auto-detection from path extension uses `mime.TypeByExtension` (stdlib). For `SetFileFromStream`, caller supplies `name`; MIME is derived from the name's extension. Falls back to `application/octet-stream` for unknown extensions.

### `RedactAnnotation`

```go
NewRedactAnnotation(page *Page, rect Rectangle) *RedactAnnotation

// Quad regions (multiple disjoint quads possible)
.QuadPoints() []QuadPoint              // QuadPoint reused from Subepic 1
.SetQuadPoints(qp []QuadPoint)         // empty/nil → use /Rect as single quad

// Mark visual color
.InteriorColor() *Color                // /IC — fill color for mark + post-apply overlay
.SetInteriorColor(c *Color)            // nil = transparent (rare)

// Overlay text — replacement text shown after Apply
.OverlayText() string                  // /OverlayText
.SetOverlayText(s string)
.OverlayTextStyle() TextStyle          // reconstructed from /DA + /Q
.SetOverlayTextStyle(s TextStyle)
.RepeatOverlayText() bool              // /Repeat — true: tile overlay across region
.SetRepeatOverlayText(repeat bool)

// Inherited from drawingAnnotationBase:
.BorderWidth()/SetBorderWidth(w)
.BorderStyle()/SetBorderStyle(s)
.DashPattern()/SetDashPattern(p)
.SetRect(r)
.SetColor(c)                           // /C — border outline color before apply

// Inherited from annotationBase: Title, Contents, PageIndex
.RegenerateAppearance()
```

**Mark-mode visual** (`/AP/N` before Apply):
1. Each `/QuadPoints` region rendered as `/IC` semi-transparent fill (default 50% black).
2. Optional `/OverlayText` preview rendered centered in each quad with `/DA` appearance.
3. Border (if `/BS` set) drawn around `/Rect`.

### `(*Document).ApplyRedactions() error`

```go
// ApplyRedactions destructively removes content covered by every
// /Redact annotation in the document, then deletes those annotations.
//
// For each redacted region:
//   1. Text glyphs whose center point falls inside the region are
//      removed from the content stream operators (Tj/TJ/'/").
//   2. Image XObject invocations (Do) whose mapped bbox intersects:
//      - fully covered → operator removed.
//      - partially overlapping → wrapped in clip path so the image
//        renders only outside the region.
//   3. Path operators (m/l/c/re/h/S/f/B/...) whose accumulated bbox
//      intersects the region — same clip-or-remove treatment as images.
//   4. If OverlayText is set: replacement text is rendered in /IC
//      color, optionally tiled across the region (/Repeat).
//   5. If RepeatOverlayText is true: text tiles horizontally with
//      appropriate spacing.
//
// On error: redactions on already-processed pages remain applied
// (best-effort semantics — the document is in a partial state). Use
// ValidateRedactions() first to dry-run-check parseability.
//
// The redact annotations are removed from /Annots after successful
// per-page rewrite. Orphan XObjects are NOT auto-cleaned — caller
// should call doc.RemoveUnusedObjects() afterwards if desired.
//
// Other annotations (Highlight, Comment, Link, etc.) are NOT affected,
// even if their /Rect overlaps a redacted region. A Highlight covering
// redacted text remains as a colored rectangle pointing at empty
// content. Caller is responsible for cleanup if needed.
//
// Form widgets covering redacted regions retain their values (form
// state lives in /AcroForm/Fields, not in the page content stream).
// Caller must clear field values explicitly before ApplyRedactions
// for fully-secure redaction of form data.
func (d *Document) ApplyRedactions() error

// ValidateRedactions performs a dry-run parse of all content streams
// containing redactions. Returns nil if all streams parseable;
// otherwise returns an error describing the first unparseable page.
// Recommended pre-flight before ApplyRedactions for safety.
func (d *Document) ValidateRedactions() error
```

### `NewJavaScriptAction(script string) *JavaScriptAction`

Existing `JavaScriptAction` from Subepic 1 (parse-only) gains a public constructor:

```go
// NewJavaScriptAction builds a /JavaScript action carrying the given
// script. The action runs when its parent annotation is activated
// (clicked) by a viewer that supports JavaScript.
//
// SECURITY WARNING: Embedding JavaScript in a PDF can introduce
// security risks for recipients. Scripts execute in the viewer's
// JavaScript engine context with access to form fields, navigation,
// and viewer-specific APIs (varies by viewer). Use only with scripts
// you authored or audited; never embed user-supplied JS without
// careful review.
//
// JavaScript actions are commonly disabled by default in viewers
// (Acrobat: Preferences > Security > "Enable JavaScript") so behavior
// is not guaranteed across all rendering environments.
func NewJavaScriptAction(script string) *JavaScriptAction
```

The existing `(*JavaScriptAction).Script()` accessor and `encode()` interface method are unchanged.

### Public regeneration

- `FileAttachmentAnnotation.RegenerateAppearance()` — no-op (no /AP).
- `RedactAnnotation.RegenerateAppearance()` — rebuilds /AP/N from current properties (mark mode visual).

## Internal infrastructure

### File embedding for FileAttachment

`SetFile(path)` flow:
1. Open file, read bytes.
2. Detect MIME via `mime.TypeByExtension(filepath.Ext(path))`. Fallback `application/octet-stream`.
3. Allocate /EmbeddedFile stream (`pdfStream` with `/Type /EmbeddedFile`, `/Subtype mime`, `/Length`, raw bytes).
4. Allocate /Filespec dict (`/Type /Filespec`, `/F basename`, `/UF basename`, `/EF << /F <ref> /UF <ref> >>`, optional `/Desc`).
5. Wire annotation's `/FS` to point to the /Filespec ref.
6. Mark annotation's `/Subtype` = `/FileAttachment`, `/Name` = current icon.

`SetFileFromStream(r, name)` does the same but reads bytes from `io.Reader` and uses `name` for filename + MIME detection.

`FileBytes()` returns a copy of the /EmbeddedFile stream's `Data`. `FileName()` reads `/F` from /Filespec. `FileMIMEType()` reads `/Subtype` from /EmbeddedFile.

`HasFile()` returns true iff `/FS` is set and resolves to a /Filespec with /EF/F populated.

### Mark-mode rendering (`appearance_redact.go`)

```go
func generateRedactAppearance(a *RedactAnnotation) *pdfStream {
    rect := a.Rect()
    width := rect.URX - rect.LLX
    height := rect.URY - rect.LLY

    b := newAppearanceBuilder()
    resources := existingAPNResources(&a.annotationBase)
    if resources == nil {
        resources = pdfDict{}
    }

    // 1. Fill each quad region with /IC (default 50% black).
    fillColor := Color{R: 0, G: 0, B: 0, A: 0.5}
    if ic := a.InteriorColor(); ic != nil {
        fillColor = *ic
    }
    quads := a.QuadPoints()
    if len(quads) == 0 {
        // Default: full /Rect as single quad.
        quads = []QuadPoint{rectAsQuadPoint(rect)}
    }
    b.PushState()
    b.SetFillColorRGB(fillColor)
    for _, qp := range quads {
        // Translate quad page-space → BBox-local.
        emitQuadFill(b, qp, rect)
    }
    b.PopState()

    // 2. Optional /OverlayText preview.
    if overlay := a.OverlayText(); overlay != "" {
        style := a.OverlayTextStyle()
        // Center each quad's text via renderTextInBuilder.
        for _, qp := range quads {
            quadRect := localizeQuad(qp, rect)
            resolve := xobjectFontResolver(a.doc, resources)
            _ = renderTextInBuilder(b, resources, overlay, style, quadRect, resolve, "", "")
        }
    }

    // 3. Border (if /BS set, rare for Redact).
    if bw := a.BorderWidth(); bw > 0 {
        drawStandardRectBorder(b, width, height, a.BorderStyle(), bw, a.DashPattern(), a.Color())
    }

    return makeFormXObjectWithResources(b.Bytes(), Rectangle{URX: width, URY: height}, resources)
}
```

### Apply machinery (`redact_apply.go` + sub-files)

`(*Document).ApplyRedactions()` orchestration:

```go
func (d *Document) ApplyRedactions() error {
    for pageIdx, pageObj := range d.pages {
        page, _ := d.Page(pageIdx + 1)
        redacts := collectRedacts(page)
        if len(redacts) == 0 {
            continue
        }
        // Collect all redact regions (page-space) from all redact annotations on this page.
        regions := []QuadPoint{}
        for _, r := range redacts {
            regions = append(regions, r.QuadPoints()...)
        }
        // Get current page content stream(s) and font/image XObject map.
        contentBytes, err := page.contentStreamBytes()
        if err != nil {
            return fmt.Errorf("page %d: %w", pageIdx+1, err)
        }
        fontMap, err := page.fontResources()
        if err != nil {
            return fmt.Errorf("page %d fonts: %w", pageIdx+1, err)
        }

        // Apply each rewriter in order.
        rewritten, err := rewriteTextOperatorsInStream(contentBytes, regions, fontMap)
        if err != nil {
            return fmt.Errorf("page %d text: %w", pageIdx+1, err)
        }
        rewritten, err = rewriteImageOperatorsInStream(rewritten, regions)
        if err != nil {
            return fmt.Errorf("page %d image: %w", pageIdx+1, err)
        }
        rewritten, err = rewritePathOperatorsInStream(rewritten, regions)
        if err != nil {
            return fmt.Errorf("page %d path: %w", pageIdx+1, err)
        }

        // Append overlay text from each redact annotation.
        rewritten = append(rewritten, buildOverlayContent(redacts, fontMap, d)...)

        // Replace page content stream with rewritten bytes.
        if err := page.replaceContentStream(rewritten); err != nil {
            return fmt.Errorf("page %d replace: %w", pageIdx+1, err)
        }

        // Remove redact annotations from /Annots and doc.objects.
        for _, r := range redacts {
            page.Annotations().Delete(r)
        }
    }
    return nil
}
```

`ValidateRedactions()` runs the same loop but only verifies `parseContentStream` succeeds for each page with redactions; no content modification.

#### Text rewriter (`redact_apply_text.go`)

Walks BT/ET blocks tracking text matrix. For each Tj/TJ/'/" operator, computes glyph-by-glyph rectangles via existing `widthFn` callbacks. Glyphs whose center is inside any redact region are removed from the operand string. TJ kerning numbers preceding removed glyphs are also removed; intermediate kerning preserved for non-redacted runs.

Re-serializes the modified token stream as bytes, preserving non-text operators verbatim.

#### Image rewriter (`redact_apply_image.go`)

Walks content stream tracking CTM (`q`, `Q`, `cm` operators). On each `Do` operator:
- Compute image bbox in page-space: CTM * unit-square.
- Intersect with redact regions:
  - Fully outside → emit Do unchanged.
  - Fully inside → drop Do operator.
  - Partial → wrap in `q + clip path + Do + Q` with clip = page rect minus redact quads (even-odd fill rule).

#### Path rewriter (`redact_apply_path.go`)

Walks path-construction operators (`m`, `l`, `c`, `v`, `y`, `re`, `h`) accumulating bounding box. On paint operator (`S`, `s`, `f`, `F`, `f*`, `B`, `B*`, `b`, `b*`, `n`):
- Compute path bbox via accumulated points + CTM.
- Same clip-or-remove logic as image rewriter.
- Reset path accumulator after paint.

Simplification: bbox-level clipping rather than path-region geometric intersection. Visually correct (path remains intact, viewed through a "window" outside redact regions).

#### Overlay text rendering

After rewrite, for each redact annotation with `/OverlayText`:
1. Build content fragment via `appearanceBuilder`.
2. Fill each quad with `/IC` color (visual block).
3. Render `/OverlayText` per `/DA` style + `/Q` alignment via `renderTextInBuilder`.
4. If `/Repeat` true: tile horizontally with appropriate spacing.
5. Append fragment to page content stream.

### `parseAnnotation` extensions

```go
case "/FileAttachment":
    return parseFileAttachmentAnnotation(base)
case "/Redact":
    return parseRedactAnnotation(base)
```

## Property → dict mapping

| Property | PDF dict location | Type |
|---|---|---|
| FileAttachment icon | `/Name` | name (`/Graph`/`/Paperclip`/`/PushPin`/`/Tag`) |
| FileAttachment file ref | `/FS` | indirect ref to /Filespec dict |
| FileAttachment description | `/FS/Desc` | string |
| Redact /QuadPoints | `/QuadPoints` | array of 8 floats per quad |
| Redact /InteriorColor | `/IC` | 3-elem RGB array |
| Redact /OverlayText | `/OverlayText` | string |
| Redact overlay style | `/DA` (font/size/color) + `/Q` (alignment) | string + int |
| Redact /Repeat | `/Repeat` | bool |

`/EmbeddedFile` stream:
- `/Type` = `/EmbeddedFile`
- `/Subtype` = MIME (e.g. `/application#2Fpdf` — escaped slash).
- `/Length` = byte count.
- Stream data = file bytes.

`/Filespec` dict:
- `/Type` = `/Filespec`
- `/F` = filename (PDFDocEncoding string).
- `/UF` = filename (UTF-16BE).
- `/EF` = `<< /F <embedded ref> /UF <embedded ref> >>`.
- Optional `/Desc` = description string.

## Setter regeneration

`RedactAnnotation` follows existing pattern:
- Every property setter mutates dict + calls `regenerateAP()`.
- `regenerateAP` calls `setAppearanceN(&a.annotationBase, generateRedactAppearance(a))`.
- Mutate-in-place semantics (no XObject leaks).

`FileAttachmentAnnotation` setters skip regenerate (no /AP). Public `RegenerateAppearance()` is a no-op.

## Error handling

- Constructors panic on `nil page`.
- `SetFile` / `SetFileFromStream` return `error` on file-open / read failures.
- `ApplyRedactions` returns wrapped error on per-page parse failure (best-effort: prior pages stay redacted).
- `ValidateRedactions` returns first parse error or nil.
- All other setters infallible.

## Testing strategy

Four levels.

### Level 1: Internal helpers (`appearance_redact_internal_test.go`, `redact_apply_internal_test.go`)

- `generateRedactAppearance` operator counts (single quad, multi-quad, with/without /IC, with overlay preview).
- `rewriteTextOperatorsInStream` cases: single glyph removal, TJ kerning drop, multi-line preservation.
- `rewriteImageOperatorsInStream`: full-inside/full-outside/partial overlap.
- `rewritePathOperatorsInStream`: same dispositions for paths.
- CTM tracking with cm + scale.

~25 internal tests, ~400 lines.

### Level 2: Declarative API round-trips (`annotation_fileattachment_test.go`, `annotation_redact_test.go`, `annotation_action_test.go`)

- FileAttachment: round-trip Icon, all 4 icons, SetFile (PDF/JPEG/text), SetFileFromStream, FileBytes defensive copy, invalid-path error, nil-page panic, default icon=Paperclip, default 24pt rect.
- Redact: round-trip QuadPoints, /OverlayText, /Repeat, OverlayTextStyle, multi-quads, default /IC=black, setter regenerate, no-leak, nil-page panic.
- NewJavaScriptAction round-trip.

~25 declarative tests, ~500 lines.

### Level 3: Apply integration (`redact_apply_test.go`)

- Text removal end-to-end (ExtractText doesn't return redacted strings).
- Multi-occurrence redaction across pages.
- Non-redacted text preserved.
- Image removal (full + partial clip).
- Path removal (full + partial clip).
- Overlay text rendered after apply.
- /Repeat tiling.
- Multiple regions on a page.
- Apply on multi-page doc.
- Best-effort partial state on error.
- ValidateRedactions positive/negative.
- Apply on doc with no redactions = no-op.
- Coexistence with Link/Highlight (preserved).

~15 integration tests, ~600 lines.

### Level 4: External cross-check (manual, in final task)

pypdf script:
1. Build doc with FileAttachment + Redact (with OverlayText) + JS action.
2. Verify pypdf reads /Subtype + /FS + /QuadPoints + /OverlayText.
3. After ApplyRedactions: pypdf `extract_text()` does NOT contain redacted strings.
4. Optional visual check in Adobe / Foxit / Firefox — FileAttachment icon clickable, Redact applied permanently.

### Test fixtures

All production tests build from `NewDocument`. FileAttachment tests embed temp files (PDF / JPEG / text) via `os.CreateTemp`. Redact tests use `Page.AddText` + image embedding APIs to create content programmatically.

## Dependencies / impact on existing code

- `annotation.go`: `parseAnnotation` switch +2 cases; `AnnotationType` enum +2. ~6 lines.
- `annotation_action.go`: + `NewJavaScriptAction` ctor. ~10 lines.
- `appearance.go`: no changes (existing helpers reused).
- `appearance_builder.go`: no changes.
- `text_add.go`: no changes (renderTextInBuilder reused for overlay text).
- No changes to writer, parser, font subsystem, or any non-annotation subsystem.
- No changes to existing 14 annotation types from prior subepics.

## Risks

1. **Content-stream rewriting fragility.** Apply walks every operator in every page content stream; obscure variants (inline images via BI/ID/EI, shading patterns, transparency groups) might confuse the walker. Mitigation: `parseContentStream` already handles all standard operators (text-extraction proves this). For un-handled operators, walker passes through unchanged.

2. **Glyph position computation accuracy.** Text rewriter relies on existing `widthFn` from font subsystem. If a font has incorrect width metrics, glyph rectangles will be wrong, leading to under/over-redaction. Trust the existing extraction pipeline (used by `ExtractTextWithLayout`).

3. **Encrypted documents.** Apply must respect existing decrypt-on-read pipeline. The existing parser handles this transparently — no special case required.

4. **Atomic vs best-effort.** Best-effort apply means partial state on failure. Documented; `ValidateRedactions()` is the safety net.

5. **Form fields covering redacted regions.** Form values live in `/AcroForm/Fields`, NOT in the page content stream. Redact apply does not touch form data. Caller must clear fields explicitly. Documented.

6. **JavaScript action constructor security.** `NewJavaScriptAction` accepts arbitrary script strings. Documented warning. Project responsibility ends at construction; runtime execution is viewer-controlled.

7. **MIME type detection on Windows.** `mime.TypeByExtension` reads system MIME table. On Windows this comes from the registry — may differ from Unix. For robustness, ship a small fallback table for common extensions (`.pdf`, `.txt`, `.png`, `.jpg`, `.docx`, `.xlsx`, `.zip`).

## Out of scope (deferred to future epics)

- Polygon and PolyLine annotations (shape primitives — separate future epic).
- Sound, Movie, Screen, 3D, RichMedia (multimedia annotations — out of scope of this library's focus).
- Embedded Files top-level API (file attachments at document level, not annotation level — `pdf-go-p1d` epic).
- Form-field-aware redaction (clear field values in same call as ApplyRedactions).
- Atomic apply (rollback on error).
- Annotation removal during apply (cleanup of Highlight/Comment over redacted regions).
- Rich-text /OverlayText.
- Custom redact appearance via `SetAppearanceImage` (custom mark-mode visual).

## Aspose.PDF for .NET fidelity

API names and shapes mirror Aspose.PDF for .NET:
- `FileAttachmentAnnotation` matches `.NET FileAttachmentAnnotation`.
- `FileAttachmentIcon` enum values match `.NET FileIcon` enumeration members.
- `RedactionAnnotation` (.NET) → `RedactAnnotation` (Go convention — drop "tion" suffix for brevity, matches our other annotation type names).
- `OverlayText`, `Repeat`, `InteriorColor`, `QuadPoints` accessor names match .NET property names.
- `Document.RedactArea()` (.NET) → `Document.ApplyRedactions()` (clearer Go naming).
- `JavaScriptAction` constructor presence matches .NET.
