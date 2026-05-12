# AES-128 Encryption Design Spec (Subepic 1 of `pdf-go-ccl`)

**Date:** 2026-05-12
**Issue:** `pdf-go-ccl` (AES-128 and AES-256 encryption, Standard Security Handler V4/V5/V6)
**Subepic 1 scope:** AES-128, V=4 R=4, /CFM /AESV2 — read + write
**Deferred to Subepic 2:** AES-256 V=5 R=6 (separate plan after Subepic 1 ships)

## Goals

- Read PDFs encrypted with AES-128 (V=4 R=4, /CFM /AESV2, single StdCF crypt filter).
- Write PDFs encrypted with AES-128 via `(*Document).SetEncryption(opts)` with `Algorithm: EncryptionAlgAES128`.
- AES-128 becomes the default value of `EncryptionOptions.Algorithm` (zero value).
- RC4-128 V2R3 (the current implementation) continues to work without regressions, explicitly selected via `EncryptionAlgRC4_128`.

## Non-Goals

- AES-256 (V=5 R=5 deprecated, V=5 R=6 ISO 32000-2) — Subepic 2.
- Per-stream `/Crypt` filter overrides (`/Filter [/Crypt …]` inside individual streams). Only single document-wide StdCF.
- Public-key encryption (`/Filter /PubSec`). Out of scope for both subepics.
- Crypt filter for embedded files distinct from default (`/EFF`). FileAttachments use StdCF.
- 40-bit RC4 (V=1) — not supported, will not be added.

## Architecture

### High-level flow

Reuse the entire password-handling and document-key-derivation machinery already in place for RC4 V2R3. AES-128 V4R4 keeps the **same** PDF Algorithms 2, 3, 5, and 7 (password padding, /O entry, document key, /U entry, owner-password recovery). The differences from RC4 are localized:

1. **Per-object key derivation** — Algorithm 1.A appends a literal 4-byte `"sAlT"` suffix before the MD5 digest, differentiating the object key from the RC4 case.
2. **Per-object cipher** — AES-128-CBC with PKCS#7 padding and a random 16-byte IV prepended to each encrypted blob.
3. **/Encrypt dict shape** — V=4 R=4 adds `/CF` (crypt filter dict) with a `/StdCF` entry whose `/CFM = /AESV2`, plus `/StmF` and `/StrF` both pointing to `/StdCF`.

### Reused from RC4 (V2R3) unchanged

- Algorithm 2 — document key from `password + /O + /P + fileID`.
- Algorithm 3 — `/O` entry computation.
- Algorithm 5 — `/U` entry computation (per ISO 32000-1 §7.6.3.4 Adobe specifies MD5+RC4 even for V=4).
- Algorithm 7 — owner-password recovery from `/O`.
- Password padding constants, fileID generation, `/P` permissions bit packing.

### New for AES-128

- Algorithm 1.A (per-object key with `"sAlT"` suffix).
- AES-CBC encryption/decryption with PKCS#7 padding helpers.
- `/Encrypt` dict V=4 serialization (including `/CF`, `/StmF`, `/StrF` entries).
- Crypt-filter parser for V=4 input.
- Dispatcher logic in `encryptBytes` / `decryptObject` selecting algorithm by `encryptState.algorithm`.

### File organization

| File | Role |
|---|---|
| `encrypt.go` (modify) | `EncryptionAlgorithm` enum, `EncryptionOptions` extension, shared password/key helpers, `encryptBytes` dispatcher |
| `encrypt_aes.go` (new) | `encryptBytesAES128`, `objectKeyAES128`, `addPKCS7` |
| `decrypt.go` (modify) | `buildDecryptState` dispatcher by `/V`, shared validation |
| `decrypt_aes.go` (new) | `buildDecryptStateV4R4`, `decryptObjectAES128`, `stripPKCS7` |
| `writer.go` (modify) | Serialize `/Encrypt` V=4 (`/CF`, `/StmF`, `/StrF`); `encFn` signature change to return error |

## Public API

```go
// EncryptionAlgorithm selects the cipher and security-handler revision
// used by (*Document).SetEncryption. The zero value is AES-128.
type EncryptionAlgorithm int

const (
    // EncryptionAlgAES128 — AES-128, Standard Security Handler V=4 R=4,
    // /CFM /AESV2. ISO 32000-1 §7.6.3.2. The default (zero value).
    EncryptionAlgAES128 EncryptionAlgorithm = iota

    // EncryptionAlgRC4_128 — RC4-128, Standard Security Handler V=2 R=3.
    // Legacy compatibility only — AES-128 is preferred for new documents.
    EncryptionAlgRC4_128
)

type EncryptionOptions struct {
    UserPassword  string
    OwnerPassword string
    Permissions   *Permissions
    Algorithm     EncryptionAlgorithm // zero value = EncryptionAlgAES128
}
```

### Behaviour

- `doc.SetEncryption(opts)` — `opts.Algorithm` chooses the cipher. Zero value yields AES-128; explicit `EncryptionAlgRC4_128` retains the legacy path.
- `pdf.Encrypt(in, out, userPwd, ownerPwd)` (top-level helper) — stays RC4-128 (legacy entry point; documented as such).
- `pdf.OpenWithPassword(path, pwd)` / `pdf.OpenStreamWithPassword(r, pwd)` — unchanged surface; the dispatcher inside picks RC4 vs AES from `/V` and `/R`.
- `(*Document).Permissions()` / `RemoveEncryption()` / `SetPassword()` / `SetPermissions()` — unchanged.
- `ErrEncrypted` — unchanged sentinel.

### Default change

After Subepic 1 ships, `SetEncryption(EncryptionOptions{})` (zero value) encrypts AES-128. This is a behaviour change versus the current code (which encrypts RC4-128 in that case). The project is unpublished, so this is acceptable.

## Read/Decrypt Details

### Dispatching by `/V`

`buildDecryptState` becomes a dispatcher:

```go
func buildDecryptState(encDict pdfDict, trailer pdfDict, password string) (*encryptState, error) {
    v := dictGetInt(encDict, "/V")
    r := dictGetInt(encDict, "/R")
    switch {
    case v == 2 && r == 3:
        return buildDecryptStateV2R3(encDict, trailer, password)
    case v == 4 && r == 4:
        return buildDecryptStateV4R4(encDict, trailer, password)
    default:
        return nil, fmt.Errorf("unsupported /V=%d /R=%d", v, r)
    }
}
```

### `buildDecryptStateV4R4`

1. Read `/CF` dict and look up the entry referenced by `/StmF` (and verify `/StrF` points to the same — divergent cases unsupported).
2. Validate `/CFM == /AESV2` in that entry; reject otherwise.
3. Validate `/Length == 16` in the crypt filter entry, and `/Length == 128` (bits) in the `/Encrypt` dict itself.
4. Run Algorithms 2, 5, and 7 unchanged (the password-handling stays identical to V2R3).
5. Set `state.algorithm = EncryptionAlgAES128` for downstream dispatch.

### Per-object decryption — Algorithm 1.A

```go
func objectKeyAES128(docKey []byte, objNum, gen int) []byte {
    buf := make([]byte, 0, len(docKey)+5+4)
    buf = append(buf, docKey...)
    buf = append(buf,
        byte(objNum), byte(objNum>>8), byte(objNum>>16),
        byte(gen), byte(gen>>8),
        's', 'A', 'l', 'T') // literal salt per ISO 32000-1 §7.6.2
    sum := md5.Sum(buf)
    return sum[:16] // full MD5 output for AES-128
}

func decryptObjectAES128(state *encryptState, objNum, gen int, ciphertext []byte) ([]byte, error) {
    key := objectKeyAES128(state.key, objNum, gen)
    if len(ciphertext) < aes.BlockSize {
        return nil, fmt.Errorf("AES ciphertext shorter than IV")
    }
    iv := ciphertext[:aes.BlockSize]
    body := ciphertext[aes.BlockSize:]
    if len(body)%aes.BlockSize != 0 {
        return nil, fmt.Errorf("AES ciphertext not block-aligned")
    }
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, err
    }
    plain := make([]byte, len(body))
    cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, body)
    return stripPKCS7(plain)
}
```

### Stream filter ordering

The existing `getObject` decrypt-then-apply-filter ordering (encrypt-after-filter per PDF spec) holds for AES-128 unchanged. Inside `getObject` the call switches from direct `applyRC4` to `decryptObject(state, objNum, gen, data)` which dispatches by algorithm.

## Write/Encrypt Details

### State extension

```go
type encryptConfig struct {
    algorithm      EncryptionAlgorithm // new
    userPassword   string
    ownerPassword  string
    permissions    int32
    hasPermissions bool
}

type encryptState struct {
    algorithm   EncryptionAlgorithm // new
    key         []byte
    fileID      []byte
    ownerEntry  []byte
    userEntry   []byte
    permissions int32
}
```

`newEncryptState(cfg)` passes algorithm into state. Algorithms 2/3/5 are invoked unchanged regardless of algorithm choice.

### Per-object encryption — dispatcher

```go
func (s *encryptState) encryptBytes(objNum, gen int, data []byte) ([]byte, error) {
    switch s.algorithm {
    case EncryptionAlgRC4_128:
        return encryptBytesRC4(s, objNum, data), nil // existing path, factored
    case EncryptionAlgAES128:
        return encryptBytesAES128(s, objNum, gen, data)
    }
    return nil, fmt.Errorf("encryptBytes: unknown algorithm %d", s.algorithm)
}
```

The signature now carries `gen`, which AES uses and RC4 ignores. All writer call sites pass `gen=0` (output objects always generation 0).

### `encryptBytesAES128`

```go
func encryptBytesAES128(s *encryptState, objNum, gen int, plaintext []byte) ([]byte, error) {
    key := objectKeyAES128(s.key, objNum, gen)
    padded := addPKCS7(plaintext, aes.BlockSize)
    iv := make([]byte, aes.BlockSize)
    if _, err := io.ReadFull(cryptorand.Reader, iv); err != nil {
        return nil, fmt.Errorf("AES IV: %w", err)
    }
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, err
    }
    body := make([]byte, len(padded))
    cipher.NewCBCEncrypter(block, iv).CryptBlocks(body, padded)
    out := make([]byte, len(iv)+len(body))
    copy(out, iv)
    copy(out[len(iv):], body)
    return out, nil
}

func addPKCS7(data []byte, blockSize int) []byte {
    pad := blockSize - (len(data) % blockSize) // always 1..blockSize
    out := make([]byte, len(data)+pad)
    copy(out, data)
    for i := len(data); i < len(out); i++ {
        out[i] = byte(pad)
    }
    return out
}
```

### `/Encrypt` dict for V=4 R=4

```
<< /Filter /Standard
   /V 4 /R 4 /Length 128
   /P n /O <…> /U <…>
   /CF << /StdCF << /Type /CryptFilter /CFM /AESV2 /AuthEvent /DocOpen /Length 16 >> >>
   /StmF /StdCF
   /StrF /StdCF
>>
```

Writer branches on `state.algorithm`:

- `EncryptionAlgRC4_128` → current dict shape (no `/CF`).
- `EncryptionAlgAES128` → above shape.

### `encFn` signature change

Current `encFn func([]byte) []byte` becomes `encFn func([]byte) ([]byte, error)`. AES-encryption can fail on `io.ReadFull(rand.Reader, iv)`. Touch sites:

- `writer.go:writeValue` for stream serialization.
- `writer.go:writeValue` for string serialization (PDF strings inside dicts/arrays).
- Top-level orchestrator wires the error up.

This is a small refactor with finite reach; existing tests catch any missed sites.

## Testing Strategy

### Internal tests (`package asposepdf`)

**`encrypt_aes_internal_test.go`:**

- `TestObjectKeyAES128_KnownVector` — verify `MD5(docKey || objNum_LE_3 || gen_LE_2 || "sAlT")` matches a hardcoded reference vector (computed offline once, embedded as `[]byte{…}`).
- `TestPKCS7_RoundTrip` — `addPKCS7` → `stripPKCS7` returns original, including edge case "input length already block-multiple" (padding still adds 16 bytes).
- `TestPKCS7_InvalidPadding` — bad pad byte (0, 17, malformed trailing bytes) → error.
- `TestEncryptBytesAES128_RoundTrip` — random plaintext through encrypt → decrypt = identical.
- `TestEncryptBytesAES128_IVRandomness` — two encryptions of the same input produce different output (random IV).

**`decrypt_aes_internal_test.go`:**

- `TestDecryptObjectAES128_ShortCiphertext` — ciphertext < 16 bytes → error.
- `TestDecryptObjectAES128_UnalignedBody` — body length not a multiple of 16 → error.
- `TestBuildDecryptStateV4R4_MissingCF` — `/Encrypt` dict without `/CF` → error.
- `TestBuildDecryptStateV4R4_WrongCFM` — `/CFM /V2` (RC4 inside V=4 crypt filter, valid per spec but we don't support it) → error.

### External tests (`package asposepdf_test`)

**`encrypt_aes_test.go`:**

- `TestSetEncryptionAES128_DefaultsToAES` — `SetEncryption(EncryptionOptions{UserPassword: "x"})` without explicit Algorithm. Parse output bytes; assert `/V=4 /R=4 /CFM /AESV2`.
- `TestSetEncryptionAES128_RoundTrip` — Save → OpenWithPassword → ExtractText returns original content.
- `TestSetEncryptionAES128_OwnerPasswordRecovery` — Open with owner password succeeds (Algorithm 7 works under V=4).
- `TestSetEncryptionAES128_PermissionsRoundTrip` — `Permissions{AllowPrint: true}` survives AES round-trip.
- `TestSetEncryptionAES128_WrongPassword` — Open with wrong password returns `ErrEncrypted`.
- `TestSetEncryptionRC4Explicit` — `Algorithm: EncryptionAlgRC4_128` still works, output has `/V=2 /R=3` with no `/CF` (regression guard).
- `TestSetEncryptionAES128_WithFileAttachment` — page with FileAttachment annotation + AES encryption → roundtrip preserves embedded file bytes.
- `TestSetEncryptionAES128_WithAcroForm` — encrypt an AcroForm-bearing document; after roundtrip field values still readable.

### Cross-tool tests (`encrypt_aes_pypdf_test.go`)

Shell out to `python -c "…"` via `exec.Command`; `t.Skip` if pypdf import fails. Pattern already used in earlier subepics.

- `TestAES128_ReadableByPypdf` — our AES-128 output → pypdf decrypts and extracts the original text.
- `TestAES128_ReadsPypdfOutput` — pypdf generates an AES-128 PDF → our `OpenWithPassword` + `ExtractText` returns the original text.

### Regression baseline

All existing RC4 tests must pass without changes after the dispatcher refactor. Tests that construct `EncryptionOptions{}` (zero value) and depend on RC4 output are updated to set `Algorithm: EncryptionAlgRC4_128` explicitly, OR migrated to expect AES-128 (preferred where the test is checking round-trip rather than byte-exact /Encrypt shape). The migration is a dedicated task in the plan.

## Risks

1. **RC4 refactor regressions.** Mitigation: extract `encryptBytesRC4` as a named function carrying current logic verbatim; rely on the full existing test suite.
2. **`encFn` signature ripple.** Mitigation: small, mechanical change; type checker catches missed sites; one task in the plan.
3. **pypdf interop edge cases.** AES-128 is widely supported by pypdf 6.x, but corner cases (unusual filter combinations) may surface. Mitigation: keep cross-tool tests as smoke checks, not byte-exact. Pure-Go ground truth is our own roundtrip.
4. **Random IV non-determinism in tests.** Mitigation: all assertions are functional (roundtrip-identity, structural shape of `/Encrypt`), never byte-exact comparisons of the encrypted body.
5. **`/EFF` for FileAttachment streams.** Embedded file streams must also be AES-encrypted under StdCF. Covered by `TestSetEncryptionAES128_WithFileAttachment`.
6. **MD5 deprecation warning.** `crypto/md5` is deprecated as a general-purpose hash, but PDF V≤4 mandates it for key derivation. Documented in spec comments; no security choice on our part.
7. **AcroForm widget annotations.** Their `/AP/N` streams and `/V` field-value strings get encrypted by the AES path. Covered by `TestSetEncryptionAES128_WithAcroForm`.

## Aspose.PDF for .NET fidelity

Aspose.PDF for .NET exposes `CryptoAlgorithm.AESx128` alongside `RC4x40`, `RC4x128`, `AESx256`. Our `EncryptionAlgorithm` parallels it (we skip the deprecated 40-bit RC4):

- .NET: `doc.Encrypt(userPwd, ownerPwd, privilege, CryptoAlgorithm.AESx128)`
- Go (this library): `doc.SetEncryption(EncryptionOptions{UserPassword, OwnerPassword, Permissions, Algorithm: EncryptionAlgAES128})`

Semantically equivalent; Go idiomatic options-struct vs .NET positional args.

## Open Questions

None — all design decisions agreed during brainstorming.

## References

- ISO 32000-1:2008 §7.6 — encryption and security handlers
- ISO 32000-1:2008 §7.6.3.2 Table 22 — `/P` permission bits
- ISO 32000-1:2008 §7.6.3.4 — Standard Security Handler password algorithms
- ISO 32000-1:2008 §7.6.2 Algorithm 1.A — per-object AES key derivation
- Adobe `pdf_reference_1-7.pdf` Chapter 3.5
