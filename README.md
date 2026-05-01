<p align="center">
  <img src="assets/logo.svg" alt="Timestripe" width="80" height="80" />
</p>

<h1 align="center">Timestripe CLI</h1>

<p align="center">
  The official command-line client for the <a href="https://timestripe.com">Timestripe</a>.
</p>

---

## Install

### Homebrew (macOS, Linux)

```bash
brew install timestripe/tap/timestripe-cli
```

Upgrade later with:

```bash
brew upgrade timestripe-cli
```

### From source

Requires Go 1.26+.

```bash
go install github.com/timestripe/timestripe-cli/cmd/timestripe@latest
```

Or clone and build:

```bash
git clone https://github.com/timestripe/timestripe-cli
cd timestripe-cli
make build   # → ./bin/timestripe
```

### Pre-built binaries

Download for macOS or Linux (amd64 / arm64) from the [Releases](https://github.com/timestripe/timestripe-cli/releases) page.

## Quick start

```bash
timestripe auth login                       # opens your browser
timestripe spaces list
timestripe boards list --space <space-id>
timestripe goals create --bucket <bucket-id> --title "Ship the thing"
```

Every command has `--help` with full flag and subcommand documentation:

```bash
timestripe --help
timestripe goals create --help
```

## Authentication

OAuth authentication with your browser:

`timestripe auth login`

Skip the browser by using a personal api key (manage keys in [Settings](https://timestripe.com/settings/#api-key)):

`timestripe auth login --token <your-api-key>`

Or supply a token per-invocation via env, bypassing stored credentials entirely:

`export TIMESTRIPE_TOKEN=<your-api-key> timestripe ...`

Related commands:

- `timestripe auth whoami` — show the authenticated user
- `timestripe auth status` — verify the stored token still works
- `timestripe auth logout` — remove stored credentials


## Commands

| Command | Purpose |
| --- | --- |
| `auth` | Log in, log out, inspect the active session |
| `spaces` | Manage spaces |
| `boards` | Manage boards |
| `buckets` | Manage buckets |
| `goals` | Manage goals |
| `memberships` | Manage workspace memberships |
| `users` | Look up users |
| `config` | Show resolved configuration |
| `version` | Print version, commit, and build date |

Most resource commands expose `list`, `get`, `create`, `update`, and `delete` subcommands. Run `--help` on any command for details.

## Output formats

Pick a format with one of these mutually-exclusive flags:

| Flag | Output |
| --- | --- |
| `--json` | JSON |
| `--yaml` | YAML |
| `--table` | Pretty table (default on a TTY) |
| `--markdown` | Markdown table |
| `--csv` | CSV |

When stdout isn't a TTY, the default switches to JSON, so piping is safe:

```bash
timestripe goals list --json | jq '.[] | select(.completed == false) | .title'
```

## Pagination

List commands accept:

| Flag | Effect |
| --- | --- |
| `--limit <n>` | Max items returned across all pages |
| `--offset <n>` | Starting offset into the result set |
| `--all` | Fetch every page; ignores `--limit` |

## Configuration

Config and credentials live in `$XDG_CONFIG_HOME/timestripe/` (default `~/.config/timestripe/`).

Environment overrides:

| Variable | Effect |
| --- | --- |
| `TIMESTRIPE_TOKEN` | Bearer token used for all requests; overrides stored credentials. |
| `TIMESTRIPE_BACKEND` | Timestripe site root (default `https://timestripe.com`). Useful for staging or self-hosted environments. |
| `XDG_CONFIG_HOME` | Override the config directory base. |

Inspect what the CLI sees:

```bash
timestripe config show
```

## License

MIT — see [LICENSE](LICENSE).

## Links

- [Timestripe](https://timestripe.com)
- [Issues](https://github.com/timestripe/timestripe-cli/issues)
- [Releases](https://github.com/timestripe/timestripe-cli/releases)
