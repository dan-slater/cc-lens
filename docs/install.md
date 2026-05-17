# Install

`cc-lens` ships as a single static binary with no runtime dependencies. Pick
whichever path fits your environment.

## With `go install` (recommended for developers)

```sh
go install github.com/dan-slater/cc-lens@latest
```

This drops `cc-lens` into `$(go env GOBIN)` (or `$(go env GOPATH)/bin`). Make
sure that's on your `PATH`.

Requires Go **1.24+**. You don't need Go to *run* the resulting binary on
another machine — copy it across and execute.

## Prebuilt binary (recommended for servers and non-Go users)

Grab the latest release from
<https://github.com/dan-slater/cc-lens/releases>. Binaries are produced for
`linux`, `darwin`, and `windows` × `amd64` and `arm64` by the
[`release` job in `.github/workflows/ci.yml`](../.github/workflows/ci.yml).

```sh
# Example: Linux amd64
curl -L -o cc-lens https://github.com/dan-slater/cc-lens/releases/latest/download/cc-lens-linux-amd64
chmod +x cc-lens
sudo mv cc-lens /usr/local/bin/
```

## From source

```sh
git clone https://github.com/dan-slater/cc-lens
cd cc-lens
go build -o cc-lens .
```

## Running as a service

A minimal systemd unit:

```ini
# /etc/systemd/system/cc-lens.service
[Unit]
Description=cc-lens
After=network.target

[Service]
Environment=CC_LENS_TOKEN=replace-me
Environment=CC_LENS_ADDR=127.0.0.1:8787
ExecStart=/usr/local/bin/cc-lens start
Restart=on-failure
DynamicUser=yes
ReadWritePaths=

[Install]
WantedBy=multi-user.target
```

Notes:
- Bind to `127.0.0.1` and put a reverse proxy in front for TLS — see
  [security.md](./security.md).
- The default user has no write access to anything outside its runtime dir;
  cc-lens reads `~/.claude/projects` from the invoking user, so on a server
  where you don't run Claude Code, the disk-lens half of `/sessions` just
  returns empty — that's fine.

## Verify

```sh
cc-lens --help
curl http://127.0.0.1:8787/healthz   # → "ok"
```

Next: [configure it](./configuration.md), then [install the hooks](./hooks.md).
