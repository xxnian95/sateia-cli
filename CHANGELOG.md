# Changelog

## Unreleased

### Changed

- Preserve and surface complete backend error context, unknown error fields,
  `Retry-After`, request IDs, and non-contract response bodies in both human
  and structured JSON output.

## 1.3.0 (2026-08-10)

### Features

- Expose a versioned machine-readable command contract through
  `--help --format json`, including field types, requirements, cross-field
  rules, examples, and explicit operation risk.
- Emit stable structured error envelopes on stderr for global `--json`
  commands while preserving non-zero exit status and API recovery metadata.
- Embed the matching `use-sateia-cli` agent skill and add hash-protected
  `skill check`, `skill install`, and `skill update` workflows.
- Add `doctor` diagnostics for CLI version, clock, server, credential,
  authenticated access, updates, and agent skill alignment.

### Changed

- Refresh live help and the bundled skill before every concrete AI-agent
  invocation, and prioritize food name and quantity at the start of notes.

## 1.2.0 (2026-08-09)

### Features

- Manage date-specific nutrition goals with get, set, and delete commands.

## 1.1.0 (2026-08-09)

### Features

- Accept a one-off CLI token through the global `--token` flag, with precedence
  over environment, token-file, and keyring credentials.
- Update nutrition records with optimistic version checks, complete nutrient
  replacement, nullable-note handling, and idempotent retry guidance.
- Soft-delete nutrition records with optimistic version checks and idempotent
  tombstone responses.

### Changed

- Add source-specific authentication recovery, safe list-query retry guidance,
  and clearer validation errors across CLI commands.
- Rewrite the user and AI-agent guide and add a separate Simplified Chinese
  README.

## 1.0.0 (2026-08-07)

### Features

- Authenticate with a short-lived pairing code and store the issued token in
  the system credential store or an explicitly selected private token file.
- Query and create nutrition records with machine-readable output and safe
  idempotent retry guidance.
- Surface structured response notices and cached stable-version update checks.
