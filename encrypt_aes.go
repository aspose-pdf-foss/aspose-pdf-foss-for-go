package asposepdf

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
