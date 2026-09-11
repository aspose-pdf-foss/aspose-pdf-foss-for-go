// SPDX-License-Identifier: MIT

package asposepdf

import "testing"

// DejaVu Sans covers Arabic twice over: it has the Presentation Forms-B
// block the phase-2 shaper maps into, and the GSUB lookups that do the same
// job from the font's own tables. The two paths must agree glyph for glyph
// — that agreement is what makes it safe to hand shaping to the font.
func TestShapeArabicMatchesPresentationForms(t *testing.T) {
	f := loadFontFile(t, "testdata/DejaVuSans.ttf")
	if !fontShapesArabic(f) {
		t.Fatal("DejaVu Sans does not advertise Arabic joining features")
	}
	isolated := map[rune]rune{}
	for base, forms := range arabicShapeTable {
		isolated[forms.iso] = base
	}
	for _, text := range []string{
		"\u0627\u0644\u0633\u0644\u0627\u0645",                    // al-salam
		"\u0645\u0631\u062d\u0628\u0627",                          // marhaba
		"\u0639\u0631\u0628\u064a",                                // arabi
		"\u0643\u062a\u0628 \u0627\u0644\u0637\u0627\u0644\u0628", // two words
	} {
		shaped := shapeRun(f, text, true)
		var got []uint16
		for _, g := range shaped {
			got = append(got, g.gid)
		}
		want := []rune(shapeArabic(text))
		if len(got) != len(want) {
			t.Errorf("%q: %d shaped glyphs, %d Forms-B glyphs", text, len(got), len(want))
			continue
		}
		for i := range got {
			if got[i] == f.glyphID(want[i]) {
				continue
			}
			// A font without an "isol" feature draws an isolated letter
			// with its base glyph; Forms-B names the same picture U+FExx.
			if base, ok := isolated[want[i]]; ok && got[i] == f.glyphID(base) {
				continue
			}
			t.Errorf("%q: glyph %d = %d, Forms-B path gives %d (%U)", text, i, got[i], f.glyphID(want[i]), want[i])
		}
	}
}

// Latin still shapes: the standard f-ligatures merge two glyphs into one,
// and kerning pulls a pair closer than the sum of its advances.
func TestShapeLatinLigatureAndKern(t *testing.T) {
	f := loadFontFile(t, "testdata/DejaVuSans.ttf")

	shaped := shapeRun(f, "fi", false)
	if len(shaped) != 1 {
		t.Errorf("\"fi\" shaped to %d glyphs, want the fi ligature", len(shaped))
	} else if shaped[0].gid == f.glyphID('f') {
		t.Error("\"fi\" was left as its component glyphs")
	}

	kerned := shapeRun(f, "AV", false)
	if len(kerned) != 2 {
		t.Fatalf("\"AV\" shaped to %d glyphs, want 2", len(kerned))
	}
	plain := int32(f.glyphAdvance(f.glyphID('A')) + f.glyphAdvance(f.glyphID('V')))
	total := kerned[0].advance + kerned[1].advance
	if total >= plain {
		t.Errorf("AV total advance = %d, unkerned = %d; want the pair kerned tighter", total, plain)
	}
}

// A vowel mark is positioned against its base letter rather than sitting at
// its own advance — the thing Presentation Forms-B cannot express.
func TestShapePositionsArabicMarks(t *testing.T) {
	f := loadFontFile(t, "testdata/DejaVuSans.ttf")
	// bism, with kasra under the beh and sukun over the seen.
	shaped := shapeRun(f, "\u0628\u0650\u0633\u0652\u0645", true)
	positioned := 0
	for _, g := range shaped {
		if g.isMark && (g.xOff != 0 || g.yOff != 0) {
			positioned++
		}
	}
	if positioned == 0 {
		t.Error("no harakat were positioned by GPOS")
	}
}

// Canonical ordering is what lets a mark stack agree with every other text
// engine; a starter is never moved.
func TestCanonicalOrder(t *testing.T) {
	// Hebrew bet + sheva + dagesh: the dagesh sorts first, as fonts (and
	// HarfBuzz) expect, even though its Unicode class number is higher.
	runes := []rune{0x05D1, 0x05B0, 0x05BC, 0x05E8}
	canonicalOrder(runes)
	if want := []rune{0x05D1, 0x05BC, 0x05B0, 0x05E8}; string(runes) != string(want) {
		t.Errorf("Hebrew order = %U, want %U", runes, want)
	}
	// Arabic beh + fatha + shadda: the shadda sorts first.
	arabic := []rune{0x0628, 0x064E, 0x0651}
	canonicalOrder(arabic)
	if want := []rune{0x0628, 0x0651, 0x064E}; string(arabic) != string(want) {
		t.Errorf("Arabic order = %U, want %U", arabic, want)
	}
	// Equal classes keep their typed order.
	same := []rune{'a', 0x0301, 0x0302}
	canonicalOrder(same)
	if string(same) != string([]rune{'a', 0x0301, 0x0302}) {
		t.Errorf("marks of equal class were reordered: %U", same)
	}
	if combiningClass('a') != 0 || combiningClass(0x064E) != 30 {
		t.Error("combining classes are wrong")
	}
}
