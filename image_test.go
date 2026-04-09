package asposepdf_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	asposepdf "github.com/aspose/pdf-for-go"
)

func TestExtractImages(t *testing.T) {
	groups := testGroups(t)
	for _, group := range groups {
		path := group[0]
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		t.Run(name, func(t *testing.T) {
			doc, err := asposepdf.Open(path)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			allImages, err := doc.ExtractImages()
			if err != nil {
				t.Fatalf("ExtractImages: %v", err)
			}

			outDir := filepath.Join(resultDir, "TestExtractImages", name)
			os.MkdirAll(outDir, 0o755)

			total := 0
			for pageIdx, images := range allImages {
				for imgIdx, img := range images {
					ext := ".png"
					if img.Format == asposepdf.ImageFormatJPEG {
						ext = ".jpg"
					}
					outPath := filepath.Join(outDir, fmt.Sprintf("page%d_img%d%s", pageIdx+1, imgIdx+1, ext))
					if err := img.Save(outPath); err != nil {
						t.Errorf("save %s: %v", outPath, err)
					}
					if img.Width <= 0 || img.Height <= 0 {
						t.Errorf("page %d img %d: invalid dimensions %dx%d", pageIdx+1, imgIdx+1, img.Width, img.Height)
					}
					if len(img.Data) == 0 {
						t.Errorf("page %d img %d: empty data", pageIdx+1, imgIdx+1)
					}
					total++
				}
			}
			t.Logf("%s: extracted %d images to %s", name, total, outDir)
		})
	}
}
