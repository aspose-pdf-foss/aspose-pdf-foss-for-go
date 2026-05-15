package asposepdf

// OutlineItemCollection represents an outline entry and the collection
// of its children. The recursive structure mirrors Aspose.PDF for .NET:
// each entry is both a tree node (with Title, Color, Action,
// Destination, etc.) and a collection (Add/At/Remove/Count for
// children). The root collection — returned by Document.Outlines() —
// has no parent and an empty Title; only its children are visible as
// top-level bookmarks.
//
// Per ISO 32000-1 §12.3.3.
type OutlineItemCollection struct {
	doc      *Document
	parent   *OutlineItemCollection
	children []*OutlineItemCollection

	// In-memory state for items not yet (or never) backed by a dict.
	title       string
	bold        bool
	italic      bool
	color       *Color
	isExpanded  bool
	action      Action
	destination Destination

	// Set when this item was parsed from an existing PDF; nil for
	// newly-created items. Currently unused (read path is Task 10);
	// kept here so the struct is final-shaped from Task 2.
	dict   pdfDict
	objNum int
}

// NewOutlineItemCollection builds an unattached outline entry bound to
// the given document. Add it to a parent via Document.Outlines().Add(...)
// or via another entry's Add(...) — until added it has no effect on
// the saved PDF.
//
// Aspose .NET: new OutlineItemCollection(doc.Outlines)
// Go:          pdf.NewOutlineItemCollection(doc)
func NewOutlineItemCollection(doc *Document) *OutlineItemCollection {
	return &OutlineItemCollection{
		doc:        doc,
		isExpanded: true, // matches Aspose .NET default
	}
}

// Document returns the document this collection is bound to.
func (o *OutlineItemCollection) Document() *Document { return o.doc }

// Parent returns the parent entry, or nil for the root collection.
func (o *OutlineItemCollection) Parent() *OutlineItemCollection { return o.parent }

// Count returns the number of direct children (placeholder until
// Task 5 adds the rest of the collection API).
func (o *OutlineItemCollection) Count() int { return len(o.children) }

// Title returns the bookmark text.
func (o *OutlineItemCollection) Title() string {
	return o.title
}

// SetTitle replaces the bookmark text.
func (o *OutlineItemCollection) SetTitle(s string) {
	o.title = s
}

// Bold corresponds to /F bit 2 in the outline item dict. Default false.
func (o *OutlineItemCollection) Bold() bool {
	return o.bold
}

func (o *OutlineItemCollection) SetBold(b bool) {
	o.bold = b
}

// Italic corresponds to /F bit 1. Default false.
func (o *OutlineItemCollection) Italic() bool {
	return o.italic
}

func (o *OutlineItemCollection) SetItalic(b bool) {
	o.italic = b
}

// Color returns the RGB label color, or nil if /C is absent (default
// black). SetColor(nil) clears /C.
func (o *OutlineItemCollection) Color() *Color {
	return o.color
}

func (o *OutlineItemCollection) SetColor(c *Color) {
	o.color = c
}

// IsExpanded controls the viewer's initial expand/collapse state.
// Encoded via the sign of /Count. Default true.
func (o *OutlineItemCollection) IsExpanded() bool {
	return o.isExpanded
}

func (o *OutlineItemCollection) SetIsExpanded(b bool) {
	o.isExpanded = b
}

// Action returns the action attached via /A. Reuses the Action
// interface defined for annotations.
func (o *OutlineItemCollection) Action() Action {
	return o.action
}

// SetAction sets the /A action. Pass nil to clear.
func (o *OutlineItemCollection) SetAction(a Action) {
	o.action = a
}

// Destination returns the explicit view destination via /Dest, or nil
// if absent. If both Destination and Action are set, /Dest takes
// priority per ISO 32000-1 §12.3.3.
func (o *OutlineItemCollection) Destination() Destination {
	return o.destination
}

// SetDestination sets the /Dest entry. Pass nil to clear.
func (o *OutlineItemCollection) SetDestination(d Destination) {
	o.destination = d
}

// Outlines returns the document's root outline collection. Always
// non-nil — an empty collection is returned for documents without
// outline content. Mirrors Aspose.PDF for .NET's Document.Outlines.
func (d *Document) Outlines() *OutlineItemCollection {
	if d.outlinesRoot == nil {
		// Task 10 will replace this with parseOutlines(d).
		d.outlinesRoot = &OutlineItemCollection{doc: d, isExpanded: true}
	}
	return d.outlinesRoot
}
