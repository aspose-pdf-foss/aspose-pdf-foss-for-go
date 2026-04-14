package asposepdf

import (
	"bytes"
	"testing"
)

func TestDetectImageFormat(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    ImageFormat
		wantErr bool
	}{
		{"JPEG", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}, ImageFormatJPEG, false},
		{"PNG", []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, ImageFormatPNG, false},
		{"unknown", []byte{0x00, 0x01, 0x02, 0x03}, 0, true},
		{"too short", []byte{0xFF}, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := detectImageFormat(tt.data)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseJPEGHeader(t *testing.T) {
	// Build minimal JPEG: SOI + SOF0 marker with 100x80, 3 components (RGB).
	sof := []byte{
		0xFF, 0xD8, // SOI
		0xFF, 0xC0, // SOF0
		0x00, 0x0B, // length = 11
		0x08,       // precision = 8
		0x00, 0x50, // height = 80
		0x00, 0x64, // width = 100
		0x03,       // 3 components = RGB
		0x01, 0x22, 0x00, // component 1
		0x02, 0x11, 0x01, // component 2
		0x03, 0x11, 0x01, // component 3
	}
	info, err := parseJPEGHeader(bytes.NewReader(sof))
	if err != nil {
		t.Fatalf("parseJPEGHeader: %v", err)
	}
	if info.width != 100 || info.height != 80 {
		t.Errorf("dimensions = %dx%d, want 100x80", info.width, info.height)
	}
	if info.components != 3 {
		t.Errorf("components = %d, want 3", info.components)
	}
}

func TestParseJPEGHeaderGray(t *testing.T) {
	sof := []byte{
		0xFF, 0xD8, // SOI
		0xFF, 0xC0, // SOF0
		0x00, 0x08, // length = 8
		0x08,       // precision = 8
		0x00, 0x20, // height = 32
		0x00, 0x40, // width = 64
		0x01,       // 1 component = Gray
		0x01, 0x11, 0x00,
	}
	info, err := parseJPEGHeader(bytes.NewReader(sof))
	if err != nil {
		t.Fatalf("parseJPEGHeader: %v", err)
	}
	if info.width != 64 || info.height != 32 {
		t.Errorf("dimensions = %dx%d, want 64x32", info.width, info.height)
	}
	if info.components != 1 {
		t.Errorf("components = %d, want 1", info.components)
	}
}
