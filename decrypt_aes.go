package asposepdf

import (
	"crypto/aes"
	"fmt"
)

// stripPKCS7 removes PKCS#7 padding from data and returns the unpadded
// bytes. data must be a positive multiple of aes.BlockSize. The final
// byte indicates the pad length (1..16); all pad bytes must equal that
// value or an error is returned.
func stripPKCS7(data []byte) ([]byte, error) {
	if len(data) == 0 || len(data)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("PKCS#7: bad length %d", len(data))
	}
	pad := int(data[len(data)-1])
	if pad == 0 || pad > aes.BlockSize {
		return nil, fmt.Errorf("PKCS#7: invalid pad byte %d", pad)
	}
	for i := len(data) - pad; i < len(data); i++ {
		if data[i] != byte(pad) {
			return nil, fmt.Errorf("PKCS#7: malformed padding at offset %d", i)
		}
	}
	return data[:len(data)-pad], nil
}
