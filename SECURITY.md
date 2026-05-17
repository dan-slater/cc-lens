# Security policy

cc-lens is intentionally small so that the attack surface is small. This
document describes how we publish, what we sign, what we won't do, and how
to report a vulnerability.

## Supported versions

Only the latest tagged release is supported with security fixes. There is no
LTS branch.

## Reporting a vulnerability

**Do not file a public GitHub issue for vulnerabilities.**

Email **dan.slater@gmail.com** with the subject `cc-lens security:` and a
short description. PGP key fingerprint on request. Expect an initial
acknowledgement within 72 hours.

If the issue materially helps an attacker before a fix ships, we ask for a
coordinated disclosure window (typically 14 days).

## What we sign

| Artifact            | Signing mechanism                                                                 |
| ------------------- | --------------------------------------------------------------------------------- |
| Git tags            | We tag with `git tag -s` (GPG); the public key is published on github.com/dan-slater.gpg. |
| Release binaries    | Built in GitHub Actions and attested via `actions/attest-build-provenance` (SLSA build provenance). |
| Container images    | Not currently shipped.                                                            |

Verifying a release binary:

```sh
gh attestation verify cc-lens-linux-amd64 --owner dan-slater
```

## What we do (operational hardening)

- **Zero production dependencies.** CI gate (`scripts/check-zero-deps.mjs` …
  actually `.github/workflows/ci.yml::zero-deps`) rejects any non-stdlib
  import in `go.mod`. Reduces supply-chain blast radius to zero.
- **Branch protection** on `main`: pull-request required, no force push, no
  deletion, linear history.
- **Signed tags** for releases (see above).
- **Build provenance** attached to every release artifact.
- **2FA + passkey** required on the maintainer GitHub account; the npm
  account is not used (we don't publish to npm).
- **CODEOWNERS** routes every PR to the maintainer for review.

## What we don't do (yet — and what we won't)

- We do **not** publish to npm. cc-lens is distributed only as a tagged
  GitHub release and a `go install` target.
- We do **not** auto-merge dependency-update PRs (we don't have dependency
  updates because there are no dependencies).
- We do **not** accept anonymous binary contributions. Compiled artifacts in
  PRs are rejected.
- We will **not** add backdoors, telemetry, or remote configuration. cc-lens
  makes no outbound HTTP calls except (a) the optional webhook URL you
  configure and (b) the `tmux-relay` polling its own `--server`.

## Threat model recap

cc-lens is designed to be reachable by **trusted hook senders on machines
you control**. See [docs/security.md](./docs/security.md) for the runtime
security model (auth, transport, request limits). This document covers the
*build and distribution* model.

## Trademarks and naming

"cc-lens" and `github.com/dan-slater/cc-lens` are the only authorized
identifiers for this project. Forks are welcome under the FSL terms (see
[LICENSE](./LICENSE)), but must rename to avoid confusion in package
managers and search results.
