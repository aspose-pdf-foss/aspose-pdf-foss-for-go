# OptimizeImages — Design Spec

## Goal

Add the ability to optimize images in existing PDFs to reduce file size. Supports two strategies: downscaling images that exceed a target DPI, and converting opaque PNG images to JPEG. This is Sub-project C of the image manipulation feature set.

## Public API

### New types and methods

```go
// OptimizeImageOptions controls image optimization behavior.
type OptimizeImageOptions struct {
    MaxDPI           float64 // images above this DPI are downscaled; 0 = no downscaling
    JPEGQuality      int     // JPEG quality 1-100; 0 = default (75)
    ConvertPNGToJPEG bool    // convert opaque PNG (no alpha) to JPEG
}

// OptimizeImages optimizes all images in the document to reduce file size.
// Returns the number of images optimized.
func (d *Document) OptimizeImages(opts OptimizeImageOptions) (int, error)
```

### Usage

```go
doc, _ := pdf.Open("large.pdf")
optimized, err := doc.OptimizeImages(pdf.OptimizeImageOptions{
    MaxDPI:           150,
    JPEGQuality:      75,
    ConvertPNGToJPEG: true,
})
fmt.Printf("optimized %d images\n", optimized)
doc.Save("smaller.pdf")
```

## Internal design

### Algorithm

`OptimizeImages` iterates all pages, collects `ImageInfos()`, and processes each image XObject. A `processed map[int]bool` keyed by object ID tracks already-handled XObjects to avoid processing shared images twice.

For each image:

1. **Compute effective DPI** — `imageDPI = imagePixelWidth / (displayWidthPt / 72.0)`. Display size comes from `ImageInfo.PageWidth` / `PageHeight` (in points, derived from CTM). If display size is zero (no CTM), skip the image.

2. **Decide if optimization is needed:**
   - Downscale: `MaxDPI > 0` and `imageDPI > MaxDPI`
   - PNG→JPEG conversion: `ConvertPNGToJPEG` is true, image is PNG (no `/Filter` or FlateDecode, `Decoded=true`), and no `/SMask` in dict (no alpha channel)
   - If neither applies: skip (do not re-encode JPEG that is already below MaxDPI)

3. **Decode pixels** — reconstruct `image.Image` from stream data:
   - JPEG (`Decoded=false`, `/Filter=/DCTDecode`): `image/jpeg.Decode()` from `stream.Data`
   - PNG-stored (`Decoded=true`): reconstruct `image.Image` from raw pixel bytes using `/Width`, `/Height`, `/ColorSpace`, `/BitsPerComponent` from the stream dict. RGB → `image.NRGBA`, Gray → `image.Gray`.

4. **Downscale if needed** — bilinear interpolation to `newWidth = displayWidthPt / 72.0 * MaxDPI`. Pure Go implementation, no external dependencies. Preserves aspect ratio.

5. **Encode back:**
   - To JPEG: `image/jpeg.Encode()` with quality → set `stream.Data`, `stream.Dict["/Filter"] = /DCTDecode`, `stream.Decoded = false`
   - To PNG (kept as PNG): extract RGB/Gray bytes from `image.Image` → set `stream.Data`, delete `/Filter`, `stream.Decoded = true`

6. **Update stream dict** — `/Width`, `/Height`, `/BitsPerComponent`, `/ColorSpace`. Remove `/DecodeParms`. Handle SMask: if converting PNG→JPEG, delete `/SMask` (only opaque PNGs are converted, so no alpha to preserve).

### Pixel reconstruction from raw stream data

PNG images in PDF are stored as raw decoded pixels (not PNG file format). To reconstruct an `image.Image`:

```go
func rawPixelsToImage(data []byte, width, height int, colorSpace string) image.Image
```

- `/DeviceRGB`: 3 bytes per pixel → `image.NRGBA` (A=255)
- `/DeviceGray`: 1 byte per pixel → `image.Gray`
- Other color spaces: skip optimization for this image (return nil)

### Bilinear downscale

```go
func downscaleImage(img image.Image, newWidth, newHeight int) *image.NRGBA
```

Standard bilinear interpolation. Input can be any `image.Image`. Output is always `*image.NRGBA`. If the image is grayscale, the output is still NRGBA but with R=G=B — the encoder handles the conversion back to gray if needed.

### JPEG re-encoding: only when needed

JPEG images that do not need downscaling are left untouched. Re-encoding JPEG→JPEG is lossy and may increase file size if the original was already well-compressed. JPEG is only re-encoded when:
- Downscaling is applied (pixels changed, must re-encode)
- The image was PNG being converted to JPEG

### Shared XObject handling

The `processed` map prevents double-processing. If the same XObject appears on multiple pages with different display sizes, the first encounter determines the DPI calculation. This is acceptable because shared XObjects typically display at the same size, and the alternative (tracking per-page display sizes) adds complexity for a rare edge case.

## Error handling

- **Decode failure** — individual image decode errors (corrupt data) are skipped with a warning; processing continues with next image. The method returns an error only if a systemic problem occurs.
- **Invalid options** — `JPEGQuality` outside 1-100 (when non-zero) returns an error before processing.
- **Zero display size** — images without CTM information (display size = 0) are skipped silently.

## Files

| File | Responsibility |
|------|----------------|
| `image_optimize.go` | `OptimizeImages`, `optimizeImage`, `downscaleImage`, `rawPixelsToImage` |
| `image_optimize_test.go` | Unit tests (package `asposepdf`) |
| `image_optimize_integration_test.go` | Integration test (package `asposepdf_test`) |

## Testing

### Unit tests (package `asposepdf`)

- `TestOptimizeImagesDownscale` — document with 100x100px image displayed at 50x50pt (144 DPI), optimize with MaxDPI=72, verify Width/Height decreased to ~50x50.
- `TestOptimizeImagesPNGToJPEG` — document with opaque PNG, ConvertPNGToJPEG=true, verify Filter changed to `/DCTDecode` and `Decoded=false`.
- `TestOptimizeImagesPNGWithAlphaNotConverted` — document with PNG+SMask, ConvertPNGToJPEG=true, verify format stays PNG (alpha preserved).
- `TestOptimizeImagesNoOp` — JPEG below MaxDPI, verify data untouched and count=0.
- `TestOptimizeImagesSharedXObject` — one XObject on two pages, verify optimized once (count=1).
- `TestDownscaleImage` — unit test for bilinear resize: 4x4 → 2x2, verify output dimensions and pixel interpolation.

### Integration test (package `asposepdf_test`)

- `TestOptimizeImagesRoundTrip` — open real PDF with images, optimize with MaxDPI=150 + ConvertPNGToJPEG, save, reopen, Validate, verify file size decreased.

## Scope boundary

This spec covers only image optimization via downscaling and PNG→JPEG conversion. It does NOT cover:
- Font subsetting or removal
- Content stream compression
- Object deduplication
- Lossy re-compression of existing JPEGs (intentionally excluded — quality loss without guaranteed size reduction)
