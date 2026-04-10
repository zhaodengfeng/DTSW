# DTSW

DTSW stands for `Does Trojan still work?`.

DTSW is a ground-up rewrite inspired by `easytrojan`, but it intentionally removes free shared domains and only supports user-owned domains. The current implementation uses:

- a Go control plane (`dtsw`)
- Xray as the first Trojan runtime adapter
- `acme.sh` as the ACME client for `Let's Encrypt` and `ZeroSSL`
- a built-in fallback HTTP service
- generated `systemd` units for runtime, fallback, and auto-renewal

## One-line install

Download the installer from the latest published release, verify the binary checksum, install `dtsw`, and automatically start the interactive setup wizard:

```bash
curl -fsSL https://github.com/zhaodengfeng/DTSW/releases/latest/download/install.sh | bash
```

This release-based installer keeps the script and downloaded binary on the same published release line instead of mixing `main` with an older release asset.

## Setup modes

For a full guided install on a server, run:

```bash
sudo dtsw setup
```

If you run setup without root, DTSW now defaults to a local config path and writes the config safely, then tells you which `sudo dtsw install --config ...` command to run next.

## Manual setup

If you prefer the flag-based path:

```bash
dtsw init --domain trojan.example.com --email admin@example.com --password change-me
sudo dtsw install --config configs/dtsw.example.json
```

## Current scope

Implemented now:

- interactive setup wizard with automatic installation when run as root
- initialize, validate, and render DTSW/Xray config
- generate runtime, fallback, and renewal `systemd` units
- install DTSW, pinned `acme.sh`, pinned Xray, config files, and services on Linux
- request and renew certificates with `Let's Encrypt` or `ZeroSSL`
- inspect health with `status` and `doctor`
- manage Trojan users with `list`, `add`, `del`, and `url`
- uninstall managed services and generated files

Not implemented yet:

- traffic statistics and quota management
- alternate runtime adapters such as `sing-box`

## Supported assumptions

- real installation targets Linux with `systemd`
- certificate automation uses a pinned `acme.sh` script plus DTSW-managed renewal timers
- HTTP-01 requires TCP `80` to be reachable
- DNS-01 requires provider credentials in `/etc/dtsw/acme.env`
- the default runtime version is pinned in code and currently set to `v26.1.13`
- the default `acme.sh` version is pinned in code and currently set to `3.1.2`

## Quick start

Run the release installer:

```bash
curl -fsSL https://github.com/zhaodengfeng/DTSW/releases/latest/download/install.sh | bash
```

Or start the wizard directly:

```bash
sudo dtsw setup
```

Generate a starter config manually:

```bash
dtsw init --domain trojan.example.com --email admin@example.com --password change-me
```

Validate it:

```bash
dtsw validate --config configs/dtsw.example.json
```

Preview the install flow without touching the machine:

```bash
dtsw install --config configs/dtsw.example.json --dry-run
```

Install on a Linux host as root:

```bash
sudo dtsw install --config /etc/dtsw/config.json
```

Check runtime and certificate state:

```bash
dtsw status --config /etc/dtsw/config.json
dtsw doctor --config /etc/dtsw/config.json
```

Manage users:

```bash
dtsw users list --config /etc/dtsw/config.json
dtsw users add --config /etc/dtsw/config.json --name secondary --password s3cret
dtsw users del --config /etc/dtsw/config.json --name secondary
```

Export a client URL:

```bash
dtsw users url --config /etc/dtsw/config.json --name primary
```

Inspect supported certificate issuers:

```bash
dtsw tls issuers
```

Preview issuance or renewal commands:

```bash
dtsw issue --config /etc/dtsw/config.json --dry-run
dtsw renew --config /etc/dtsw/config.json --dry-run
```

Remove managed services and config:

```bash
sudo dtsw uninstall --config /etc/dtsw/config.json
```

## Design choices

- Domain ownership is mandatory. IP addresses, free wildcard-style domain shortcuts, and bundled public domains are out of scope.
- Certificate lifecycle stays outside the runtime, so CA switching, diagnostics, and reload behavior are controlled by DTSW rather than embedded in the proxy engine.
- Fallback traffic is handled by `dtsw fallback-serve`, so the first usable version does not need Nginx or Caddy just to answer plain HTTP traffic.
- Xray is the first runtime backend because it is a conservative default for a Trojan-focused migration path. The codebase keeps room for later runtime adapters.
- Config files containing passwords are written with `0600` permissions for security.
