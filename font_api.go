package asposepdf

import (
	"fmt"
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
