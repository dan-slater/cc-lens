# Contributing to cc-lens

Thank you for considering a contribution. A few constraints keep the project
honest:

## Hard rules

1. **Zero production dependencies.** Production code must compile against the
   Go standard library alone. CI rejects any non-stdlib import in
   non-test files.
2. **Stay small.** The production code budget is roughly 500 source lines. If
   your change pushes us materially over, justify it.
3. **License.** Contributions are accepted under the project's
   [FSL-1.1-ALv2](./LICENSE) license. By opening a PR you confirm you have
   the right to submit the work under those terms. No GPL/AGPL/CDDL code or
   test data — those licenses are incompatible with FSL redistribution.
4. **No new background goroutines without a shutdown path.**
5. **Tests use `testing` + `net/http/httptest` only.** No `testify`, no `gomock`.

## What I want PRs for

- Bug fixes with a failing test attached.
- Cross-platform installers (Windows hook command, PowerShell variant).
- New event consumers (webhook, NATS, MQTT, file-tail) as separate optional
  subcommands — same shape as `tmux-relay`.
- Docs improvements, especially "I tried to do X and got confused" stories.

## What I will probably push back on

- Adding a database or persistent ring buffer (use a webhook to your own DB).
- Adding a frontend (build it as a separate repo against the HTTP API).
- Adding ESLint-style strict linters or pre-commit hooks.
- Adding a JSON-schema validator dep when 10 lines of `encoding/json` work.

## Development loop

```sh
go build ./...
go test ./...
go vet ./...
gofmt -w .
```

CI runs the same four commands plus a cross-compile matrix.

## Release

1. Bump version in `main.go` (when we add one).
2. Tag: `git tag v0.x.y && git push --tags`.
3. CI builds binaries for `{linux,darwin,windows}` × `{amd64,arm64}` and
   attaches them to the GitHub Release.
