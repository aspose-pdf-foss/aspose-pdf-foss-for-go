# RemoveUnusedObjects — Design Spec

## Goal

Add the ability to remove orphaned (unreachable) objects from a Document's in-memory object store. After operations like `ImageInfo.Remove()`, the underlying XObject and SMask remain in `doc.objects` but are no longer referenced from any page. `RemoveUnusedObjects` scans the object graph and deletes these orphaned entries, reducing output file size.

## Public API

### New method on `*Document`

```go
// RemoveUnusedObjects removes objects from the document that are not
// reachable from any page. Returns the number of objects removed.
func (d *Document) RemoveUnusedObjects() int
```

### Usage

```go
doc, _ := pdf.Open("input.pdf")
page, _ := doc.Page(1)
infos, _ := page.ImageInfos()
infos[0].Remove()

removed := doc.RemoveUnusedObjects()
fmt.Printf("removed %d unused objects\n", removed)
doc.Save("output.pdf")
```

## Internal design

### Algorithm

1. **Collect reachable object IDs** — for each page in `d.pages`, recursively walk the object graph starting from the page object. Each `pdfRef` is an edge. Walk dict values, array elements, stream dicts, and raw stream bytes (regex `\b(\d+)\s+\d+\s+R\b` for inline references). A `visited map[int]bool` set prevents cycles.

2. **Compute difference** — all IDs in `doc.objects` not present in `visited` are orphaned.

3. **Delete orphaned** — `delete(d.objects, id)` for each orphaned ID.

4. **Return count** — number of deleted objects.

### Reachability walker

A new function `collectReachableIDs` in `doc.go`:

```go
// collectReachableIDs returns the set of object IDs reachable from the given root objects.
func collectReachableIDs(objects map[int]*pdfObject, roots []*pdfObject) map[int]bool
```

This is similar in spirit to `collectPageDeps` but lighter:
- `collectPageDeps` builds a new `map[int]*pdfObject` with deep copies — used by Split/Extract.
- `collectReachableIDs` only marks IDs in a `visited` set — no copying.
- Same recursive walk logic: dicts, arrays, streams, stream bytes regex.
- Same cycle protection via `visited`.
- Does NOT skip `/Pages` or `/Catalog` nodes (they were already removed from `doc.objects` during parsing — see `parseAllObjects` cleanup in `doc.go`).

### Roots

The only roots are `d.pages` entries. Other structural data (`/Info`, `/Encrypt`) is rebuilt by `buildDocumentPDF` from Document fields (`d.metadata`, `d.encryptConfig`), not from `doc.objects`.

### Why not reuse `collectPageDeps`

`collectPageDeps` creates a new map with object copies — designed for Split/Extract where you need independent object sets. For `RemoveUnusedObjects` we only need a visited set (read-only walk). Writing a dedicated walker avoids unnecessary allocations.

## Error handling

No errors returned. The walk is a pure in-memory graph traversal over `doc.objects`. If an object ID referenced by a `pdfRef` is missing from `doc.objects`, it is simply skipped (already the behavior of `resolveRef`). Cyclic references are handled by the `visited` set.

## Files

| File | Responsibility |
|------|----------------|
| `document.go` | `RemoveUnusedObjects` method |
| `doc.go` | `collectReachableIDs` helper |
| `document_test.go` | Unit tests |
| `document_integration_test.go` | Integration test with real PDF |

## Testing

### Unit tests (package `asposepdf`)

- `TestRemoveUnusedObjectsBasic` — document with one orphaned object (not referenced from any page), verify it is removed and return value is 1.
- `TestRemoveUnusedObjectsNone` — all objects reachable, verify return is 0 and `doc.objects` unchanged.
- `TestRemoveUnusedObjectsAfterRemoveImage` — call `ImageInfo.Remove()` then `RemoveUnusedObjects()`, verify the XObject is deleted from `doc.objects`.
- `TestRemoveUnusedObjectsSharedXObject` — one XObject referenced from two pages, remove from one page, verify `RemoveUnusedObjects()` does NOT remove the XObject (still reachable from the other page).
- `TestRemoveUnusedObjectsCyclicRefs` — two orphaned objects referencing each other, verify both are removed without infinite loop.

### Integration test (package `asposepdf_test`)

- `TestRemoveUnusedObjectsRoundTrip` — open real PDF, remove an image, call `RemoveUnusedObjects`, save, reopen, Validate. Verify the output is valid and file size decreased.

## Scope boundary

This spec covers only removing unreachable objects from `doc.objects`. It does NOT cover:
- Image compression/optimization (Sub-project C: OptimizeImages)
- Removing duplicate objects (deduplication)
- Linearization or incremental save
- Removing unused fonts or other specific resource types by policy
