# Security Review (Codebase Investigation)

Date: 2026-02-20

This document captures security issues and improvement areas identified in the current `helm-values-checker` codebase.

## High-Risk Findings

### 1) SSRF + Local File Read via JSON Schema `$ref` Resolution

**Where**
- `internal/validator/schema.go:91` (`gojsonschema.NewBytesLoader(schemaBytes)`)
- `internal/validator/schema.go:98` (`gojsonschema.Validate(schemaLoader, docLoader)`)

**Why it matters**
- The JSON schema being validated is sourced from the chart (`values.schema.json`). A malicious chart can embed `$ref` entries that point to external `http(s)://...` or `file://...` targets.
- `github.com/xeipuuv/gojsonschema`’s default loader factory supports resolving `http(s)` and `file` references. That means schema validation can:
  - make outbound network calls (SSRF), and/or
  - attempt to read local files (e.g., `file:///etc/passwd`) during validation.

**Impact**
- Running validation against untrusted charts can cause unintended network access from the environment executing the tool.
- Local file read attempts (even if not exfiltrated directly by this tool) are still a serious capability increase and can interact badly with logging/CI artifacts or side channels.

**Recommended remediation**
- Fail validation if the schema contains external references:
  - allow only fragment refs (`$ref: "#/..."`) and disallow all other schemes (`http`, `https`, `file`, etc.).
- Alternatively, replace schema validation with a library that supports a restricted reference loader, or implement a custom loader/factory that rejects non-fragment references and ensure both schema and document loaders use it.

### 2) Schema Validation Can Fail Open (Silent Drop of Findings)

**Where**
- `internal/validator/schema.go:82-88` (YAML marshal/unmarshal errors return `findings` empty)
- `internal/validator/schema.go:92-101` (JSON marshal / schema validate errors return `findings` empty)

**Why it matters**
- When schema processing fails, the tool currently returns “no schema findings” rather than surfacing an error.
- This can let crafted inputs (or malformed schemas) bypass required-field and deprecated-key detection without alerting the user.

**Impact**
- CI/pipelines may incorrectly pass a values file because schema checks were skipped due to an internal error.

**Recommended remediation**
- Change `validateSchema(...)` to return `([]model.Finding, error)` and propagate the error up to `validator.Validate` (`internal/validator/validator.go:59-61`) so the CLI can exit with tool-error code `3`.

## Medium-Risk Findings

### 3) Terminal Escape Injection in Text Output

**Where**
- `internal/output/formatter.go:30-33` (prints `f.Message` and `f.Suggestion`)
- `internal/output/formatter.go:45-46` (prints `f.Message`)
- `internal/validator/schema.go:186-189` (schema `description` can be appended into warning messages)

**Why it matters**
- Findings include data derived from untrusted sources (charts, schemas, and user-provided YAML).
- If those strings contain ANSI escape sequences/control characters, terminal output can be spoofed (e.g., hide errors, rewrite lines, fake “success”).

**Recommended remediation**
- Sanitize untrusted strings before printing:
  - strip ASCII control characters (especially `\x1b` ESC), or
  - escape them before rendering (e.g., replace with visible sequences).

### 4) Unbounded Remote Chart / Values File Processing (DoS Risk)

**Where**
- Remote chart pull and untar: `internal/chart/resolver.go:70-121`
- Whole-file read: `internal/validator/validator.go:14` (`os.ReadFile`)

**Why it matters**
- A large chart (or a values file) can consume disk or memory and degrade/kill CI jobs.
- While Helm’s libraries do much of the heavy lifting, the plugin should still apply reasonable limits when processing potentially untrusted inputs.

**Recommended remediation**
- Add size limits:
  - `stat` values files before reading, and enforce a max size (configurable).
  - enforce a max extracted chart size / file count (best effort), and document it.

## Low-Risk / Hardening Opportunities

### 5) Temp Directory Cleanup Is Skipped Due to `os.Exit` in `RunE`

**Where**
- Deferred cleanup: `cmd/validate.go:63` (`defer resolved.Cleanup()`)
- Early exits: `cmd/validate.go:61`, `cmd/validate.go:71-72`, `cmd/validate.go:93-95` (`os.Exit`)

**Why it matters**
- `os.Exit(...)` bypasses deferred functions. This can leak temporary directories created during remote chart resolution, gradually filling disk in repeated runs (especially in CI).

**Recommended remediation**
- Avoid `os.Exit` inside Cobra `RunE`. Prefer returning an error and letting `main` decide exit codes.
- If you must keep `os.Exit`, explicitly call `resolved.Cleanup()` before exiting.

### 6) Installer Script Downloads Executables Without Integrity Verification

**Where**
- `install-binary.sh:48` (`curl ... | tar xz ...`)
- Checksums are produced by GoReleaser: `.goreleaser.yml:23-24`

**Why it matters**
- `curl | tar` provides no verification of what is executed. TLS helps, but integrity should be verified against published checksums/signatures.

**Recommended remediation**
- Download tarball + `checksums.txt`, verify SHA256, then extract.
- Add `curl -fSL` and avoid pipes for better error handling.

### 7) CI/Release Supply-Chain Hardening

**Where**
- Actions pinned to major tags: `.github/workflows/ci.yml`, `.github/workflows/release.yml`
- GoReleaser uses `version: latest`: `.github/workflows/release.yml:24-27`

**Why it matters**
- Pinning to major tags increases exposure to upstream changes.

**Recommended remediation**
- Pin GitHub Actions to commit SHAs.
- Pin GoReleaser to a specific version.
- Set minimal workflow permissions (CI currently uses defaults).

## Suggested Fix Order

1. Block external schema `$ref` (SSRF/file read) and fail closed on schema errors.
2. Remove `os.Exit` from Cobra `RunE` so cleanup reliably runs.
3. Sanitize output strings to prevent terminal injection.
4. Add input/chart size limits.
5. Add checksum verification to `install-binary.sh`.
6. Harden GitHub Actions pinning and permissions.

