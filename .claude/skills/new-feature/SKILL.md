---
name: new-feature
description: Implement a new public API function for the pdf-for-go library
---

Implement a new public API function for the pdf-for-go library.

Feature to implement: $ARGUMENTS

Follow these steps:

1. **Design** — describe the public signature (function name, parameters, return values) and confirm it fits the existing API style in `splitter.go` and `merger.go`.

2. **Implementation** — create a new `<feature>.go` file in the root package `pdfsplitter`. Follow existing patterns:
   - Public function in its own file
   - Internal helpers (PDF building logic) in `writer.go` if they extend the write pipeline
   - No external dependencies

3. **Tests** — create `<feature>_test.go`. Use only synthetic PDFs built with `buildMinimalPDF()` for unit tests. If a real PDF from `test_data/` is needed, ask the user which file to use before writing the test.

4. **Update CLAUDE.md** — add the new function to the Public API list.

5. **Verify** — run `go test ./...` and `go build ./...` and confirm everything passes.
