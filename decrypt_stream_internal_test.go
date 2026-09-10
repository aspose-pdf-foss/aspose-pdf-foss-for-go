// SPDX-License-Identifier: MIT

package asposepdf

import (
	"bytes"
	"compress/zlib"
	"testing"
)

// The parser cannot tell ciphertext from a Flate stream, and the assumption
// that encrypted bytes never survive their declared filter is false: a zlib
// header is only "CM == 8 and (CMF<<8|FLG) % 31 == 0", which random bytes
// satisfy about once in 500. flateDecode is deliberately tolerant of a
// truncated tail, so such a stream decodes to a byte or two of garbage
// instead of failing — which is how an encrypted stream used to be destroyed
// before it was ever decrypted (a font's /ToUnicode silently became one byte,
// turning a space into U+FFFD in the extracted text).
func TestFlateAcceptsRandomLookingHeader(t *testing.T) {
	// 0x28 0x15 is a real ciphertext prefix observed in the wild: CM = 8 and
	// 0x2815 % 31 == 0, so zlib accepts the header.
	if got := (0x28<<8 | 0x15) % 31; got != 0 {
		t.Fatalf("test premise wrong: header checksum = %d", got)
	}
	if _, err := zlib.NewReader(bytes.NewReader([]byte{0x28, 0x15, 0x8a, 0x07, 0xb3, 0xe6})); err != nil {
		t.Fatalf("zlib rejected a valid-looking header: %v", err)
	}
}

// newTestEncryptState builds a write-side state for the given algorithm.
func newTestEncryptState(t *testing.T, alg EncryptionAlgorithm) *encryptState {
	t.Helper()
	st, err := newEncryptState(&encryptConfig{
		algorithm: alg, userPassword: "u", ownerPassword: "o",
	})
	if err != nil {
		t.Fatalf("newEncryptState: %v", err)
	}
	return st
}

// A stream whose ciphertext happened to decode at parse time must still be
// decrypted — from the raw file bytes the parser kept — rather than left as
// the garbage that came out of the filter.
func TestDecryptRecoversAccidentallyDecodedStream(t *testing.T) {
	plain := []byte("/CIDInit /ProcSet findresource begin 12 dict begin begincmap endcmap end")

	for _, alg := range []EncryptionAlgorithm{
		EncryptionAlgAES128, EncryptionAlgAES256, EncryptionAlgRC4_128,
	} {
		state := newTestEncryptState(t, alg)

		var zbuf bytes.Buffer
		zw := zlib.NewWriter(&zbuf)
		_, _ = zw.Write(plain)
		_ = zw.Close()

		ciphertext, err := state.encryptBytes(7, 0, zbuf.Bytes())
		if err != nil {
			t.Fatalf("alg %v: encryptBytes: %v", alg, err)
		}

		// Simulate the parser's rare accident: it decoded the ciphertext into
		// a scrap of garbage, marked the stream decoded, and kept the file
		// bytes in raw.
		obj := &pdfObject{Num: 7, Gen: 0, Value: &pdfStream{
			Dict:    pdfDict{"/Filter": pdfName("/FlateDecode"), "/Length": len(ciphertext)},
			Data:    []byte("_"),
			Decoded: true,
			raw:     ciphertext,
		}}
		if err := decryptObject(obj, state); err != nil {
			t.Fatalf("alg %v: decryptObject: %v", alg, err)
		}
		got := obj.Value.(*pdfStream)
		if !got.Decoded {
			t.Errorf("alg %v: stream left undecoded", alg)
		}
		if !bytes.Equal(got.Data, plain) {
			t.Errorf("alg %v: recovered %q, want %q", alg, got.Data, plain)
		}
	}
}

// The ordinary case — the parser left the ciphertext undecoded — keeps
// working, and a stream built in memory (no raw bytes) is left alone.
func TestDecryptOrdinaryAndInMemoryStreams(t *testing.T) {
	state := newTestEncryptState(t, EncryptionAlgAES128)
	plain := []byte("hello encrypted stream")
	ciphertext, err := state.encryptBytes(3, 0, plain)
	if err != nil {
		t.Fatal(err)
	}

	obj := &pdfObject{Num: 3, Gen: 0, Value: &pdfStream{
		Dict: pdfDict{"/Length": len(ciphertext)}, Data: ciphertext,
	}}
	if err := decryptObject(obj, state); err != nil {
		t.Fatal(err)
	}
	if got := obj.Value.(*pdfStream).Data; !bytes.Equal(got, plain) {
		t.Errorf("undecoded path: got %q, want %q", got, plain)
	}

	inMemory := []byte("already clean")
	obj2 := &pdfObject{Num: 3, Gen: 0, Value: &pdfStream{
		Dict: pdfDict{}, Data: inMemory, Decoded: true,
	}}
	if err := decryptObject(obj2, state); err != nil {
		t.Fatal(err)
	}
	if got := obj2.Value.(*pdfStream).Data; !bytes.Equal(got, inMemory) {
		t.Errorf("in-memory stream was mangled: got %q", got)
	}
}

// Streams that a conforming producer leaves in the clear inside an encrypted
// file must not be decrypted: cross-reference streams never are, and the
// metadata stream is exempt when /EncryptMetadata is false.
func TestDecryptSkipsExemptStreams(t *testing.T) {
	state := newTestEncryptState(t, EncryptionAlgAES128)
	state.plainMetadata = true

	// An exempt stream is stored in the clear, so its file bytes are the
	// content itself and must come back untouched.
	content := []byte("plain content that must survive")
	for _, name := range []string{"/XRef", "/Metadata"} {
		obj := &pdfObject{Num: 5, Gen: 0, Value: &pdfStream{
			Dict: pdfDict{"/Type": pdfName(name)}, Data: content, Decoded: true, raw: content,
		}}
		if err := decryptObject(obj, state); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := obj.Value.(*pdfStream).Data; !bytes.Equal(got, content) {
			t.Errorf("%s stream was decrypted: got %q", name, got)
		}
	}

	// A stream of any other type is encrypted like the rest of the document.
	ct, err := state.encryptBytes(5, 0, content)
	if err != nil {
		t.Fatal(err)
	}
	obj := &pdfObject{Num: 5, Gen: 0, Value: &pdfStream{
		Dict: pdfDict{"/Type": pdfName("/ObjStm")}, Data: ct,
	}}
	if err := decryptObject(obj, state); err != nil {
		t.Fatal(err)
	}
	if got := obj.Value.(*pdfStream).Data; !bytes.Equal(got, content) {
		t.Errorf("ordinary stream not decrypted: got %q", got)
	}

	// With metadata encryption on (the default this library writes), a
	// /Metadata stream is decrypted like any other.
	state.plainMetadata = false
	plain := []byte("<?xpacket begin=...?>")
	ciphertext, err := state.encryptBytes(9, 0, plain)
	if err != nil {
		t.Fatal(err)
	}
	metaObj := &pdfObject{Num: 9, Gen: 0, Value: &pdfStream{
		Dict: pdfDict{"/Type": pdfName("/Metadata")}, Data: ciphertext,
	}}
	if err := decryptObject(metaObj, state); err != nil {
		t.Fatal(err)
	}
	if got := metaObj.Value.(*pdfStream).Data; !bytes.Equal(got, plain) {
		t.Errorf("encrypted metadata not decrypted: got %q", got)
	}
}
