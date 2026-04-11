# DTSW

DTSW stands for `Does Trojan still work?`.

DTSW is a ground-up rewrite inspired by `easytrojan`, but it intentionally removes free shared domains and only supports user-owned domains. The current implementation uses:

- a Go control plane (`dtsw`)
- Xray as the first Trojan runtime adapter
- `acme.sh` as the ACME client for `Let's Encrypt` and `ZeroSSL`
- a Caddy-backed static fallback website for fresh installs
- generated `systemd` units for runtime, fallback, and auto-renewal

## One-line install

Download the installer from the latest published release, verify the binary checksum, install `dtsw`, and immediately start the interactive setup wizard:

```bash
curl -fsSL https://github.com/zhaodengfeng/DTSW/releases/latest/download/install.sh | bash
```

After setup finishes, DTSW now:

- saves the configuration
- installs or repairs the server automatically when running as root
- reuses an existing valid certificate if one is already present
- prints the client-ready connection details
- opens the management panel automatically so the user can keep choosing actions without typing commands

## Interactive flow

The normal user flow is menu-driven:

1. Run the release installer.
2. Answer the guided setup questions.
3. Let DTSW install the server.
4. Use the management panel to view client information, inspect status, repair the installation, upgrade Xray, renew certificates, manage multiple users, or uninstall DTSW.

Running `dtsw` without arguments now opens the interactive launcher. If DTSW finds a saved configuration, the launcher lets you:

- open the management panel
- install or repair with the saved configuration
- show the primary client configuration
- rerun guided setup

## Current scope

Implemented now:

- interactive setup wizard with automatic installation when run as root
- interactive launcher for zero-argument startup
- interactive management panel with one-click Xray upgrades, repair actions, menu-driven user management, and menu-driven uninstall
- initialize, validate, and render DTSW/Xray config
- generate runtime, fallback, and renewal `systemd` units
- install DTSW, pinned `acme.sh`, pinned Xray, pinned Caddy, config files, fallback site content, and services on Linux
- reuse an existing valid certificate on reinstall instead of requesting a new one
- print client-ready connection details after installation
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
- first-time setup resolves the latest stable Xray release and writes it into the config; if lookup fails, DTSW falls back to the bundled version `v26.1.13`
- fresh installs default to a Caddy-served static fallback website on `127.0.0.1:8080`
- the default Caddy version is pinned in code and currently set to `v2.10.2`
- the default `acme.sh` version is pinned in code and currently set to `3.1.2`

## Advanced commands

The interactive menus are the primary path, but command-based operations still exist for automation and debugging:

```bash
dtsw
dtsw status --config /etc/dtsw/config.json
dtsw doctor --config /etc/dtsw/config.json
sudo dtsw runtime upgrade --config /etc/dtsw/config.json --latest
```

## Design choices

- Domain ownership is mandatory. IP addresses, free wildcard-style domain shortcuts, and bundled public domains are out of scope.
- Certificate lifecycle stays outside the runtime, so CA switching, diagnostics, and reload behavior are controlled by DTSW rather than embedded in the proxy engine.
- Fresh installs serve fallback traffic with a DTSW-managed Caddy static site, while the older built-in fallback page remains available as a compatibility mode in config.
- Xray is the first runtime backend because it is a conservative default for a Trojan-focused migration path. The codebase keeps room for later runtime adapters.
- Config files containing passwords are written with `0600` permissions for security.
