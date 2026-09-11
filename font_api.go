// SPDX-License-Identifier: MIT

package asposepdf

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Font is implemented by standard 14 fonts and embedded TTF fonts.
// Use the package-level vars (FontHelvetica, ...) or LoadFont to obtain a Font.
type Font interface {
	// BaseFont returns the PostScript name, e.g. "Helvetica" or "ArialMT".
	BaseFont() string
	// IsEmbedded reports whether font data is embedded in the PDF (true for TTF, false for standard 14).
	IsEmbedded() bool
}

// standardFont is the built-in Font implementation for the 14 standard PDF fonts.
type standardFont struct {
	name string // PostScript name without leading slash, e.g. "Helvetica"
}

func (s standardFont) BaseFont() string { return s.name }
func (s standardFont) IsEmbedded() bool { return false }

// Standard 14 PDF fonts. These Fonts need not be embedded — every PDF viewer
// is required to render them.
var (
	FontHelvetica            Font = standardFont{name: "Helvetica"}
	FontHelveticaBold        Font = standardFont{name: "Helvetica-Bold"}
	FontHelveticaOblique     Font = standardFont{name: "Helvetica-Oblique"}
	FontHelveticaBoldOblique Font = standardFont{name: "Helvetica-BoldOblique"}
	FontTimesRoman           Font = standardFont{name: "Times-Roman"}
	FontTimesBold            Font = standardFont{name: "Times-Bold"}
	FontTimesItalic          Font = standardFont{name: "Times-Italic"}
	FontTimesBoldItalic      Font = standardFont{name: "Times-BoldItalic"}
	FontCourier              Font = standardFont{name: "Courier"}
	FontCourierBold          Font = standardFont{name: "Courier-Bold"}
	FontCourierOblique       Font = standardFont{name: "Courier-Oblique"}
	FontCourierBoldOblique   Font = standardFont{name: "Courier-BoldOblique"}
	FontSymbol               Font = standardFont{name: "Symbol"}
	FontZapfDingbats         Font = standardFont{name: "ZapfDingbats"}
)

// standardFontIndex maps the canonical lower-case PostScript name to a standardFont.
var standardFontIndex = map[string]Font{
	"helvetica":             FontHelvetica,
	"helvetica-bold":        FontHelveticaBold,
	"helvetica-oblique":     FontHelveticaOblique,
	"helvetica-boldoblique": FontHelveticaBoldOblique,
	"times-roman":           FontTimesRoman,
	"times-bold":            FontTimesBold,
	"times-italic":          FontTimesItalic,
	"times-bolditalic":      FontTimesBoldItalic,
	"courier":               FontCourier,
	"courier-bold":          FontCourierBold,
	"courier-oblique":       FontCourierOblique,
	"courier-boldoblique":   FontCourierBoldOblique,
	"symbol":                FontSymbol,
	"zapfdingbats":          FontZapfDingbats,
}

// FindFont returns a standard 14 Font by PostScript name. The lookup is
// case-insensitive. Returns an error if the name is not a standard 14 name.
func FindFont(name string) (Font, error) {
	if f, ok := standardFontIndex[strings.ToLower(name)]; ok {
		return f, nil
	}
	return nil, fmt.Errorf("find font: unknown standard font %q", name)
}

// embeddedFont is a TTF loaded into a specific Document.
type embeddedFont struct {
	doc          *Document
	ttf          *ttfFont
	baseFont     string          // PostScript name cache
	fontObjectID int             // ID of the Type0 font dict in doc.objects
	usedGlyphs   map[uint16]bool // glyph IDs actually emitted to a content stream

	// glyphText names the glyphs shaping substituted (ligatures, contextual
	// forms), so /ToUnicode maps them back to the characters they stand for;
	// nominal marks the glyphs the cmap assigns to an ordinary character,
	// whose mapping must not be overridden; textDirty says /ToUnicode needs
	// rebuilding.
	glyphText map[uint16]string
	nominal   map[uint16]bool
	textDirty bool
}

func (e *embeddedFont) BaseFont() string { return e.baseFont }
func (e *embeddedFont) IsEmbedded() bool { return true }

// claimGlyphText decides whether /ToUnicode can carry the text a shaped
// glyph stands for, recording the mapping when it can. The font's own glyph
// for a character maps through the cmap already; a substituted glyph — a
// ligature, a contextual form — gets its text recorded the first time it is
// drawn, even over a presentation-form code point (DejaVu's "ffi" ligature
// is also its glyph for U+FB03, but the text drawn was "ffi"). It returns
// false when the glyph already stands for something else: Noto Naskh draws
// zain as its reh glyph plus a dot, and relabelling that glyph would turn
// every reh in the document into a zain. The caller then supplies the text
// with /ActualText instead.
func (e *embeddedFont) claimGlyphText(gid uint16, text string) bool {
	if text == "" || gid == 0 {
		return false
	}
	if prev, ok := e.glyphText[gid]; ok {
		return prev == text
	}
	if r := []rune(text); len(r) == 1 && e.ttf.glyphID(r[0]) == gid {
		return true
	}
	if e.nominal == nil {
		e.nominal = make(map[uint16]bool, len(e.ttf.runeToGlyph))
		for r, g := range e.ttf.runeToGlyph {
			if !isPresentationForm(r) {
				e.nominal[g] = true
			}
		}
	}
	if e.nominal[gid] {
		return false
	}
	if e.glyphText == nil {
		e.glyphText = map[uint16]string{}
	}
	e.glyphText[gid] = text
	e.textDirty = true
	return true
}

// isPresentationForm reports whether r is a compatibility presentation form
// (ligatures such as U+FB03, Arabic contextual forms) — code points that name
// a glyph shape rather than a character, so a shaped glyph may reclaim them.
func isPresentationForm(r rune) bool {
	return (r >= 0xFB00 && r <= 0xFDFF) || (r >= 0xFE70 && r <= 0xFEFF)
}

// useGlyph records that glyph gid was emitted to a content stream, so
// (*Document).SubsetFonts knows to keep it. Called from the text encoders.
func (e *embeddedFont) useGlyph(gid uint16) {
	if e.usedGlyphs == nil {
		e.usedGlyphs = map[uint16]bool{}
	}
	e.usedGlyphs[gid] = true
}

// LoadFont reads a TTF file, parses it, embeds it into the document, and returns
// a Font that can be used in TextStyle.Font. The full TTF is embedded without
// subsetting, so large fonts increase output size. The returned Font is bound
// to this document and must not be used with pages from other documents.
func (d *Document) LoadFont(path string) (Font, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load font: %w", err)
	}
	return d.loadFontFromBytes(data)
}

// LoadFontFromStream is like LoadFont but reads from an io.Reader.
func (d *Document) LoadFontFromStream(r io.Reader) (Font, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("load font: read stream: %w", err)
	}
	return d.loadFontFromBytes(data)
}

func (d *Document) loadFontFromBytes(data []byte) (Font, error) {
	ttf, err := parseTTF(data)
	if err != nil {
		return nil, fmt.Errorf("load font: %w", err)
	}
	if ttf.postScriptName == "" {
		return nil, fmt.Errorf("load font: no PostScript name or Full Name in the font's name table")
	}
	// OpenType-CFF (.otf) has no glyf table, so it can't be embedded as a
	// TrueType /FontFile2. It is instead embedded as a /FontFile3 (Subtype
	// OpenType) with a CIDFontType0 descendant (embedCFFFont). CID-keyed CFF
	// programs (those carrying a ROS) are not yet supported: our encoder
	// emits glyph IDs as Identity-H CIDs, which only equals the glyph for a
	// non-CID-keyed CFF (ISO 32000-1 §9.7.4.2).
	var fontID int
	if ttf.cff != nil {
		if ttf.cff.isCID {
			return nil, fmt.Errorf("load font: CID-keyed OpenType-CFF (.otf) embedding is not yet supported")
		}
		fontID = embedCFFFont(d, ttf)
	} else {
		fontID = embedFont(d, ttf)
	}
	ef := &embeddedFont{
		doc:          d,
		ttf:          ttf,
		baseFont:     ttf.postScriptName,
		fontObjectID: fontID,
		usedGlyphs:   map[uint16]bool{},
	}
	d.embeddedFonts = append(d.embeddedFonts, ef)
	return ef, nil
}
