## Why

QNAP NAS devices lack a mechanism to automatically expose Docker containers via mDNS hostnames like `<container-name>.local`. Currently, users must manually configure reverse proxy rules in the QNAP Web UI and separately manage mDNS/DNS entries. This change delivers a self-contained QPKG that watches Docker containers, generates QNAP reverse proxy configuration, and publishes mDNS advertisements — all without binding ports 80/443 or running a separate proxy.

## What Changes

- A new Go daemon (`qnap-docker-mdnsd`) that:
  - Watches Docker containers via `/var/run/docker.sock` for lifecycle events
  - Parses `qnap-docker-mdns.*` container labels to determine eligibility, hostnames, aliases, and ports
  - Probes host-published TCP ports through loopback to auto-detect HTTP-capable backends
  - Generates managed JSON entries in `/etc/config/reverseproxy/reverseproxy.json` with embedded ownership markers
  - Triggers QNAP's `scan_config` to regenerate Apache per-port files from JSON
  - Validates generated config and gracefully reloads the reverse proxy
  - Launches and supervises `avahi-publish-address` processes per hostname/IP pair
  - Emits QNAP notices (`notice_log_tool`) and syslog (`logger`) for operational failures and recovery
  - Retries operational failures with exponential backoff and deduplicates notifications
  - Uses advisory `flock` for single-instance enforcement
- A QPKG control script (`qnap-docker-mdns.sh`) for install, start, stop, restart, and uninstall
- YAML configuration with packaged defaults and operator-managed runtime overrides
- Dated backup/rollback for `reverseproxy.json` before substantive changes
- A `Makefile` for local development, cross-compilation, and QPKG assembly
- Operator documentation covering the QNAP reverse proxy UI refresh quirk

## Capabilities

### New Capabilities

- `docker-discovery`: Watch Docker containers via Unix socket, inspect labels and port bindings, probe HTTP endpoints through loopback, and build desired backend state
- `proxy-rendering`: Generate managed JSON entries in QNAP's `reverseproxy.json` with embedded ownership markers, hostname-collision detection, alias expansion, 64-rule limit enforcement, and deterministic conflict resolution
- `config-reconciliation`: Safely merge managed JSON entries into `reverseproxy.json`, create dated backups, run `scan_config`, validate generated Apache config, reload reverse proxy, and roll back on failure
- `mdns-publication`: Discover NAS LAN IPv4 addresses, launch/supervise `avahi-publish-address` processes per hostname/IP pair, detect mDNS collisions, and reconcile publishers on restart
- `notifications-logging`: Emit deduplicated QNAP notices via `notice_log_tool` for actionable misconfiguration, operational failures, and recovery; emit syslog entries via `logger`; persist open-problem state across restarts
- `event-loop`: Subscribe to Docker lifecycle events, debounce rapid bursts, coordinate event-triggered and periodic reconciliation, enforce single-instance locking
- `qpkg-packaging`: QPKG metadata (`qpkg.cfg`), control script, default config and sample, cross-compilation workflow, `Makefile`, and operator documentation

### Modified Capabilities

- None (no existing specs to modify)

## Impact

- **New daemon binary**: `qnap-docker-mdnsd` — a Go daemon shipped as a static binary for QNAP architectures
- **New QPKG package**: Installed under the QNAP-managed QPKG path, with files in `$QPKG_ROOT/`
- **QNAP system files**: Managed writes to `/etc/config/reverseproxy/reverseproxy.json` (via JSON entries with ownership markers); triggers `scan_config` which regenerates `/etc/reverseproxy/extra/*.conf`; launches `avahi-publish-address` child processes
- **No external dependencies**: Pure-Go static binary, no runtime requirements beyond QNAP base OS
- **No port conflicts**: Daemon does not bind ports 80 or 443
