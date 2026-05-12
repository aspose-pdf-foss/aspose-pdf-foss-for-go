package asposepdf

import (
	"bytes"
	"crypto/aes"
	"encoding/hex"
	"testing"
)

func TestAddPKCS7_VariousLengths(t *testing.T) {
	cases := []struct {
		in       []byte
		wantPad  int
		wantTail []byte // last few bytes (pad value, repeated)
	}{
		{[]byte{}, 16, bytes.Repeat([]byte{16}, 16)},
		{[]byte{0x01}, 15, bytes.Repeat([]byte{15}, 15)},
		{[]byte{0x01, 0x02, 0x03}, 13, bytes.Repeat([]byte{13}, 13)},
		{bytes.Repeat([]byte{0x42}, 15), 1, []byte{1}},
		{bytes.Repeat([]byte{0x42}, 16), 16, bytes.Repeat([]byte{16}, 16)},
		{bytes.Repeat([]byte{0x42}, 17), 15, bytes.Repeat([]byte{15}, 15)},
	}
	for _, tc := range cases {
		got := addPKCS7(tc.in, aes.BlockSize)
		if len(got)%aes.BlockSize != 0 {
			t.Errorf("len(addPKCS7(%d-byte input)) = %d, not block-multiple", len(tc.in), len(got))
		}
		if len(got)-len(tc.in) != tc.wantPad {
			t.Errorf("addPKCS7(%d-byte input) added %d pad bytes, want %d",
				len(tc.in), len(got)-len(tc.in), tc.wantPad)
		}
		tail := got[len(got)-len(tc.wantTail):]
		if !bytes.Equal(tail, tc.wantTail) {
			t.Errorf("addPKCS7 trailing bytes = %v, want %v", tail, tc.wantTail)
		}
	}
}

func TestStripPKCS7_RoundTrip(t *testing.T) {
	inputs := [][]byte{
		{},
		{0x01},
		{0x01, 0x02, 0x03},
		bytes.Repeat([]byte{0x42}, 15),
		bytes.Repeat([]byte{0x42}, 16),
		bytes.Repeat([]byte{0x42}, 100),
	}
	for _, in := range inputs {
		padded := addPKCS7(in, aes.BlockSize)
		out, err := stripPKCS7(padded)
		if err != nil {
			t.Errorf("stripPKCS7 on padded %d-byte input: %v", len(in), err)
			continue
		}
		if !bytes.Equal(in, out) {
			t.Errorf("roundtrip differs for %d-byte input: got %v, want %v", len(in), out, in)
		}
	}
}

func TestStripPKCS7_InvalidPadding(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"not block-aligned", bytes.Repeat([]byte{0x01}, 15)},
		{"pad byte zero", append(bytes.Repeat([]byte{0x42}, 15), 0)},
		{"pad byte too large", append(bytes.Repeat([]byte{0x42}, 15), 17)},
		{"mismatched pad bytes", append(bytes.Repeat([]byte{0x42}, 13), 3, 3, 4)},
	}
	for _, tc := range cases {
		if _, err := stripPKCS7(tc.data); err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}
}

func TestObjectKeyAES128_KnownVector(t *testing.T) {
	// Reference vector verified offline:
	//   docKey = 16 bytes of 0xAB
	//   objNum = 0x010203, gen = 0x0405
	//   suffix = "sAlT" (literal, per ISO 32000-1 §7.6.2)
	//   key = MD5(docKey || objNum_LE_3 || gen_LE_2 || "sAlT")
	// Offline computation:
	//   import hashlib
	//   buf = bytes([0xAB]*16) + bytes([0x03, 0x02, 0x01, 0x05, 0x04]) + b"sAlT"
	//   print(hashlib.md5(buf).hexdigest())
	// → "517f71c032e35e41161763d66b87fcc9"
	docKey := bytes.Repeat([]byte{0xAB}, 16)
	got := objectKeyAES128(docKey, 0x010203, 0x0405)
	want, _ := hex.DecodeString("517f71c032e35e41161763d66b87fcc9")
	if !bytes.Equal(got, want) {
		t.Errorf("objectKeyAES128 = %x, want %x", got, want)
	}
	if len(got) != 16 {
		t.Errorf("objectKeyAES128 length = %d, want 16 for AES-128", len(got))
	}
}

func TestObjectKeyAES128_DiffersFromRC4Key(t *testing.T) {
	// AES key derivation appends "sAlT" before MD5 — must produce a
	// different output than the RC4 path for the same docKey/objNum/gen.
	docKey := bytes.Repeat([]byte{0x55}, 16)
	state := &encryptState{key: docKey}
	rc4Key := state.objectKey(42)
	aesKey := objectKeyAES128(docKey, 42, 0)
	if bytes.Equal(rc4Key, aesKey[:len(rc4Key)]) {
		t.Errorf("AES and RC4 keys must differ for the same input")
	}
}
