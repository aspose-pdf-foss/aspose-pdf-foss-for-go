// SPDX-License-Identifier: MIT

package asposepdf

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/binary"
	"fmt"
	"io"
)

// Public-key security handler (ISO 32000-1 §7.6.4, /Filter /Adobe.PubSec):
// the document is encrypted for a set of X.509 certificates, and only the
// holders of the matching private keys can open it — no shared password.
//
//	doc.SetEncryption(pdf.EncryptionOptions{
//	    Recipients: []pdf.Recipient{{Certificate: alice}, {Certificate: bob, Permissions: &readOnly}},
//	    Algorithm:  pdf.EncryptionAlgAES256,
//	})
//	...
//	doc, err := pdf.OpenWithCertificate("secret.pdf", aliceCert, aliceKey)
//
// Mechanics: a random 20-byte seed plus that recipient's 4 permission bytes
// are sealed into one CMS EnvelopedData per recipient (RSA key transport,
// AES-CBC content encryption) and stored in /Recipients. The file encryption
// key is the digest of the seed followed by every recipient blob in
// /Recipients order — SHA-1 for AESV2, SHA-256 for AESV3 — so any recipient
// who recovers the seed derives the same key. Per-object encryption is then
// identical to the standard handler, so the whole write/read pipeline is
// shared. Permissions are per recipient (each envelope carries its own),
// which the password handler cannot express.
//
// Scope: RSA recipients (PKCS#1 v1.5 key transport) with AES-128 (V4,
// /adbe.pkcs7.s5 + /CFM /AESV2) or AES-256 (V5, /CFM /AESV3). The legacy RC4
// sub-filters (s3/s4) are not written; EC recipients (key agreement) are not
// supported.

// Recipient is one certificate that may open a public-key-encrypted document,
// optionally with its own permission set.
type Recipient struct {
	// Certificate is the recipient's X.509 certificate (RSA public key).
	Certificate *x509.Certificate
	// Permissions limits what this recipient may do; nil grants everything.
	Permissions *Permissions
}

// CMS object identifiers (RFC 5652 §6, NIST AES OIDs).
var (
	oidEnvelopedData = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 3}
	oidAES128CBC     = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 1, 2}
	oidAES256CBC     = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 1, 42}
)

// ktRecipientInfo is a CMS KeyTransRecipientInfo (RFC 5652 §6.2.1) using the
// issuerAndSerialNumber choice, which is what PDF producers emit.
type ktRecipientInfo struct {
	Version                int
	RID                    issuerAndSerial
	KeyEncryptionAlgorithm algorithmIdentifier
	EncryptedKey           []byte
}

// encryptedContentInfo is RFC 5652 §6.1. The content is carried inline as an
// implicitly tagged [0] OCTET STRING.
type encryptedContentInfo struct {
	ContentType                asn1.ObjectIdentifier
	ContentEncryptionAlgorithm algorithmIdentifier
	EncryptedContent           []byte `asn1:"tag:0,optional"`
}

// envelopedDataASN is RFC 5652 §6.1 with the optional originatorInfo and
// unprotectedAttrs omitted (version 0).
type envelopedDataASN struct {
	Version              int
	RecipientInfos       []ktRecipientInfo `asn1:"set"`
	EncryptedContentInfo encryptedContentInfo
}

// pubsecPermissionBits adapts a /P bitfield to the convention public-key
// handlers use: bit 1 is set (rather than reserved), bits 7-8 are clear, and
// the high bits 13-32 are set — matching Acrobat and PDFBox, so viewers read
// the same permissions we intend.
func pubsecPermissionBits(p int32) int32 {
	v := uint32(p)
	v |= 1 << 0       // bit 1
	v &^= 1<<6 | 1<<7 // bits 7, 8
	v |= 0xFFFFF000   // bits 13..32
	return int32(v)   //nolint:gosec // deliberate bitfield round-trip
}

// buildRecipientEnvelope seals blob for cert as a CMS EnvelopedData, using
// AES-CBC of the given key size for the content and RSA key transport for the
// content-encryption key.
func buildRecipientEnvelope(blob []byte, cert *x509.Certificate, aesKeyBytes int) ([]byte, error) {
	if cert == nil {
		return nil, fmt.Errorf("pubsec: nil recipient certificate")
	}
	rsaPub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("pubsec: recipient %q has a %T public key; only RSA is supported",
			cert.Subject.CommonName, cert.PublicKey)
	}

	cek := make([]byte, aesKeyBytes)
	if _, err := io.ReadFull(cryptorand.Reader, cek); err != nil {
		return nil, fmt.Errorf("pubsec: generate content key: %w", err)
	}
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(cryptorand.Reader, iv); err != nil {
		return nil, fmt.Errorf("pubsec: generate IV: %w", err)
	}
	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, fmt.Errorf("pubsec: content cipher: %w", err)
	}
	padded := addPKCS7(blob, aes.BlockSize)
	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, padded)

	encKey, err := rsa.EncryptPKCS1v15(cryptorand.Reader, rsaPub, cek)
	if err != nil {
		return nil, fmt.Errorf("pubsec: wrap content key: %w", err)
	}

	ivDER, err := asn1.Marshal(iv)
	if err != nil {
		return nil, fmt.Errorf("pubsec: encode IV: %w", err)
	}
	contentAlg := oidAES256CBC
	if aesKeyBytes == 16 {
		contentAlg = oidAES128CBC
	}

	ed := envelopedDataASN{
		Version: 0,
		RecipientInfos: []ktRecipientInfo{{
			Version: 0,
			RID: issuerAndSerial{
				IssuerRaw:    asn1.RawValue{FullBytes: cert.RawIssuer},
				SerialNumber: cert.SerialNumber,
			},
			KeyEncryptionAlgorithm: algorithmIdentifier{
				Algorithm:  oidRSAEncryption,
				Parameters: asn1NULL(),
			},
			EncryptedKey: encKey,
		}},
		EncryptedContentInfo: encryptedContentInfo{
			ContentType: oidData,
			ContentEncryptionAlgorithm: algorithmIdentifier{
				Algorithm:  contentAlg,
				Parameters: asn1.RawValue{FullBytes: ivDER},
			},
			EncryptedContent: ciphertext,
		},
	}
	edDER, err := asn1.Marshal(ed)
	if err != nil {
		return nil, fmt.Errorf("pubsec: encode EnvelopedData: %w", err)
	}
	out, err := asn1.Marshal(contentInfo{
		ContentType: oidEnvelopedData,
		Content: asn1.RawValue{
			Class:      asn1.ClassContextSpecific,
			Tag:        0,
			IsCompound: true,
			Bytes:      edDER,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("pubsec: encode ContentInfo: %w", err)
	}
	return out, nil
}

// openRecipientEnvelope decrypts der when it is addressed to cert; addressed
// is false when this envelope belongs to a different recipient.
func openRecipientEnvelope(der []byte, cert *x509.Certificate, key crypto.Decrypter) (blob []byte, addressed bool, err error) {
	var ci contentInfo
	if _, err := asn1.Unmarshal(der, &ci); err != nil {
		return nil, false, fmt.Errorf("pubsec: parse ContentInfo: %w", err)
	}
	if !ci.ContentType.Equal(oidEnvelopedData) {
		return nil, false, fmt.Errorf("pubsec: recipient blob is not EnvelopedData")
	}
	var ed envelopedDataASN
	if _, err := asn1.Unmarshal(ci.Content.Bytes, &ed); err != nil {
		return nil, false, fmt.Errorf("pubsec: parse EnvelopedData: %w", err)
	}

	var encKey []byte
	for _, ri := range ed.RecipientInfos {
		if ri.RID.SerialNumber.Cmp(cert.SerialNumber) == 0 &&
			bytesEqual(ri.RID.IssuerRaw.FullBytes, cert.RawIssuer) {
			encKey = ri.EncryptedKey
			break
		}
	}
	if encKey == nil {
		return nil, false, nil
	}

	cek, err := key.Decrypt(cryptorand.Reader, encKey, nil)
	if err != nil {
		return nil, true, fmt.Errorf("pubsec: unwrap content key: %w", err)
	}
	alg := ed.EncryptedContentInfo.ContentEncryptionAlgorithm
	if !alg.Algorithm.Equal(oidAES128CBC) && !alg.Algorithm.Equal(oidAES256CBC) {
		return nil, true, fmt.Errorf("pubsec: unsupported content cipher %v", alg.Algorithm)
	}
	var iv []byte
	if _, err := asn1.Unmarshal(alg.Parameters.FullBytes, &iv); err != nil {
		return nil, true, fmt.Errorf("pubsec: parse content IV: %w", err)
	}
	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, true, fmt.Errorf("pubsec: content cipher: %w", err)
	}
	ct := ed.EncryptedContentInfo.EncryptedContent
	if len(iv) != aes.BlockSize || len(ct) == 0 || len(ct)%aes.BlockSize != 0 {
		return nil, true, fmt.Errorf("pubsec: malformed enveloped content")
	}
	plain := make([]byte, len(ct))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, ct)
	plain, err = stripPKCS7(plain)
	if err != nil {
		return nil, true, fmt.Errorf("pubsec: %w", err)
	}
	if len(plain) < 24 {
		return nil, true, fmt.Errorf("pubsec: recipient payload is %d bytes, want at least 24", len(plain))
	}
	return plain, true, nil
}

// bytesEqual compares two byte slices (kept local so the ASN.1 code reads
// without a bytes import in this file).
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// pubsecFileKey derives the file encryption key: the digest over the seed
// followed by every recipient blob in /Recipients order (and 0xFFFFFFFF when
// metadata is left unencrypted), truncated to keyLen bytes. SHA-256 for
// AES-256, SHA-1 otherwise (ISO 32000-1 §7.6.4.3 + Adobe extension level 3).
func pubsecFileKey(seed []byte, recipients [][]byte, keyLen int, sha256Based, encryptMetadata bool) []byte {
	var sum []byte
	if sha256Based {
		h := sha256.New()
		h.Write(seed)
		for _, r := range recipients {
			h.Write(r)
		}
		if !encryptMetadata {
			h.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF})
		}
		sum = h.Sum(nil)
	} else {
		h := sha1.New() //nolint:gosec // mandated by ISO 32000-1 §7.6.4.3 for AESV2
		h.Write(seed)
		for _, r := range recipients {
			h.Write(r)
		}
		if !encryptMetadata {
			h.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF})
		}
		sum = h.Sum(nil)
	}
	if keyLen > len(sum) {
		keyLen = len(sum)
	}
	return sum[:keyLen]
}

// newEncryptStatePubSec seals the file key for every recipient and returns
// the write-side state. AES-256 (V5) is the default; AES-128 (V4) is the only
// other supported choice.
func newEncryptStatePubSec(cfg *encryptConfig) (*encryptState, error) {
	if len(cfg.recipients) == 0 {
		return nil, fmt.Errorf("pubsec: no recipients")
	}
	aes256 := cfg.algorithm != EncryptionAlgAES128
	if cfg.algorithm == EncryptionAlgRC4_128 {
		return nil, fmt.Errorf("pubsec: RC4 is not supported for certificate encryption; " +
			"use EncryptionAlgAES128 or EncryptionAlgAES256")
	}
	keyLen := 16
	if aes256 {
		keyLen = 32
	}

	seed := make([]byte, 20)
	if _, err := io.ReadFull(cryptorand.Reader, seed); err != nil {
		return nil, fmt.Errorf("pubsec: generate seed: %w", err)
	}

	blobs := make([][]byte, 0, len(cfg.recipients))
	for _, r := range cfg.recipients {
		perms := cfg.effectivePermissions()
		if r.Permissions != nil {
			perms = r.Permissions.toPDFBits()
		}
		payload := make([]byte, 24)
		copy(payload, seed)
		binary.BigEndian.PutUint32(payload[20:], uint32(pubsecPermissionBits(perms))) //nolint:gosec // bitfield
		env, err := buildRecipientEnvelope(payload, r.Certificate, keyLen)
		if err != nil {
			return nil, err
		}
		blobs = append(blobs, env)
	}

	return &encryptState{
		algorithm:   cfg.algorithm,
		key:         pubsecFileKey(seed, blobs, keyLen, aes256, true),
		permissions: cfg.effectivePermissions(),
		pubSec:      true,
		recipients:  blobs,
	}, nil
}

// buildDecryptStatePubSec parses an /Adobe.PubSec /Encrypt dictionary and
// recovers the file key using the caller's certificate and private key.
func buildDecryptStatePubSec(encDict pdfDict, cert *x509.Certificate, key crypto.Decrypter) (*encryptState, error) {
	if cert == nil || key == nil {
		return nil, fmt.Errorf("PDF is encrypted for certificate recipients; " +
			"use OpenWithCertificate with the matching certificate and private key")
	}
	v := dictGetInt(encDict, "/V")
	alg := EncryptionAlgAES256
	if v == 4 {
		alg = EncryptionAlgAES128
	}

	// /Recipients lives in the crypt filter for V4/V5, or in the encryption
	// dictionary itself for the older V1/V2 layouts.
	recipVal := encDict["/Recipients"]
	encryptMetadata := true
	if b, ok := encDict["/EncryptMetadata"].(bool); ok {
		encryptMetadata = b
	}
	if cf, ok := encDict["/CF"].(pdfDict); ok {
		name := dictGetName(encDict, "/StmF")
		if name == "" {
			name = "/DefaultCryptFilter"
		}
		filterDict, ok := cf[name].(pdfDict)
		if !ok {
			// Fall back to the single crypt filter present, whatever its name.
			for _, v := range cf {
				if d, isDict := v.(pdfDict); isDict {
					filterDict = d
					break
				}
			}
		}
		if filterDict != nil {
			if rv, ok := filterDict["/Recipients"]; ok {
				recipVal = rv
			}
			switch dictGetName(filterDict, "/CFM") {
			case "/AESV2":
				alg = EncryptionAlgAES128
			case "/AESV3":
				alg = EncryptionAlgAES256
			}
			if b, ok := filterDict["/EncryptMetadata"].(bool); ok {
				encryptMetadata = b
			}
		}
	}

	arr, ok := recipVal.(pdfArray)
	if !ok || len(arr) == 0 {
		return nil, fmt.Errorf("public-key /Encrypt dict has no /Recipients")
	}
	blobs := make([][]byte, 0, len(arr))
	for _, v := range arr {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("public-key /Recipients entry is not a string")
		}
		blobs = append(blobs, []byte(s))
	}

	var payload []byte
	for _, b := range blobs {
		p, addressed, err := openRecipientEnvelope(b, cert, key)
		if err != nil {
			if addressed {
				return nil, err
			}
			continue // a malformed foreign envelope must not block our own
		}
		if addressed {
			payload = p
			break
		}
	}
	if payload == nil {
		return nil, fmt.Errorf("the document is not encrypted for certificate %q",
			cert.Subject.CommonName)
	}

	keyLen := 16
	if alg == EncryptionAlgAES256 {
		keyLen = 32
	}
	perms := int32(binary.BigEndian.Uint32(payload[20:24])) //nolint:gosec // bitfield
	return &encryptState{
		algorithm:   alg,
		key:         pubsecFileKey(payload[:20], blobs, keyLen, alg == EncryptionAlgAES256, encryptMetadata),
		permissions: perms,
		pubSec:      true,
		recipients:  blobs,
	}, nil
}

// buildPubSecEncryptDict builds the /Encrypt dictionary for the public-key
// handler: no /O, /U or /P (permissions travel inside the envelopes), and
// /Recipients carried by the crypt filter (ISO 32000-1 §7.6.4.2).
func buildPubSecEncryptDict(s *encryptState) pdfDict {
	recips := make(pdfArray, 0, len(s.recipients))
	for _, r := range s.recipients {
		recips = append(recips, string(r))
	}
	cfm, length, v := pdfName("/AESV3"), 32, 5
	if s.algorithm == EncryptionAlgAES128 {
		cfm, length, v = "/AESV2", 16, 4
	}
	return pdfDict{
		"/Filter":    pdfName("/Adobe.PubSec"),
		"/SubFilter": pdfName("/adbe.pkcs7.s5"),
		"/V":         v,
		"/Length":    length * 8,
		"/CF": pdfDict{
			"/DefaultCryptFilter": pdfDict{
				"/Type":            pdfName("/CryptFilter"),
				"/CFM":             cfm,
				"/AuthEvent":       pdfName("/DocOpen"),
				"/Length":          length,
				"/Recipients":      recips,
				"/EncryptMetadata": true,
			},
		},
		"/StmF":            pdfName("/DefaultCryptFilter"),
		"/StrF":            pdfName("/DefaultCryptFilter"),
		"/EncryptMetadata": true,
	}
}
