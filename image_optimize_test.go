package asposepdf

import (
	"image"
	"image/color"
	"testing"
)

func TestRawPixelsToImageRGB(t *testing.T) {
	// 2x2 red/green/blue/white image.
	pixels := []byte{
		255, 0, 0, 0, 255, 0,
		0, 0, 255, 255, 255, 255,
	}
	img := rawPixelsToImage(pixels, 2, 2, "/DeviceRGB")
	if img == nil {
		t.Fatal("expected non-nil image")
	}
	bounds := img.Bounds()
	if bounds.Dx() != 2 || bounds.Dy() != 2 {
		t.Fatalf("bounds = %v, want 2x2", bounds)
	}
	r, g, b, a := img.At(0, 0).RGBA()
	if r>>8 != 255 || g>>8 != 0 || b>>8 != 0 || a>>8 != 255 {
		t.Errorf("pixel (0,0) = (%d,%d,%d,%d), want red", r>>8, g>>8, b>>8, a>>8)
	}
}

func TestRawPixelsToImageGray(t *testing.T) {
	pixels := []byte{0, 128, 255, 64}
	img := rawPixelsToImage(pixels, 2, 2, "/DeviceGray")
	if img == nil {
		t.Fatal("expected non-nil image")
	}
	g := img.(*image.Gray)
	if g.GrayAt(0, 0).Y != 0 {
		t.Errorf("pixel (0,0) = %d, want 0", g.GrayAt(0, 0).Y)
	}
	if g.GrayAt(1, 0).Y != 128 {
		t.Errorf("pixel (1,0) = %d, want 128", g.GrayAt(1, 0).Y)
	}
}

func TestRawPixelsToImageUnsupported(t *testing.T) {
	img := rawPixelsToImage([]byte{0, 0, 0, 0}, 1, 1, "/DeviceCMYK")
	if img != nil {
		t.Error("expected nil for unsupported color space")
	}
}

func TestDownscaleImage(t *testing.T) {
	// 4x4 image: top-left quadrant red, top-right green, bottom-left blue, bottom-right white.
	src := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			var c color.NRGBA
			switch {
			case x < 2 && y < 2:
				c = color.NRGBA{R: 255, G: 0, B: 0, A: 255}
			case x >= 2 && y < 2:
				c = color.NRGBA{R: 0, G: 255, B: 0, A: 255}
			case x < 2 && y >= 2:
				c = color.NRGBA{R: 0, G: 0, B: 255, A: 255}
			default:
				c = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
			}
			src.SetNRGBA(x, y, c)
		}
	}

	dst := downscaleImage(src, 2, 2)
	if dst.Bounds().Dx() != 2 || dst.Bounds().Dy() != 2 {
		t.Fatalf("bounds = %v, want 2x2", dst.Bounds())
	}

	// Each output pixel should be the average of a 2x2 block from the source.
	// (0,0) = average of red block = (255, 0, 0)
	r, g, b, _ := dst.At(0, 0).RGBA()
	if r>>8 != 255 || g>>8 != 0 || b>>8 != 0 {
		t.Errorf("pixel (0,0) = (%d,%d,%d), want red", r>>8, g>>8, b>>8)
	}
	// (1,0) = average of green block = (0, 255, 0)
	r, g, b, _ = dst.At(1, 0).RGBA()
	if r>>8 != 0 || g>>8 != 255 || b>>8 != 0 {
		t.Errorf("pixel (1,0) = (%d,%d,%d), want green", r>>8, g>>8, b>>8)
	}
}

func TestDownscaleImagePreservesNonSquare(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 100, 50))
	dst := downscaleImage(src, 50, 25)
	if dst.Bounds().Dx() != 50 || dst.Bounds().Dy() != 25 {
		t.Fatalf("bounds = %v, want 50x25", dst.Bounds())
	}
}
