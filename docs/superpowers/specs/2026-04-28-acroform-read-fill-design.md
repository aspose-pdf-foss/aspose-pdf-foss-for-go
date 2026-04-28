# AcroForm Read + Fill — Design Spec

**Epic:** `pdf-go-re2` (subepic 1 of 2 in the AcroForm program; form-design CRUD is the deferred subepic 2).

**Goal:** Programmatically read every field of an existing PDF form and set its value, so the canonical "fill PDF template and return it" workflow is supported. Out of scope here: creating new fields, deleting fields, restructuring the field tree (subepic 2), self-rendered `/AP` appearance streams (separate epic), and form flattening (separate epic).

## Why this epic

Filling pre-built PDF templates is one of the top three enterprise use cases for any PDF library. Right now this library can preserve `/AcroForm` across save (catalog preservation, `pdf-go-bos`, closed) but exposes zero way to read or modify fields. A user who opens `PdfWithAcroForm.pdf`, captures values, fills in new ones, and saves cannot do step two. This epic closes that gap.

## What "fill PDF template" means

Concretely:

1. Load a template PDF that ships with form fields already laid out by a designer.
2. Iterate the fields, decide what to write (from a database row, a request body, etc.).
3. Write the values into the existing fields.
4. Save the result.
5. Hand the file back. When opened in any major viewer (Adobe Acrobat / Reader, Chrome, Edge, Preview, Foxit), the new values must be visible.

Step 5 is the subtle one. PDF's `/Encrypt`-after-`/Filter` rule is its analog here: the *value* is what's stored at the model layer (`/V`), but viewers render from a separate cached *appearance* (`/AP`). After we change `/V`, the cached `/AP` is stale. Two options for re-syncing:

* Set `/AcroForm/NeedAppearances = true`. Every modern interactive viewer regenerates appearances on display when this flag is set. Trivial to implement; covers ~95% of viewers.
* Generate `/AP` ourselves. Required for headless server-side rendering pipelines that consume the PDF without ever opening it interactively, and for PDF/A workflows that explicitly forbid `/NeedAppearances=true`.

This epic implements only the first option. Self-rendered appearances are a separate epic (currently held in the backlog).

## Reference API

Every public symbol mirrors **Aspose.PDF for .NET** in name and semantics, so users moving from the .NET product to this Go library find familiar shapes. Where Go syntactically can't reproduce a C# construct (no properties, no indexer assignment), we use idiomatic Go equivalents.

### Form access

```go
// Form returns the document's AcroForm. Always non-nil; for a document
// without /AcroForm, an empty Form is returned (Fields() is empty,
// Field(name) returns nil, HasField returns false).
func (d *Document) Form() *Form

type Form struct { /* unexported state */ }

func (f *Form) Fields() []Field
func (f *Form) Field(name string) Field          // by FullName; nil if not found
func (f *Form) HasField(name string) bool
func (f *Form) NeedAppearances() bool
func (f *Form) SetNeedAppearances(v bool)
```

### Common interface

```go
type Field interface {
    PartialName() string  // /T value at this field's node
    FullName() string     // dotted full path: parent.T + "." + this.T
    Value() string        // generic stringified value
    SetValue(s string) error
    IsReadOnly() bool
    IsRequired() bool
    PageIndex() int       // 1-based; 0 if the field has no widget on a specific page
    Rect() Rectangle      // first widget's /Rect; zero rect if no widgets
}
```

`SetValue(string)` on the base interface dispatches to the concrete type:

* `TextBoxField` — sets the text.
* `CheckboxField` — accepts `"true"` / `"false"` / `"on"` / `"off"` / `"yes"` / `"no"` (case-insensitive); falls through to `Field.SetValue` returning `error` for anything else.
* `RadioButtonField` — argument must equal one of `Options()[i].Name()`. Empty string `""` clears the selection (writes `/Off`).
* `ComboBoxField` — for non-edit-mode (the common case), argument must match one of `Options()[i].Value` or `.Export`. For combo boxes with the Edit `/Ff` bit set, arbitrary text is accepted.
* `ListBoxField` — single value must match an option (no multi-select via base `SetValue`); multi-select via `SetSelected(indices ...int)` only. `SetSelected()` with no arguments clears all selections.
* `ButtonField` — push button; `SetValue` is an error.

### Concrete types

```go
type TextBoxField struct { /* */ }
func (f *TextBoxField) MaxLen() int                    // 0 = no limit
func (f *TextBoxField) IsMultiline() bool
func (f *TextBoxField) IsPassword() bool

type CheckboxField struct { /* */ }
func (f *CheckboxField) Checked() bool
func (f *CheckboxField) SetChecked(v bool)

type RadioButtonField struct { /* */ }
func (f *RadioButtonField) Options() []*RadioButtonOptionField

type RadioButtonOptionField struct { /* */ }
func (o *RadioButtonOptionField) Name() string         // export value (= /AS state)
func (o *RadioButtonOptionField) Selected() bool
func (o *RadioButtonOptionField) SetSelected(v bool)   // setting true clears siblings

type ComboBoxField struct { /* */ }
func (f *ComboBoxField) Options() []ChoiceOption
func (f *ComboBoxField) Selected() int                 // -1 if none
func (f *ComboBoxField) SetSelected(index int) error

type ListBoxField struct { /* */ }
func (f *ListBoxField) Options() []ChoiceOption
func (f *ListBoxField) MultiSelect() bool
func (f *ListBoxField) Selected() []int
func (f *ListBoxField) SetSelected(indices ...int) error

type ButtonField struct { /* */ }
// Push button: no value semantics. Implements Field with SetValue returning error.

type ChoiceOption struct {
    Value  string  // displayed text
    Export string  // export value when distinct from Value (often empty)
}
```

## Internal model

### Field tree to flat list

PDF's `/AcroForm/Fields` is recursive: a field can have `/Kids`, themselves fields. Common patterns:

* Radio button group: parent has `/FT /Btn` and `/Ff` radio bit; kids are widget-like dicts with `/AS` distinguishing options.
* Multi-widget field: one field with multiple kid widgets sharing a single value across pages.
* Hierarchical naming: a parent `/T = "shipping"` and child `/T = "street"` form FullName `"shipping.street"`.

`Form.Fields()` returns a **flat slice of leaf fields with computed FullName**. Hierarchy is internal. Subepic 2 (form design) may expose tree manipulation; this epic does not.

Inheritable attributes per ISO 32000-1 §12.7.3.1 — `/FT`, `/Ff`, `/V`, `/DV`, `/DA` — are resolved by walking up the tree at parse time. The leaf record carries the resolved values; consumers never see a partially specified field.

### Live view, not snapshot

`Field` is a live handle on the underlying `pdfDict`. `SetValue` mutates the dict in place; the next `Save` writes the new value. Matches the existing `Page` / `Document` mutability convention. Aspose.PDF for .NET behaves the same way.

`Form.SetNeedAppearances(true)` mutates `/AcroForm/NeedAppearances`. Any value-changing call (`SetValue`, `SetChecked`, `SetSelected`) on any field automatically sets `NeedAppearances=true` so users do not need to think about it. Power users who want to disable can call `SetNeedAppearances(false)` after the fact.

### String encoding

Field values can be stored as either PDFDocEncoding strings or UTF-16BE-with-BOM (per ISO 32000-1 §7.9.2.2). On read, both forms are decoded into Go `string`. On write, ASCII values use literal/PDFDocEncoding form; values containing non-ASCII characters are encoded as UTF-16BE with the `0xFE 0xFF` BOM. This is the same convention every interoperating PDF tool uses.

### Multi-widget fields

A field with several widget kids on different pages: `PageIndex()` returns the page index of the **first** widget, `Rect()` returns its `/Rect`. Documented behavior. Most templates have a single widget per field; the multi-widget case is rare for the field types in scope.

### `/AcroForm` creation on first set

If a document has no `/AcroForm` and the user calls `SetNeedAppearances(true)` on the empty Form, no `/AcroForm` is created — there are no fields to render. `Form.Field("anything")` continues to return nil. `/AcroForm` is added to the document only when the form-design epic introduces field-creation methods.

## Files

* `form.go` (new) — `Form`, `Document.Form()`, `Field` interface, parsing + flattening of the field tree, FullName computation, value encoding/decoding.
* `form_fields.go` (new) — concrete types: `TextBoxField`, `CheckboxField`, `RadioButtonField`, `RadioButtonOptionField`, `ComboBoxField`, `ListBoxField`, `ButtonField`, `ChoiceOption`. Each implements `Field`.
* `form_test.go` (new) — public-API tests on `testdata/PdfWithAcroForm.pdf` and round-trip. May split per type if file grows beyond ~500 lines.
* `form_internal_test.go` (new) — unit tests on FullName resolution, inheritance walk, encoding roundtrip on synthetic byte fixtures.
* `testdata/testfiles.json` — register `TestFormFillRoundTrip`, `TestFormReadFields` against the existing `PdfWithAcroForm.pdf`.

No modifications to existing parser or writer files are required: catalog preservation already ships `/AcroForm` through Save, and the parser already exposes the underlying dicts via `d.objects`.

## Test strategy

External oracle: pypdf 6.x reads form values via `reader.get_form_text_fields()` and `reader.get_fields()`. Where pypdf disagrees with our values, we fail the test rather than chase pypdf bugs — but in practice agreement is the bar.

Test cases:

1. **Read all six fields from `PdfWithAcroForm.pdf`**: textField, checkboxField, radiobuttonField, listboxField, comboboxField, buttonField. Assert types, names, current values match what pypdf reports.
2. **Round-trip per type**: for each field type, set a new value via the typed setter, save, reopen, read, assert.
3. **`/NeedAppearances` auto-set**: verify a `SetValue` call leaves the saved file with `/AcroForm/NeedAppearances = true`.
4. **Generic `Field.Value()` for enumeration**: iterate `Fields()`, print `FullName + Value`, compare with pypdf's enumeration output.
5. **`SetValue` validation**: setting an invalid radio/combo option returns a non-nil error and does not mutate state.
6. **External viewer compatibility**: independent pypdf check after our save reads the values we wrote.

Edge cases to cover with synthetic fixtures (built in tests):

* Hierarchical names with two-level `/T` resolution.
* Field with `/V` but no widgets (data-only field).
* Inherited `/DA` from `/AcroForm` root rather than from the field itself.
* UTF-16BE-with-BOM Cyrillic value round-trip.

## Non-goals (explicit)

To keep the scope tight:

* **No field creation, deletion, renaming, or repositioning.** Subepic 2.
* **No `/AP` regeneration.** Separate epic; we set `/NeedAppearances=true` instead.
* **No form flattening.** Separate epic.
* **No signature fields (`/Sig`).** Separate epic (`pdf-go-bm9`).
* **No structural mutators on existing fields** (`SetReadOnly`, `SetRequired`, `SetMaxLen`, `SetMultiline`, `SetPassword`, `SetMultiSelect`). Read-only in this epic; setters are subepic 2.
* **No XFA forms.** Aspose supports them too; we do not. XFA is a different beast and is being deprecated by Adobe anyway.

## Acceptance

* `doc.Form().Fields()` returns the six fields of `PdfWithAcroForm.pdf` with correct types and current values.
* For each field type, `SetValue` (or the typed setter) on the live handle, `Save`, reopen via `Open`, re-read returns the new value.
* The saved file opens in Adobe Acrobat / Chrome and shows the new values without any user action — verified manually for one fixture, plus pypdf reads back what we wrote.
* `go test ./...` green; new tests cover all six field types plus enumeration and `/NeedAppearances` auto-set.
