// SPDX-License-Identifier: MIT

package asposepdf

import (
	"math"
	"strings"
)

func parseViewBox(s string) (svgViewBox, bool) {
	nums, ok := parseSVGNumberList(s)
	if !ok || len(nums) != 4 {
		return svgViewBox{}, false
	}
	return svgViewBox{nums[0], nums[1], nums[2], nums[3]}, true
}

func parsePreserveAspect(s string) svgPreserveAspect {
	s = strings.TrimSpace(s)
	if s == "" {
		return svgPreserveAspect{align: "xMidYMid", meetOrSlice: "meet"}
	}
	parts := strings.Fields(s)
	p := svgPreserveAspect{align: "xMidYMid", meetOrSlice: "meet"}
	if len(parts) >= 1 {
		p.align = parts[0]
	}
	if len(parts) >= 2 {
		p.meetOrSlice = parts[1]
	}
	return p
}

// computeViewBoxMatrix returns the CTM that maps SVG content (in viewBox units, Y-down)
// into the user-supplied Rectangle (PDF user space, Y-up).
func computeViewBoxMatrix(viewBox *svgViewBox, intrinsicW, intrinsicH float64, par svgPreserveAspect, rect Rectangle) svgMatrix {
	var srcX, srcY, srcW, srcH float64
	if viewBox != nil {
		srcX, srcY, srcW, srcH = viewBox.x, viewBox.y, viewBox.w, viewBox.h
	} else if intrinsicW > 0 && intrinsicH > 0 {
		srcW, srcH = intrinsicW, intrinsicH
	} else {
		srcW = rect.URX - rect.LLX
		srcH = rect.URY - rect.LLY
	}
	if srcW <= 0 || srcH <= 0 {
		return matrixIdentity()
	}

	dstW := rect.URX - rect.LLX
	dstH := rect.URY - rect.LLY

	var scaleX, scaleY float64
	if par.align == "none" {
		scaleX = dstW / srcW
		scaleY = dstH / srcH
	} else {
		sx := dstW / srcW
		sy := dstH / srcH
		var s float64
		if par.meetOrSlice == "slice" {
			s = math.Max(sx, sy)
		} else {
			s = math.Min(sx, sy)
		}
		scaleX, scaleY = s, s
	}

	renderW := srcW * scaleX
	renderH := srcH * scaleY

	var alignX, alignY float64
	switch {
	case strings.HasPrefix(par.align, "xMin"):
		alignX = 0
	case strings.HasPrefix(par.align, "xMax"):
		alignX = dstW - renderW
	default:
		alignX = (dstW - renderW) / 2
	}
	switch {
	case strings.HasSuffix(par.align, "YMin"):
		// "YMin" = top in SVG; after Y-flip, that's URY in PDF (highest Y).
		alignY = dstH - renderH
	case strings.HasSuffix(par.align, "YMax"):
		alignY = 0
	default:
		alignY = (dstH - renderH) / 2
	}

	// Composite: 1) translate by -srcX,-srcY  2) scale(scaleX, -scaleY) [Y-flip]
	//            3) translate to rect.LLX+alignX, rect.LLY+alignY+renderH
	t1 := matrixTranslate(-srcX, -srcY)
	s := svgMatrix{scaleX, 0, 0, -scaleY, 0, 0}
	t2 := matrixTranslate(rect.LLX+alignX, rect.LLY+alignY+renderH)
	return matrixMul(t2, matrixMul(s, t1))
}
