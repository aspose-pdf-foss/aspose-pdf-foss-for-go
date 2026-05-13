package asposepdf

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
)

// decryptObjectAES256 is the inverse of encryptBytesAES256. The first
// 16 bytes of ciphertext are the IV; the remainder is AES-256-CBC
// ciphertext of PKCS#7-padded plaintext under the FEK.
func decryptObjectAES256(s *encryptState, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < aes.BlockSize {
		return nil, fmt.Errorf("AES-256 ciphertext shorter than IV (%d bytes)", len(ciphertext))
	}
	iv := ciphertext[:aes.BlockSize]
	body := ciphertext[aes.BlockSize:]
	if len(body)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("AES-256 body not block-aligned (%d bytes)", len(body))
	}
	block, err := aes.NewCipher(s.key) // 32-byte key → AES-256
	if err != nil {
		return nil, fmt.Errorf("AES-256 NewCipher: %w", err)
	}
	plain := make([]byte, len(body))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, body)
	return stripPKCS7(plain)
}
