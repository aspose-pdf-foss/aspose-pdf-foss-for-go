package asposepdf

import (
	"fmt"
	"strings"
)

// Metadata contains document information from the PDF Info dictionary.
// Fields not present in the source PDF are empty strings.
type Metadata struct {
	Title        string
	Author       string
	Subject      string
	Keywords     string
	Creator      string
	Producer     string
	CreationDate string
	ModDate      string
	Custom       map[string]string // arbitrary Info dict entries
}

// GetMetadata reads the Info metadata from a PDF file.
//
// Example:
//
//	meta, err := asposepdf.GetMetadata("input.pdf")
//	fmt.Println(meta.Title, meta.Author)
func GetMetadata(inputPath string) (Metadata, error) {
	doc, err := openDocument(inputPath)
	if err != nil {
		return Metadata{}, fmt.Errorf("open PDF: %w", err)
	}
	return readMetadata(doc)
}

// Metadata returns the Info metadata from the primary source document.
// For documents assembled from multiple sources, metadata from the first
// source document is returned.
func (d *Document) Metadata() (Metadata, error) {
	if len(d.pages) == 0 {
		return Metadata{}, fmt.Errorf("document has no pages")
	}
	return readMetadata(d.pages[0].src)
}

// readMetadata extracts the Info dictionary from a parsed document.
func readMetadata(doc *rawDocument) (Metadata, error) {
	infoRef, ok := doc.trailer["/Info"]
	if !ok {
		return Metadata{}, nil
	}
	infoDict, err := doc.resolveDict(infoRef)
	if err != nil {
		return Metadata{}, fmt.Errorf("read Info dict: %w", err)
	}

	standardKeys := map[string]bool{
		"/Title": true, "/Author": true, "/Subject": true, "/Keywords": true,
		"/Creator": true, "/Producer": true, "/CreationDate": true, "/ModDate": true,
	}
	var custom map[string]string
	for k, v := range infoDict {
		if standardKeys[k] {
			continue
		}
		if s, ok := v.(string); ok && s != "" {
			if custom == nil {
				custom = make(map[string]string)
			}
			custom[strings.TrimPrefix(k, "/")] = s
		}
	}

	return Metadata{
		Title:        infoString(infoDict, "/Title"),
		Author:       infoString(infoDict, "/Author"),
		Subject:      infoString(infoDict, "/Subject"),
		Keywords:     infoString(infoDict, "/Keywords"),
		Creator:      infoString(infoDict, "/Creator"),
		Producer:     infoString(infoDict, "/Producer"),
		CreationDate: infoString(infoDict, "/CreationDate"),
		ModDate:      infoString(infoDict, "/ModDate"),
		Custom:       custom,
	}, nil
}

// infoString returns a string field from the Info dictionary, or "" if absent.
func infoString(d pdfDict, key string) string {
	v, ok := d[key]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
