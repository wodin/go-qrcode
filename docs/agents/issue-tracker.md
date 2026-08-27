# Issue tracker: GitHub

Issues and specs for this repo live as GitHub issues on the fork,
**`wodin/go-qrcode`**. Use the `gh` CLI for all operations.

## Fork and upstream

This clone has two remotes:

| Remote     | Repo               | Access | Role                          |
| ---------- | ------------------ | ------ | ----------------------------- |
| `origin`   | `wodin/go-qrcode`  | write  | Our issue tracker. All writes. |
| `upstream` | `skip2/go-qrcode`  | read   | Read-only source. Never write. |

`gh repo set-default wodin/go-qrcode` has been run, so a bare `gh issue ...`
inside this clone resolves to the fork. If a command ever resolves elsewhere,
pass `--repo wodin/go-qrcode` explicitly.

**Never attempt a write against `skip2/go-qrcode`** — issue creation, comments,
labels and closes all fail there. If work belongs upstream, it goes out as a
pull request from a branch on `origin`, not as an upstream issue.

### Upstream issues are a read-only source

`skip2/go-qrcode` has an active issue tracker (~25 open). Those issues are
legitimate prior art: read them, cite them, mine them for reproduction cases.

- **Read**: `gh issue view <n> --repo skip2/go-qrcode --comments`
- **List / search**: `gh issue list --repo skip2/go-qrcode --state open --search "<terms>"`

**Referencing convention.** Both repos number from 1 and their numbers do not
correspond. Inside our issues:

- A bare `#42` always means **our** issue 42 on the fork.
- An upstream issue is always written **`skip2#42`** — GitHub renders that as a
  cross-repo link, so it stays clickable and unambiguous.

Never write a bare `#42` meaning upstream. Fork issue 42 will eventually exist
and will be something else entirely.

## Conventions

- **Create an issue**: `gh issue create --title "..." --body "..."`. Use a heredoc for multi-line bodies.
- **Read an issue**: `gh issue view <number> --comments`, filtering comments by `jq` and also fetching labels.
- **List issues**: `gh issue list --state open --json number,title,body,labels,comments --jq '[.[] | {number, title, body, labels: [.labels[].name], comments: [.comments[].body]}]'` with appropriate `--label` and `--state` filters.
- **Comment on an issue**: `gh issue comment <number> --body "..."`
- **Apply / remove labels**: `gh issue edit <number> --add-label "..."` / `--remove-label "..."`
- **Close**: `gh issue close <number> --comment "..."`

## Pull requests as a triage surface

**PRs as a request surface: no.** _(Set to `yes` if this repo treats external PRs as feature requests; `/triage` reads this flag.)_

When set to `yes`, PRs run through the same labels and states as issues, using the `gh pr` equivalents:

- **Read a PR**: `gh pr view <number> --comments` and `gh pr diff <number>` for the diff.
- **List external PRs for triage**: `gh pr list --state open --json number,title,body,labels,author,authorAssociation,comments` then keep only `authorAssociation` of `CONTRIBUTOR`, `FIRST_TIME_CONTRIBUTOR`, or `NONE` (drop `OWNER`/`MEMBER`/`COLLABORATOR`).
- **Comment / label / close**: `gh pr comment`, `gh pr edit --add-label`/`--remove-label`, `gh pr close`.

GitHub shares one number space across issues and PRs, so a bare `#42` may be either — resolve with `gh pr view 42` and fall back to `gh issue view 42`.

## When a skill says "publish to the issue tracker"

Create a GitHub issue on `wodin/go-qrcode`.

## When a skill says "fetch the relevant ticket"

Run `gh issue view <number> --comments`.

## Wayfinding operations

Used by `/wayfinder`. The **map** is a single issue with **child** issues as tickets.

- **Map**: a single issue labelled `wayfinder:map`, holding the Notes / Decisions-so-far / Fog body. `gh issue create --label wayfinder:map`.
- **Child ticket**: an issue linked to the map as a GitHub sub-issue (`gh api` on the sub-issues endpoint). Where sub-issues aren't enabled, add the child to a task list in the map body and put `Part of #<map>` at the top of the child body. Labels: `wayfinder:<type>` (`research`/`prototype`/`grilling`/`task`). Once claimed, the ticket is assigned to the driving dev.
- **Blocking**: GitHub's **native issue dependencies** — the canonical, UI-visible representation. Add an edge with `gh api --method POST repos/<owner>/<repo>/issues/<child>/dependencies/blocked_by -F issue_id=<blocker-db-id>`, where `<blocker-db-id>` is the blocker's numeric **database id** (`gh api repos/<owner>/<repo>/issues/<n> --jq .id`, _not_ the `#number` or `node_id`). GitHub reports `issue_dependencies_summary.blocked_by` (open blockers only — the live gate). Where dependencies aren't available, fall back to a `Blocked by: #<n>, #<n>` line at the top of the child body. A ticket is unblocked when every blocker is closed.
- **Frontier query**: list the map's open children (`gh issue list --state open`, scoped to the map's sub-issues / task list), drop any with an open blocker (`issue_dependencies_summary.blocked_by > 0`, or an open issue in the `Blocked by` line) or an assignee; first in map order wins.
- **Claim**: `gh issue edit <n> --add-assignee @me` — the session's first write.
- **Resolve**: `gh issue comment <n> --body "<answer>"`, then `gh issue close <n>`, then append a context pointer (gist + link) to the map's Decisions-so-far.
