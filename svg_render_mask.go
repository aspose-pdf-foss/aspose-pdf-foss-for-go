// SPDX-License-Identifier: MIT

package asposepdf

import (
	"bytes"
)

// buildMaskFormXObject renders the mask's children into a PDF Form XObject
// with /Type /XObject /Subtype /Form /Group <</Type /Group /S /Transparency /CS /DeviceGray>>.
// Returns a pdfRef to the indirect Form XObject.
//
// bbox is the bounding box for the mask in user-space units (typically the bbox of the
// shape being masked, OR the page MediaBox for userSpaceOnUse masks).
//
// The mask's color space is DeviceGray (luminance-based per SVG spec).
// White (1.0) = fully visible, black (0.0) = fully hidden.
func buildMaskFormXObject(p *Page, svg *SVG, mask *svgMask, bbox Rectangle) (pdfRef, error) {
	if mask == nil || len(mask.children) == 0 {
		return pdfRef{}, nil // caller treats Num==0 as "no mask"
	}

	// Render mask children into a content stream buffer.
	var buf bytes.Buffer
	renderSVGNodes(&buf, p, svg, mask.children, defaultSVGStyle())

	// Build the /Resources entry: share page resources so the mask content can
	// reference fonts, patterns, and XObjects already registered on the page.
	resources := p.pageResources()
	if resources == nil {
		resources = pdfDict{}
	}

	// Build Form XObject dict per ISO 32000-1 §8.10.2 and §11.6.5.2.
	dict := pdfDict{
		"/Type":     pdfName("/XObject"),
		"/Subtype":  pdfName("/Form"),
		"/FormType": 1,
		"/BBox":     pdfArray{bbox.LLX, bbox.LLY, bbox.URX, bbox.URY},
		// Transparency group: /CS /DeviceGray encodes the mask as luminance.
		// Per SVG spec §15.6: mask luminance = 0.2125·R + 0.7154·G + 0.0721·B,
		// but since mask content already targets gray-channel output, DeviceGray
		// is the correct PDF color space here.
		"/Group": pdfDict{
			"/Type": pdfName("/Group"),
			"/S":    pdfName("/Transparency"),
			"/CS":   pdfName("/DeviceGray"),
		},
		"/Resources": resources,
	}

	// Register the Form XObject as an indirect object.
	streamObj := &pdfStream{
		Dict:    dict,
		Data:    buf.Bytes(),
		Decoded: true,
	}
	objID := p.doc.nextID
	p.doc.nextID++
	p.doc.objects[objID] = &pdfObject{Num: objID, Value: streamObj}

	return pdfRef{Num: objID}, nil
}
