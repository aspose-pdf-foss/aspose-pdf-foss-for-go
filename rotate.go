package asposepdf

import "fmt"

// RotationAngle represents a valid PDF page rotation in clockwise degrees.
// Only the values defined as constants (Rotate90, Rotate180, Rotate270) are valid.
type RotationAngle int

const (
	// Rotate0 is the default orientation (no rotation).
	Rotate0 RotationAngle = 0
	// Rotate90 rotates a page 90 degrees clockwise.
	Rotate90 RotationAngle = 90
	// Rotate180 rotates a page 180 degrees (upside down).
	Rotate180 RotationAngle = 180
	// Rotate270 rotates a page 270 degrees clockwise (90 degrees counter-clockwise).
	Rotate270 RotationAngle = 270
)

// validate returns an error if a is not a valid PDF rotation angle.
func (a RotationAngle) validate() error {
	if a != Rotate0 && a != Rotate90 && a != Rotate180 && a != Rotate270 {
		return fmt.Errorf("angle must be Rotate0, Rotate90, Rotate180, or Rotate270; got %d", a)
	}
	return nil
}

// Rotate applies a clockwise rotation to selected pages of inputPath and saves the result to outputPath.
// The rotation is added to any existing page rotation (mod 360).
// If no page numbers are provided, all pages are rotated. Page numbers are 1-based.
//
// Example — rotate all pages 90 degrees:
//
//	err := asposepdf.Rotate("input.pdf", "output.pdf", asposepdf.Rotate90)
//
// Example — rotate only pages 1 and 3:
//
//	err := asposepdf.Rotate("input.pdf", "output.pdf", asposepdf.Rotate180, 1, 3)
func Rotate(inputPath, outputPath string, angle RotationAngle, pageNums ...int) error {
	if err := angle.validate(); err != nil {
		return err
	}

	doc, err := openDocument(inputPath)
	if err != nil {
		return fmt.Errorf("open PDF: %w", err)
	}

	pages, err := doc.pages()
	if err != nil {
		return fmt.Errorf("read pages: %w", err)
	}

	// Build a set of 0-based indices to rotate.
	toRotate := make(map[int]bool)
	if len(pageNums) == 0 {
		for i := range pages {
			toRotate[i] = true
		}
	} else {
		for _, n := range pageNums {
			if n < 1 || n > len(pages) {
				return fmt.Errorf("page number %d out of range (1..%d)", n, len(pages))
			}
			toRotate[n-1] = true
		}
	}

	// Build per-page patches with the new /Rotate value.
	pagePatches := make(map[int]pdfDict)
	for i, p := range pages {
		if !toRotate[i] {
			continue
		}
		existing, err := pageRotation(doc, p)
		if err != nil {
			return err
		}
		pagePatches[p.objNum] = pdfDict{"/Rotate": (int(existing) + int(angle)) % 360}
	}

	data, err := buildMultiPagePDFEx(doc, pages, pagePatches)
	if err != nil {
		return err
	}
	return writeFile(outputPath, data)
}

// pageRotation returns the current /Rotate value for a page (defaults to 0 if absent).
func pageRotation(doc *rawDocument, p *pageInfo) (RotationAngle, error) {
	obj, err := doc.getObject(p.objNum)
	if err != nil {
		return 0, fmt.Errorf("get page object %d: %w", p.objNum, err)
	}
	d, ok := obj.Value.(pdfDict)
	if !ok {
		return 0, nil
	}
	return RotationAngle(dictGetInt(d, "/Rotate")), nil
}
