// SPDX-License-Identifier: MIT

package asposepdf

type svgFilterPrimitive struct {
	kind         string // e.g. "feDropShadow", "feGaussianBlur" (others ignored at render time)
	dx, dy       float64
	floodColor   *Color
	floodOpacity float64
}

type svgFilter struct {
	primitives []svgFilterPrimitive
}

func (*svgFilter) svgNodeKind() string { return "filter" }

// findDropShadow returns the first feDropShadow primitive in this filter,
// or nil if none. Phase 3d only renders feDropShadow; other primitives are
// parsed for future use but silently skipped at render time.
func (f *svgFilter) findDropShadow() *svgFilterPrimitive {
	for i := range f.primitives {
		if f.primitives[i].kind == "feDropShadow" {
			return &f.primitives[i]
		}
	}
	return nil
}
