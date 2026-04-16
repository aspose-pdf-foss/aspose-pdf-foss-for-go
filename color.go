package asposepdf

// Color represents an RGBA color with values in [0, 1].
type Color struct {
	R float64
	G float64
	B float64
	A float64
}

// HAlign specifies horizontal text alignment within a rectangle.
type HAlign int

const (
	HAlignLeft   HAlign = iota // default
	HAlignCenter
	HAlignRight
)

// VAlign specifies vertical text alignment within a rectangle.
type VAlign int

const (
	VAlignTop    VAlign = iota // default
	VAlignMiddle
	VAlignBottom
)

// Font identifies one of the standard 14 PDF fonts.
type Font int

const (
	FontHelvetica Font = iota
	FontHelveticaBold
	FontHelveticaOblique
	FontHelveticaBoldOblique
	FontTimesRoman
	FontTimesBold
	FontTimesItalic
	FontTimesBoldItalic
	FontCourier
	FontCourierBold
	FontCourierOblique
	FontCourierBoldOblique
	FontSymbol
	FontZapfDingbats
)

// TextStyle defines reusable text formatting properties.
type TextStyle struct {
	Font          Font
	Size          float64 // in points; 0 treated as 12
	Color         *Color  // nil → black opaque {0,0,0,1}
	Background    *Color  // nil → no background
	HAlign        HAlign  // default: HAlignLeft
	VAlign        VAlign  // default: VAlignTop
	LineSpacing   float64 // multiplier of font size; 0 treated as 1.2
	Underline     bool
	Strikethrough bool
}

// fontPDFName returns the PDF base font name for a Font constant.
func fontPDFName(f Font) string {
	switch f {
	case FontHelvetica:
		return "/Helvetica"
	case FontHelveticaBold:
		return "/Helvetica-Bold"
	case FontHelveticaOblique:
		return "/Helvetica-Oblique"
	case FontHelveticaBoldOblique:
		return "/Helvetica-BoldOblique"
	case FontTimesRoman:
		return "/Times-Roman"
	case FontTimesBold:
		return "/Times-Bold"
	case FontTimesItalic:
		return "/Times-Italic"
	case FontTimesBoldItalic:
		return "/Times-BoldItalic"
	case FontCourier:
		return "/Courier"
	case FontCourierBold:
		return "/Courier-Bold"
	case FontCourierOblique:
		return "/Courier-Oblique"
	case FontCourierBoldOblique:
		return "/Courier-BoldOblique"
	case FontSymbol:
		return "/Symbol"
	case FontZapfDingbats:
		return "/ZapfDingbats"
	default:
		return "/Helvetica"
	}
}
