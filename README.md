# Sateia CLI

English | [简体中文](README.zh-CN.md)

`sateia` reads and creates nutrition records on a Sateia server. Records
created by the CLI synchronize to the Sateia app. The CLI supports interactive
login, managed tokens for automation, one-page queries, machine-readable
output, and idempotent create retries.

The CLI currently lists and creates records. It does not edit or delete them.

## Install

Go 1.26 or newer is required.

```sh
go install github.com/xxnian95/sateia-cli/cmd/sateia@latest
sateia --version
```

If the second command is not found, add the Go binary directory to `PATH` and
retry it.

## Quick start

1. Run `hostname` and choose a stable name for this machine, such as
   `agent-host-01 (Sateia CLI)`.
2. In Sateia app, open **Settings > CLI Access** and create a one-time code.
3. Exchange the code and store the resulting token:

   ```sh
   sateia auth login \
     --device-code ABCD-EFGH \
     --device-name "agent-host-01 (Sateia CLI)"
   ```

4. Verify the stored credential:

   ```sh
   sateia auth status
   ```

5. Inspect the operation you need:

   ```sh
   sateia record list --help
   sateia record create --help
   ```

The one-time code is valid for five minutes and can be exchanged only once.
The device name is metadata supplied by the CLI; it does not need to match an
older label shown by the app.

## Authentication

### Credential sources

The CLI selects one credential for each invocation, in this order:

| Priority | Source | Persistence | Intended use |
| --- | --- | --- | --- |
| 1 | `--token` | Not persisted | One-off commands |
| 2 | `SATEIA_TOKEN` | Managed by the environment | Automation with an environment-managed secret |
| 3 | `SATEIA_TOKEN_FILE` | Managed as a secret file; `auth login --token-file` can create it | Containers, agents, and mounted secrets |
| 4 | System keyring | Stored by `auth login` | Interactive machines |

`auth status` reports `credential_source` without printing the token.

A command-line token can appear in shell history or process listings. Prefer
`SATEIA_TOKEN` or `SATEIA_TOKEN_FILE` for automation. To use `--token` for one
invocation:

```sh
sateia --token "$TOKEN" auth status
```

### Headless Linux

Linux keyring storage requires Secret Service and a user D-Bus session. On a
headless machine without them, create a new private token file while exchanging
the device code:

```sh
sateia auth login \
  --device-code ABCD-EFGH \
  --device-name "agent-host-01 (Sateia CLI)" \
  --token-file "$HOME/.config/sateia/token"

export SATEIA_TOKEN_FILE="$HOME/.config/sateia/token"
sateia auth status
```

The parent directory must already exist. The CLI creates the file with mode
`0600` and refuses to overwrite an existing path. It reserves the file before
consuming the single-use code.

### Log out or revoke

```sh
sateia auth logout
```

`auth logout` removes only the token stored in the system keyring. It does not
delete `--token`, `SATEIA_TOKEN`, or `SATEIA_TOKEN_FILE` credentials, and it
does not revoke any server token. Remove environment- and file-managed secrets
at their source. Revoke a token in **Sateia app > Settings > CLI Access** when
it must stop working everywhere.

## List records

The CLI returns one page from an explicit consumed-time window:

```sh
sateia record list \
  --consumed-from 2026-08-01T00:00:00+08:00 \
  --consumed-before 2026-08-08T00:00:00+08:00 \
  --limit 50 \
  --json
```

`--consumed-from` is inclusive and `--consumed-before` is exclusive. Both are
RFC 3339 timestamps. The limit must be from 1 to 100. Results are newest first,
and deleted records are omitted unless `--include-deleted` is set.

If `has_more` is `true`, repeat the command with exactly the same time bounds,
`--include-deleted` choice, and limit, then pass `next_cursor` as `--cursor`.
Cursors are opaque and bound to the complete filter set; never edit or decode
them. Omit `--cursor` to start again from the first page.

## Create a record

Creating a record writes server data:

```sh
sateia record create \
  --energy 520 \
  --protein 28.5 \
  --carbohydrate 62 \
  --fat 18 \
  --note "Lunch" \
  --consumed-at 2026-08-07T12:30:00+08:00 \
  --json
```

Energy is measured in kilocalories. Protein, carbohydrate, and fat are measured
in grams. All four values are required non-negative decimals with at most six
fractional digits. `--consumed-at` accepts RFC 3339 with an explicit UTC offset
and defaults to the current time.

The CLI generates `record_id` and `mutation_id`. If a create request has an
ambiguous network or temporary server failure, the error prints both values.
Retry the exact same payload with the printed `--record-id` and
`--mutation-id`. Changing the payload while reusing `mutation_id` causes an
idempotency conflict; generating new identifiers may create a duplicate.

## Machine-readable output

Use `--json` when another program or an AI agent consumes the result. Successful
JSON responses include a top-level `_notice` array:

- `NEXT_PAGE` indicates that another query page is available.
- `UPDATE_AVAILABLE` includes the exact command for installing the latest
  stable CLI version.

Server-backed responses also include `request_id` when the server provides one.
Report it when diagnosing an error so an operator can correlate server logs and
audit events. It is request metadata, not a record identifier or cursor.

The update check runs after a successful command and is cached for 24 hours.
Update-check failures never change the requested command's result. Set
`SATEIA_NO_UPDATE_NOTIFIER=1` to disable the check in a network-isolated run.

## Guidance for AI agents

Live help is the command contract. An agent should:

1. Run `sateia --help` and `sateia environment` before its first operation.
2. Run `hostname` before device-code login and propose a stable machine name.
3. Never print a token or include it in logs, repository files, or messages.
   Use only the selected secret-managed environment or token file.
4. Use explicit time bounds for reads and keep all filters unchanged across
   cursor pages.
5. Run `sateia auth status` before a write.
6. Create a record only after the user requests the write. Do not invent a
   nutrient value or replace a known consumed time with the current time.
7. Prefer `--json` and require a zero exit status plus a decoded success
   response before reporting success.
8. Preserve both identifiers and the exact payload when retrying an ambiguous
   create request.

The repository also includes an agent skill at
[`skills/use-sateia-cli/SKILL.md`](skills/use-sateia-cli/SKILL.md). Live help
takes precedence if an installed CLI version differs from the skill.

## Troubleshooting

- **No credential was found:** run `sateia auth login`, supply `--token`, or set
  `SATEIA_TOKEN` or `SATEIA_TOKEN_FILE`.
- **The Linux credential store is unavailable:** use a managed token or
  `auth login --token-file <new-path>`, or start Secret Service and a user D-Bus
  session. This error does not prove that no keyring token exists.
- **`UNAUTHENTICATED`:** run `sateia auth status`. The error explains how to
  replace the selected argument, environment, token-file, or keyring source.
- **`INVALID_CURSOR`:** restore the exact filters that produced the cursor, or
  omit `--cursor` and start again. Never modify a cursor.
- **An ambiguous create failure:** retry the exact payload with the printed
  `--record-id` and `--mutation-id`.
- **A server error includes `request_id`:** include that value when asking an
  operator to inspect logs.

Run `sateia environment` for the complete credential, recovery, output, and
safe-agent guide.

## Server and local configuration

The server is selected in this order:

1. `--server`
2. `SATEIA_SERVER`
3. the server saved by `auth login`
4. `https://xxnian.site/sateia-server`

Remote servers must use HTTPS. HTTP is accepted only for loopback addresses.
`SATEIA_CONFIG_DIR` changes where the CLI stores non-secret configuration and
the update-check cache. The token secret is never written to `config.json`.

## License

MIT
