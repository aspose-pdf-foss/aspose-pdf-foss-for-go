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

