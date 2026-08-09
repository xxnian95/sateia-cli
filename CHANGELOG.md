# Changelog

## Unreleased

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
