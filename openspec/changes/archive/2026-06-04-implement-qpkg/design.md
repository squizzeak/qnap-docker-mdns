## Context

QNAP NAS devices run a reverse proxy (Apache-based) whose configuration is stored in `/etc/config/reverseproxy/reverseproxy.json`. QNAP's `scan_config` function regenerates Apache per-port files from this JSON. The daemon must integrate with this existing mechanism rather than running an independent proxy.

Docker containers run locally on the NAS and expose host-published TCP ports reachable through `localhost`. The daemon discovers containers via the Docker Unix socket and manages the QNAP reverse proxy to route `<container-name>.local` hostnames to the correct backend.

mDNS is handled by Avahi, which is available on QNAP. The daemon launches `avahi-publish-address` processes per hostname/IP pair.

## Goals / Non-Goals

**Goals:**
- Automatically expose Docker containers as `<container-name>.local` via QNAP's reverse proxy
- Publish mDNS advertisements for each hostname to supported NAS LAN IPv4 addresses
- Preserve unmanaged reverse proxy entries and survive QNAP UI edits
- Fully reversible install, upgrade, and uninstall
- Safe reconciliation with backups, validation, and rollback
- Operational notifications through QNAP's native notice system

**Non-Goals:**
- HTTPS VirtualHosts (future)
- Let's Encrypt integration (future)
- Web UI for management (future)
- Running as an independent reverse proxy or binding ports 80/443
- Container Station integration (future)

## Decisions

### Implementation language: Go
Pure-Go static binary with `CGO_ENABLED=0`. Cross-compiles from `darwin/arm64` to `linux/amd64` for x86_64 QNAP targets. Native Docker SDK via `github.com/docker/docker/client`. No runtime dependencies on the NAS.

### JSON-driven reverse proxy strategy
The daemon writes managed entries to `/etc/config/reverseproxy/reverseproxy.json` (not directly to Apache files) and triggers QNAP's `scan_config` to regenerate per-port files. This ensures QNAP Web UI consistency and survives `scan_config` runs from external triggers.

### Embedded ownership markers
Managed JSON entries carry `qnap_docker_mdns_managed: true` and a stable `qnap_docker_mdns_key`. This allows the daemon to identify its own entries without relying on display names or external state. Custom fields survive QNAP UI saves and regeneration on tested QTS versions.

### Alias expansion as separate JSON entries
Rather than using Apache `ServerAlias`, each alias becomes its own `reverseproxy.json` entry with a distinct stable key. This keeps the QNAP UI consistent (each hostname appears as its own rule).

### Docker API vs CLI
Using `github.com/docker/docker/client` over `/var/run/docker.sock` avoids subprocess management and provides structured container metadata. Event subscriptions for start/stop/die/destroy/rename trigger reconciliation.

### Port auto-detection via HTTP probing
Before selecting a backend port, the daemon probes each host-published TCP port through `localhost` with a short HTTP request. Only ports returning a parseable HTTP response are candidates. This avoids proxying to non-HTTP services.

### Avahi-publish-address for mDNS
One `avahi-publish-address -a` subprocess per hostname and NAS LAN IPv4 address pair. The daemon tracks child PIDs and reconciles on startup by inspecting running helpers' command-line metadata.

### Advisory flock for single-instance
Uses `flock` on `/var/run/qnap-docker-mdns/daemon.lock` rather than PID files. Kernel-released on crash, so stale paths don't block recovery.

### Deduplicated notifications with persisted state
Open problems are tracked in a JSON state file under `/var/run/qnap-docker-mdns/`. Each problem signature maps to exactly one failure notice and one recovery notice. State survives daemon restart but not NAS reboot.

## Risks / Trade-offs

| Risk | Mitigation |
|------|------------|
| `scan_config` from QNAP UI between daemon writes could lose managed entries | Periodic reconciliation (default 5 min) re-merges managed entries; no timing-sensitive coordination needed |
| Custom JSON fields could be stripped by future QTS updates | Stage 1 platform integration tests this; if stripped, daemon falls back to `name` suffix matching as secondary ownership signal |
| Avahi not available on all QNAP models | mDNS publication failure is treated as operational (not blocking reverse proxy); retried with backoff |
| Apache reload command varies across QTS versions | Configurable `reload_command` and `validate_command` in YAML; Stage 1 determines safe defaults |
| 64-rule limit in `reverseproxy.json` | Daemon enforces the limit and emits user-visible notice before exceeding it |
| Docker socket not accessible from QPKG service context | Stage 1 validates access; documented as prerequisite |
