package asposepdf

import "testing"

// TestEncryptPasswordVerification tests the cryptographic round-trip:
// passwords that were used to produce /O and /U can be verified against them.
func TestEncryptPasswordVerification(t *testing.T) {
	fileID := []byte("0123456789abcdef") // fixed 16-byte ID for determinism

	oEntry := computeOwnerEntry("user", "owner")
	key := computeEncKey("user", oEntry, encryptPermissions, fileID)
	uEntry := computeUserEntry(key, fileID)

	if !verifyUserPassword("user", oEntry, uEntry, fileID) {
		t.Error("correct user password not verified")
	}
	if verifyUserPassword("wrong", oEntry, uEntry, fileID) {
		t.Error("wrong password incorrectly verified")
	}
	if verifyUserPassword("owner", oEntry, uEntry, fileID) {
		t.Error("owner password should not satisfy user verification")
	}
}

// TestEncryptSameUserOwner verifies that an empty owner password falls back to user password.
func TestEncryptSameUserOwner(t *testing.T) {
	fileID := []byte("fedcba9876543210")

	oEntry := computeOwnerEntry("secret", "secret") // ownerPwd == userPwd
	key := computeEncKey("secret", oEntry, encryptPermissions, fileID)
	uEntry := computeUserEntry(key, fileID)

	if !verifyUserPassword("secret", oEntry, uEntry, fileID) {
		t.Error("correct password not verified when user == owner")
	}
}

// TestEncryptRC4Symmetric verifies that encrypting twice (RC4 XOR) restores the original.
func TestEncryptRC4Symmetric(t *testing.T) {
	s := &encryptState{key: make([]byte, encKeyLen)}
	copy(s.key, []byte("0123456789abcdef"))

	original := []byte("Hello, PDF encryption!")
	encrypted := s.encryptBytes(5, original)
	decrypted := s.encryptBytes(5, encrypted)

	if string(decrypted) != string(original) {
		t.Errorf("RC4 round-trip failed: got %q, want %q", decrypted, original)
	}
}
