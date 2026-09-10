// SPDX-License-Identifier: MIT

package asposepdf_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	pdf "github.com/aspose-pdf-foss/aspose-pdf-foss-for-go"
)

// buildPortfolio makes a two-attachment portfolio with a three-column schema
// and per-file values.
func buildPortfolio(t *testing.T) *pdf.Document {
	t.Helper()
	doc := pdf.NewDocument(300, 200)
	ef := doc.EmbeddedFiles()
	inv, err := ef.AddFromStream("invoice-042.pdf", strings.NewReader("%PDF-1.4 fake"))
	if err != nil {
		t.Fatal(err)
	}
	note, err := ef.AddFromStream("notes.txt", strings.NewReader("terms agreed"))
	if err != nil {
		t.Fatal(err)
	}

	col := doc.Collection()
	col.SetView(pdf.CollectionViewDetails)
	col.SetInitialFile("invoice-042.pdf")
	col.Schema().Add("Invoice", "Invoice #", pdf.CollectionFieldText)
	col.Schema().Add("Amount", "Total", pdf.CollectionFieldNumber)
	col.Schema().Add("Issued", "Issued on", pdf.CollectionFieldDate)
	col.Schema().Add("Size", "Bytes", pdf.CollectionFieldSize)
	col.SetSort("Amount", false)

	inv.CollectionItem().SetText("Invoice", "INV-042")
	inv.CollectionItem().SetNumber("Amount", 1499.5)
	inv.CollectionItem().SetDate("Issued", time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC))
	note.CollectionItem().SetText("Invoice", "—")
	return doc
}

// A portfolio survives Save+Open: view, initial file, sort, schema columns
// (order, header, type, flags) and per-file item values.
func TestCollectionRoundTrip(t *testing.T) {
	doc := buildPortfolio(t)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	re, err := pdf.OpenStream(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	col := re.Collection()
	if !col.IsPortfolio() {
		t.Fatal("reopened document is not a portfolio")
	}
	if col.View() != pdf.CollectionViewDetails {
		t.Errorf("View = %v; want Details", col.View())
	}
	if got := col.InitialFile(); got != "invoice-042.pdf" {
		t.Errorf("InitialFile = %q", got)
	}
	if field, asc, ok := col.Sort(); !ok || field != "Amount" || asc {
		t.Errorf("Sort = (%q, %v, %v); want (Amount, false, true)", field, asc, ok)
	}
	if n := len(col.Files()); n != 2 {
		t.Errorf("Files = %d; want 2", n)
	}

	keys := col.Schema().Keys()
	want := []string{"Invoice", "Amount", "Issued", "Size"}
	if len(keys) != len(want) {
		t.Fatalf("schema keys = %v; want %v", keys, want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Errorf("schema order[%d] = %q; want %q (declaration order)", i, keys[i], want[i])
		}
	}
	amount := col.Schema().Field("Amount")
	if amount == nil {
		t.Fatal("Amount column missing")
	}
	if amount.DisplayName() != "Total" {
		t.Errorf("DisplayName = %q; want Total", amount.DisplayName())
	}
	if amount.Type() != pdf.CollectionFieldNumber {
		t.Errorf("Type = %v; want Number", amount.Type())
	}
	if !amount.IsVisible() || amount.IsEditable() {
		t.Errorf("visible=%v editable=%v; want true/false", amount.IsVisible(), amount.IsEditable())
	}
	size := col.Schema().Field("Size")
	if size.Type() != pdf.CollectionFieldSize {
		t.Error("derived Size column lost its type")
	}
	if !size.IsDerived() || amount.IsDerived() {
		t.Errorf("IsDerived: Size=%v Amount=%v; want true/false", size.IsDerived(), amount.IsDerived())
	}

	inv := re.EmbeddedFiles().Get("invoice-042.pdf")
	if inv == nil {
		t.Fatal("attachment missing after round-trip")
	}
	item := inv.CollectionItem()
	if got := item.Text("Invoice"); got != "INV-042" {
		t.Errorf("Text(Invoice) = %q", got)
	}
	if got, ok := item.Number("Amount"); !ok || got != 1499.5 {
		t.Errorf("Number(Amount) = %v, %v", got, ok)
	}
	got, ok := item.Date("Issued")
	if !ok || got.Year() != 2026 || got.Month() != time.September || got.Day() != 10 {
		t.Errorf("Date(Issued) = %v, %v", got, ok)
	}
	if fields := item.Fields(); len(fields) != 3 {
		t.Errorf("item fields = %v; want 3", fields)
	}
	// The attachment's own bytes still work.
	data, err := inv.Data()
	if err != nil || !bytes.Contains(data, []byte("fake")) {
		t.Errorf("attachment data lost: %q, %v", data, err)
	}
}

// A plain document reports sane defaults; mutating turns it into a portfolio
// and Remove turns it back, keeping the attachments.
func TestCollectionCreateAndRemove(t *testing.T) {
	doc := pdf.NewDocument(300, 200)
	col := doc.Collection()
	if col.IsPortfolio() {
		t.Fatal("fresh document reports as a portfolio")
	}
	if col.View() != pdf.CollectionViewDetails || col.InitialFile() != "" || col.Schema().Count() != 0 {
		t.Error("defaults on a non-portfolio document are not neutral")
	}
	if _, _, ok := col.Sort(); ok {
		t.Error("non-portfolio reports a sort")
	}

	if _, err := doc.EmbeddedFiles().AddFromStream("a.txt", strings.NewReader("a")); err != nil {
		t.Fatal(err)
	}
	col.SetView(pdf.CollectionViewTiles)
	if !col.IsPortfolio() || col.View() != pdf.CollectionViewTiles {
		t.Fatal("SetView did not create the portfolio")
	}

	col.Remove()
	if col.IsPortfolio() {
		t.Error("Remove left the /Collection behind")
	}
	if doc.EmbeddedFiles().Count() != 1 {
		t.Error("Remove dropped the attachments")
	}
}

// Schema editing: re-adding replaces, ordering is explicit, removal works.
func TestCollectionSchemaEditing(t *testing.T) {
	doc := pdf.NewDocument(300, 200)
	s := doc.Collection().Schema()
	s.Add("A", "First", pdf.CollectionFieldText)
	b := s.Add("B", "Second", pdf.CollectionFieldNumber)
	if s.Count() != 2 {
		t.Fatalf("Count = %d; want 2", s.Count())
	}
	// Re-adding replaces in place.
	s.Add("A", "Renamed", pdf.CollectionFieldDate)
	if f := s.Field("A"); f.DisplayName() != "Renamed" || f.Type() != pdf.CollectionFieldDate {
		t.Errorf("re-Add did not replace: %q %v", f.DisplayName(), f.Type())
	}
	if s.Count() != 2 {
		t.Errorf("re-Add changed the column count to %d", s.Count())
	}
	// Explicit ordering wins.
	b.SetOrder(-1)
	if keys := s.Keys(); keys[0] != "B" {
		t.Errorf("SetOrder ignored: %v", keys)
	}
	b.SetVisible(false)
	b.SetEditable(true)
	if b.IsVisible() || !b.IsEditable() {
		t.Error("visibility/editability flags not stored")
	}
	if !s.Remove("A") || s.Count() != 1 {
		t.Error("Remove(A) failed")
	}
	if s.Remove("A") {
		t.Error("Remove of a missing column reported success")
	}
}

// Item values are per-attachment and independently removable.
func TestCollectionItemValues(t *testing.T) {
	doc := buildPortfolio(t)
	files := doc.EmbeddedFiles()
	inv, note := files.Get("invoice-042.pdf"), files.Get("notes.txt")

	if got := note.CollectionItem().Text("Invoice"); got != "—" {
		t.Errorf("second attachment value = %q", got)
	}
	if _, ok := note.CollectionItem().Number("Amount"); ok {
		t.Error("unset numeric field reported a value")
	}
	if _, ok := note.CollectionItem().Date("Issued"); ok {
		t.Error("unset date field reported a value")
	}
	if !inv.CollectionItem().Remove("Amount") {
		t.Error("Remove(Amount) failed")
	}
	if _, ok := inv.CollectionItem().Number("Amount"); ok {
		t.Error("removed field still reads back")
	}
	if inv.CollectionItem().Remove("Nope") {
		t.Error("removing a missing field reported success")
	}
	if got := inv.CollectionItem().Text("Invoice"); got != "INV-042" {
		t.Errorf("sibling field damaged by Remove: %q", got)
	}
}
