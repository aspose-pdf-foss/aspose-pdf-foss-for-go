package asposepdf

import (
	"crypto/md5"
)

// addPKCS7 appends PKCS#7 padding to data. The padding length is always
// in 1..blockSize (even when len(data) is already a multiple of blockSize,
// a full block of padding is appended) per RFC 5652 §6.3.
func addPKCS7(data []byte, blockSize int) []byte {
	pad := blockSize - (len(data) % blockSize)
	out := make([]byte, len(data)+pad)
	copy(out, data)
	for i := len(data); i < len(out); i++ {
		out[i] = byte(pad)
	}
	return out
}

// objectKeyAES128 derives the per-object AES-128 key per PDF Algorithm 1.A
// (ISO 32000-1 §7.6.2). The literal 4-byte "sAlT" suffix differentiates
// the key from the RC4 Algorithm 1 computation on the same document key.
func objectKeyAES128(docKey []byte, objNum, gen int) []byte {
	buf := make([]byte, 0, len(docKey)+5+4)
	buf = append(buf, docKey...)
	buf = append(buf,
		byte(objNum), byte(objNum>>8), byte(objNum>>16),
		byte(gen), byte(gen>>8),
		's', 'A', 'l', 'T')
	sum := md5.Sum(buf)
	return sum[:16] // full MD5 output for AES-128
}
