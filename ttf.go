package asposepdf

import (
	"encoding/binary"
	"fmt"
)

// ttfFont holds the parsed fields required for PDF embedding and text measurement.
type ttfFont struct {
	data []byte // raw TTF bytes (written verbatim into /FontFile2)

	// From head.
	unitsPerEm uint16
	xMin, yMin int16
	xMax, yMax int16

	// From hhea.
	ascent, descent        int16
	numOfLongHorMetrics    uint16

	// From maxp.
	numGlyphs uint16

	// From hmtx.
	glyphWidths []uint16 // advanceWidth per glyphID (FUnits)

	// From cmap.
	runeToGlyph map[rune]uint16

	// From OS/2.
	capHeight   int16
	weight      uint16
	flagsBold   bool
	flagsItalic bool

	// From post.
	italicAngle  float64
	isFixedPitch bool

	// From name.
	postScriptName string
}

// tableRecord is an entry in the TTF table directory.
type tableRecord struct {
	offset uint32
	length uint32
}

// parseTTF parses a TrueType font file and returns the ttfFont ready for embedding.
// Only the tables required for CIDFontType2 / Type0 embedding are read.
func parseTTF(data []byte) (*ttfFont, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("parse ttf: file too small (%d bytes)", len(data))
	}
	scaler := binary.BigEndian.Uint32(data[0:4])
	if scaler != 0x00010000 && scaler != 0x74727565 { // 'true'
		return nil, fmt.Errorf("parse ttf: not a TrueType file (scaler 0x%08X)", scaler)
	}

	numTables := int(binary.BigEndian.Uint16(data[4:6]))
	if len(data) < 12+numTables*16 {
		return nil, fmt.Errorf("parse ttf: truncated table directory")
	}
	tables := make(map[string]tableRecord, numTables)
	for i := 0; i < numTables; i++ {
		off := 12 + i*16
		tag := string(data[off : off+4])
		tables[tag] = tableRecord{
			offset: binary.BigEndian.Uint32(data[off+8 : off+12]),
			length: binary.BigEndian.Uint32(data[off+12 : off+16]),
		}
	}

	required := []string{"head", "hhea", "hmtx", "maxp", "name", "cmap", "OS/2", "post"}
	for _, tag := range required {
		if _, ok := tables[tag]; !ok {
			return nil, fmt.Errorf("parse ttf: missing required table %q", tag)
		}
	}

	f := &ttfFont{data: data}

	// Per-table parsers are added in subsequent tasks. The skeleton returns
	// a font with only data populated; full parsing is wired in Tasks 6–9.

	return f, nil
}

// tableSlice returns the bytes of the named table or nil if absent.
func tableSlice(data []byte, tables map[string]tableRecord, tag string) []byte {
	t, ok := tables[tag]
	if !ok {
		return nil
	}
	end := t.offset + t.length
	if end > uint32(len(data)) {
		return nil
	}
	return data[t.offset:end]
}
