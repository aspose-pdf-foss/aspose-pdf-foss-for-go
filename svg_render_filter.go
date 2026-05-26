// SPDX-License-Identifier: MIT

package asposepdf

import (
	"bytes"
	"fmt"
)

// applySVGFilter, when style.filter references a known filter with a feDropShadow
// primitive, emits a shadow rect of the shape's bbox at offset (dx, dy) with the
// flood color and opacity BEFORE the original shape's content. Other filter
// primitives are silently skipped (no PDF mapping without rasterization).
//
// This is a degraded approximation — real SVG drop-shadow follows the silhouette
// and adds Gaussian blur; we emit a flat-color bbox-shaped shadow.
func applySVGFilter(buf *bytes.Buffer, p *Page, svg *SVG, style svgStyle, shape svgNode) {
	if style.filter == "" || svg == nil {
		return
	}
	f, ok := svg.defs[style.filter].(*svgFilter)
	if !ok {
		return
	}
	ds := f.findDropShadow()
	if ds == nil {
		return
	}
	x0, y0, x1, y1 := svgShapeBBox(shape)
	if x1-x0 <= 0 || y1-y0 <= 0 {
		return // no bbox — can't render shadow
	}
	color := ds.floodColor
	if color == nil {
		color = &Color{R: 0, G: 0, B: 0, A: 1}
	}
	// Combine flood-opacity with color's alpha
	alpha := ds.floodOpacity * color.A
	buf.WriteString("q\n")
	if alpha < 1 {
		if gsName, err := p.ensureExtGState(alpha); err == nil {
			fmt.Fprintf(buf, "%s gs\n", gsName)
		}
	}
	fmt.Fprintf(buf, "%s %s %s rg\n",
		formatFloat(color.R), formatFloat(color.G), formatFloat(color.B))
	fmt.Fprintf(buf, "%s %s %s %s re f\n",
		formatFloat(x0+ds.dx),
		formatFloat(y0+ds.dy),
		formatFloat(x1-x0),
		formatFloat(y1-y0))
	buf.WriteString("Q\n")
}
