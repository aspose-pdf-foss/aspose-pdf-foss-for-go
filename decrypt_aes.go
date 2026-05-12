package asposepdf

import (
	"crypto/aes"
	"crypto/cipher"
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

// decryptObjectAES128 is the inverse of encryptBytesAES128. The first
// 16 bytes of ciphertext are the IV; the remainder is AES-128-CBC
// ciphertext of PKCS#7-padded plaintext under the per-object key.
func decryptObjectAES128(s *encryptState, objNum, gen int, ciphertext []byte) ([]byte, error) {
	key := objectKeyAES128(s.key, objNum, gen)
	if len(ciphertext) < aes.BlockSize {
		return nil, fmt.Errorf("AES ciphertext shorter than IV (%d bytes)", len(ciphertext))
	}
	iv := ciphertext[:aes.BlockSize]
	body := ciphertext[aes.BlockSize:]
	if len(body)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("AES ciphertext body not block-aligned (%d bytes)", len(body))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	plain := make([]byte, len(body))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, body)
	return stripPKCS7(plain)
}
