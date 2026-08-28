# go-qrcode

## Repository

`origin` is the fork **`wodin/go-qrcode`** — the only remote we can write to.
`upstream` is **`skip2/go-qrcode`**, read-only. Push branches and open issues on
`origin`; never assume a command targeting "the repo" means upstream.

## Build & test

Run the tests:

```
go test ./...
```

`.gitignore` carries a blanket `*.png`. Don't narrow that rule — no PNG in this
repo is meant to be tracked.

The tests write no PNG into the working tree. A decode round-trip mismatch
still dumps the failing symbol for inspection, but into
`$TMPDIR/go-qrcode-failed-symbols/<test name>/`, and logs the path
(`writeFailedSymbol`, `qrcode_decode_test.go`). Those directories deliberately
survive the run; don't move them to `t.TempDir`, which would delete each symbol
before anyone could open the path that was logged.

Cross-compile the CLI for Windows:

```
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -ldflags="-s -w" -trimpath -o qrcode.exe ./qrcode
```

## Agent skills

### Issue tracker

GitHub Issues on the fork `wodin/go-qrcode`, via the `gh` CLI; upstream issues
are a read-only source. See `docs/agents/issue-tracker.md`.

### Triage labels

The five canonical roles, unmapped (label string = role name). See
`docs/agents/triage-labels.md`.

### Domain docs

Single-context: `CONTEXT.md` + `docs/adr/` at the repo root. See
`docs/agents/domain.md`.
