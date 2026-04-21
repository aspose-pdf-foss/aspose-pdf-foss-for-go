package asposepdf

import (
	"os"
	"strings"
	"testing"
)

func loadDejaVu(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/DejaVuSans.ttf")
	if err != nil {
		t.Fatalf("read DejaVuSans.ttf: %v", err)
	}
	return data
}

func TestParseTTF_NotTTF(t *testing.T) {
	_, err := parseTTF([]byte("not a font file, just garbage"))
	if err == nil {
		t.Fatal("expected error for non-TTF input")
	}
	if !strings.Contains(err.Error(), "TrueType") {
		t.Errorf("error = %q, want to mention TrueType", err.Error())
	}
}

func TestParseTTF_TooSmall(t *testing.T) {
	_, err := parseTTF([]byte{0x00, 0x01, 0x00, 0x00})
	if err == nil {
		t.Fatal("expected error for truncated file")
	}
}

func TestParseTTF_DejaVuBasic(t *testing.T) {
	f, err := parseTTF(loadDejaVu(t))
	if err != nil {
		t.Fatalf("parseTTF: %v", err)
	}
	if f == nil {
		t.Fatal("parseTTF returned nil font")
	}
	if len(f.data) == 0 {
		t.Error("ttfFont.data is empty")
	}
}
