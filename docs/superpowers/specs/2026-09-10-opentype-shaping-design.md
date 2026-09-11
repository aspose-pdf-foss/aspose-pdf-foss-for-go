# OpenType Shaping (GSUB/GPOS) — Design

Date: 2026-09-10 · Epic: pdf-go-26u4 (beads) · Status: approved

RTL support, phase 3. Phase 1 was the bidi reordering (`bidi.go`, UAX #9);
phase 2 was Arabic contextual shaping through the Unicode Presentation
Forms-B block (`arabic_shape.go`). Both work purely on the logical string,
which is why a font that does *not* cover Forms-B — Amiri, Noto Naskh
Arabic, most modern Arabic faces — renders as disconnected isolated letters
today, and why harakat (vowel marks) sit at their default advance instead of
over their base letter.

## Goal

A real OpenType shaper: run the font's own GSUB/GPOS lookups over a text run
and emit *positioned glyph IDs*, so any OpenType font renders the way it was
designed. Pure Go, zero dependencies, no new public API — `AddText` simply
gets correct for scripts that need shaping.

## Why this is invasive

The text pipeline is codepoint-based end to end: `renderTextInBuilder`
wraps a `string`, reorders it into visual order, measures it rune by rune
(`widthFn`) and hands the whole line to `encodeFn`, which maps rune → GID
one at a time and emits one `Tj`. A shaper breaks all four assumptions:
glyphs are not runes (ligatures merge, decompositions split), advances come
from GPOS as well as `hmtx`, glyphs carry (x,y) placement offsets, and the
run must be shaped in *logical* order before it is reordered.

## Scope

**In:** GDEF (glyph classes, mark attachment classes, mark glyph sets),
GSUB lookup types 1–7, GPOS lookup types 1–9 (single, pair, cursive,
mark-to-base, mark-to-ligature, mark-to-mark, context, chain context,
extension), Coverage 1/2, ClassDef 1/2, LookupFlag skipping, feature plans
for Arabic and the default (Latin/Cyrillic/Greek) script.

**Out:** Device/VariationIndex tables (no variable-font instancing — we
never write one), the Indic/Khmer/Myanmar/Hangul reordering shapers,
vertical writing (GSUB `vert`/`vrt2`, GPOS `vkrn`), the AAT `morx` fallback,
and `usMaxContext`-driven optimisation. Unsupported constructs are skipped,
never fatal: a font that trips over something shapes as far as it got.

## Architecture

Four files, added beside the existing font code:

1. **`opentype.go`** — the table reader. `parseOTLayout(f *ttfFont)` fills a
   lazily-built `otLayout{gdef, gsub, gpos}` hung off `ttfFont` the way
   `cff` already is (`ttfFont.ot`, built on first shaping request, cached,
   `nil` when the font has neither table). Structures mirror the spec:
   `otScriptList`/`otFeature`/`otLookup{kind, flag, markFilter, subtables}`,
   `otCoverage` (binary-searched ranges or a glyph list), `otClassDef`,
   `otValueRecord`, `otAnchor`. Offsets are validated on read; a malformed
   subtable is dropped, not returned as an error.

2. **`shape.go`** — the engine. The buffer is
   `[]shapedGlyph{gid uint16, cluster int, advance, xOff, yOff int32,
   mask uint32, class otGlyphClass}` in font units. `shapeRun(f *ttfFont,
   text string, rtl bool) []shapedGlyph`:
   - map runes → GIDs through `cmap` (cluster = rune index),
   - assign per-glyph feature masks: Arabic joining state (reusing
     `arabicShapeTable`'s `joinsLeft`/`joinsRight`) selects exactly one of
     `isol`/`init`/`medi`/`fina` per glyph; every glyph carries the
     always-on features of the plan,
   - run the GSUB lookups of the plan's features in *lookup-list order*
     (the spec's ordering, not the feature order) over the buffer,
   - run the GPOS lookups the same way, then resolve cursive and mark
     attachment chains into final `xOff`/`yOff`.
   - Feature plan, Arabic: `ccmp locl isol init medi fina rlig calt liga
     mset` then `curs kern mark mkmk`. Default: `ccmp locl liga clig calt`
     then `kern mark mkmk`. Script tag chosen from the run's first strong
     character (`arab`, `hebr`, else `latn`/`DFLT`).
   - Lookup application shares one `otSequenceMatcher` (glyph skipping per
     LookupFlag + GDEF) between GSUB and GPOS, and one generic
     context/chain-context matcher parameterised by a "apply lookup N at
     index i" callback.

3. **`text_shape.go`** — the pipeline seam. `shapedLine` holds the
   positioned glyphs of one visual line plus its total width in points.
   `shapeVisualLine(f *ttfFont, line string, baseLevel int)` splits the line
   into bidi level runs (a new `bidiLevelRuns` helper beside
   `bidiVisualString`, from the levels `bidiResolve` already computes),
   shapes each run in logical order with its own direction, reverses the
   glyph sequence of RTL runs, and concatenates the runs in visual order.
   Emission writes one `TJ` per line: consecutive glyphs accumulate into a
   hex string, a glyph whose position differs from its natural advance
   contributes a kern number (thousandths of an em, positive = left), and a
   glyph with a vertical offset is bracketed by `<yOff> Ts … 0 Ts` (text
   rise leaves the pen untouched, so marks cost no advance bookkeeping).

4. **Wiring in `text_add.go`.** `renderTextInBuilder` type-asserts the
   resolved font: an `*embeddedFont` whose `ttf` has layout tables takes the
   shaped path (measure + emit), everything else keeps today's exact
   behaviour byte for byte. Glyphs reach the subsetter through the existing
   `embeddedFont.useGlyph`.

## Interaction with phase 2 (Forms-B)

`shapeArabic` must not run when GSUB will do the same job — double shaping
would map an already-substituted Forms-B glyph again. Rule: if the resolved
font is an `*embeddedFont` whose GSUB advertises the `arab` script with any
of `init`/`medi`/`fina`, the Forms-B pass is skipped and the joining forms
come from the font. Otherwise (Standard-14, Type3, a TTF without GSUB) the
phase-2 path stays exactly as it is. DejaVu Sans — the bundled test face —
covers both, so this rule is what the tests pin down.

## Wrapping and measurement

Word wrapping keeps using the rune-based `widthFn`: it runs before shaping
and its job is only to choose break points. Line *width* (for alignment and
for the underline/strikethrough rects) comes from the shaped advances, so
what is drawn and what is measured agree. Documented consequence: a line
whose ligatures shrink it may be wrapped slightly conservatively — the same
tradeoff every "shape after wrap" engine makes, and invisible at normal
sizes.

## As built (2026-09-11)

What the implementation added beyond the plan above, each forced by a
difference against HarfBuzz or by an extraction failure:

- **Normalization for the font** (`normalizeForFont`, `combining.go` +
  generated `normalize_data.go`): characters the font lacks are decomposed,
  marks are put in canonical order, and base + mark pairs recompose when the
  font has the precomposed glyph. Ordering uses HarfBuzz's *modified*
  combining classes for Hebrew and Arabic — the raw Unicode numbers put a
  sheva before a dagesh and a fatha before a shadda, the reverse of how
  fonts attach them.
- **Joining types from the UCD** (`arabic_joining.go`, generated): the
  Forms-B table only knows the letters Forms-B has glyphs for, so Persian
  and Urdu letters did not join.
- **`abvm`/`blwm` in every GPOS plan**: Noto Sans Arabic files its
  mark-to-base lookups under them.
- **Default ignorables** (ZWNJ, ZWJ, direction marks) end as zero-advance
  space glyphs, as in HarfBuzz.
- **Emission keeps plain text plain**: corrections below a thousandth of an
  em (rounding in `/W`) are carried forward, so unkerned lines stay a single
  `Tj`, byte-identical to the rune path.
- **Extraction.** A glyph's `/ToUnicode` entry can only say one thing.
  Substituted glyphs claim an entry for their text
  (`embeddedFont.claimGlyphText`), allowed to override presentation-form
  code points; a glyph already nominal for another character — Noto Naskh
  draws zain as its reh glyph plus a dot — and decomposition pieces and
  hidden joiners are wrapped in `/ActualText` spans instead. The reader
  learned multi-character destinations (`fontInfo.toUnicodeSeq`), which
  also fixes ligature extraction in third-party PDFs.

Verification: `uharfbuzz` differential harness, 54/54 strings identical
(glyph ids, advances, offsets) across DejaVu Sans, Amiri, Noto Naskh Arabic
and Noto Sans Arabic; MuPDF renders of the sample pages match ours; the
feature showcase's RTL block is pixel-identical before and after.

## Testing

- **Unit, on `testdata/DejaVuSans.ttf`** (GSUB `arab` init/medi/fina/rlig +
  `latn` liga; GPOS kern/mark/mkmk — it exercises lookup types 1, 3, 4, 6/2
  and GPOS 2/2, 4, 5, 6): table parsing invariants; Arabic joining forms
  match the Forms-B glyphs phase 2 produces (the two paths must agree on a
  font that supports both); `fi`/`fl` ligate; a kern pair moves; harakat get
  a non-zero `yOff`.
- **Golden emission**: the content stream of a shaped line is a `TJ` with
  the expected GIDs and kern numbers.
- **Visual**: render Arabic and Hebrew sample pages with Amiri and Noto
  Naskh Arabic (installed, not bundled — the test skips when absent) and
  check them by eye, plus a before/after of the showcase's Arabic block to
  prove no regression on the Forms-B path.
- **Corpus**: none — shaping is a write-side feature; the read side
  (extraction, rendering) is untouched.
