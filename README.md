# DTSW

DTSW stands for `Does Trojan still work?`.

DTSW is a ground-up rewrite direction inspired by `easytrojan`, but it intentionally removes free shared domains and only supports user-owned domains. The current implementation uses:

- a Go control plane (`dtsw`)
- Xray as the first Trojan runtime adapter
- `acme.sh` as the ACME client for `Let's Encrypt` and `ZeroSSL`
- built-in fallback HTTP service
- generated `systemd` units for runtime, fallback, and auto-renewal

## One-line install

Install the DTSW CLI on a Linux server:

```bash
curl -fsSL https://raw.githubusercontent.com/zhaodengfeng/DTSW/main/install.sh | bash
```

Then generate a starter config and install as root:

```bash
dtsw init --domain trojan.example.com --email admin@example.com --password change-me
sudo dtsw install --config configs/dtsw.example.json
```

## Current scope

Implemented now:

- initialize, validate, and render DTSW/Xray config
- generate runtime, fallback, and renewal `systemd` units
- install DTSW, `acme.sh`, pinned Xray, config files, and services on Linux
- request and renew certificates with `Let's Encrypt` or `ZeroSSL`
- inspect health with `status` and `doctor`
- manage Trojan users with `list`, `add`, `del`, and `url`
- uninstall managed services and generated files

Not implemented yet:

- traffic statistics and quota management
- alternate runtime adapters such as `sing-box`
- packaged upgrade workflow beyond the installer script and GitHub releases

## Supported assumptions

- real installation targets Linux with `systemd`
- certificate automation uses `acme.sh`
- HTTP-01 requires TCP `80` to be reachable
- DNS-01 requires provider credentials in `/etc/dtsw/acme.env`
- the default runtime version is pinned in code and currently set to `v26.1.13`

## Quick start

Generate a starter config:

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
dtsw install --config /etc/dtsw/config.json
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
dtsw uninstall --config /etc/dtsw/config.json
```

## Design choices

- Domain ownership is mandatory. IP addresses, free wildcard-style domain shortcuts, and bundled public domains are out of scope.
- Certificate lifecycle stays outside the runtime, so CA switching, diagnostics, and reload behavior are controlled by DTSW rather than embedded in the proxy engine.
- Fallback traffic is handled by `dtsw fallback-serve`, so the first usable version does not need Nginx or Caddy just to answer plain HTTP traffic.
- Xray is the first runtime backend because it is a conservative default for a Trojan-focused migration path. The codebase keeps room for later runtime adapters.
