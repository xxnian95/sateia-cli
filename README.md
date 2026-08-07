# Sateia CLI

`sateia` is the public command-line client for querying and writing nutrition
records on a Sateia server. It exchanges a short-lived code displayed by the
Sateia app for an independently revocable CLI token, stores that token in the
operating system credential store, and uses it as a Bearer token for requests.

## Install from source

Go 1.26 or newer is required.

```sh
go install github.com/xxnian95/sateia-cli/cmd/sateia@latest
```

## Log in

In the Sateia app, open **Settings > CLI Access**, enter a name for this CLI
installation, and create a code. Then run the command below and enter the same
device name when prompted:

```sh
sateia auth login
```

The code is valid for five minutes and can be exchanged only once. The CLI
token is stored in macOS Keychain, Linux Secret Service, or Windows Credential
Manager. It is never written to `config.json`.

For a non-interactive terminal, pass the one-time code and installation name:

```sh
sateia auth login --device-code ABCD-EFGH --device-name "Pengnian Mac"
```

Check or remove the current credential with:

```sh
sateia auth status
sateia auth logout
```

Logout removes the local credential. Revoke a CLI token from the Sateia app
when the token must also become invalid on the server.

Automation can provide a token through `SATEIA_TOKEN`; the environment takes
precedence over the credential store. Do not pass tokens as command-line flags,
where they can be exposed through shell history or process inspection.

## List records

Query one page in an explicit consumed-time window:

```sh
sateia record list \
  --consumed-from 2026-08-01T00:00:00+08:00 \
  --consumed-before 2026-08-08T00:00:00+08:00 \
  --limit 50 \
  --json
```

The lower bound is inclusive and the upper bound is exclusive. Results are
newest first. Deleted records are excluded unless `--include-deleted` is set.
If `has_more` is true, repeat the command with exactly the same filters and add
`--cursor` with the returned `next_cursor`. The CLI deliberately fetches one
page at a time so automation retains control of limits and retries.

## Create a record

```sh
sateia record create \
  --energy 520 \
  --protein 28.5 \
  --carbohydrate 62 \
  --fat 18 \
  --note "Pengnian lunch"
```

`--consumed-at` accepts an RFC 3339 timestamp and defaults to the current time.
The timestamp's UTC offset is preserved as the record's local-day offset.

Use `--json` for structured output containing both `mutation_id` and the created
record:

```sh
sateia record create \
  --energy 520 --protein 28.5 --carbohydrate 62 --fat 18 \
  --consumed-at 2026-08-07T12:30:00+08:00 \
  --json
```

The CLI generates both `record_id` and `mutation_id`. If a request has an
ambiguous network failure, the error prints both identifiers. Repeat the exact
request with `--record-id` and `--mutation-id` to get the server's idempotent
result without creating a second record.

## Configuration

The server is selected in this order:

1. `--server`
2. `SATEIA_SERVER`
3. the server saved during `auth login`
4. `https://xxnian.site/sateia-server`

Remote servers must use HTTPS. Plain HTTP is accepted only for loopback local
development. `SATEIA_CONFIG_DIR` can relocate the non-secret configuration
directory for isolated environments and tests.

Run `sateia --help` or `sateia <command> --help` for the complete command
reference. Run `sateia environment` for credential precedence, storage, and a
safe agent workflow.

## Agent skill

The repository includes a product-neutral agent skill at
[`skills/use-sateia-cli/SKILL.md`](skills/use-sateia-cli/SKILL.md). Agents should
use it for authentication, nutrition record queries and pagination, record
writes, output interpretation, and idempotent failure recovery. The skill
treats live CLI help as the authoritative command contract.

## Development

```sh
make check
make build
```

The CLI follows the contract in the Sateia API OpenAPI document. Contract JSON
fields remain English `lower_snake_case`, and enum values remain English
`UPPER_SNAKE_CASE`.

## License

MIT
