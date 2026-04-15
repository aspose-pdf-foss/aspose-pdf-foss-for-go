package asposepdf

import (
	"image"
	"image/color"
)

// downscaleImage resizes img to newWidth x newHeight using bilinear interpolation.
func downscaleImage(img image.Image, newWidth, newHeight int) *image.NRGBA {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, newWidth, newHeight))

	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			// Map destination pixel to source coordinates.
			srcX := (float64(x)+0.5)*float64(srcW)/float64(newWidth) - 0.5
			srcY := (float64(y)+0.5)*float64(srcH)/float64(newHeight) - 0.5

			x0 := int(srcX)
			y0 := int(srcY)
			if x0 < 0 {
				x0 = 0
			}
			if y0 < 0 {
				y0 = 0
			}
			x1 := x0 + 1
			y1 := y0 + 1
			if x1 >= srcW {
				x1 = srcW - 1
			}
			if y1 >= srcH {
				y1 = srcH - 1
			}

			dx := srcX - float64(x0)
			dy := srcY - float64(y0)
			if dx < 0 {
				dx = 0
			}
			if dy < 0 {
				dy = 0
			}

			r00, g00, b00, a00 := img.At(bounds.Min.X+x0, bounds.Min.Y+y0).RGBA()
			r10, g10, b10, a10 := img.At(bounds.Min.X+x1, bounds.Min.Y+y0).RGBA()
			r01, g01, b01, a01 := img.At(bounds.Min.X+x0, bounds.Min.Y+y1).RGBA()
			r11, g11, b11, a11 := img.At(bounds.Min.X+x1, bounds.Min.Y+y1).RGBA()

			r := bilinear(r00, r10, r01, r11, dx, dy)
			g := bilinear(g00, g10, g01, g11, dx, dy)
			b := bilinear(b00, b10, b01, b11, dx, dy)
			a := bilinear(a00, a10, a01, a11, dx, dy)

			dst.SetNRGBA(x, y, color.NRGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
				A: uint8(a >> 8),
			})
		}
	}
	return dst
}

// bilinear performs bilinear interpolation on four uint32 corner values
// (as returned by color.RGBA()) given fractional offsets dx, dy in [0,1].
func bilinear(v00, v10, v01, v11 uint32, dx, dy float64) uint32 {
	top := float64(v00)*(1-dx) + float64(v10)*dx
	bot := float64(v01)*(1-dx) + float64(v11)*dx
	return uint32(top*(1-dy) + bot*dy)
}

// rawPixelsToImage reconstructs an image.Image from raw PDF pixel bytes.
// PDF stores decoded image data as raw pixels, not as PNG file format.
// Returns nil for unsupported color spaces.
func rawPixelsToImage(data []byte, width, height int, colorSpace string) image.Image {
	switch colorSpace {
	case "/DeviceRGB":
		img := image.NewNRGBA(image.Rect(0, 0, width, height))
		stride := width * 3
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				off := y*stride + x*3
				if off+2 >= len(data) {
					break
				}
				img.SetNRGBA(x, y, color.NRGBA{R: data[off], G: data[off+1], B: data[off+2], A: 255})
			}
		}
		return img
	case "/DeviceGray":
		img := image.NewGray(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				off := y*width + x
				if off >= len(data) {
					break
				}
				img.SetGray(x, y, color.Gray{Y: data[off]})
			}
		}
		return img
	default:
		return nil
	}
}
