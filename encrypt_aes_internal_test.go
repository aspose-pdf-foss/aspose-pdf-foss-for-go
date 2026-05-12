package asposepdf

import (
	"bytes"
	"crypto/aes"
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
