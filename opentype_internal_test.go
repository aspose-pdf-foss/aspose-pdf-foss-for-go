// SPDX-License-Identifier: MIT

package asposepdf

import (
	"os"
	"testing"
)

// loadTestTTF parses a font file straight into a ttfFont, bypassing
// document embedding — the shaping tests need the parsed tables only.
func loadFontFile(t *testing.T, path string) *ttfFont {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("font not available: %v", err)
	}
	f, err := parseSFNTAt(data, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return f
}

// The bundled DejaVu Sans carries both layout tables; the reader must find
// its scripts, the features the Arabic plan asks for, and every lookup.
func TestOTLayoutParsesDejaVu(t *testing.T) {
	f := loadFontFile(t, "testdata/DejaVuSans.ttf")
	l := f.layout()
	if l == nil {
		t.Fatal("DejaVu Sans reports no layout tables")
	}
	if l.gsub == nil || l.gpos == nil {
		t.Fatalf("gsub = %v, gpos = %v; want both", l.gsub != nil, l.gpos != nil)
	}
	if l.gdef == nil || l.gdef.glyphClass.empty() {
		t.Error("GDEF glyph classes missing")
	}

	// fontTools reports 40 GSUB and 16 GPOS lookups for this face.
	if got := len(l.gsub.lookups); got != 40 {
		t.Errorf("GSUB lookups = %d, want 40", got)
	}
	if got := len(l.gpos.lookups); got != 16 {
		t.Errorf("GPOS lookups = %d, want 16", got)
	}
	for _, script := range []string{"arab", "latn", "hebr", "DFLT"} {
		if _, ok := l.gsub.scripts[script]; !ok {
			t.Errorf("GSUB script %q missing", script)
		}
	}
	for _, tag := range []string{"init", "medi", "fina"} {
		if !l.gsub.hasFeature("arab", tag) {
			t.Errorf("arab feature %q missing", tag)
		}
	}
	if !l.gsub.hasFeature("latn", "liga") {
		t.Error("latn feature \"liga\" missing")
	}
	for _, tag := range []string{"kern", "mark", "mkmk"} {
		if !l.gpos.hasFeature("arab", tag) {
			t.Errorf("arab GPOS feature %q missing", tag)
		}
	}

	// A feature plan resolves to lookups in ascending order, which is the
	// order the spec requires them applied in, each carrying the mask of
	// the feature that asked for it.
	plan := otBuildPlan(l.gsub, "arab", arabicGSUBFeatures)
	if len(plan) == 0 {
		t.Fatal("the Arabic plan selected no lookups")
	}
	joining := uint32(0)
	for i, pl := range plan {
		if i > 0 && pl.index <= plan[i-1].index {
			t.Fatalf("plan lookups not ascending: %v", plan)
		}
		joining |= pl.mask & (otMaskInit | otMaskMedi | otMaskFina | otMaskIsol)
	}
	// DejaVu, like most faces, ships no "isol" feature — the isolated form
	// is the default glyph — but the other three joining forms must be there.
	if want := otMaskInit | otMaskMedi | otMaskFina; joining&want != want {
		t.Errorf("joining feature masks = %#b; want at least %#b", joining, want)
	}
}

// Coverage and class lookups are bisections over two different encodings;
// both must agree with a linear scan of what they were built from.
func TestOTCoverageAndClassDef(t *testing.T) {
	cov := otCoverage{glyphs: []uint16{3, 9, 40, 41, 900}}
	for i, g := range cov.glyphs {
		if got := cov.index(g); got != i {
			t.Errorf("glyph %d: index = %d, want %d", g, got, i)
		}
	}
	if cov.index(4) != -1 || cov.index(0) != -1 || cov.index(1000) != -1 {
		t.Error("absent glyph reported as covered")
	}

	ranged := otCoverage{ranges: []otCovRange{{10, 12, 0}, {20, 20, 3}, {30, 33, 4}}}
	for g, want := range map[uint16]int{10: 0, 11: 1, 12: 2, 20: 3, 30: 4, 33: 7} {
		if got := ranged.index(g); got != want {
			t.Errorf("ranged glyph %d: index = %d, want %d", g, got, want)
		}
	}
	if ranged.index(13) != -1 || ranged.index(34) != -1 {
		t.Error("gap glyph reported as covered")
	}

	cd := otClassDef{start: 5, values: []uint16{1, 1, 2}}
	if cd.class(5) != 1 || cd.class(7) != 2 || cd.class(8) != 0 || cd.class(4) != 0 {
		t.Error("format 1 class definition is wrong")
	}
	cd2 := otClassDef{ranges: []otClassRange{{1, 4, 2}, {10, 10, 5}}}
	if cd2.class(3) != 2 || cd2.class(10) != 5 || cd2.class(6) != 0 {
		t.Error("format 2 class definition is wrong")
	}
}

// A value record's byte length depends on which fields its format bits
// declare, device-table offsets included — get this wrong and every record
// after the first in an array is misread.
func TestOTValueRecordSize(t *testing.T) {
	for format, want := range map[uint16]int{0: 0, 0x0004: 2, 0x0005: 4, 0x00FF: 16} {
		if got := otValueRecordSize(format); got != want {
			t.Errorf("format %#04x: size = %d, want %d", format, got, want)
		}
	}
	// XAdvance only: the value sits at the start of the record.
	b := []byte{0xFF, 0x9C} // -100
	if v := otParseValueRecord(b, 0, 0x0004); v.xAdvance != -100 || v.zero() {
		t.Errorf("value record = %+v, want xAdvance -100", v)
	}
}

// A font with no layout tables must report so rather than inventing an
// empty one — that is the flag the text pipeline branches on.
func TestOTLayoutAbsent(t *testing.T) {
	f := &ttfFont{data: []byte("not a font"), tables: map[string]tableRecord{}}
	if l := f.layout(); l != nil {
		t.Errorf("layout() = %v for a font with no GSUB/GPOS, want nil", l)
	}
}
