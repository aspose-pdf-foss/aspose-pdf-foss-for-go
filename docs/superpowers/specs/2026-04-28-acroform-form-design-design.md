# AcroForm Form-Design CRUD — Design Spec

**Epic:** `pdf-go-4df`. Subepic 2 of the AcroForm program. Subepic 1 (Read + Fill) shipped as `pdf-go-re2` and is the foundation this epic extends.

**Goal:** Programmatically build PDF forms in Go — create text inputs, checkboxes, radio groups, combo and list boxes, push buttons; edit field structural metadata (read-only, required, max length, multi-line, multi-select); add/remove choice options; remove fields. The output is a saved PDF whose `/AcroForm` and per-page `/Annots` are correctly coordinated, and any compliant viewer (Adobe Acrobat, Chrome, Edge, Preview, Foxit) presents an interactive form.

## Why this epic

`pdf-go-re2` closed the **read template + fill template** workflow. This epic closes the **build form from scratch** workflow — the other half of the enterprise form story. With both shipped, a user can:

- generate a form from data (e.g. a contract template populated with fields per row of a database),
- modify a designer-built template's structure (add a missing checkbox, remove a deprecated field),
- generate empty forms for distribution.

Without this epic, every user who needs to *create* form fields must hand-edit PDF dictionaries or use a third-party authoring tool.

## API surface (Aspose.PDF for .NET fidelity)

### Form-level constructors

```go
// All return (concrete-type pointer, error). On any error nothing is mutated
// in the document. /AcroForm is auto-created if absent.
func (f *Form) AddTextField(pageNum int, rect Rectangle, name string) (*TextBoxField, error)
func (f *Form) AddCheckbox(pageNum int, rect Rectangle, name string) (*CheckboxField, error)
func (f *Form) AddRadioGroup(name string, items []RadioItem) (*RadioButtonField, error)
func (f *Form) AddComboBox(pageNum int, rect Rectangle, name string, options []ChoiceOption) (*ComboBoxField, error)
func (f *Form) AddListBox(pageNum int, rect Rectangle, name string, options []ChoiceOption) (*ListBoxField, error)
func (f *Form) AddPushButton(pageNum int, rect Rectangle, name string, caption string) (*ButtonField, error)

// RadioItem describes one widget inside a radio group. Widgets may live
// on different pages; each carries its own page number.
type RadioItem struct {
    PageNum int       // 1-based
    Rect    Rectangle
    Export  string    // export value; must be unique within the group
}
```

### Form-level structural mutator

```go
// RemoveField removes the named field plus all its widget annotations
// from /AcroForm/Fields and from each affected page's /Annots. Returns
// true if the field was found and removed; false otherwise.
func (f *Form) RemoveField(name string) bool
```

### Per-type structural mutators

`Field` interface stays exactly as in `pdf-go-re2`. Structural mutators are **not** added to the interface — each concrete type carries the setters that apply to it, matching Aspose.PDF for .NET conventions and avoiding a swollen interface.

```go
// TextBoxField
func (f *TextBoxField) SetReadOnly(bool)
func (f *TextBoxField) SetRequired(bool)
func (f *TextBoxField) SetMaxLen(int)        // 0 = no limit
func (f *TextBoxField) SetMultiline(bool)
func (f *TextBoxField) SetPassword(bool)

// CheckboxField
func (f *CheckboxField) SetReadOnly(bool)
func (f *CheckboxField) SetRequired(bool)

// RadioButtonField
func (f *RadioButtonField) SetReadOnly(bool)
func (f *RadioButtonField) SetRequired(bool)

// ComboBoxField
func (f *ComboBoxField) SetReadOnly(bool)
func (f *ComboBoxField) SetRequired(bool)
func (f *ComboBoxField) SetEditable(bool)         // /Ff bit 19
func (f *ComboBoxField) AddOption(o ChoiceOption)
func (f *ComboBoxField) RemoveOption(index int) error

// ListBoxField
func (f *ListBoxField) SetReadOnly(bool)
func (f *ListBoxField) SetRequired(bool)
func (f *ListBoxField) SetMultiSelect(bool)
func (f *ListBoxField) AddOption(o ChoiceOption)
func (f *ListBoxField) RemoveOption(index int) error

// ButtonField
func (f *ButtonField) SetReadOnly(bool)
```

Every mutator triggers `noteFormMutated` so the next `Save` writes `/AcroForm/NeedAppearances=true` (existing infrastructure from `pdf-go-re2`).

## Internal mechanics

### Auto-create `/AcroForm`

First call to any `Form.AddXxx` on a document without `/AcroForm` invokes the existing `ensureRoot()` helper to insert a minimal `/AcroForm` dict on the catalog. Additionally, `ensureFontHelv()` (new internal helper) registers a `/AcroForm/DR/Font/Helv` resource pointing to the standard 14 Helvetica font object — this is the font referenced by the default `/DA` string. If `/DR/Font/Helv` already exists from a prior call, it's reused; idempotent.

### Default appearance (`/DA`)

Every newly created field receives `/DA = "0 g /Helv 12 Tf"` — black, Helvetica, 12pt. This is hardcoded for the epic; per-field appearance customization is deferred to a future mini-epic that introduces an options pattern (`Form.AddTextField(..., opts ...TextFieldOption)`). Hardcoding now keeps the API surface flat and matches the 95% case for "build form from scratch."

### Widget annotation coordination

Single-widget fields (Text, Checkbox, Combo, List, Button) use the **combined field+widget pattern** allowed by ISO 32000-1 §12.5.6.19 — one dict carries both field-level keys (`/FT`, `/T`, `/V`, `/Ff`, `/DA`) and widget-level keys (`/Type=/Annot`, `/Subtype=/Widget`, `/Rect`, `/P`, `/AP`). Procedure:

1. Allocate a new object ID. Build the combined dict.
2. Append `pdfRef` to `/AcroForm/Fields` (creating the array if absent).
3. Append the same `pdfRef` to the target page's `/Annots` (creating the array if absent).
4. Insert the new `*pdfObject` into `d.objects`.
5. Update `Form.cache` and `Form.fieldsList` so `Field(name)` and `Fields()` return the canonical instance.
6. Call `noteFormMutated`.

Multi-widget fields (RadioButtonField) use a **parent + kid widgets** structure:

1. Build the parent field dict: `/FT=/Btn`, Radio bit set in `/Ff`, `/V=/Off`, `/Kids=[]`.
2. For each `RadioItem`: build a widget-only dict with `/Type=/Annot`, `/Subtype=/Widget`, `/Parent` pointing to the parent ref, `/Rect`, `/AP/N` containing two keys (`/Off` and `/<Export>`) mapping to placeholder dict refs (empty XObject-equivalent — viewers regenerate via `/NeedAppearances=true`).
3. Append each widget's pdfRef to its target page's `/Annots`.
4. Push each widget ref into the parent's `/Kids` array.
5. Append parent ref to `/AcroForm/Fields`.
6. Update caches; `noteFormMutated`.

### `RemoveField` cascade

1. Look up the field by `name` in `Form.cache`. Return false if not present.
2. Remove its ref from `/AcroForm/Fields`.
3. Collect every widget under the field — for combined-pattern fields the widget dict equals the field dict; for radio groups walk the parent's `/Kids`.
4. For each widget, find which page's `/Annots` array references it; splice the ref out.
5. Delete every involved object from `d.objects`.
6. Rebuild `Form.cache` and `Form.fieldsList` from the new `/AcroForm/Fields` (cheaper than incremental editing).
7. `noteFormMutated`.

### Validation + error contract

| Condition | Behavior |
|---|---|
| Duplicate field name | error, document untouched |
| `pageNum < 1` or `pageNum > PageCount()` | error |
| Empty `name` | error (PDF spec allows empty `/T` but disambiguation breaks; reject) |
| Empty `options` for ComboBox / ListBox | allowed (caller can `AddOption` later) |
| `len(items) == 0` for RadioGroup | error |
| Duplicate `Export` within RadioGroup items | error |
| `RemoveOption(index)` out of range | error |

All "error" paths leave the document state unchanged. Internally this is achieved by validating *first*, mutating *second* — never partial.

### Live-handle invariant under structure changes

`Form.fieldsList` and `Form.cache` are rebuilt after `AddXxx` and `RemoveField`. The pointer returned from `AddXxx` is stored in both, so subsequent `Form.Field(name)` returns the same instance. After `RemoveField`, any `Field` handle still held by the caller becomes a "dangling handle" — its underlying `pdfDict` is no longer in `d.objects`, but the handle itself is still a live Go pointer. Documented behavior: don't use handles after the field is removed.

## Files

| File | Action |
|---|---|
| `form.go` | Add `AddTextField`/`AddCheckbox`/`AddRadioGroup`/`AddComboBox`/`AddListBox`/`AddPushButton`/`RemoveField` methods; internal helpers `ensureFontHelv()`, `appendWidgetToPage`, `removeWidgetFromPage`, `rebuildFieldCache`. |
| `form_fields.go` | Add per-type structural mutators (`SetReadOnly`/`SetRequired`/`SetMaxLen`/`SetMultiline`/`SetPassword`/`SetMultiSelect`/`SetEditable`); `AddOption`/`RemoveOption` on `ComboBoxField` and `ListBoxField`; `RadioItem` struct. |
| `form_design_test.go` (new) | Public-API tests for all Add/Remove paths and structural mutators (~18 tests). |
| `testdata/testfiles.json` | Register the few tests that touch existing fixtures; most use `NewDocument`. |
| `CLAUDE.md` | List new methods. |
| `README.md` | "Forms — building from scratch" subsection with example. |

## Test strategy

Programmatic-first: most tests start from `pdf.NewDocument(595, 842)`, build a form, save to buffer, reopen via `OpenStream`, verify via the Read+Fill API delivered in `pdf-go-re2`.

Coverage groups:

1. **Round-trip per Add method** — `AddTextField`, `AddCheckbox`, `AddRadioGroup` (single-page and cross-page variants), `AddComboBox`, `AddListBox`, `AddPushButton`. Six tests.
2. **Validation** — duplicate name, invalid pageNum, empty name, empty radio items, duplicate export within group. Four to five tests.
3. **`RemoveField`** — happy path, multi-widget cascade, not-found returns false. Three tests.
4. **Structural mutators** — `SetMaxLen`, `SetMultiline`, `SetPassword`, `SetReadOnly`, `SetRequired`, `SetMultiSelect`, `SetEditable`, `AddOption`/`RemoveOption`. Roughly six tests, one per non-trivial mutator.
5. **`/NeedAppearances`** — verify auto-set after one `AddXxx` call. One regression test.

External oracle: pypdf 6.x reads back via `r.get_fields()` and confirms field types, names, options, and structural flags. One manual cross-verification step in the finalization task; no automated wrapping inside the Go test suite.

## Non-goals (explicit)

- **`Field.Rename(newName)`** — would require updating `/AcroForm/Fields` ordering, `Form.cache` rekeying, and FullName invalidation. Deferred.
- **Repositioning existing fields** (`SetRect`, move widget to another page) — deferred.
- **Hierarchical names** with auto-created parent fields (`shipping.address.street`) — deferred to a separate mini-epic. Flat names only here.
- **Self-rendered `/AP` appearance streams** — separate epic. We rely on `/NeedAppearances=true`.
- **Form flattening** — separate epic.
- **Signature fields** (`/Sig`) — `pdf-go-bm9`.
- **XFA forms** — not supported by this library.
- **Tab order customization** (`/AcroForm/CO`) — viewers use `/Fields` order by default; we append in caller order. Custom tab orders deferred.
- **Per-field `/DA` customization** — hardcoded `0 g /Helv 12 Tf` for now; options-pattern customization deferred.

## Acceptance

- On a blank `pdf.NewDocument(595, 842)`, a sequence of one `AddXxx` per field type produces a saved PDF whose Read+Fill API enumerates exactly six fields with correct concrete types and values.
- `RemoveField` removes both the `/AcroForm/Fields` entry and all related widgets from the appropriate `/Annots`.
- Each structural mutator survives a Save+Reopen round-trip via the Read+Fill getters.
- pypdf reports the same field structure we built.
- `go test ./...` green.
