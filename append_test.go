package asposepdf_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	asposepdf "github.com/aspose/pdf-for-go"
)

const appendTestData = "testdata/append"

func TestDocumentAppend(t *testing.T) {
	entries, err := os.ReadDir(appendTestData)
	if err != nil {
		t.Fatalf("read %s: %v", appendTestData, err)
	}

	// Open all files; skip encrypted or otherwise unreadable ones.
	type namedDoc struct {
		path string
		doc  *asposepdf.Document
	}
	var docs []namedDoc
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		p := filepath.Join(appendTestData, e.Name())
		doc, err := asposepdf.Open(p)
		if err != nil {
			t.Logf("skipping %s: %v", e.Name(), err)
			continue
		}
		docs = append(docs, namedDoc{path: p, doc: doc})
	}
	if len(docs) < 2 {
		t.Skipf("need at least 2 openable files in %s, got %d", appendTestData, len(docs))
	}

	outDir := filepath.Join(resultDir, "TestDocumentAppend")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Merge each file with each other file (once per pair).
	for i := 0; i < len(docs); i++ {
		for j := i + 1; j < len(docs); j++ {
			a, b := docs[i], docs[j]
			t.Run(fmt.Sprintf("%s+%s", stem(a.path), stem(b.path)), func(t *testing.T) {
				combined := a.doc.Append(b.doc)
				want := a.doc.PageCount() + b.doc.PageCount()
				if combined.PageCount() != want {
					t.Fatalf("expected %d pages, got %d", want, combined.PageCount())
				}

				outPath := filepath.Join(outDir, fmt.Sprintf("%s+%s.pdf", stem(a.path), stem(b.path)))
				if err := combined.Save(outPath); err != nil {
					t.Fatalf("Save: %v", err)
				}

				report, err := asposepdf.Validate(outPath)
				if err != nil {
					t.Fatalf("Validate: %v", err)
				}
				checkValidation(t, outPath, report)
			})
		}
	}

	// Merge all openable files into one document.
	t.Run("all", func(t *testing.T) {
		wantPages := 0
		allDocs := make([]*asposepdf.Document, len(docs))
		for i, nd := range docs {
			allDocs[i] = nd.doc
			wantPages += nd.doc.PageCount()
		}

		combined := allDocs[0].Append(allDocs[1:]...)
		if combined.PageCount() != wantPages {
			t.Fatalf("expected %d pages, got %d", wantPages, combined.PageCount())
		}

		outPath := filepath.Join(outDir, "all.pdf")
		if err := combined.Save(outPath); err != nil {
			t.Fatalf("Save: %v", err)
		}

		report, err := asposepdf.Validate(outPath)
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
		checkValidation(t, outPath, report)
		t.Logf("merged %d files → %d pages", len(docs), combined.PageCount())
	})
}

func stem(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}
