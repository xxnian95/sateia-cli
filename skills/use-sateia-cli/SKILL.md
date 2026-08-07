---
name: use-sateia-cli
description: Use the public Sateia CLI to authenticate, query nutrition records, and create server-side nutrition records safely. Apply when a user or agent needs to install or discover the sateia command, exchange an app-issued device code, use SATEIA_TOKEN in headless automation, read or write energy and macronutrient data, interpret CLI output, paginate queries, or retry an ambiguous record write without duplication.
---

# Use Sateia CLI

Use the CLI's live help as the command contract. Protect credentials and preserve
the record and mutation identifiers across ambiguous retries.

## Discover the installed CLI

1. Run `command -v sateia`.
2. If installed, run `sateia --help` and `sateia environment` before acting.
3. If working from this repository, use `make build` and `./bin/sateia` when the
   global command is absent.
4. If neither is available, report that installation is required. Do not invent
   flags from this skill when live help disagrees.

## Select authentication

- For interactive use, ask the user to create a code in Sateia app > Settings >
  CLI Access. Use the exact device name entered in the app:

  ```sh
  sateia auth login --device-code ABCD-EFGH --device-name "Pengnian Mac"
  ```

- For headless automation, use `SATEIA_TOKEN`. Never print it, pass it as a
  command-line argument, store it in repository files, or include it in logs.
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

## Recover from failures

- Validation error: correct the stated input and submit a new request. Never
  reuse a `mutation_id` with a different payload.
- Authentication error: repair authentication first, then retry the exact
  original record request with its printed identifiers.
- Invalid pairing code: create a new code and use the exact matching device name.
- Ambiguous network or retryable server failure after a record request: reuse
  both identifiers printed by the CLI and repeat the exact same record payload.
  Never generate new identifiers for that retry.
- `IDEMPOTENCY_CONFLICT`: stop. The mutation identifier was reused with different
  input; recover the original request instead of guessing.

## Log out or revoke

- `sateia auth logout` removes only the local credential.
- To invalidate a token on the server, instruct the user to revoke it in Sateia
  app > Settings > CLI Access.

## Report safely

- Include the server URL, credential source, record identifiers, and server error
  code when useful.
- For queries, report the exact consumed-time window, whether deleted records
  were included, and whether more pages remain.
- Never include the token secret or full environment dumps.
- Distinguish a successful CLI exit and decoded record response from a request
  that merely reached the server.
