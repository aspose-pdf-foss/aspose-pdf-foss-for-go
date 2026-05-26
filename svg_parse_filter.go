// SPDX-License-Identifier: MIT

package asposepdf

import "encoding/xml"

// parseSVGFilter reads a <filter> element. Children are SVG filter primitives.
// Phase 3d only stores feDropShadow attrs; other primitives are stored with kind
// but no detailed attrs (silently skipped at render).
func parseSVGFilter(d *xml.Decoder, svg *SVG, parent *svgGroup, start xml.StartElement) (*svgFilter, error) {
	f := &svgFilter{}
	for {
		tok, err := d.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.EndElement:
			return f, nil
		case xml.StartElement:
			prim := svgFilterPrimitive{kind: t.Name.Local, floodOpacity: 1}
			for _, a := range t.Attr {
				switch a.Name.Local {
				case "dx":
					prim.dx, _ = parseSVGLength(a.Value)
				case "dy":
					prim.dy, _ = parseSVGLength(a.Value)
				case "flood-color":
					if c, ok := parseSVGColor(a.Value); ok && c != nil {
						prim.floodColor = c
					}
				case "flood-opacity":
					if v, ok := parseSVGNumber(a.Value); ok {
						prim.floodOpacity = clamp01(v)
					}
				}
			}
			f.primitives = append(f.primitives, prim)
			_ = d.Skip()
		}
	}
}
