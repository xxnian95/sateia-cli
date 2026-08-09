---
name: use-sateia-cli
description: Use the public Sateia CLI to authenticate, query, create, update, and delete server-side nutrition records safely. Use when a user or agent needs to install or discover the sateia command, identify the current machine for device-code authentication, supply a token with --token, SATEIA_TOKEN, or SATEIA_TOKEN_FILE, read or mutate energy and macronutrient data, interpret CLI output, paginate queries, or retry an ambiguous record write without duplication.
---

# Use Sateia CLI

## Overview

Use the CLI's live help as the command contract. Protect credentials and preserve
the record and mutation identifiers across ambiguous retries.

## When to Use

- Authenticate an interactive machine, headless server, container, or agent.
- Query nutrition records with explicit time bounds and cursor pagination.
- Create, update, or delete a nutrition record after the user explicitly
  requests that exact write.
- Diagnose credential-source, validation, or retry failures without exposing a
  token or duplicating a record.

## Discover the installed CLI

1. Run `command -v sateia`.
2. If installed, run `sateia --help` and `sateia environment` before acting.
3. If working from this repository, use `make build` and `./bin/sateia` when the
   global command is absent.
4. If neither is available, report that installation is required. Do not invent
   flags from this skill when live help disagrees.

## Select authentication

- Before exchanging a device code, run `hostname`. Choose a stable,
  recognizable device name for the current machine, such as
  `agent-host-01 (Sateia CLI)`. Do not reuse a generic name across machines.
- Ask the user to create a code in Sateia app > Settings > CLI Access. The CLI
  supplies its device name during exchange as token metadata, so it does not
  need to match a legacy name shown while creating the code. Pass the chosen
  current-machine name to the CLI:

  ```sh
  sateia auth login --device-code ABCD-EFGH --device-name "Pengnian Mac"
  ```

- For a one-off command, `--token` supplies a token with highest precedence and
  does not persist it. Warn that command arguments may be exposed through shell
  history or process inspection.
- For headless automation, prefer `SATEIA_TOKEN`. Never print it, store it in
  repository files, or include it in logs.
- Credential precedence is `--token`, `SATEIA_TOKEN`, `SATEIA_TOKEN_FILE`, then
  the system keyring. If a secret is supplied as a mounted file, set
  `SATEIA_TOKEN_FILE`.
- On headless Linux without Secret Service, `auth status` cannot inspect the
  keyring. Use a managed token or create a new path with `auth login
  --token-file`. The path's parent must exist and the CLI must be allowed to
  create the file with mode `0600`.

  ```sh
  sateia auth login \
    --device-code ABCD-EFGH \
    --device-name "agent-host-01 (Sateia CLI)" \
    --token-file /secure/path/sateia-token
  export SATEIA_TOKEN_FILE=/secure/path/sateia-token
  sateia auth status
  ```

- Never claim that a Secret Service error means the user is logged out. It
  means the credential backend could not be inspected.
- Use `--server` or `SATEIA_SERVER` only when the user selects a non-default
  server. Never downgrade a remote server to HTTP.
- Run `sateia auth status` before a write. Treat a non-zero exit as a blocker.

## Query nutrition records

Use a read before considering a write when the user's request can be answered
from existing records.

1. Run `sateia record list --help` and follow the live flag contract.
2. Choose an explicit consumed-time window. `--consumed-from` is inclusive and
   `--consumed-before` is exclusive; both are RFC 3339 timestamps.
3. Set `--limit` from 1 to 100. Deleted records are excluded unless the user
   explicitly needs them and you add `--include-deleted`.
4. Prefer `--json` and inspect `records`, `has_more`, and `next_cursor`.

```sh
sateia record list \
  --consumed-from 2026-08-01T00:00:00+08:00 \
  --consumed-before 2026-08-08T00:00:00+08:00 \
  --limit 50 \
  --json
```

The CLI returns only one page. If the user asked for all matching records and
`has_more` is true, repeat the command with the exact same time bounds,
`--include-deleted` choice, and limit, adding `--cursor` with the returned
`next_cursor`. Treat cursors as opaque pagination values: do not modify or
decode them. Stop when `has_more` is false. Never invent a time window when the
user's intended bounds are ambiguous.

## Create a nutrition record

Only write when the user explicitly requests a nutrition record mutation.

1. Run `sateia record create --help`.
2. Collect all four required amounts: energy in kilocalories, and protein,
   carbohydrate, and fat in grams. Use non-negative decimal strings, not values
   containing unit suffixes.
3. Preserve the user's consumption time. When supplied, use RFC 3339 with its
   original UTC offset. Do not silently replace a known meal time with now.
4. Prefer `--json` so the response includes both `mutation_id` and the record.

```sh
sateia record create \
  --energy 520 \
  --protein 28.5 \
  --carbohydrate 62 \
  --fat 18 \
  --note "Pengnian lunch" \
  --consumed-at 2026-08-07T12:30:00+08:00 \
  --json
```

Report the created `record_id`, `mutation_id`, version, and consumed time. Do not
claim success from HTTP reachability alone; require a zero exit and a decoded
success response.

## Update a nutrition record

1. Run `sateia record update --help` and identify the exact record and its
   current version from a trusted read result.
2. Change only fields requested by the user. If any nutrient changes, supply
   all four nutrient flags because the complete nutrient map is replaced.
3. Use `--note ""` only for an explicit empty string and `--clear-note` only
   when the user intends to store null.
4. Prefer `--json` and report the returned record version and mutation ID.

```sh
sateia record update \
  --record-id 014b2680-df5b-4c8d-97ef-abde0a9746d6 \
  --expected-version 1 \
  --energy 610 \
  --protein 32 \
  --carbohydrate 70 \
  --fat 22 \
  --json
```

Never infer `record_id` or `expected_version`. A version conflict requires a
fresh read and user intent review before submitting a new mutation.

## Delete a nutrition record

Run `sateia record delete --help`. Delete only the exact record the user named
or unambiguously selected, and use its reviewed current version.

```sh
sateia record delete \
  --record-id 014b2680-df5b-4c8d-97ef-abde0a9746d6 \
  --expected-version 2 \
  --json
```

Deletion is a soft delete, but the CLI cannot restore it. Require a zero exit
and decoded tombstone before reporting success.

## Recover from failures

- Validation error: correct the stated input and submit a new request. Never
  reuse a `mutation_id` with a different payload.
- Authentication error: repair authentication first, then retry the exact
  original record request with its printed identifiers.
- Invalid list cursor: restore every filter used to obtain the cursor, or omit
  `--cursor` and start from the first page. Never edit or decode the cursor.
- Invalid `--token`, `SATEIA_TOKEN`, or `SATEIA_TOKEN_FILE`: follow the
  source-specific guidance from `sateia auth status`; do not log out an
  unrelated keyring credential.
- Invalid pairing code: create a new code and use the exact matching device name.
- Ambiguous network or retryable server failure after a record request: reuse
  every identifier printed by the CLI and repeat the exact same payload.
  Update and delete retries must preserve `record_id`, `expected_version`, and
  `mutation_id`. Never generate a new mutation ID for that retry.
- `IDEMPOTENCY_CONFLICT`: stop. The mutation identifier was reused with different
  input; recover the original request instead of guessing.
- `VERSION_CONFLICT`: stop. Read and review the current record version before
  issuing a new mutation with a new mutation ID. Never guess the version.

## Common Rationalizations

- "The device name must match an app label." It does not; the CLI supplies the
  current machine's name during exchange as token metadata.
- "A Secret Service error means no token exists." It only proves that the
  credential backend could not be inspected.
- "A retry can use new record identifiers." An ambiguous create retry must use
  the exact original record and mutation identifiers.

## Red Flags

- A generic device name reused across machines.
- A token exposed in logs, repository files, or assistant output, or a
  command-line token used without warning about shell history and process
  inspection.
- Device-code login on headless Linux without keyring access or `--token-file`.
- Reusing an existing token-file path or changing filters with a pagination
  cursor.
- Guessing a record ID or expected version, partially specifying a replacement
  nutrient map, or reusing a mutation ID after changing a write payload.
- Claiming success without a zero exit and a decoded success response.

## Log out or revoke

- `sateia auth logout` removes only a local keyring credential.
- Environment and token-file credentials are managed by their owner and are
  not deleted by `sateia auth logout`.
- To invalidate a token on the server, instruct the user to revoke it in Sateia
  app > Settings > CLI Access.

## Report safely

- Include the server URL, credential source, record identifiers, and server error
  code when useful.
- Preserve and report top-level `request_id` when diagnosing a server request.
  It correlates the CLI response with server logs and audit events; it is not a
  nutrition record identifier and must not be used for pagination or retries.
- For queries, report the exact consumed-time window, whether deleted records
  were included, and whether more pages remain.
- Never include the token secret or full environment dumps.
- Distinguish a successful CLI exit and decoded record response from a request
  that merely reached the server.

## Verification

- Run `sateia auth status` and require a zero exit before a write.
- Confirm `credential_source` is the intended argument, environment, token
  file, or keyring source without printing the secret.
- For queries, inspect `has_more` and `next_cursor` until the requested scope is
  complete.
- Inspect the top-level `_notice` list after a successful JSON command.
  `NEXT_PAGE` describes pagination, while `UPDATE_AVAILABLE` contains an exact
  update command. Finish the user's current operation before acting on an
  informational notice. Report the update briefly; do not install it unless the
  user asked for an update.
- Set `SATEIA_NO_UPDATE_NOTIFIER=1` only when a hermetic run must avoid the
  cached public GitHub tag check.
- For writes, report the returned `record_id`, `mutation_id`, and version. Also
  report consumed time for create and update, or deletion time for delete.
