---
name: timestripe
description: Use this skill whenever the user wants to interact with Timestripe — reading or modifying their spaces, boards, buckets, goals (also called tasks, todos, or items), comments, events (activity), folders, memberships, or user profile — via the `timestripe` CLI. Trigger on mentions of "timestripe", "my goals", "my tasks", "my todos", "my items", "my spaces", "comments", "activity"/"events", "folders", horizons ("day/week/month/quarter/year/decade/life"), or when the user asks to list, create, update, complete, or delete goals/tasks/todos/items or related entities. Also use to authenticate the CLI or inspect API configuration. Skip for unrelated project management tools.
---

# Timestripe CLI

`timestripe` is the official command-line client for the Timestripe API. It is
designed to be pipe-friendly and predictable so that agents and scripts can
drive it without parsing human-formatted output.

## Before doing anything: install

Check whether the CLI is available:

```bash
command -v timestripe >/dev/null || echo "timestripe not installed"
```

If missing, install one of these ways. Prefer Homebrew on macOS and Linux:

```bash
brew install timestripe/tap/timestripe-cli
```

If Go is available:

```bash
go install github.com/timestripe/timestripe-cli/cmd/timestripe@latest
```

Otherwise grab a pre-built binary (macOS / Linux, amd64 / arm64) from
https://github.com/timestripe/timestripe-cli/releases and place it on `PATH`.

Verify the install:

```bash
timestripe version
```

## Before doing anything: authenticate

The CLI requires credentials. Check first:

```bash
timestripe auth status
```

If `Not signed in.`, pick one:

```bash
# Personal API token (best for scripts / CI)
timestripe auth login --token <API_KEY>

# Or OAuth2 + PKCE (opens a browser; fixed loopback on 127.0.0.1:53682)
timestripe auth login

# Add --read-only to request the read_only scope (write commands will fail)
timestripe auth login --read-only
```

Alternatively, pass the token through the environment without persisting it:

```bash
TIMESTRIPE_TOKEN=<API_KEY> timestripe users me --json
```

To point at a non-production backend:

```bash
TIMESTRIPE_BACKEND=https://staging.timestripe.com timestripe users me --json
```

## Always prefer `--json`

When running from an agent, always pass `--json`. The default on a TTY is
`--table`, which is unstable to parse. The JSON envelope for list commands is:

```json
{
  "pageInfo": { "count": 412, "hasMore": true, "next": "...", "previous": null },
  "items": [ /* resources */ ]
}
```

Single-resource commands (`get`, `create`, `update`, `me`) emit the bare
resource object.

Supported formats (mutually exclusive): `--json`, `--yaml`, `--markdown`,
`--table`, `--csv`. `--csv` is scalar-only.

## Command surface

```
timestripe auth         login | logout | whoami | status
                        (login alias: signin; logout alias: signout;
                         all four also available as top-level commands,
                         e.g. `timestripe login`, `timestripe whoami`)
timestripe spaces       list | get <id> | create | update <id> | delete <id>
timestripe boards       list | get <id> | create | update <id> | delete <id>
timestripe buckets      list | get <id> | create | update <id> | delete <id>
timestripe goals        list | get <id> | create | update <id> | delete <id>
                        | attach <id> <file>
                        (aliases: tasks, todos, items — the resource's
                         semantics are up to the user)
timestripe comments     list | get <id> | create | update <id> | delete <id>
timestripe events       list | get <id>                     (read-only)
timestripe folders      list | get <id> | create | update <id> | delete <id>
                        | goals list | goals get <id>
                        | goals add | goals update <id> | goals remove <id>
timestripe memberships  list | get <id>                     (read-only)
timestripe users        list | get <id> | me                (read-only)
timestripe config       show
timestripe completion   bash | zsh | fish | powershell
timestripe version
```

Webhooks, goal completion, and space cloning are intentionally not exposed.

## Pagination (list commands)

- `--limit N` — total items to return across all pages (default `30`).
  The CLI auto-paginates under the hood.
- `--offset N` — starting offset into the result set (default `0`).
- `--all` — fetch every page; ignores `--limit`.

To know if more results exist, inspect `pageInfo.hasMore` in the JSON envelope.

## Filtering, search, and sort (list commands)

Most list commands accept server-side filters. Prefer these over post-filtering
with `jq` — they are cheaper and correct across all pages.

- `spaces list`: `--search`
- `boards list`: `--space-id`, `--archived`, `--search`, `--sort`
- `buckets list`: `--board-id`, `--search`, `--sort`
- `goals list`: `--space-id`, `--bucket-id`, `--assignee-id`, `--parent-id`,
  `--checked`, `--color`, `--horizon` (repeatable), `--date-from`, `--date-to`,
  `--updated-since`, `--search`, `--sort`
- `comments list`: `--goal-id`, `--space-id`, `--user-id`,
  `--created-from`, `--created-to`, `--modified-from`, `--modified-to`, `--sort`
- `events list`: `--from`, `--to`, `--goal-id`, `--space-id`, `--user-id`,
  `--type` (e.g. `GOAL_CREATED`, `COMMENT_CREATED`, `BOARD_MODIFIED`), `--sort`
- `folders list`: `--space-id`, `--user-id`, `--is-private`, `--sort`
- `folders goals list`: `--folder-id`, `--goal-id`, `--sort`
- `memberships list`: `--space-id`, `--user-id`
- `users list`: `--email`, `--search`

Nullable foreign-key filters (`--assignee-id`, `--bucket-id`, `--parent-id`)
accept the literal string `null` to match items where that field is unset.
`--sort` accepts the API field name; prefix with `-` for descending, e.g.
`--sort -modified_datetime`. Dates are `YYYY-MM-DD`; `--updated-since` is RFC3339.

## Create and update

`create` and `update <id>` read a JSON body from `--file`:

```bash
# From a file
timestripe goals create --file ./new-goal.json

# From stdin
echo '{"space_id":"...","name":"Ship v1","horizon":"week"}' | \
  timestripe goals create --file -
```

`update` performs a PATCH (send only the fields you want to change).

## Data model cheatsheet

- **Space** — top-level container.
- **Board** — belongs to a space (`space_id`), optional `layout`. `description`
  is Markdown.
- **Bucket** — belongs to a board (`board_id`), ordered via `sequence_no`.
- **Goal** (also addressable as `tasks`, `todos`, or `items` — the exact
  semantics are up to the user) — belongs to a space (`space_id`) and
  optionally a bucket (`bucket_id`). Has a `horizon` of `day | week | month |
  quarter | year | decade | life`, an optional `date` (ISO `YYYY-MM-DD`), a
  `checked` boolean, and a `color` from a fixed palette. `description` is
  Markdown.
- **Comment** — Markdown text attached to a goal. Has `goal_id`, `description`
  (Markdown), `user_id` (author, set by the server), and create/modify
  timestamps.
- **Event** — server-emitted activity record. Read-only. `type` is one of
  `GOAL_CREATED | GOAL_DELETED | GOAL_DONE | GOAL_MODIFIED | GOAL_ASSIGNED |
  GOAL_RESCHEDULED | USER_CREATED | SPACE_CREATED | SPACE_CREATED_BY_CLONING |
  SPACE_USED_FOR_CLONING | COMMENT_CREATED | COMMENT_DELETED |
  COMMENT_MODIFIED | MENTION_CREATED | MEMBERSHIP_CREATED |
  MEMBERSHIP_DELETED | BOARD_CREATED | BOARD_MODIFIED | BOARD_DELETED |
  BUCKET_CREATED | BUCKET_MODIFIED | BUCKET_DELETED |
  CLIMB_SUBSCRIPTION_CREATED | CLIMB_SUBSCRIPTION_DELETED`. Carries the
  acting `user_id` and any related `goal_id` / `board_id` / `bucket_id` /
  `comment_id` / `space_id` / `other_user_id` plus their denormalized names.
- **Folder** — a per-user grouping of goals inside a space. Has `space_id`,
  `name`, `sequence_no`, `is_private`, and a nullable `user_id` (null = shared).
- **FolderGoal** — pivot row that places a goal into a folder with a
  `sequence_no`. Manage via `timestripe folders goals add | remove | update`.
- **Membership** — links a user to a space with a `role` of
  `OWNER | ADMIN | EDITOR | VIEWER`.

Most fields are optional on read. IDs are strings.

## Modeling checklists

Use goals with subgoals (set `parent_id` to the parent goal's ID) for
checklists, not Markdown checklists embedded in a board's or goal's
`description`. Subgoals get individual `checked` state, can be filtered and
queried, and survive edits to the parent. Reach for `parent_id` whenever you'd
otherwise reach for `- [ ]`.

## Exit codes

- `0` — success.
- non-zero — failure; a human-readable message goes to stderr. Check `$?` and
  surface the stderr text to the user verbatim.

## Common recipes

```bash
# Who am I?
timestripe users me --json

# All goals in the current account, as one array
timestripe goals list --all --json | jq '.items'

# Goals on the "week" horizon only
timestripe goals list --horizon week --all --json | jq '.items'

# Open goals with a due date this month, newest first
timestripe goals list --checked=false --date-from 2026-04-01 --date-to 2026-04-30 \
  --sort -date --json

# Create a goal from a heredoc
cat <<'EOF' | timestripe goals create --file -
{
  "space_id": "spc_abc",
  "name": "Draft the Q3 plan",
  "horizon": "week",
  "date": "2026-04-27"
}
EOF

# Mark a goal as checked (PATCH)
echo '{"checked": true}' | timestripe goals update gl_xyz --file -

# Attach a file to a goal (appended inline to its description)
timestripe goals attach gl_xyz ./screenshot.png

# Delete a bucket
timestripe buckets delete bkt_xyz

# CSV export of all spaces for a spreadsheet
timestripe spaces list --all --csv > spaces.csv
```

## Discoverability

`timestripe --help` and `timestripe <command> --help` document every flag,
including pagination and format flags. Shell completion is available via
`timestripe completion <bash|zsh|fish|powershell>`.

## What not to do

- Do not scrape `--table` output; always use `--json`.
- Do not hand-assemble URLs against the API — use the CLI.
- Do not store API tokens in shell history; prefer `TIMESTRIPE_TOKEN` in an
  env file or `timestripe auth login --token` (persisted to keychain).
- Do not stuff Markdown checklists into a board's or goal's `description` —
  create subgoals (`parent_id`) instead.
