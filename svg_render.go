// SPDX-License-Identifier: MIT

package asposepdf

import (
	"bytes"
	"fmt"
)

// renderSVG renders a parsed *SVG into the given Page within rect.
// Best-effort: unsupported nodes are silently skipped (already filtered at parse time).
func renderSVG(p *Page, svg *SVG, rect Rectangle) error {
	if svg == nil || svg.root == nil {
		return nil
	}
	if rect.URX-rect.LLX <= 0 || rect.URY-rect.LLY <= 0 {
		return nil
	}

	var buf bytes.Buffer
	outer := computeViewBoxMatrix(svg.viewBox, svg.width, svg.height, svg.par, rect)

	buf.WriteString("q\n")
	writeCMOperator(&buf, outer)
	renderSVGNodes(&buf, p, svg.root.children, svg.root.style)
	buf.WriteString("Q\n")

	return p.appendToContentStream(buf.Bytes())
}

// writeCMOperator writes a PDF `cm` operator (concat matrix to current CTM) from svgMatrix.
func writeCMOperator(buf *bytes.Buffer, m svgMatrix) {
	fmt.Fprintf(buf, "%s %s %s %s %s %s cm\n",
		formatFloat(m[0]), formatFloat(m[1]),
		formatFloat(m[2]), formatFloat(m[3]),
		formatFloat(m[4]), formatFloat(m[5]))
}

func renderSVGNodes(buf *bytes.Buffer, p *Page, nodes []svgNode, parentStyle svgStyle) {
	for _, n := range nodes {
		renderSVGNode(buf, p, n, parentStyle)
	}
}

func renderSVGNode(buf *bytes.Buffer, p *Page, n svgNode, parentStyle svgStyle) {
	switch node := n.(type) {
	case *svgGroup:
		renderSVGGroup(buf, p, node)
	case *svgRect:
		renderSVGRect(buf, p, node)
	case *svgCircle:
		renderSVGCircle(buf, p, node)
	case *svgEllipse:
		renderSVGEllipse(buf, p, node)
	case *svgLine:
		renderSVGLine(buf, p, node)
	case *svgPolyline:
		renderSVGPolyline(buf, p, node)
	case *svgPolygon:
		renderSVGPolygon(buf, p, node)
	case *svgPath:
		renderSVGPath(buf, p, node)
	}
}

func renderSVGGroup(buf *bytes.Buffer, p *Page, g *svgGroup) {
	if !g.style.display {
		return
	}
	buf.WriteString("q\n")
	if g.transform != nil {
		writeCMOperator(buf, *g.transform)
	}
	if err := applyGroupOpacity(buf, p, g.style); err != nil {
		// best-effort: skip opacity on error
		_ = err
	}
	renderSVGNodes(buf, p, g.children, g.style)
	buf.WriteString("Q\n")
}

// Shape rendering stubs — filled in by Task 12.
func renderSVGRect(buf *bytes.Buffer, p *Page, r *svgRect)          {}
func renderSVGCircle(buf *bytes.Buffer, p *Page, c *svgCircle)      {}
func renderSVGEllipse(buf *bytes.Buffer, p *Page, e *svgEllipse)    {}
func renderSVGLine(buf *bytes.Buffer, p *Page, l *svgLine)          {}
func renderSVGPolyline(buf *bytes.Buffer, p *Page, pl *svgPolyline) {}
func renderSVGPolygon(buf *bytes.Buffer, p *Page, pg *svgPolygon)   {}
func renderSVGPath(buf *bytes.Buffer, p *Page, sp *svgPath)         {}

// applyGroupOpacity emits a `/GSx gs` operator if the group has opacity < 1.
// Returns any error from ensureExtGState.
func applyGroupOpacity(buf *bytes.Buffer, p *Page, s svgStyle) error {
	if s.opacity < 1 {
		gsName, err := p.ensureExtGState(s.opacity)
		if err != nil {
			return err
		}
		fmt.Fprintf(buf, "/%s gs\n", gsName)
	}
	return nil
}
