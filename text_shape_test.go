// SPDX-License-Identifier: MIT

package asposepdf

import (
	"bytes"
	"os"
	"sort"
	"strings"
	"testing"
)

// drawAndReopen draws text with a font loaded from path, saves the document
// and opens it again, so what is checked is what a reader of the file sees.
func drawAndReopen(t *testing.T, fontPath, text string, style TextStyle) *Document {
	t.Helper()
	if _, err := os.Stat(fontPath); err != nil {
		t.Skipf("font not available: %v", err)
	}
	doc := NewDocument(600, 200)
	font, err := doc.LoadFont(fontPath)
	if err != nil {
		t.Fatalf("load font: %v", err)
	}
	style.Font = font
	if style.Size == 0 {
		style.Size = 18
	}
	page, _ := doc.Page(1)
	if err := page.AddText(text, style, Rectangle{LLX: 20, LLY: 40, URX: 580, URY: 180}); err != nil {
		t.Fatalf("add text: %v", err)
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	re, err := OpenStream(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	return re
}

// Ligatures shrink "ffi" to one glyph; the /ToUnicode entry the shaper
// records must bring every letter back, or search and extraction break for
// any document drawn with a ligating font.
func TestShapedLigaturesExtractBack(t *testing.T) {
	doc := drawAndReopen(t, "testdata/DejaVuSans.ttf", "office affluent fifty AVATAR", TextStyle{})
	page, _ := doc.Page(1)
	got, err := page.ExtractText()
	if err != nil {
		t.Fatal(err)
	}
	for _, word := range []string{"office", "affluent", "fifty", "AVATAR"} {
		if !strings.Contains(got, word) {
			t.Errorf("extracted %q, missing %q", got, word)
		}
	}
	if strings.ContainsRune(got, '\uFFFD') {
		t.Errorf("extracted text has replacement characters: %q", got)
	}
}

// Arabic in a font that has no Presentation Forms-B glyphs at all only
// connects through GSUB; the contextual forms have no cmap entry, so they
// extract only if the shaper recorded their letters.
func TestShapedArabicGSUBOnlyFontExtractsBack(t *testing.T) {
	const text = "\u0627\u0644\u0633\u0644\u0627\u0645 \u0639\u0644\u064a\u0643\u0645"
	doc := drawAndReopen(t, "C:/Windows/Fonts/NotoNaskhArabic-Regular.ttf", text, TextStyle{})
	page, _ := doc.Page(1)
	got, err := page.ExtractText()
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsRune(got, '\uFFFD') {
		t.Fatalf("extracted text has replacement characters: %q", got)
	}
	letters := func(s string) string {
		var rs []rune
		for _, r := range s {
			if r != ' ' && r != '\n' {
				rs = append(rs, r)
			}
		}
		sort.Slice(rs, func(i, j int) bool { return rs[i] < rs[j] })
		return string(rs)
	}
	if letters(got) != letters(text) {
		t.Errorf("extracted letters %q, want the letters of %q", got, text)
	}
}

// A multi-character ToUnicode destination must extract as all of its
// characters, and a surrogate pair as the one character it encodes.
func TestParseCMapSequences(t *testing.T) {
	cmap := []byte("2 beginbfchar\n<0010> <00660069>\n<0011> <D835DC00>\nendbfchar\n")
	runes, seqs := parseCMapFull(cmap)
	if string(seqs[0x10]) != "fi" || runes[0x10] != 'f' {
		t.Errorf("ligature entry = %q / %q, want \"fi\"", string(seqs[0x10]), runes[0x10])
	}
	if runes[0x11] != 0x1D400 || seqs[0x11] != nil {
		t.Errorf("surrogate pair entry = %U, want U+1D400", runes[0x11])
	}
}
