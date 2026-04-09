package asposepdf

import (
	"testing"
)

func TestExtractXObjectImageJPEGPassthrough(t *testing.T) {
	// Minimal synthetic JPEG data (just the SOI and EOI markers).
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x02, 0xFF, 0xD9}

	imgStream := &pdfStream{
		Dict: pdfDict{
			"/Subtype":          pdfName("/Image"),
			"/Width":            100,
			"/Height":           80,
			"/BitsPerComponent":  8,
			"/ColorSpace":       pdfName("/DeviceRGB"),
			"/Filter":           pdfName("/DCTDecode"),
		},
		Data:    jpegData,
		Decoded: false,
	}

	objects := map[int]*pdfObject{
		1: {Value: imgStream},
	}
	resources := pdfDict{
		"/XObject": pdfDict{
			"/Im0": pdfRef{Num: 1},
		},
	}

	ctm := identityMatrix()
	ctm[4] = 72
	ctm[5] = 500
	ctm[0] = 200
	ctm[3] = 160

	img, ok := extractXObjectImage(objects, resources, "/Im0", ctm)
	if !ok {
		t.Fatal("extractXObjectImage returned false for JPEG image")
	}
	if img.Format != ImageFormatJPEG {
		t.Errorf("format = %d, want ImageFormatJPEG", img.Format)
	}
	if img.Width != 100 || img.Height != 80 {
		t.Errorf("dimensions = %dx%d, want 100x80", img.Width, img.Height)
	}
	if img.BPC != 8 {
		t.Errorf("BPC = %d, want 8", img.BPC)
	}
	if img.ColorSpace != ColorSpaceDeviceRGB {
		t.Errorf("colorSpace = %d, want DeviceRGB", img.ColorSpace)
	}
	if len(img.Data) != len(jpegData) {
		t.Errorf("data len = %d, want %d", len(img.Data), len(jpegData))
	}
	if img.X != 72 || img.Y != 500 {
		t.Errorf("position = (%g, %g), want (72, 500)", img.X, img.Y)
	}
	if img.PageWidth != 200 || img.PageHeight != 160 {
		t.Errorf("page size = (%g, %g), want (200, 160)", img.PageWidth, img.PageHeight)
	}
}

func TestExtractXObjectImageSkipsNonImage(t *testing.T) {
	formStream := &pdfStream{
		Dict: pdfDict{
			"/Subtype": pdfName("/Form"),
			"/BBox":    pdfArray{0, 0, 100, 100},
		},
		Data: []byte{},
	}

	objects := map[int]*pdfObject{
		1: {Value: formStream},
	}
	resources := pdfDict{
		"/XObject": pdfDict{
			"/Fm0": pdfRef{Num: 1},
		},
	}

	_, ok := extractXObjectImage(objects, resources, "/Fm0", identityMatrix())
	if ok {
		t.Error("expected false for Form XObject, got true")
	}
}

func TestExtractXObjectImagePNGFlateDecode(t *testing.T) {
	// FlateDecode image with pre-decoded RGB pixels should produce a PNG.
	pixels := make([]byte, 10*10*3) // 10x10 RGB
	imgStream := &pdfStream{
		Dict: pdfDict{
			"/Subtype":         pdfName("/Image"),
			"/Width":           10,
			"/Height":          10,
			"/BitsPerComponent": 8,
			"/ColorSpace":      pdfName("/DeviceRGB"),
			"/Filter":          pdfName("/FlateDecode"),
		},
		Data:    pixels,
		Decoded: true,
	}

	objects := map[int]*pdfObject{
		1: {Value: imgStream},
	}
	resources := pdfDict{
		"/XObject": pdfDict{
			"/Im0": pdfRef{Num: 1},
		},
	}

	img, ok := extractXObjectImage(objects, resources, "/Im0", identityMatrix())
	if !ok {
		t.Fatal("expected true for FlateDecode image, got false")
	}
	if img.Format != ImageFormatPNG {
		t.Errorf("format = %d, want ImageFormatPNG", img.Format)
	}
	if img.Width != 10 || img.Height != 10 {
		t.Errorf("dimensions = %dx%d, want 10x10", img.Width, img.Height)
	}
	if len(img.Data) == 0 {
		t.Error("expected non-empty PNG data")
	}
}

func TestResolveColorSpaceVariants(t *testing.T) {
	objects := map[int]*pdfObject{}

	tests := []struct {
		name string
		dict pdfDict
		want ImageColorSpace
	}{
		{"no key", pdfDict{}, ColorSpaceDeviceRGB},
		{"DeviceRGB", pdfDict{"/ColorSpace": pdfName("/DeviceRGB")}, ColorSpaceDeviceRGB},
		{"DeviceGray", pdfDict{"/ColorSpace": pdfName("/DeviceGray")}, ColorSpaceDeviceGray},
		{"DeviceCMYK", pdfDict{"/ColorSpace": pdfName("/DeviceCMYK")}, ColorSpaceDeviceCMYK},
		{"ICCBased array", pdfDict{"/ColorSpace": pdfArray{pdfName("/ICCBased"), pdfRef{Num: 99}}}, ColorSpaceICCBased},
		{"Indexed array", pdfDict{"/ColorSpace": pdfArray{pdfName("/Indexed"), pdfName("/DeviceRGB"), 255, "palette"}}, ColorSpaceIndexed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveColorSpace(objects, tt.dict)
			if got != tt.want {
				t.Errorf("resolveColorSpace = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPrimaryFilter(t *testing.T) {
	tests := []struct {
		name string
		dict pdfDict
		want string
	}{
		{"no filter", pdfDict{}, ""},
		{"single name", pdfDict{"/Filter": pdfName("/DCTDecode")}, "/DCTDecode"},
		{"array", pdfDict{"/Filter": pdfArray{pdfName("/FlateDecode"), pdfName("/ASCII85Decode")}}, "/FlateDecode"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := primaryFilter(tt.dict)
			if got != tt.want {
				t.Errorf("primaryFilter = %q, want %q", got, tt.want)
			}
		})
	}
}
