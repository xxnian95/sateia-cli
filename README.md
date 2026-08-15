<p align="center">
  <img src="docs/assets/sateia-app-icon.png" width="160" alt="Sateia app icon">
</p>

# Sateia CLI

<p align="center">
  The command-line companion for Sateia nutrition tracking.
</p>

<p align="center">
  English | <a href="README.zh-CN.md">简体中文</a>
</p>

## About Sateia

Sateia is an iPhone nutrition log for dietary energy, protein, carbohydrate,
and fat. The app keeps its working data on the device and provides daily
progress, seven-day trends, record history, and manual entry. With permission,
it can read supported nutrition data from Apple Health and write Sateia records
back to Apple Health.

The Sateia app can also download records created through the managed service,
API, or this CLI. This makes the CLI useful for terminal workflows, personal
automation, and AI agents while the app remains the place to review progress
and manage synchronization.

Server synchronization is one-way for nutrition data: the app downloads
server and CLI records, but never uploads manual entries, local nutrition
history, or Apple Health samples. Apple Health access is optional and remains
under iOS privacy controls. Sateia is not a medical device and does not provide
diagnosis, treatment, or professional dietary advice.

[App overview](https://xxnian.site/sateia-server/app) ·
[Support](https://xxnian.site/sateia-server/support) ·
[Privacy](https://xxnian.site/sateia-server/privacy) ·
[Health data practices](https://xxnian.site/sateia-server/health-data-practices) ·
[Terms](https://xxnian.site/sateia-server/terms)

## What the CLI provides

`sateia` reads and changes nutrition records on a Sateia server. Records
created or changed by the CLI synchronize to the Sateia app and may be written
to Apple Health by the app after authorization.

| Capability | Commands and behavior |
| --- | --- |
| Authentication | Pair through a five-minute, single-use code from the app; store credentials in the system keyring or a managed token file |
| Nutrition records | List, create, update, and soft-delete energy, protein, carbohydrate, and fat records |
| Daily goals | Get, set, and delete date-specific nutrition goals |
| Automation | Use stable JSON output, environment-managed credentials, optimistic concurrency, and idempotent mutation retries |
| AI agents | Inspect machine-readable help, run read-only diagnostics, and install the bundled `use-sateia-cli` skill |

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
   sateia goal --help
   sateia record list --help
   sateia record create --help
   sateia record update --help
   sateia record delete --help
   ```

AI agents must run the exact concrete command with `--help` immediately before
every invocation, even when they have used it before. Repeat this before each
pagination request and retry because installed CLI guidance may change. Prefer
`--help --format json` to receive a stable command schema containing field
types, required fields, cross-field rules, examples, and command risk.

The one-time code is valid for five minutes and can be exchanged only once.
The device name is metadata supplied by the CLI; it does not need to match an
older label shown by the app.

## Daily goals

Daily goals are keyed by a calendar date in `yyyy-MM-dd` form. The date has no
time or time-zone semantics. A custom goal takes precedence over the goals in
the app's Settings for that date; deleting it restores the Settings fallback.

```sh
sateia goal set 2026-08-09 \
  --energy 2200 \
  --protein 140 \
  --carbohydrate 240 \
  --fat 70

sateia goal get 2026-08-09 --json
sateia goal delete 2026-08-09
```

All four nutrient flags are required by `goal set`. Values must be
non-negative decimals with at most six fractional digits.

## Practical workflows

The CLI accepts structured nutrition values; it does not inspect images or
calculate nutrition by itself. An AI agent can interpret a meal photo,
nutrition label, recipe, or conversation, but it should show the proposed
record to the user before writing whenever the values are estimated or the
request is ambiguous. In a record note, put the food name and quantity or
serving first so truncated client previews remain useful. Append provenance,
import source, and external IDs afterward.

| User request | Agent workflow | CLI operation |
| --- | --- | --- |
| “Record this meal from the photo.” | Identify foods, resolve portion and time, present an estimate, then ask for confirmation | `record create` |
| “I ate 1.5 servings from this label.” | Read per-serving values, calculate the consumed amount, and confirm the result | `record create` |
| “How much did I eat today?” | Query the local-day window, follow every page, then total the returned values | `record list` |
| “The lunch entry is wrong.” | Find the exact record and version, confirm the correction, then update it | `record update` |
| “Delete the duplicate entry.” | Find both records, identify the duplicate, then soft-delete that exact version | `record delete` |

### Record a meal from a photo with an AI agent

A photo does not reliably reveal weight, hidden ingredients, cooking oil, or
the exact nutrition of a dish. The agent should ask for details that materially
change the estimate, such as portion size, ingredients, restaurant item, and
meal time. If exact values are unavailable, label the result as an estimate.

An example conversation:

> **User:** Record this lunch from the photo. It was about 200 g of chicken,
> one bowl of rice, and the meal was at 12:30 today.
>
> **Agent:** I estimate 610 kcal, 52 g protein, 68 g carbohydrate, and 14 g
> fat. I will note that the values were estimated from the photo. Save this
> record?
>
> **User:** Yes.

After confirmation, the agent verifies authentication and creates the record:

```sh
sateia auth status

sateia record create \
  --energy 610 \
  --protein 52 \
  --carbohydrate 68 \
  --fat 14 \
  --note "Chicken 200 g + rice 1 bowl; estimated from meal photo after user confirmation" \
  --consumed-at 2026-08-09T12:30:00+08:00 \
  --json
```

The numbers above are illustrative, not a reusable estimate for similar
photos. The agent should report the returned `record_id`, `mutation_id`,
version, and consumed time. A photo without an explicit request to save data
must not create a record.

### Record food from a nutrition-label photo

The agent should transcribe the label, identify whether values are per serving
or per package, and ask how much the user consumed. For example, if one serving
contains 240 kcal, 8 g protein, 36 g carbohydrate, and 7 g fat, then 1.5
servings becomes 360 kcal, 12 g protein, 54 g carbohydrate, and 10.5 g fat.
After the user confirms both the serving count and calculated values:

```sh
sateia record create \
  --energy 360 \
  --protein 12 \
  --carbohydrate 54 \
  --fat 10.5 \
  --note "Example snack, 1.5 servings; calculated from confirmed nutrition label" \
  --consumed-at 2026-08-09T15:20:00+08:00 \
  --json
```

### Review a day of intake

The CLI returns records rather than a calculated daily total. An agent can
query one local calendar day, follow `next_cursor` until `has_more` is false,
and sum the four nutrient fields from the complete result set:

```sh
sateia record list \
  --consumed-from 2026-08-09T00:00:00+08:00 \
  --consumed-before 2026-08-10T00:00:00+08:00 \
  --limit 100 \
  --json
```

When another page exists, repeat the command with the same bounds and limit,
adding the returned `next_cursor` as `--cursor`. The agent should state the
queried time zone and whether deleted records were included when reporting the
total.

### Correct an existing record

First query a narrow time window and identify the intended record by its
`record_id`, consumed time, values, and current `version`. Present the proposed
change before writing. A nutrient correction replaces all four nutrient
values:

```sh
sateia record update \
  --record-id 014b2680-df5b-4c8d-97ef-abde0a9746d6 \
  --expected-version 1 \
  --energy 580 \
  --protein 48 \
  --carbohydrate 64 \
  --fat 13 \
  --note "Chicken and rice, corrected portion; user supplied the portion size" \
  --json
```

For a note-only correction, omit the nutrient flags. On `VERSION_CONFLICT`,
query the record again and review the new state instead of guessing a version.

### Delete a duplicate record

Query the relevant time window and compare the candidate records before
deleting anything. Once the user or agent has unambiguously identified the
duplicate, use its exact ID and current version:

```sh
sateia record delete \
  --record-id 014b2680-df5b-4c8d-97ef-abde0a9746d6 \
  --expected-version 2 \
  --json
```

The command soft-deletes the record and returns a tombstone. The CLI cannot
restore a deleted record, so an uncertain match should remain unchanged until
the user clarifies which entry is the duplicate.

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

## Update a record

Updating a record requires its current version and changes only the supplied
mutable fields:

```sh
sateia record update \
  --record-id 014b2680-df5b-4c8d-97ef-abde0a9746d6 \
  --expected-version 1 \
  --energy 610 \
  --protein 32 \
  --carbohydrate 70 \
  --fat 22 \
  --note "Corrected lunch" \
  --json
```

Omitted mutable flags remain unchanged. Nutrients are replaced as one complete
map, so `--energy`, `--protein`, `--carbohydrate`, and `--fat` must be supplied
together. `--consumed-at` updates both the timestamp and its UTC offset. Use
`--note ""` to store an empty string or `--clear-note` to store `null`.

The CLI generates `mutation_id`. After an ambiguous network or temporary
server failure, retry the exact same command with the reported `--mutation-id`,
`--record-id`, and `--expected-version`. If any mutable field changes, use a new
mutation ID. On `VERSION_CONFLICT`, review the current record and version before
issuing a new update; never guess the version.

## Delete a record

Deletion creates a server-side tombstone and requires the current version:

```sh
sateia record delete \
  --record-id 014b2680-df5b-4c8d-97ef-abde0a9746d6 \
  --expected-version 2 \
  --json
```

The deletion is idempotent when retried with the same generated `mutation_id`,
record ID, and expected version. Replaying a successful deletion returns the
original tombstone without another version increment. The CLI cannot restore a
deleted record.

## Machine-readable output

`--json` is global. Use it when another program or an AI agent consumes the
result. Successful JSON responses are written to stdout and include a top-level
`_notice` array:

- `NEXT_PAGE` indicates that another query page is available.
- `UPDATE_AVAILABLE` includes the exact command for installing the latest
  stable CLI version.

Server-backed responses also include `request_id` when the server provides one.
Report it when diagnosing an error so an operator can correlate server logs and
audit events. It is request metadata, not a record identifier or cursor.

The update check runs after a successful command and is cached for 24 hours.
Update-check failures never change the requested command's result. Set
`SATEIA_NO_UPDATE_NOTIFIER=1` to disable the check in a network-isolated run.

Failures in `--json` mode are written to stderr with a non-zero exit status:

```json
{
  "ok": false,
  "error": {
    "type": "api",
    "code": "VERSION_CONFLICT",
    "message": "update record failed",
    "hint": "Stop: review the current record and version",
    "retryable": false,
    "request_id": "request_xxx",
    "context": {
      "expected_version": 2,
      "current_version": 3
    },
    "backend_error": {
      "code": "VERSION_CONFLICT",
      "message": "expected_version 2 does not match current version 3",
      "retryable": false,
      "context": {
        "expected_version": 2,
        "current_version": 3
      }
    }
  }
}
```

Use `error.code`, `retryable`, `retry_after`, `request_id`, `violations`,
`context`, and `hint` instead of parsing the human-readable message. The CLI
also preserves the server's complete error object in `backend_error`. If a
proxy or upstream returns a non-contract body, the CLI surfaces it as
`response_body` instead of discarding it. Bodies over the defensive 2 MiB
limit set `response_body_truncated` rather than silently appearing complete.

## Diagnostics and bundled agent skill

Run the complete read-only diagnostic suite:

```sh
sateia doctor --help --format json
sateia doctor --json
```

`doctor` checks the CLI version, local clock, selected server, credential
source, authenticated read access, available update, and installed agent skill.
Warnings do not make `healthy` false; required-check failures do.

The exact `use-sateia-cli` skill is embedded in every CLI build:

```sh
sateia skill check --json
sateia skill install
sateia skill update
```

Managed installations include a hash manifest. `skill update` installs a
missing skill or replaces an outdated, unchanged managed copy. It refuses to
overwrite `MODIFIED`, `UNMANAGED`, or `INVALID` targets unless `--force` is
explicitly supplied.

## Guidance for AI agents

Live help is the command contract. An agent should:

1. Run `sateia --help` and `sateia environment` before its first operation.
2. Run `hostname` before device-code login and propose a stable machine name.
3. Never print a token or include it in logs, repository files, or messages.
   Use only the selected secret-managed environment or token file.
4. Use explicit time bounds for reads and keep all filters unchanged across
   cursor pages.
5. Run `sateia auth status` before a write.
6. Perform only the exact create, update, or delete requested by the user. Do
   not invent a record ID, expected version, nutrient value, or consumed time.
7. Prefer `--json` and require a zero exit status plus a decoded success
   response before reporting success.
8. Preserve the mutation ID and exact payload when retrying an ambiguous
   write. Update and delete retries must also preserve record ID and expected
   version.

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
- **An ambiguous update or delete failure:** retry the unchanged request with
  the printed `--record-id`, `--expected-version`, and `--mutation-id`.
- **`VERSION_CONFLICT`:** review the current record and version, then issue the
  intended mutation with a new mutation ID. Never guess a version.
- **`IDEMPOTENCY_CONFLICT`:** recover the original request associated with the
  mutation ID; do not reuse it with changed content.
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
