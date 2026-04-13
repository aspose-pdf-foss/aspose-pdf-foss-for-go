# ImageInfo Lazy Extraction — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `ImageInfo` type for lightweight image metadata listing with on-demand `Extract()` for selective decoding.

**Architecture:** Refactor `image.go` to split the walker into two phases: (1) `collectImageInfos` gathers metadata + internal refs without decoding, (2) `ImageInfo.Extract()` performs decoding on demand. `ExtractImages` is rewritten as `ImageInfos` + `Extract` to eliminate code duplication.

**Tech Stack:** Pure Go — no new dependencies.

---

## File Structure

| File | Responsibility |
|---|---|
| `image.go` | Add `ImageInfo` type, `collectImageInfos`, `Extract()`, `ImageInfos()` methods; refactor `ExtractImages`; remove `extractImagesFromOps` |
| `image_internal_test.go` | Add unit tests for `ImageInfo` metadata collection and `Extract()` |
| `image_test.go` | Add integration test `TestImageInfos`; add `TestExtractImages` entry to testfiles.json if needed |
| `CLAUDE.md` | Add `ImageInfo`, `ImageInfos()` to public API docs |
| `README.md` | Add `ImageInfos` example to Image Extraction section |

---

### Task 1: ImageInfo type and collectImageInfos

Add the `ImageInfo` type with public metadata fields and private extraction refs. Implement `collectImageInfos` — the walker that gathers metadata without decoding. Add `Page.ImageInfos()` and `Document.ImageInfos()`.

**Files:**
- Modify: `image.go`
- Modify: `image_internal_test.go`

- [ ] **Step 1: Write failing unit test for ImageInfo metadata**

Add to `image_internal_test.go`:

```go
func TestImageInfoMetadata(t *testing.T) {
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x02, 0xFF, 0xD9}

	imgStream := &pdfStream{
		Dict: pdfDict{
			"/Subtype":         pdfName("/Image"),
			"/Width":           100,
			"/Height":          80,
			"/BitsPerComponent": 8,
			"/ColorSpace":      pdfName("/DeviceRGB"),
			"/Filter":          pdfName("/DCTDecode"),
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

	// Build a content stream: q 200 0 0 160 72 500 cm /Im0 Do Q
	ops := []contentOp{
		{Operator: "q"},
		{Operator: "cm", Operands: []pdfValue{200.0, 0.0, 0.0, 160.0, 72.0, 500.0}},
		{Operator: "Do", Operands: []pdfValue{pdfName("Im0")}},
		{Operator: "Q"},
	}

	infos := collectImageInfos(objects, ops, resources)
	if len(infos) != 1 {
		t.Fatalf("got %d infos, want 1", len(infos))
	}
	info := infos[0]
	if info.Width != 100 || info.Height != 80 {
		t.Errorf("dimensions = %dx%d, want 100x80", info.Width, info.Height)
	}
	if info.BPC != 8 {
		t.Errorf("BPC = %d, want 8", info.BPC)
	}
	if info.ColorSpace != ColorSpaceDeviceRGB {
		t.Errorf("colorSpace = %d, want DeviceRGB", info.ColorSpace)
	}
	if info.Format != ImageFormatJPEG {
		t.Errorf("format = %d, want ImageFormatJPEG", info.Format)
	}
	if info.Name != "/Im0" {
		t.Errorf("name = %q, want /Im0", info.Name)
	}
	if info.X != 72 || info.Y != 500 {
		t.Errorf("position = (%g, %g), want (72, 500)", info.X, info.Y)
	}
	if info.PageWidth != 200 || info.PageHeight != 160 {
		t.Errorf("page size = (%g, %g), want (200, 160)", info.PageWidth, info.PageHeight)
	}
	if info.Inline {
		t.Error("expected Inline=false")
	}
}

func TestImageInfoFlateDecode(t *testing.T) {
	pixels := make([]byte, 10*10*3)
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
			"/Im1": pdfRef{Num: 1},
		},
	}

	ops := []contentOp{
		{Operator: "Do", Operands: []pdfValue{pdfName("Im1")}},
	}

	infos := collectImageInfos(objects, ops, resources)
	if len(infos) != 1 {
		t.Fatalf("got %d infos, want 1", len(infos))
	}
	if infos[0].Format != ImageFormatPNG {
		t.Errorf("format = %d, want ImageFormatPNG", infos[0].Format)
	}
	if infos[0].Name != "/Im1" {
		t.Errorf("name = %q, want /Im1", infos[0].Name)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run "TestImageInfoMetadata|TestImageInfoFlateDecode" -v ./... 2>&1 | head -10`
Expected: FAIL — `collectImageInfos` undefined.

- [ ] **Step 3: Implement ImageInfo type and collectImageInfos**

In `image.go`, add the `ImageInfo` type after the `Image` type (around line 40):

```go
// ImageInfo holds metadata about an image found on a page without decoding pixel data.
// Call Extract() to perform the actual decoding and get the full Image.
type ImageInfo struct {
	Width      int             // pixel width
	Height     int             // pixel height
	BPC        int             // bits per component (original)
	ColorSpace ImageColorSpace // original PDF color space
	Format     ImageFormat     // output format (PNG or JPEG)
	X, Y       float64         // position on page (lower-left, in points)
	PageWidth  float64         // display width on page (in points)
	PageHeight float64         // display height on page (in points)
	Inline     bool            // true if from inline image (BI/ID/EI)
	Name       string          // XObject name (e.g. "/Im0"); empty for inline

	// private — for deferred extraction
	objects map[int]*pdfObject
	stream  *pdfStream
	formVal pdfValue
	dict    pdfDict  // inline: normalized dict
	rawData []byte   // inline: raw image bytes
	ctm     [6]float64
}
```

Add `collectImageInfos` — replaces `extractImagesFromOps` with metadata-only collection:

```go
// collectImageInfos walks content stream ops, tracking CTM, and collects image metadata
// without decoding pixel data.
func collectImageInfos(objects map[int]*pdfObject, ops []contentOp, resources pdfDict) []ImageInfo {
	var infos []ImageInfo
	ctm := identityMatrix()
	var ctmStack [][6]float64

	for _, op := range ops {
		switch op.Operator {
		case "cm":
			if len(op.Operands) >= 6 {
				var m [6]float64
				for i := 0; i < 6; i++ {
					m[i] = operandFloat(op.Operands[i])
				}
				ctm = matMul(m, ctm)
			}
		case "q":
			ctmStack = append(ctmStack, ctm)
		case "Q":
			if len(ctmStack) > 0 {
				ctm = ctmStack[len(ctmStack)-1]
				ctmStack = ctmStack[:len(ctmStack)-1]
			}
		case "Do":
			if len(op.Operands) >= 1 {
				name := operandName(op.Operands[0])
				if info, ok := xobjectImageInfo(objects, resources, name, ctm); ok {
					infos = append(infos, info)
				} else {
					formInfos := formXObjectImageInfos(objects, resources, name, ctm)
					infos = append(infos, formInfos...)
				}
			}
		case "BI":
			if len(op.Operands) >= 2 {
				if info, ok := inlineImageInfo(op.Operands[0], op.Operands[1], ctm); ok {
					infos = append(infos, info)
				}
			}
		}
	}
	return infos
}
```

Add `xobjectImageInfo` — metadata-only version of `extractXObjectImage`:

```go
// xobjectImageInfo collects metadata for an XObject image without decoding pixels.
func xobjectImageInfo(objects map[int]*pdfObject, resources pdfDict, name string, ctm [6]float64) (ImageInfo, bool) {
	if name == "" || resources == nil {
		return ImageInfo{}, false
	}
	xobjVal, ok := resources["/XObject"]
	if !ok {
		return ImageInfo{}, false
	}
	xobjDict, ok := resolveRefToDict(objects, xobjVal)
	if !ok {
		return ImageInfo{}, false
	}
	formVal, ok := xobjDict[name]
	if !ok {
		return ImageInfo{}, false
	}
	resolved := resolveRef(objects, formVal)
	stream, ok := resolved.(*pdfStream)
	if !ok {
		return ImageInfo{}, false
	}
	if dictGetName(stream.Dict, "/Subtype") != "/Image" {
		return ImageInfo{}, false
	}

	width := dictGetInt(stream.Dict, "/Width")
	height := dictGetInt(stream.Dict, "/Height")
	bpc := dictGetInt(stream.Dict, "/BitsPerComponent")
	if width <= 0 || height <= 0 {
		return ImageInfo{}, false
	}

	cs := resolveColorSpace(objects, stream.Dict)
	filter := primaryFilter(stream.Dict)

	// Determine output format.
	format := ImageFormatPNG
	if filter == "/DCTDecode" {
		format = ImageFormatJPEG
		// JPEG with soft mask must be re-encoded as PNG.
		if _, hasSMask := stream.Dict["/SMask"]; hasSMask {
			format = ImageFormatPNG
		}
	}

	return ImageInfo{
		Width:      width,
		Height:     height,
		BPC:        bpc,
		ColorSpace: cs,
		Format:     format,
		X:          ctm[4],
		Y:          ctm[5],
		PageWidth:  math.Sqrt(ctm[0]*ctm[0] + ctm[1]*ctm[1]),
		PageHeight: math.Sqrt(ctm[2]*ctm[2] + ctm[3]*ctm[3]),
		Name:       name,
		objects:    objects,
		stream:     stream,
		formVal:    formVal,
		ctm:        ctm,
	}, true
}
```

Add `inlineImageInfo` — metadata-only for inline images:

```go
// inlineImageInfo collects metadata for an inline image without decoding pixels.
func inlineImageInfo(dictVal, dataVal pdfValue, ctm [6]float64) (ImageInfo, bool) {
	dict, ok := dictVal.(pdfDict)
	if !ok {
		return ImageInfo{}, false
	}
	rawData, ok := dataVal.(string)
	if !ok {
		return ImageInfo{}, false
	}

	width := dictGetInt(dict, "/Width")
	height := dictGetInt(dict, "/Height")
	bpc := dictGetInt(dict, "/BitsPerComponent")
	if width <= 0 || height <= 0 {
		return ImageInfo{}, false
	}
	if bpc == 0 {
		bpc = 8
	}

	cs := resolveColorSpaceInline(dict)
	filter := primaryFilter(dict)

	format := ImageFormatPNG
	if filter == "/DCTDecode" {
		format = ImageFormatJPEG
	}

	return ImageInfo{
		Width:      width,
		Height:     height,
		BPC:        bpc,
		ColorSpace: cs,
		Format:     format,
		X:          ctm[4],
		Y:          ctm[5],
		PageWidth:  math.Sqrt(ctm[0]*ctm[0] + ctm[1]*ctm[1]),
		PageHeight: math.Sqrt(ctm[2]*ctm[2] + ctm[3]*ctm[3]),
		Inline:     true,
		dict:       dict,
		rawData:    []byte(rawData),
		ctm:        ctm,
	}, true
}
```

Add `formXObjectImageInfos` — Form XObject recursion for metadata:

```go
// formXObjectImageInfos collects image metadata from a Form XObject's content stream.
func formXObjectImageInfos(objects map[int]*pdfObject, resources pdfDict, name string, ctm [6]float64) []ImageInfo {
	if name == "" || resources == nil {
		return nil
	}
	xobjVal, ok := resources["/XObject"]
	if !ok {
		return nil
	}
	xobjDict, ok := resolveRefToDict(objects, xobjVal)
	if !ok {
		return nil
	}
	formVal, ok := xobjDict[name]
	if !ok {
		return nil
	}
	resolved := resolveRef(objects, formVal)
	stream, ok := resolved.(*pdfStream)
	if !ok {
		return nil
	}
	if dictGetName(stream.Dict, "/Subtype") != "/Form" {
		return nil
	}

	var data []byte
	if stream.Decoded {
		data = stream.Data
	} else {
		var err error
		data, err = decodeStream(stream.Dict, stream.Data)
		if err != nil {
			return nil
		}
	}

	ops, err := parseContentStream(data)
	if err != nil {
		return nil
	}

	formCTM := ctm
	if matVal, ok := stream.Dict["/Matrix"]; ok {
		if arr, ok := matVal.(pdfArray); ok && len(arr) == 6 {
			var fm [6]float64
			for i := 0; i < 6; i++ {
				fm[i] = operandFloat(arr[i])
			}
			formCTM = matMul(fm, ctm)
		}
	}

	formResources := resources
	if resVal, ok := stream.Dict["/Resources"]; ok {
		if rd, ok := resolveRefToDict(objects, resVal); ok {
			formResources = rd
		}
	}

	infos := collectImageInfos(objects, ops, formResources)
	for i := range infos {
		infos[i].X += formCTM[4]
		infos[i].Y += formCTM[5]
	}
	return infos
}
```

Add `Page.ImageInfos()` and `Document.ImageInfos()`:

```go
// ImageInfos returns metadata for all images found on the page without decoding pixel data.
func (p *Page) ImageInfos() ([]ImageInfo, error) {
	data, err := p.contentStreams()
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}

	ops, err := parseContentStream(data)
	if err != nil {
		return nil, err
	}

	resources := p.pageResources()
	return collectImageInfos(p.doc.objects, ops, resources), nil
}

// ImageInfos returns image metadata for all pages (one slice per page) without decoding pixel data.
func (d *Document) ImageInfos() ([][]ImageInfo, error) {
	pages := d.Pages()
	result := make([][]ImageInfo, len(pages))
	for i, p := range pages {
		infos, err := p.ImageInfos()
		if err != nil {
			return nil, err
		}
		result[i] = infos
	}
	return result, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run "TestImageInfoMetadata|TestImageInfoFlateDecode" -v ./... 2>&1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add image.go image_internal_test.go
git commit -m "feat: add ImageInfo type with metadata-only collection"
```

---

### Task 2: ImageInfo.Extract() and refactor ExtractImages

Implement `Extract()` on `ImageInfo` using existing decode logic. Refactor `ExtractImages` to delegate to `ImageInfos` + `Extract`. Remove `extractImagesFromOps` and `extractFormXObjectImages`.

**Files:**
- Modify: `image.go`
- Modify: `image_internal_test.go`

- [ ] **Step 1: Write failing unit test for Extract()**

Add to `image_internal_test.go`:

```go
func TestImageInfoExtract(t *testing.T) {
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x02, 0xFF, 0xD9}

	imgStream := &pdfStream{
		Dict: pdfDict{
			"/Subtype":         pdfName("/Image"),
			"/Width":           100,
			"/Height":          80,
			"/BitsPerComponent": 8,
			"/ColorSpace":      pdfName("/DeviceRGB"),
			"/Filter":          pdfName("/DCTDecode"),
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

	ops := []contentOp{
		{Operator: "Do", Operands: []pdfValue{pdfName("Im0")}},
	}

	infos := collectImageInfos(objects, ops, resources)
	if len(infos) != 1 {
		t.Fatalf("got %d infos, want 1", len(infos))
	}

	img, err := infos[0].Extract()
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if img.Format != ImageFormatJPEG {
		t.Errorf("format = %d, want ImageFormatJPEG", img.Format)
	}
	if len(img.Data) != len(jpegData) {
		t.Errorf("data len = %d, want %d", len(img.Data), len(jpegData))
	}
	if img.Width != 100 || img.Height != 80 {
		t.Errorf("dimensions = %dx%d, want 100x80", img.Width, img.Height)
	}
}

func TestImageInfoExtractPNG(t *testing.T) {
	pixels := make([]byte, 10*10*3)
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

	ops := []contentOp{
		{Operator: "Do", Operands: []pdfValue{pdfName("Im0")}},
	}

	infos := collectImageInfos(objects, ops, resources)
	if len(infos) != 1 {
		t.Fatalf("got %d infos, want 1", len(infos))
	}

	img, err := infos[0].Extract()
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if img.Format != ImageFormatPNG {
		t.Errorf("format = %d, want ImageFormatPNG", img.Format)
	}
	if len(img.Data) == 0 {
		t.Error("expected non-empty PNG data")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run "TestImageInfoExtract" -v ./... 2>&1 | head -10`
Expected: FAIL — `Extract` method not defined on `ImageInfo`.

- [ ] **Step 3: Implement Extract() method**

Add to `image.go`:

```go
// Extract decodes the image and returns the full Image with pixel data.
func (info *ImageInfo) Extract() (*Image, error) {
	img := &Image{
		Width:      info.Width,
		Height:     info.Height,
		BPC:        info.BPC,
		ColorSpace: info.ColorSpace,
		Format:     info.Format,
		X:          info.X,
		Y:          info.Y,
		PageWidth:  info.PageWidth,
		PageHeight: info.PageHeight,
		Inline:     info.Inline,
	}

	if info.Inline {
		return extractInlineImageData(img, info.dict, info.rawData)
	}
	return extractXObjectImageData(img, info.objects, info.stream, info.formVal)
}

// extractXObjectImageData decodes an XObject image stream into an Image.
func extractXObjectImageData(img *Image, objects map[int]*pdfObject, stream *pdfStream, formVal pdfValue) (*Image, error) {
	filter := primaryFilter(stream.Dict)

	if filter == "/DCTDecode" {
		// Check for soft mask — JPEG can't hold alpha, must re-encode as PNG.
		if smaskVal, ok := stream.Dict["/SMask"]; ok {
			alphaMask := decodeSoftMask(objects, smaskVal)
			if alphaMask != nil {
				jpegData := stream.Data
				if stream.Decoded {
					jpegData = getRawStreamData(objects, formVal)
				}
				if jpegData == nil {
					return nil, fmt.Errorf("cannot read JPEG data for re-encoding")
				}
				pixels, _, _, err := decodeJPEGToPixels(jpegData)
				if err != nil {
					return nil, err
				}
				pngData, err := encodePNG(pixels, img.Width, img.Height, 8, 3, alphaMask)
				if err != nil {
					return nil, err
				}
				img.Data = pngData
				img.Format = ImageFormatPNG
				return img, nil
			}
		}

		// No alpha — JPEG passthrough.
		img.Data = stream.Data
		if stream.Decoded {
			img.Data = getRawStreamData(objects, formVal)
			if img.Data == nil {
				return nil, fmt.Errorf("cannot read raw JPEG data")
			}
		}
		img.Format = ImageFormatJPEG
		return img, nil
	}

	// Decode pixels and encode as PNG.
	var rawPixels []byte
	if stream.Decoded {
		rawPixels = stream.Data
	} else {
		var err error
		rawPixels, err = decodeStream(stream.Dict, stream.Data)
		if err != nil {
			return nil, err
		}
	}

	bpc := img.BPC
	components := colorSpaceComponents(objects, stream.Dict, img.ColorSpace)
	if bpc == 0 {
		bpc = 8
	}

	if img.ColorSpace == ColorSpaceIndexed {
		palette, baseComponents := resolveIndexedPalette(objects, stream.Dict)
		rawPixels = expandIndexed(rawPixels, palette, baseComponents)
		components = baseComponents
	}

	var alphaMask []byte
	if smaskVal, ok := stream.Dict["/SMask"]; ok {
		alphaMask = decodeSoftMask(objects, smaskVal)
	}

	pngData, err := encodePNG(rawPixels, img.Width, img.Height, bpc, components, alphaMask)
	if err != nil {
		return nil, err
	}

	img.Data = pngData
	img.Format = ImageFormatPNG
	return img, nil
}

// extractInlineImageData decodes an inline image into an Image.
func extractInlineImageData(img *Image, dict pdfDict, rawData []byte) (*Image, error) {
	filter := primaryFilter(dict)
	data := rawData

	if filter == "/DCTDecode" {
		img.Data = data
		img.Format = ImageFormatJPEG
		return img, nil
	}

	if filter != "" {
		var err error
		data, err = applyFilter(filter, data)
		if err != nil {
			return nil, err
		}
	}

	components := componentsByCS(img.ColorSpace)
	bpc := img.BPC
	if bpc == 0 {
		bpc = 8
	}

	pngData, err := encodePNG(data, img.Width, img.Height, bpc, components, nil)
	if err != nil {
		return nil, err
	}
	img.Data = pngData
	img.Format = ImageFormatPNG
	return img, nil
}
```

Add `"fmt"` to the import list of `image.go` if not already present.

- [ ] **Step 4: Refactor ExtractImages to use ImageInfos + Extract**

Replace the existing `Page.ExtractImages()` method with:

```go
// ExtractImages returns all images found on the page.
func (p *Page) ExtractImages() ([]Image, error) {
	infos, err := p.ImageInfos()
	if err != nil {
		return nil, err
	}
	var images []Image
	for i := range infos {
		img, err := infos[i].Extract()
		if err != nil {
			continue
		}
		images = append(images, *img)
	}
	return images, nil
}
```

Remove the old functions that are no longer needed:
- `extractImagesFromOps` (replaced by `collectImageInfos`)
- `extractXObjectImage` (replaced by `xobjectImageInfo` + `extractXObjectImageData`)
- `extractInlineImage` (replaced by `inlineImageInfo` + `extractInlineImageData`)
- `extractFormXObjectImages` (replaced by `formXObjectImageInfos`)

- [ ] **Step 5: Run all tests**

Run: `go test ./... 2>&1`
Expected: PASS — all existing tests plus new tests pass.

- [ ] **Step 6: Commit**

```bash
git add image.go image_internal_test.go
git commit -m "feat: add ImageInfo.Extract() and refactor ExtractImages to use ImageInfos"
```

---

### Task 3: Integration test and docs update

Add integration test, update CLAUDE.md and README.md.

**Files:**
- Modify: `image_test.go`
- Modify: `CLAUDE.md`
- Modify: `README.md`

- [ ] **Step 1: Write integration test**

Add to `image_test.go`:

```go
func TestImageInfos(t *testing.T) {
	groups := testGroups(t)
	for _, group := range groups {
		path := group[0]
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		t.Run(name, func(t *testing.T) {
			doc, err := asposepdf.Open(path)
			if err != nil {
				t.Fatalf("open: %v", err)
			}

			// Get infos (lightweight).
			allInfos, err := doc.ImageInfos()
			if err != nil {
				t.Fatalf("ImageInfos: %v", err)
			}

			// Get images (full extraction) for comparison.
			allImages, err := doc.ExtractImages()
			if err != nil {
				t.Fatalf("ExtractImages: %v", err)
			}

			if len(allInfos) != len(allImages) {
				t.Fatalf("page count: infos=%d, images=%d", len(allInfos), len(allImages))
			}

			for pageIdx := range allInfos {
				if len(allInfos[pageIdx]) != len(allImages[pageIdx]) {
					t.Errorf("page %d: infos=%d, images=%d",
						pageIdx+1, len(allInfos[pageIdx]), len(allImages[pageIdx]))
					continue
				}
				for imgIdx, info := range allInfos[pageIdx] {
					img := allImages[pageIdx][imgIdx]
					if info.Width != img.Width || info.Height != img.Height {
						t.Errorf("page %d img %d: info=%dx%d, image=%dx%d",
							pageIdx+1, imgIdx+1, info.Width, info.Height, img.Width, img.Height)
					}
					if info.Format != img.Format {
						t.Errorf("page %d img %d: info format=%d, image format=%d",
							pageIdx+1, imgIdx+1, info.Format, img.Format)
					}
				}
			}

			// Verify Extract() works on each info.
			totalExtracted := 0
			for _, infos := range allInfos {
				for i := range infos {
					extracted, err := infos[i].Extract()
					if err != nil {
						t.Errorf("Extract: %v", err)
						continue
					}
					if len(extracted.Data) == 0 {
						t.Error("extracted image has empty data")
					}
					totalExtracted++
				}
			}
			t.Logf("%s: %d infos, all extracted successfully", name, totalExtracted)
		})
	}
}
```

Add `"TestImageInfos"` entry to `testdata/testfiles.json` using the same files as `TestExtractImages`:

```json
"TestImageInfos": [
  ["PdfWithImages.pdf"],
  ["PdfWithImages2.pdf"]
]
```

- [ ] **Step 2: Run integration test**

Run: `go test -run TestImageInfos -v ./... 2>&1`
Expected: PASS.

- [ ] **Step 3: Update CLAUDE.md**

In the Public API section under `page.go`, after the `ExtractImages` entries, add:

```
- `ImageInfo` struct — Width, Height, BPC, ColorSpace, Format, X, Y, PageWidth, PageHeight, Inline, Name
- `(*ImageInfo).Extract() (*Image, error)` — decodes the image and returns the full Image with pixel data
- `(*Page).ImageInfos() ([]ImageInfo, error)` — returns metadata for all images without decoding
- `(*Document).ImageInfos() ([][]ImageInfo, error)` — returns image metadata for all pages without decoding
```

- [ ] **Step 4: Update README.md**

In the Image Extraction section, after the existing `ExtractImages` example and before the description paragraph, add:

```go
// List image metadata without decoding (fast)
allInfos, err := doc.ImageInfos()
for pageIdx, infos := range allInfos {
    for _, info := range infos {
        fmt.Printf("page %d: %dx%d %s %s\n",
            pageIdx+1, info.Width, info.Height, info.Name, info.ColorSpace)
    }
}

// Selectively extract only large images
for _, infos := range allInfos {
    for i, info := range infos {
        if info.Width >= 500 {
            img, _ := infos[i].Extract()
            img.Save(fmt.Sprintf("large_%s.png", info.Name))
        }
    }
}
```

- [ ] **Step 5: Run all tests**

Run: `go test ./... 2>&1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add image_test.go testdata/testfiles.json CLAUDE.md README.md
git commit -m "docs: add ImageInfo integration test, update CLAUDE.md and README"
```
