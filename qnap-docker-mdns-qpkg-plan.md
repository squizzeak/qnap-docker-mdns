# qnap-docker-mdns QPKG - Design Plan

## Goal

Create a QNAP-specific QPKG for the `qnap-docker-mdns` project that automatically exposes Docker containers as:

    <container-name>.local

using QNAP's built-in reverse proxy infrastructure, with mandatory mDNS advertisement, rather than running an independent proxy.

The package should:

1. Watch Docker containers and labels.
2. Generate QNAP reverse proxy VirtualHost entries.
3. Reload the QNAP reverse proxy when configuration changes.
4. Advertise mDNS hostnames.
5. Be fully reversible and survive upgrades.

---

## Architecture

QNAP Reverse Proxy (port 80/443)
        |
        v
Generated VirtualHosts
        |
        v
Docker Containers

The application does NOT bind ports 80 or 443.

Instead, it modifies QNAP reverse proxy configuration and reloads the service.

---

## Current Discovery

QNAP stores reverse proxy configuration in two locations:

### Apache config (routing)

    /etc/reverseproxy/extra/80.conf

Example:

    <VirtualHost *:80>
        ServerName plex.example.com
        ProxyPreserveHost on
        ProxyPass / http://localhost:5055/
        ProxyPassReverse / http://localhost:5055/
    </VirtualHost>

This is the per-port file that Apache reads directly. QNAP generates these files
from its JSON config database via the `scan_config` function in
`/etc/init.d/reverse_proxy.sh`.

### JSON config database (UI visibility)

    /etc/config/reverseproxy/reverseproxy.json

This JSON file is the QNAP UI's source of truth. Each rule's `name` field
becomes the Rule Name shown in the reverse proxy management table.

Observed QNAP UI behavior on at least one target NAS: after `reverseproxy.json`
is changed on disk, the reverse proxy web UI does not automatically refresh the
currently open Network Access view. User-facing documentation must instruct the
operator to click away from the `Network Access` section in the left sidebar
and then return to it to see the updated rule list.

Example entry:

```json
{
    "id": 1,
    "name": "Seer",
    "protocol": "http",
    "des_protocol": "http",
    "server_name": "plex.example.com",
    "hsts": false,
    "host_name": "localhost",
    "access": 1,
    "port": 80,
    "des_port": 5055,
    "proxy_timeout": 60,
    "header": []
}
```

### Access control profiles

QNAP stores reverse proxy access-control profiles in:

    /etc/config/reverseproxy/access.json

and generates Apache include fragments in:

    /etc/reverseproxy/access/

Observed built-in `local` profile on one target NAS:

- `access.json` entry `name: "local"`
- generated Apache include path follows `/etc/reverseproxy/access/<id>.conf`

Observed reverse proxy generation behavior from `/etc/init.d/reverse_proxy.sh`:

- each reverse proxy rule may carry an `access` field in
  `reverseproxy.json`
- when `access` is non-zero and the corresponding include file exists, QNAP
  emits `Include /etc/reverseproxy/access/<id>.conf` inside the generated
  `VirtualHost`
- observed testing on at least one target NAS shows unknown additional fields in
  `reverseproxy.json` survive both `scan_config` regeneration and a QNAP UI
  edit/save round trip

For this package, generated reverse proxy rules must always use the built-in
`local` access profile. The daemon must discover that profile's numeric `id`
from `/etc/config/reverseproxy/access.json` by matching `name: "local"`, then
set that discovered `id` in each managed JSON rule's `access` field and verify
that QNAP generates the matching `/etc/reverseproxy/access/<id>.conf` include in
the resulting Apache `VirtualHost`.

If the built-in `local` profile cannot be identified reliably, the daemon must
fall back to `access: 0` for managed rules. This leaves the reverse proxy rule
open to all clients, which is acceptable for this package because published
hostnames are restricted to `.local` mDNS names.

### scan_config behavior

The `scan_config` function in `/etc/init.d/reverse_proxy.sh`:

1. Deletes ALL files in `/etc/reverseproxy/extra/`.
2. Reads every entry from `reverseproxy.json`.
3. Generates a per-port `.conf` file for each unique listening port.
4. QNAP's UI also calls this function when rules are saved.

This means any `.conf` file we write independently of the JSON will be wiped
whenever `scan_config` runs from the UI or system events.

JSON-first strategy:

- JSON config: write matching entries into `reverseproxy.json` with
  `(managed)` in the `name` field so rules appear in the QNAP UI.
- Apache config: let QNAP regenerate `/etc/reverseproxy/extra/*.conf` from
  `reverseproxy.json` via `scan_config`.
- Validation: treat the generated per-port Apache files as derived output that
  must be inspected after `scan_config` to confirm expected routing and access
  includes were produced.
- Graceful reload via `/usr/local/apache/bin/apache_proxy -k graceful -f /etc/apache-sys-proxy.conf` after successful regeneration on systems where Stage 1 confirms that command controls the active reverse proxy.

---

## Container Labels

Containers are exposed only when explicitly enabled.

Example for a container with a single exposed port:

```yaml
labels:
  qnap-docker-mdns.enable: "true"
```

Optional:

```yaml
labels:
  qnap-docker-mdns.enable: "true"
  qnap-docker-mdns.port: "3000"
  qnap-docker-mdns.hostname: "grafana.local"
  qnap-docker-mdns.aliases: "metrics.local,graphs.local"
```

Defaults:

- hostname = `<container-name>.local`
- aliases = none
- port = the only candidate port when exactly one exists

A candidate port is a host-published TCP port that can accept HTTP proxy traffic through `localhost:<port>` on the NAS.

HTTP endpoint validation rules:

- A port is a candidate only if it is reachable through loopback on the NAS and returns a parseable HTTP response to a simple probe such as `GET /` or `HEAD /` over plain HTTP.
- Use a short timeout for the probe so an unresponsive container does not stall reconciliation.
- Treat connection refusal, timeout, TLS-only response, or other non-HTTP response as probe failure.
- If `qnap-docker-mdns.port` is set, that port must also pass the HTTP probe before the container is considered eligible.
- If no published port passes the HTTP probe, the container must not be published.
- Probe every eligible host-published TCP port before deciding whether the port
  label is required.

Port selection rules:

- If exactly one candidate port is available, the daemon uses it automatically.
- If a container exposes multiple host-published TCP ports, auto-detect the
  HTTP endpoint by probing all of them and use it automatically when exactly one
  candidate port is found.
- If more than one candidate port is available after probing, `qnap-docker-mdns.port` is required.
- If no candidate ports are available, the container is skipped and treated as actionable misconfiguration.
- `qnap-docker-mdns.port` must match one of the container's host-published TCP ports.
- A host-published TCP port is a candidate only when the binding is reachable through loopback on the NAS.
- Bindings published only to a non-loopback host address are out of scope and must be skipped as actionable misconfiguration.

Hostname rules:

- Hostnames and aliases must normalize to lowercase FQDNs under the configured suffix unless an explicit FQDN is supplied.
- Duplicate hostnames or aliases within one container definition are invalid.
- Resolve managed-name collisions with deterministic alphanumeric precedence based on the daemon's stable normalized name ordering during reconciliation.
- If two enabled containers claim the same primary hostname, the lexically first container in that stable normalized ordering keeps the hostname and later conflicting containers are skipped for that primary hostname until the conflict is resolved.
- If an alias collides with another managed hostname or alias, keep the lexically first claim in stable normalized reconciliation order and skip only the later colliding alias while continuing to publish the remaining non-conflicting names for that container.
- If one container's primary hostname collides with another container's alias, keep the lexically first claim in stable normalized reconciliation order and skip only the later conflicting name.
- If a requested primary hostname collides with an unmanaged reverse proxy `ServerName` or `ServerAlias` already present outside daemon-managed JSON entries, the container must be skipped as actionable misconfiguration.
- If a requested alias collides with an unmanaged reverse proxy `ServerName` or `ServerAlias` already present outside daemon-managed JSON entries, skip only the colliding alias and continue publishing the remaining non-conflicting names for that container.

---

## Generated Configuration

The daemon writes managed reverse proxy state to `reverseproxy.json` and relies
on QNAP to regenerate the Apache per-port files from that JSON.

### Apache per-port file (routing)

The per-port Apache file (e.g., `/etc/reverseproxy/extra/80.conf`) is treated as
derived output generated by QNAP from `reverseproxy.json`.

Expected generated result for one managed hostname:

```apache
<VirtualHost *:80>
    ServerName grafana.local

    Include /etc/reverseproxy/access/<discovered-local-access-id>.conf

    ProxyPreserveHost on

    ProxyPass / http://localhost:3000/ connectiontimeout=60 timeout=60
    ProxyPassReverse / http://localhost:3000/
</VirtualHost>
```

All generated backends must target `http://localhost:<selected-host-published-port>/`.

Aliases must be represented as additional managed JSON entries rather than as
Apache `ServerAlias` directives. A container with aliases therefore produces one
managed reverse proxy rule per published hostname.

All generated Apache `VirtualHost` entries must include the discovered
`/etc/reverseproxy/access/<id>.conf` for the built-in QNAP `local`
access-control profile.

The daemon must not edit these generated Apache files directly; it must verify
that the generated output matches the managed JSON rules after `scan_config`.

### JSON config database (UI visibility)

A matching entry is written to `/etc/config/reverseproxy/reverseproxy.json` so
the rule appears in the QNAP UI with the Rule Name column showing `(managed)`.

Daemon ownership of JSON entries must NOT be inferred from the human-visible
`name` field alone. Ownership is tracked directly inside each daemon-managed
JSON rule through custom fields that survive QNAP regeneration and UI saves, so
the daemon can update only the entries it created.

Generated JSON entry:

```json
{
    "list": {
        "5": {
            "id": 5,
            "name": "grafana (managed)",
            "protocol": "http",
            "des_protocol": "http",
            "server_name": "grafana.local",
            "hsts": false,
            "host_name": "localhost",
            "access": 1,
            "port": 80,
            "des_port": 3000,
            "proxy_timeout": 60,
            "header": [],
            "qnap_docker_mdns_managed": true,
            "qnap_docker_mdns_key": "container:grafana|hostname:grafana.local"
        }
    }
}
```

Rule name convention:

- `name`: `<primary-hostname> (managed)` so the QNAP UI Rule Name column
  clearly shows which rules are daemon-managed.
- `server_name`: the generated hostname (e.g., `grafana.local`).
- `host_name`: `localhost` (backends always target `localhost`).
- `access`: the discovered numeric `id` for the built-in `local` access-control
  profile.
- `des_port`: the selected host-published backend port.
- `port`: the Apache listening port this rule belongs to (typically `80`).
- `qnap_docker_mdns_managed`: `true` so the daemon can recognize entries it owns
  without relying on the display name.
- `qnap_docker_mdns_key`: the stable managed-rule identity used to match an
  existing owned entry even if other editable fields change.

Managed rule expansion:

- The primary hostname and each non-colliding alias expand into separate managed
  reverse proxy JSON entries.
- Each expanded entry uses its own `server_name`, human-visible `name`, and
  stable `qnap_docker_mdns_key`.
- All expanded entries for the same container reuse the same selected backend
  port and discovered `local` access-profile ID.

ID assignment:

- Read the existing `reverseproxy.json` and find the maximum `id` across
  ALL entries (managed and unmanaged).
- Assign IDs starting from `max_id + 1` for new managed rules.
- QNAP supports up to 64 reverse proxy rules total (`reverseproxy.json`
  list limit). The daemon must respect this limit: if adding a new
  managed rule would exceed 64 total entries, do not add it and emit a
  user-visible notice.
- Track assigned IDs in the managed JSON entries themselves so re-reconciliation
  updates the same rule in place rather than creating duplicates.
- Mark daemon-owned JSON entries with `qnap_docker_mdns_managed: true` and a
  stable `qnap_docker_mdns_key` derived from the managed rule identity.
- The embedded ownership marker and key are the source of truth for which JSON
  entries the daemon may update or delete; the display `name` suffix is
  informational only.
- Any JSON entry that lacks `qnap_docker_mdns_managed: true` must be treated as
  unmanaged by default.
- Missing custom ownership fields must never cause missing-key errors; absent
  `qnap_docker_mdns_managed` means "not ours", and `qnap_docker_mdns_key` must
  only be read after the managed marker is present and true.
- When a managed container is removed, delete only the JSON entry whose custom
  ownership marker is present and whose `qnap_docker_mdns_key` matches that
  managed rule.
- If a marked entry is malformed or missing its stable key, do not mutate it
  automatically; emit an operational failure so an administrator can inspect it.

---

## JSON-Driven Reverse Proxy Strategy

The daemon treats `reverseproxy.json` as the source of truth for managed reverse
proxy rules and keeps the generated Apache output consistent by invoking and
verifying QNAP's `scan_config` flow.

### Apache per-port file

Never write the per-port file directly.

After the daemon updates `reverseproxy.json`, it must run `scan_config`, then
inspect the regenerated per-port file to confirm that expected `VirtualHost`
entries, backend ports, and `/etc/reverseproxy/access/<id>.conf` include lines
were produced for each managed hostname.

### JSON config database

On every reconciliation:

1. Read `/etc/config/reverseproxy/reverseproxy.json`.
2. Scan the JSON entries for daemon ownership markers and stable keys, treating
   any entry without `qnap_docker_mdns_managed: true` as unmanaged.
3. Discover the built-in `local` access profile `id` from
   `/etc/config/reverseproxy/access.json` by matching `name: "local"`.
4. Expand each eligible container into one desired managed rule for the primary
   hostname plus one desired managed rule for each non-colliding alias.
5. For each active managed rule, update the existing owned JSON entry in place
   when a marked entry with the matching stable key already exists.
6. For each active managed rule that has no owned JSON entry yet, allocate the
   next available `id` (`max_id + 1`), create the JSON entry, and persist the
   new ownership marker fields plus the discovered `local` access-profile `id`
   in that entry.
7. Remove only JSON entries that are explicitly marked as daemon-managed and no
   longer have a matching active managed rule by stable key.
8. If the rendered JSON content differs from the current file, create a dated
   backup of the current `reverseproxy.json` before replacing it.
9. Write the updated JSON back atomically.
10. Run `/etc/init.d/reverse_proxy.sh scan_config` so QNAP regenerates the
    per-port Apache files from the updated JSON.
11. Validate the generated Apache config with the platform-confirmed validation
    command.
12. Gracefully reload the reverse proxy with the platform-confirmed reload
    command.
13. Inspect the generated per-port files and confirm they contain the expected
    `VirtualHost` entries and discovered `local` access-profile include.

Concurrency note:

- The daemon does not need a special coordination mechanism with QNAP UI writes.
- If the QNAP UI and the daemon both modify `reverseproxy.json` near the same
  time, the next periodic reconciliation pass is responsible for re-merging the
  daemon-managed entries back to the desired state.
- Preserve unmanaged entries on every merge so eventual reconciliation converges
  without requiring the daemon to own the entire file.

### scan_config recovery

If external `scan_config` runs (from QNAP UI save, reboot, or other trigger),
QNAP regenerates the per-port files from JSON. The daemon's next reconciliation
must re-verify that the managed JSON entries still exist and that the generated
Apache output still matches them.

### Uninstall

1. Remove daemon entries from `reverseproxy.json`.
2. Run `scan_config` so QNAP regenerates the per-port files without the removed
   managed entries.
3. Stop daemon-managed Avahi publishers.
4. Graceful reload.

---

## Backup Strategy

Before each substantive change to `reverseproxy.json`:

```bash
cp /etc/config/reverseproxy/reverseproxy.json \
   "/etc/config/reverseproxy/reverseproxy.json.qnap-docker-mdns.$(date +%Y%m%d-%H%M%S).bak"
```

Backup rules:

- Create a new dated backup only when the rendered JSON content differs from the
  current on-disk file.
- Treat the most recent dated backup created immediately before a failed write
  attempt as the rollback source for that reconciliation attempt.
- Retain at most `max_backups` dated backup files and prune the oldest ones
  after a successful new backup is created.
- Default `max_backups` to `100`.
- These backups exist for rollback, troubleshooting, and disaster recovery, not
  as the primary uninstall path.

---

## Docker Discovery

Use Docker API over:

    /var/run/docker.sock

Do not use the Docker CLI as the primary integration path.

Rationale:

- the daemon needs structured container metadata without parsing human-oriented command output
- the daemon needs stable access to labels, published port bindings, names, and lifecycle events
- the daemon needs event subscription support, which the Docker API provides directly
- the Docker CLI is a thin client over the same daemon API and adds parsing overhead and subprocess management without adding capabilities needed here

How this works on QNAP:

- the daemon opens the Unix domain socket at `/var/run/docker.sock`
- requests are sent to the local Docker Engine API over that socket
- no TCP listener is required
- no separate Docker service discovery step is required beyond access to the local socket

Read:

- running containers
- labels
- container names
- network information
- exposed TCP ports
- host-published TCP port bindings
- host bind addresses for published ports

Maintain in-memory registry:

```go
type Backend struct {
    ContainerName string
    Hostnames []string
    Port string
}
```

Only containers with host-published TCP ports that are reachable through NAS loopback are supported.

Containers reachable only by Docker bridge IP, container DNS name, unpublished internal ports, or non-loopback-only host bindings are out of scope and must be skipped.

The discovery logic must treat Docker port bindings conservatively:

- bindings on `0.0.0.0` are supported because they are reachable through `localhost`
- bindings on `127.0.0.1` are supported
- bindings on specific non-loopback IPv4 addresses are out of scope for this version
- IPv6-only bindings are out of scope for this version

---

## Docker Event Watching

Subscribe to:

- start
- stop
- die
- destroy
- rename

Event handling rules:

- Treat any Docker event for a tracked container as a signal to re-inspect that container's current labels and published-port bindings.
- Treat lifecycle events as a trigger to re-read container metadata from the Docker API rather than trusting event payloads alone.
- Container stop, destroy, rename, or replacement events that make a container no longer eligible must remove its reverse proxy mapping and terminate its Avahi publishers on the next reconciliation pass.
- Run a periodic full rescan as a safety net so missed or coalesced Docker events cannot leave stale reverse proxy or mDNS state behind.
- Use a default debounce window of 500ms after the most recent event before reconciling.
- Run a full rescan every 5 minutes by default, with a small startup jitter, and make the interval configurable.

Rebuild configuration when changes occur.

Debounce updates.

Example:

- wait 500ms after final event
- regenerate config
- reload reverse proxy

---

## Reverse Proxy Reload

Implementation must determine correct QNAP reload command.

Candidates to test:

```bash
/usr/local/apache/bin/apache_proxy -t -f /etc/apache-sys-proxy.conf
/usr/local/apache/bin/apache_proxy -k graceful -f /etc/apache-sys-proxy.conf
/usr/local/apache/bin/apachectl -t -f /etc/apache-sys-proxy.conf
/usr/local/apache/bin/apachectl graceful -f /etc/apache-sys-proxy.conf
/etc/init.d/reverse_proxy.sh restart
/etc/init.d/Qthttpd.sh restart
```

The daemon should support configuration:

```yaml
reload_command: "/usr/local/apache/bin/apache_proxy -k graceful -f /etc/apache-sys-proxy.conf"
validate_command: "/usr/local/apache/bin/apache_proxy -t -f /etc/apache-sys-proxy.conf"
probe_timeout: 2s

reconcile:
  debounce: 500ms
  full_rescan_interval: 5m

retry:
  immediate_retries: 1
  initial_backoff: 5s
  max_backoff: 5m
  jitter_percent: 20

backups:
  max_backups: 100
```

to accommodate QTS differences.

Observed QNAP systems may run the reverse proxy through `/usr/local/apache/bin/apache_proxy -f /etc/apache-sys-proxy.conf`.

Verified note:

- On at least one target QNAP system, `/usr/local/apache/bin/apache_proxy` is a symlink to `/usr/local/apache/bin/apache`.
- Even so, `apache_proxy -f /etc/apache-sys-proxy.conf` remains the preferred invocation name for this package because it matches the live reverse proxy process observed on the NAS.
- On at least one target QNAP system, the running reverse proxy master process is `/usr/sbin/reverseproxy -k start -f /etc/reverseproxy/reverseproxy.conf` and accepts `SIGHUP` without error.
- Even when a signal-based Apache reload path is available, JSON changes still require `scan_config` first because the per-port Apache files are regenerated from `reverseproxy.json` rather than re-read directly from JSON by the running process.
- Treat `apachectl` as a secondary option only if it is confirmed to control the same reverse proxy configuration.
- Fall back to QNAP wrapper scripts only when the Apache-native commands do not cover the managed reverse proxy configuration on the target QTS version.

---

## mDNS Support

mDNS publication is required.

For every generated hostname and alias, the daemon must publish mDNS address records that resolve to each supported NAS LAN IPv4 address.

Implementation approach:

1. Discover the supported NAS LAN IPv4 addresses from the QNAP itself.
2. For each active hostname and each supported NAS LAN IPv4 address, launch an `avahi-publish-address -a` process.
3. Keep those publisher processes running as long as the container mapping is active.
4. Terminate and remove the publishers when the container stops, is renamed,
   is replaced by an ineligible container instance, or the supported NAS LAN
   IPv4 address set changes.
5. Reconcile publishers after daemon restart so orphaned Avahi processes are not left behind.

Reverse proxy backends must use `localhost:<selected-host-published-port>` rather than container DNS names or container bridge IP addresses.

Prescribed LAN IP discovery method:

1. Enumerate non-loopback IPv4 addresses assigned to NAS interfaces that are intended for LAN client access.
2. Confirm each candidate address is on an interface where the selected host-published ports are reachable from clients.
3. Use every confirmed address for `avahi-publish-address -a` publication for each hostname.
4. Use `ip route get 1.1.1.1` and default-route interface inspection as discovery inputs, not as a hard limit of one address.
5. If no supported non-loopback IPv4 address can be determined, treat this as an operational failure, log it, and retry with backoff.

Interface selection heuristic:

- Start from non-loopback IPv4 addresses present on interfaces that have usable LAN routes in the routing table.
- Exclude link-local, loopback, and other addresses that are not intended for LAN client access.
- Confirm that each remaining address is valid for the generated reverse proxy backends before advertising it.
- Keep the heuristic configurable in case QTS layout or interface semantics differ across models.

Example:

```bash
ip route get 1.1.1.1
# 1.1.1.1 via 192.168.1.1 dev eth0 src 192.168.1.10
```

Example commands:

```bash
avahi-publish-address grafana.local 192.168.1.10
avahi-publish-address grafana.local 192.168.2.10
avahi-publish-address metrics.local 192.168.1.10
```

mDNS publication notes:

- mDNS allows multiple address records for the same hostname.
- This package may publish more than one IPv4 address for the same hostname when the NAS should be reachable through multiple LAN interfaces.
- Clients may choose any advertised address, so every published address must be valid for the generated reverse proxy backends.
- If an existing mDNS hostname resolves only to the NAS LAN IPv4 addresses selected by this package, it may be treated as compatible and re-announced.
- If an existing primary hostname resolves to any address outside the selected NAS LAN IPv4 address set, and the daemon is not taking ownership through exact-match helper adoption, the daemon must refuse to publish that primary hostname in both mDNS and the generated reverse proxy configuration and notify the user about the collision.
- If an existing alias resolves to any address outside the selected NAS LAN IPv4 address set, skip only that colliding alias, continue publishing the remaining non-conflicting names for the container, and notify the user about the skipped alias.

Automation rules:

- Publish one `avahi-publish-address` process per hostname and IPv4 address pair.
- Use the standard `avahi-publish-address` binary shipped on QNAP rather than making the command configurable.
- Track child PIDs so the daemon can stop or restart the correct publisher.
- Deduplicate the address set so the same hostname and IP pair is never published twice.
- Check for existing mDNS advertisements before publishing.
- If publication fails, keep the reverse proxy route active, de-announce any daemon-owned stale publisher for that hostname and IP pair, and retry announcement.
- Verify publication with `avahi-browse` and repeated `avahi-resolve-host-name <hostname>.local` checks during validation and troubleshooting.
- `avahi-set-host-name` should not be used because the package must publish per-container aliases rather than change the NAS global hostname.

---

## QPKG Layout

```text
qnap-docker-mdns/
├── qpkg.cfg
├── shared/
│   ├── qnap-docker-mdnsd
│   ├── qnap-docker-mdns.sh
│   ├── config.yaml
│   └── config.local.yaml.sample
├── icons/
└── README.md
```

QPKG conventions:

- Follow the standard QPKG package layout with `qpkg.cfg` at the package root.
- Treat `shared/` as the payload that will be installed under the package root on the NAS.
- Install the package under the QNAP-managed QPKG location and resolve the runtime root from the QPKG environment rather than hard-coding a volume path.
- Resolve the installed package path from `/etc/config/qpkg.conf` or the QPKG environment provided by the control script instead of assuming a fixed volume mount.
- Keep daemon binaries, default config, and helper scripts inside the QPKG root so install, upgrade, and uninstall remain self-contained.
- Use the QPKG control script convention for `start`, `stop`, `restart`, and `remove` behavior through the package service script.
- Follow the standard QDK init-script guard that exits early on `start` when the QPKG is disabled.

Installed runtime layout target:

```text
$QPKG_ROOT/
├── qnap-docker-mdnsd
├── qnap-docker-mdns.sh
├── config.yaml
├── config.local.yaml.sample
└── config.local.yaml
```

Configuration layout rules:

- Ship package defaults separately from operator-managed runtime config.
- Keep default settings in `$QPKG_ROOT/config.yaml`.
- Ship a commented sample override file at `$QPKG_ROOT/config.local.yaml.sample` that instructs administrators to copy or rename it to `config.local.yaml`.
- Keep runtime overrides in `$QPKG_ROOT/config.local.yaml` so upgrades can replace packaged assets without overwriting local configuration.
- Create `config.local.yaml` on install only if it does not already exist.
- Load defaults first, then overlay runtime overrides from `config.local.yaml`.

---

## Service Responsibilities

The daemon:

1. Watches Docker.
2. Builds registry.
3. Generates managed reverse proxy rules, expanding aliases into additional hostname-specific JSON entries.
4. Writes managed JSON entries.
5. Runs `scan_config`, validates generated config, and reloads proxy.
6. Publishes mDNS names.
7. Reconciles Avahi publisher processes.
8. Emits deduplicated QNAP notices for failures that affect reconciliation through `notice_log_tool`.
9. Emits notice-level QNAP recovery notices through `notice_log_tool --severity 5` when previously reported problems are resolved.
10. Logs operational failures and retries them with backoff.

---

## Configuration File

Example:

```yaml
config_defaults_file: $QPKG_ROOT/config.yaml
config_runtime_file: $QPKG_ROOT/config.local.yaml
domain_suffix: local

reverse_proxy:
  json_db: /etc/config/reverseproxy/reverseproxy.json
  conf_dir: /etc/reverseproxy/extra
  apache_listen_port: 80
  reload_command: /usr/local/apache/bin/apache_proxy -k graceful -f /etc/apache-sys-proxy.conf
  validate_command: /usr/local/apache/bin/apache_proxy -t -f /etc/apache-sys-proxy.conf
  access_profile_name: local

backups:
  max_backups: 100

docker:
  socket: /var/run/docker.sock

state:
  runtime_dir: /var/run/qnap-docker-mdns
  notice_state_file: /var/run/qnap-docker-mdns/notice-state.json
  lock_file: /var/run/qnap-docker-mdns/daemon.lock
```

---

## Persistent Runtime State

The daemon must persist the minimum runtime state needed to survive restart without losing reconciliation context.

Persisted state must include:

- open failure notices keyed by problem signature
- the metadata needed to emit exactly one matching recovery notice after the problem clears
- active `avahi-publish-address` ownership metadata if needed to clean up orphaned processes safely
- daemon lock ownership metadata such as PID and startup time for diagnostics

Persistence rules:

- Runtime state should live under `/var/run/qnap-docker-mdns` by default.
- The daemon must recreate the runtime directory on start if it is missing.
- Persisted notice state may be cleared on NAS reboot, but must survive daemon restart while the NAS remains up.
- If persisted state is missing or unreadable, the daemon must rebuild what it can from live system state and continue without crashing.
- Persisted state should also record daemon-owned hostname/IP advertisements so stale announcements can be explicitly de-announced before re-announcement.
- The daemon should treat `avahi-publish-address` as a helper process that keeps its advertised record alive while it runs, with Avahi itself maintaining the underlying mDNS service state.

Locking rules:

- Use an advisory exclusive `flock` on the opened lock file at `state.lock_file` for the daemon lifetime as the primary single-instance guard.
- Record diagnostic metadata such as PID and startup time in the lock file after the lock is acquired.
- Treat the held file lock, not the mere existence of the lock-file path, as the source of truth.
- If the daemon crashes, the kernel-released file lock must allow a clean restart without manual lock-file deletion.
- The control script may read the lock-file metadata for operator diagnostics, but it must not treat a stale file on disk as a live lock when no process holds the advisory lock.

Avahi helper ownership rules:

- Primary ownership signal is persisted daemon state for the exact `hostname + IP` advertisement pair.
- If persisted state is missing or incomplete after restart, the daemon may adopt a running `avahi-publish-address` helper only after identifying its advertised `hostname + IP` pair from process-observable state such as the helper command line or equivalent runtime metadata.
- DNS resolution alone is not sufficient proof of which helper owns which advertisement, because resolution shows the visible record set rather than helper process identity.
- If the observed `hostname + IP` pair exactly matches the daemon's current desired publication state, the helper may be adopted and an exact user-generated match becomes service-managed from that point onward.
- Membership in the selected NAS LAN IPv4 address set is a required filter for adoption, but is not by itself proof of ownership.
- The daemon must not adopt or terminate unrelated `avahi-publish-address` helpers that advertise non-desired hostnames, even if they point to NAS LAN IPs.

---

## QNAP Notifications And Logging

The daemon must integrate with QNAP-native notification and logging facilities.

Failures that affect reconciliation must notify the user through QNAP's notice system with `notice_log_tool`.

Operational failures must also be written to syslog through `logger`.

Confirmed severity mapping for `notice_log_tool` on QNAP:

- `3` = Err
- `4` = Warning
- `5` = Notice

Prescribed actionable-misconfiguration notice method:

```bash
notice_log_tool \
  --append "[qnap-docker-mdns] container <name>: multiple HTTP-capable host-published TCP ports found, but qnap-docker-mdns.port is not set" \
  --severity 4 \
  --user admin \
  --serviceName qnap-docker-mdns \
  --facility 13
```

Prescribed operational-failure notice method:

```bash
notice_log_tool \
  --append "[qnap-docker-mdns] reverse proxy reload failed: exit_status=1 command=/usr/local/apache/bin/apache_proxy -k graceful -f /etc/apache-sys-proxy.conf" \
  --severity 3 \
  --user admin \
  --serviceName qnap-docker-mdns \
  --facility 13
```

Prescribed notice-level recovery notice method:

```bash
notice_log_tool \
  --append "[qnap-docker-mdns] container <name>: previously reported problem resolved" \
  --severity 5 \
  --user admin \
  --serviceName qnap-docker-mdns \
  --facility 13
```

Notification rules:

- Use `notice_log_tool` for user-facing notices.
- Use `notice_log_tool --severity 4` for actionable misconfiguration that requires administrator intervention.
- Use `notice_log_tool --severity 3` for operational failures that affect reconciliation and need user visibility.
- Use `notice_log_tool --severity 5` only for recovery notices after a matching failure notice was already emitted.
- Use `--user admin`.
- Use `--serviceName qnap-docker-mdns`.
- Use `--facility 13` for the App facility.
- Deduplicate active-failure notifications per container or failure domain and problem signature.
- Emit a notice-level recovery notice with `notice_log_tool --severity 5` when a previously reported problem is cleared by successful reconciliation.
- Emit one recovery notification for each active problem instance that transitions from open to resolved.
- Stop re-emitting an active-failure notification once that same problem is already open.
- Do not use `log_tool` for the primary notification path in this package.
- Persist open-problem notice state so restart does not cause duplicate failure notices or suppress the matching recovery notice.

Actionable misconfiguration includes:

- `qnap-docker-mdns.enable=true` with multiple candidate ports and no `qnap-docker-mdns.port`
- `qnap-docker-mdns.port` set to a value that does not match a host-published TCP port
- enabled container with no suitable host-published TCP port
- malformed hostname or alias labels
- requested primary hostname collides with an unmanaged reverse proxy `ServerName` or `ServerAlias`
- requested alias collides with an unmanaged reverse proxy `ServerName` or `ServerAlias`
- requested primary hostname collides with an existing mDNS advertisement that resolves outside the selected NAS LAN IPv4 address set

Operational failures that must notify and log include:

- reverse proxy validation, write, or reload failures
- Docker API or Docker watch failures
- LAN IP discovery failures
- Avahi publisher start, stop, or reconciliation failures
- retry exhaustion for any operational failure domain

Prescribed operational logging method:

```bash
logger -t qnap-docker-mdns -p daemon.err "reverse proxy reload failed: exit_status=1 command=/usr/local/apache/bin/apache_proxy -k graceful -f /etc/apache-sys-proxy.conf"
```

Operational logging rules:

- Use `logger` for reverse proxy validation, write, or reload failures.
- Use `logger` for Docker API or Docker watch failures.
- Use `logger` for LAN IP discovery failures.
- Use `logger` for Avahi publisher start, stop, or reconciliation failures.
- Use `logger` for retry scheduling and retry exhaustion events.
- Pair operational failure logs with matching `notice_log_tool` failure notices.
- Emit a notice-level recovery notice with `notice_log_tool --severity 5` when a previously reported operational failure recovers.

Severity guidance:

- `daemon.notice` for transient retry scheduling
- `daemon.warning` for retryable operational failures
- `daemon.err` for unrecoverable failures or exhausted retries

---

## Safety Requirements

The daemon must:

- validate generated config before writing
- write atomically
- rollback on failure
- preserve user configuration
- never overwrite unmanaged sections
- avoid leaving orphaned `avahi-publish-address` processes behind
- avoid spamming duplicate QNAP notices for persistent failures
- log operational failures with enough context to diagnose the failing command, container, and retry state
- emit notice-level recovery notices only after successful recovery from a previously reported problem that already produced a failure notice
- run as a single active daemon instance guarded by a lock file
- use advisory `flock` as the primary single-instance mechanism so stale lock files do not block recovery after crashes
- preserve active reverse proxy routes when only mDNS publication is failing, while continuing to retry de-announcement and re-announcement

---

## Validation

Before marking a reconciliation successful:

1. Generate the desired managed JSON rule set.
2. Write `reverseproxy.json` atomically using embedded ownership markers.
3. Run `scan_config` to regenerate Apache per-port files from JSON.
4. Validate syntax with a QTS-compatible command verified during platform integration.
5. Reload proxy.
6. Start or reconcile Avahi publishers.
7. Verify generated reverse proxy output and mDNS resolution succeed.

Rollback rules:

- Roll back `reverseproxy.json` and regenerate Apache output from the most recent dated backup created for that reconciliation attempt when `scan_config`, config validation, or reverse proxy reload fails.
- If writing `reverseproxy.json` itself fails, leave the active JSON and generated Apache output unchanged and retry.
- If mDNS publication fails after the reverse proxy reload has already succeeded, keep the reverse proxy route active, mark mDNS publication incomplete, and retry de-announcement and re-announcement until publication succeeds or the container becomes ineligible.

Operational failures must be retried with backoff.

Previously reported failures must emit a notice-level recovery notice through `notice_log_tool --severity 5` after successful reconciliation.

Retry rules:

1. Retry only operational failures, not misconfiguration.
2. Use exponential backoff with 20% jitter.
3. Retry immediately once, then back off starting at 5 seconds.
4. Cap the retry delay at 5 minutes.
5. Reset backoff after one successful reconciliation in the affected failure domain.

Failure domains should be tracked independently for:

- Docker watch connection
- config write, validation, and reload
- LAN IP discovery
- Avahi publisher operations

Validation command rules:

- Stage 1 must identify the concrete Apache or reverse proxy validation command available on the target QTS version.
- If no standalone validation command exists, the daemon must validate through the safest available QTS-supported dry-run or temp-file check before replacing the active file.
- The plan must not rely on reload alone as the first syntax check for newly generated config.
- Prefer Apache-native syntax validation against the active reverse proxy binary and config, such as `/usr/local/apache/bin/apache_proxy -t -f /etc/apache-sys-proxy.conf`, when it validates the active QNAP reverse proxy configuration accurately on the target system.
- If `apache_proxy` is a symlink to `apache`, still prefer the `apache_proxy` invocation when it matches the live QNAP reverse proxy process and config path.

---

## Implementation Stages

### Stage 1: Platform Integration

Scope:

- confirm reverse proxy config path and reload behavior on target QTS versions
- confirm reverse proxy access-profile path and built-in `local` profile discovery by `name: "local"` on target QTS versions, plus fallback behavior when it cannot be identified
- confirm `/var/run/docker.sock` access from the QPKG service context
- confirm `notice_log_tool`, `logger`, `avahi-publish-address`, and `ip route` availability on target systems
- confirm Apache-native validation and graceful reload commands on target systems
- confirm whether arbitrary custom fields such as `managed` survive QNAP UI save
  and `scan_config` regeneration unchanged in `reverseproxy.json`
- confirm whether direct `SIGHUP` to the running reverse proxy master behaves
  equivalently to the chosen reload command after regenerated config changes

Deliverables:

- verified reverse proxy reload command default
- verified reverse proxy validation command default
- verified reverse proxy access-profile discovery for built-in `local` rules
- verified reverse proxy regeneration plus reload sequence for JSON changes
- verified QNAP notice and logging command shapes
- verified Docker socket access assumptions
- verified custom JSON ownership-marker fields for managed reverse proxy rules
- short compatibility notes for tested QTS environments

Exit criteria:

- the daemon can reach Docker through `/var/run/docker.sock`
- the selected reload command works on the target NAS
- the selected validation command works on the target NAS
- the built-in `local` access profile can be discovered reliably from
  `access.json` on the target NAS
- if the built-in `local` profile cannot be discovered reliably, fallback to
  `access: 0` is confirmed to behave as expected on the target NAS
- the required JSON change sequence (`scan_config` then reload) is confirmed on
  the target NAS
- the prescribed QNAP notice, recovery-notice, and logging commands are confirmed
- custom JSON ownership-marker fields in `reverseproxy.json` are confirmed to
  survive the target NAS UI and regeneration paths

Task list:

- inspect the target NAS for `/etc/reverseproxy/extra/80.conf` and confirm expected include behavior
- inspect `/etc/config/reverseproxy/access.json` and `/etc/reverseproxy/access/*.conf` and confirm the built-in `local` profile can be discovered by name and mapped to the correct include path
- verify fallback behavior when the built-in `local` profile cannot be identified and `access: 0` must be used
- test Apache-native config validation and graceful reload commands against the active reverse proxy binary and config before relying on QNAP wrapper scripts
- test each candidate reverse proxy reload command and record the working default
- test whether direct `SIGHUP` to the running reverse proxy master behaves equivalently to the chosen reload command after regenerated config changes
- test whether custom ownership-marker fields survive QNAP UI edits and
  regeneration unchanged in `reverseproxy.json`
- verify the QPKG service user can open `/var/run/docker.sock`
- verify `notice_log_tool`, `logger`, `avahi-publish-address`, and `ip route` can be executed from the service environment
- verify warning, error, and notice entries from `notice_log_tool` appear as expected in the QNAP notice UI
- verify which NAS interface addresses should be published for multi-interface systems
- capture any QTS-version-specific deviations that must remain configurable

### Stage 2: State Model And Label Parsing

Scope:

- define backend, hostname, alias, and port-selection data structures
- parse `qnap-docker-mdns.*` labels
- classify container states as valid, actionable misconfiguration, or operational failure
- define durable notice-state and lock-state data structures

Deliverables:

- backend model covering container name, hostnames, selected host-published port, and status
- label parsing and normalization logic
- failure classification rules and notice deduplication keys
- persisted notice-state schema and problem-signature format
- stable normalized alphanumeric ordering and deterministic collision policy

Exit criteria:

- single-port and multi-port selection behavior is deterministic
- invalid label cases are classified consistently

Task list:

- define the internal backend struct fields needed for reconciliation and status tracking
- define parsing rules for `qnap-docker-mdns.enable`, `.port`, `.hostname`, and `.aliases`
- define hostname normalization and alias splitting behavior
- define stable normalized alphanumeric container and name ordering used for deterministic collision handling across managed entries and unmanaged reverse proxy entries
- define validation for malformed labels and unsupported port selections
- define the notice deduplication key format for active failures and their matching recovery notices
- define the persisted runtime-state schema for open problems and lock ownership

### Stage 3: Docker Discovery

Scope:

- perform initial container scan through the Docker API
- collect labels, container names, exposed ports, and host-published TCP bindings
- collect host bind addresses alongside published TCP bindings
- probe candidate ports for HTTP behavior through loopback
- build desired backend state from running containers

Deliverables:

- initial discovery routine using `/var/run/docker.sock`
- filtering for supported loopback-reachable host-published TCP backends only
- HTTP probe routine for candidate-port validation
- actionable misconfiguration detection for unsupported or ambiguous containers

Exit criteria:

- enabled containers with one valid candidate port are discovered correctly
- enabled containers with no valid candidate port or more than one HTTP-capable candidate port are classified correctly
- non-loopback-only and IPv6-only bindings are rejected consistently

Task list:

- connect to Docker through `/var/run/docker.sock`
- list running containers and inspect their labels, names, and published port bindings
- filter published ports down to host-published TCP candidates usable through `localhost`
- reject bindings that are not reachable through NAS loopback
- probe candidate ports with a short HTTP request and keep only ports that return a parseable HTTP response
- select the backend port automatically when probing leaves exactly one HTTP-capable candidate
- require `qnap-docker-mdns.port` only when probing leaves more than one HTTP-capable candidate
- mark containers as actionable misconfiguration when zero HTTP-capable candidates exist, or when more than one HTTP-capable candidate exists without a valid label override
- build the initial desired-state registry from the filtered container set

### Stage 4: Reverse Proxy Rendering

Scope:

- render the managed reverse proxy rule set for `reverseproxy.json`
- render matching JSON entries for `/etc/config/reverseproxy/reverseproxy.json`
- generate `localhost:<host-published-port>` backends only
- discover the built-in QNAP `local` access profile ID from `access.json` for all generated rules
- preserve unmanaged reverse proxy configuration
- detect hostname collisions before rendering
- render non-conflicting aliases as additional hostname-specific managed JSON entries

Deliverables:

- JSON rule renderer for the QNAP config database
- verification logic for the Apache per-port files generated by `scan_config`
- stable output ordering to avoid unnecessary churn
- hostname-collision detection against managed and unmanaged `ServerName` and `ServerAlias` entries
- deterministic alphanumeric managed-name conflict resolution
- human-visible managed labeling for each generated reverse proxy entry
- QNAP JSON rule rendering with `(managed)` suffix in the `name` field and
  embedded `qnap_docker_mdns_managed` / `qnap_docker_mdns_key` ownership markers
- QNAP access-profile rendering that sets `access` to the discovered `local`
  profile ID and verifies the generated include path
- fallback access-profile rendering that uses `access: 0` when the built-in
  `local` profile cannot be identified reliably
- alias expansion from one container definition into multiple managed JSON rules

Exit criteria:

- generated JSON rules contain only daemon-managed changes
- generated JSON rules use sequential IDs following the existing max ID in `reverseproxy.json` for new rules, preserve IDs for existing owned rules matched by embedded ownership key, and include `(managed)` in the `name` field
- rendered backends always target `localhost`
- generated Apache and JSON rules always use the discovered built-in `local` access profile
- generated Apache and JSON rules fall back safely to `access: 0` when the built-in `local` access profile cannot be identified
- conflicting hostnames never produce ambiguous rendered output
- colliding aliases are skipped while non-conflicting aliases still render as additional managed hostname entries
- managed-name collisions resolve deterministically according to the stable normalized alphanumeric ordering
- each generated reverse proxy entry is visibly marked as managed for administrators

Task list:

- design the QNAP JSON rule schema for each managed entry
- implement JSON rule rendering with next-available ID assignment for new owned rules, in-place updates for existing owned rules matched by embedded ownership key, `(managed)` in the `name` field, and custom ownership-marker fields
- discover the numeric access-profile ID for `name: "local"` from `access.json`
- set `access` to the discovered `local` profile ID in every generated JSON rule
- set `access: 0` when the built-in `local` access profile cannot be identified reliably
- emit a human-visible managed label such as `<primary-hostname> (managed)` for each generated reverse proxy entry without changing routed hostnames
- apply the stable normalized alphanumeric winner rule before rendering managed hostnames and aliases
- ensure rendered JSON targets always use `host_name: localhost` and `des_port: <port>`
- scan unmanaged reverse proxy config for `ServerName` and `ServerAlias` collisions before committing a render
- suppress only the specific aliases that collide rather than dropping the whole container mapping when the primary hostname remains valid
- expand each surviving alias into its own managed JSON rule with a distinct stable ownership key
- sort rendered output deterministically so the JSON file does not churn unnecessarily
- compare regenerated managed JSON output against current owned entries to detect no-op reconciliations
- verify the generated Apache per-port file contains the expected `VirtualHost`, backend port, and discovered `local` access-profile include for each managed hostname after `scan_config`

### Stage 5: Safe Config Reconciliation

Scope:

- create dated backups before substantive JSON changes
- synchronize the QNAP JSON config database with matching managed entries
- regenerate Apache per-port files from JSON with `scan_config`
- validate generated Apache config and roll back on failure by restoring the per-attempt dated JSON backup before leaving changed state behind
- validate through a concrete QTS-supported command path
- remove only daemon-managed config on uninstall

Deliverables:

- backup creation logic for `reverseproxy.json`
- JSON database read/write path with embedded ownership markers, next-ID assignment for new rules, and 64-rule limit
- rolling dated backup creation and pruning with configurable `max_backups`
- rollback path that restores the per-attempt dated JSON backup and regenerates Apache output when `scan_config`, validation, or reload fails
- concrete validation command integration
- generated-Apache verification after `scan_config`
- uninstall reconciliation path that removes JSON entries and regenerates the
  per-port files while preserving user-managed config

Exit criteria:

- unmanaged configuration remains intact
- generated per-port files and JSON database are always consistent after reconciliation
- failed `scan_config`, validation errors, or reload failures restore the prior
  good JSON and Apache-generated state
- dated JSON backups are created only for substantive changes and pruned to the configured retention limit
- the 64-rule limit is respected: no more than 64 total entries exist in
  `reverseproxy.json` after any reconciliation
- existing owned JSON rules preserve their assigned IDs across reconciliation by
  matching embedded ownership keys

Task list:

- create dated backup files for `/etc/config/reverseproxy/reverseproxy.json`
- compare rendered JSON against the current file and skip both backup creation and write when no substantive change exists
- prune old dated backups after successful writes according to `backups.max_backups`
- implement JSON database file read, merge, and write with atomic replacement
- implement next-ID assignment by scanning existing JSON entries for the
  maximum `id` value
- enforce the 64-rule limit: count total entries before adding, skip and notify
  if the limit would be exceeded
- remove stale managed entries from JSON only when they are marked with
  `qnap_docker_mdns_managed: true` and no longer have a matching active
  container by stable key
- add or update managed rules in JSON by matching `qnap_docker_mdns_key`,
  creating new entries with `max_id + 1` only when no prior owned entry exists
- treat entries that lack `qnap_docker_mdns_managed: true` as unmanaged and
  skip them without attempting to read `qnap_docker_mdns_key`
- run `/etc/init.d/reverse_proxy.sh scan_config` after JSON writes so QNAP regenerates the per-port files
- validate the generated config before committing it as live traffic using the
  platform-confirmed command
- restore the per-attempt dated backup and rerun `scan_config` if validation or reload fails
- verify the regenerated per-port files contain the expected `VirtualHost` entries and discovered access-profile include
- implement uninstall reconciliation that removes JSON entries and reruns
  `scan_config` while leaving user-edited entries untouched

### Stage 6: Reverse Proxy Reload

Scope:

- execute the configured reload command
- classify failures as operational
- integrate reload failures with retry and rollback behavior
- prefer Apache-native graceful reload through the active reverse proxy binary and config when compatible with the target QTS reverse proxy deployment

Deliverables:

- reload command execution path
- operational logging around reload attempts and failures
- retry hooks for reload-related reconciliation failures
- documented precedence between `apache_proxy`, `apachectl`, and QNAP wrapper reload commands

Exit criteria:

- successful config changes reload the QNAP reverse proxy
- failed reloads are logged and retried with backoff

Task list:

- implement configurable reload command execution
- capture exit status, stderr, and timing for reload attempts
- classify reload failures as operational failures rather than misconfiguration
- connect reload failures to rollback logic when the previous config should be restored
- emit operational logs and schedule retry with backoff when reload fails
- prefer `apache_proxy -k graceful -f /etc/apache-sys-proxy.conf` where validated, with `apachectl` or QNAP wrapper fallback when necessary

### Stage 7: mDNS Publication

Scope:

- discover NAS LAN IP using the prescribed route-based method
- discover the supported NAS LAN IPv4 address set for publication
- manage one `avahi-publish-address` process per hostname or alias and published IPv4 address pair
- reconcile publisher lifecycle on add, change, stop, and restart
- detect and handle conflicting pre-existing mDNS advertisements

Deliverables:

- LAN IP discovery and filtering routine
- Avahi publisher supervisor and PID tracking
- cleanup logic for stale or orphaned publisher processes
- mDNS collision detection against existing LAN advertisements
- helper adoption rules for safe restart reconciliation
- split default/runtime config loading for upgrade-safe local overrides
- helper inspection method based on process-observable `hostname + IP` data

Exit criteria:

- each active hostname resolves to one of the supported NAS LAN IPv4 addresses
- publisher processes are removed when mappings disappear
- conflicting mDNS names that resolve outside the NAS LAN IPv4 set are skipped and notified consistently
- restart reconciliation adopts only matching desired `hostname + IP` helper processes and does not claim unrelated advertisements except exact desired matches, which become service-managed
- helper adoption never relies on DNS resolution alone to infer helper ownership
- a primary hostname blocked by an external mDNS collision is omitted from both mDNS publication and generated reverse proxy configuration unless it becomes eligible for exact-match adoption

Task list:

- implement LAN IPv4 discovery by enumerating supported non-loopback interface addresses
- use `ip route get 1.1.1.1` and default-route inspection as discovery inputs where helpful
- filter the discovered address set down to addresses intended for LAN clients and valid for the reverse proxy backends
- launch one `avahi-publish-address -a` process per published hostname entry and published IPv4 address
- track publisher PIDs and the hostname and IP address each PID owns
- detect existing mDNS advertisements for requested hostnames before publishing
- refuse to publish a primary hostname in both mDNS and generated reverse proxy configuration when any existing advertisement resolves outside the selected NAS LAN IPv4 address set and the daemon is not taking ownership
- skip only the specific aliases whose existing advertisements resolve outside the selected NAS LAN IPv4 address set
- stop publishers when a container stops, is replaced by an ineligible instance, or the hostname set is reduced
- stop or replace publishers when the supported NAS LAN IPv4 address set changes
- de-announce any daemon-owned stale advertisement before retrying publication of the same hostname and IP pair
- on startup, inspect running `avahi-publish-address` helpers through command-line or equivalent runtime metadata to recover their advertised `hostname + IP` pairs
- do not rely on `avahi-resolve-host-name` alone to decide helper ownership, because resolution cannot prove which helper owns a record
- on startup, adopt only running `avahi-publish-address` helpers whose observed exact advertised `hostname + IP` pair matches current desired state and whose IP belongs to the selected NAS LAN IPv4 set
- reconcile orphaned publishers on daemon startup
- validate advertised hostname/address sets with `avahi-browse` and repeated `avahi-resolve-host-name` checks

### Stage 8: Notifications, Logging, And Retry

Scope:

- emit QNAP notice entries for failures that affect reconciliation
- emit QNAP recovery notices with `notice_log_tool --severity 5` when previously reported failures are resolved
- emit syslog entries for operational failures
- retry operational failures with exponential backoff and jitter
- persist open-problem state across daemon restart
- notify the user about blocked publication caused by conflicting existing mDNS advertisements
- notify the user about blocked publication caused by conflicting unmanaged reverse proxy hostnames or aliases

Deliverables:

- `notice_log_tool` failure-notice wrapper
- `notice_log_tool` recovery-notice wrapper using `--severity 5`
- `logger` wrapper for operational failures
- retry scheduler with per-domain backoff state
- notice deduplication, open-problem tracking, and recovery signaling
- persisted notice-state store under the runtime directory
- collision-notice handling for blocked mDNS publication
- collision-notice handling for blocked mDNS publication and unmanaged reverse proxy hostname collisions
- config loader that merges package defaults with runtime-local overrides

Exit criteria:

- repeated failures do not spam notifications
- previously reported failures generate one recovery notice through `notice_log_tool --severity 5` when cleared
- retry state resets after recovery in the affected domain
- daemon restart does not reopen already-notified failures as new notices

Task list:

- wrap `notice_log_tool` for warning and error failure notices
- wrap `notice_log_tool` for notice-level recovery notices using `--severity 5`
- wrap `logger` for operational failures with severity mapping to `daemon.notice`, `daemon.warning`, and `daemon.err`
- implement per-container or per-domain per-problem notification deduplication
- track open problems so each one can emit exactly one matching recovery notice
- persist and reload open-problem state under `/var/run/qnap-docker-mdns`
- implement per-domain retry state with exponential backoff and jitter
- reset retry state after one successful reconciliation in the affected domain
- emit a user-visible notice when a hostname is blocked by an existing non-NAS mDNS advertisement
- emit a user-visible notice when a primary hostname is blocked by an unmanaged reverse proxy `ServerName` or `ServerAlias`, and when an alias is skipped for that reason

### Stage 9: Event Loop

Scope:

- subscribe to Docker lifecycle events
- debounce rapid event bursts
- merge event-driven reconciliation with scheduled retry work
- enforce single-instance execution and serialized reconciliation
- detect eligibility changes caused by container lifecycle changes, replacement, rename, or published-port changes observed during reconciliation

Deliverables:

- Docker event subscriber
- debounce mechanism
- reconciliation coordinator for normal and retry-triggered runs
- lock-file based single-instance guard
- periodic full-rescan safety net for stale-state cleanup

Exit criteria:

- start, stop, die, destroy, and rename events trigger correct reconciliation
- container stop, destroy, rename, or replacement with an ineligible instance removes stale reverse proxy and mDNS state on reconciliation
- transient failures recover without requiring daemon restart
- a second daemon instance cannot start while the first one holds the lock

Task list:

- subscribe to Docker lifecycle events for start, stop, die, destroy, and rename
- debounce event bursts so rapid container churn triggers one reconciliation pass
- merge event-driven reconciliation and scheduled retry work into a single coordinator
- ensure overlapping reconciliation requests collapse safely into the latest desired state
- re-inspect container labels and published-port bindings during reconciliation rather than trusting event payloads alone
- remove reverse proxy entries and Avahi publishers when a container stops,
  is destroyed, is renamed out of eligibility, or is replaced by an ineligible
  instance
- add a periodic full rescan to catch missed stop/remove/rename events and
  container replacement that changed eligibility
- clear resolved failure notification state only after sending the matching recovery notice
- acquire and hold an exclusive lock for the daemon lifetime
- make the QPKG control script refuse duplicate starts when the lock is already held

### Stage 10: QPKG Packaging

Scope:

- package daemon, service script, config, templates, and metadata
- define install, start, stop, and uninstall behavior
- define upgrade behavior
- ensure cleanup of managed config and publisher processes
- package native daemon binaries for supported QNAP CPU architectures
- package runtime-directory initialization and lock handling
- ship defaults separately from runtime-local config

Deliverables:

- `qpkg.cfg`
- `shared/qnap-docker-mdnsd`
- `shared/qnap-docker-mdns.sh`
- default config and template files
- separate default and runtime-local config files
- uninstall cleanup behavior
- upgrade behavior that stops the daemon, replaces the binary, and restarts into startup reconciliation without removing managed proxy config
- architecture-specific Go build artifacts and packaging notes
- documented cross-compilation workflow for building Linux QNAP binaries from a
  `darwin/arm64` development machine
- documented package-tree and installed-runtime layout that follows QPKG conventions
- runtime-directory and advisory-lock handling notes
- operator documentation covering the QNAP reverse proxy UI refresh quirk after
  on-disk JSON updates
- documented QDK packaging workflow covering `qbuild`, config registration, and
  a faster unpacked `qinstall.sh` iteration loop
- documented current QDK acquisition path and any QNAP developer-account
  prerequisites needed to obtain the SDK/tooling
- documented `Makefile` targets for common local build, package, and test flows

Exit criteria:

- package installs and starts cleanly on QNAP
- uninstall removes managed artifacts and stops daemon-managed publishers
- upgrade preserves the managed reverse proxy config and re-announces mDNS entries after restart reconciliation
- upgrade preserves operator-managed runtime config while allowing packaged defaults and binaries to be replaced
- shipped binaries run on the intended QNAP architecture without requiring Go on the NAS
- duplicate service starts are prevented cleanly and stale lock files do not block restart after a crash

Task list:

- create `qpkg.cfg` with the package metadata and service hooks
- add `shared/qnap-docker-mdnsd` and `shared/qnap-docker-mdns.sh`
- add the default `config.yaml` and commented `config.local.yaml.sample`
- make `shared/qnap-docker-mdns.sh` the QPKG control script entrypoint for `start`, `stop`, `restart`, and uninstall cleanup flows
- resolve runtime paths from the QPKG environment such as `QPKG_ROOT`, and verify the installed path can also be derived from `/etc/config/qpkg.conf`
- follow the standard QDK disabled-package guard so `start` exits cleanly when the QPKG is disabled
- implement install-time setup for config, permissions, and service registration
- create `config.local.yaml` from the sample only when it does not already exist and leave an existing runtime config untouched
- decide whether `config.local.yaml` should be registered through `QPKG_CONFIG`, `qbuild --force-config`, or `add_qpkg_config` in `pkg_init()` so upgrades preserve operator-managed runtime config without spurious overwrite prompts
- create the runtime directory and lock-file path with the expected ownership and permissions
- create runtime-only directories during install so upgrades do not overwrite their contents
- implement stop and uninstall cleanup for daemon-managed JSON entries, regenerated Apache output, and Avahi publisher processes
- implement advisory `flock` locking in the daemon and make the control script report lock-holder metadata for duplicate-start diagnostics
- implement upgrade behavior as stop, replace binary and packaged default assets and `config.local.yaml.sample`, preserve `config.local.yaml`, then start so the daemon re-announces mDNS entries during startup reconciliation without removing the existing managed reverse proxy config first
- make uninstall drive a final reconciliation that removes matching JSON entries from `reverseproxy.json`, reruns `scan_config`, and stops daemon-owned mDNS advertisements
- use QDK pre-build or post-build hooks when architecture-specific package contents such as per-arch binaries require `QDK_EXTRA_FILE` changes
- document how to obtain the current QDK/SDK from QNAP Developer Center, including any required QID sign-in or developer-partner application flow if direct public downloads are not exposed
- add a repo-level `Makefile` that wraps common developer workflows such as local builds, cross-builds for QNAP targets, QPKG assembly, and validation commands
- build Go binaries for the supported QNAP targets such as `linux/amd64`, `linux/arm64`, or `linux/arm` as needed
- document the default x86 QNAP build command for developers working on Apple
  Silicon Macs: `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o dist/qnap-docker-mdnsd-amd64 ./cmd/qnap-docker-mdnsd`
- treat cross-compilation from `darwin/arm64` to `linux/amd64` as the default
  development workflow for x86_64 QNAP targets unless a dependency forces CGO
- prefer static builds with `CGO_ENABLED=0` unless a required dependency forces CGO
- if a dependency forces CGO, move the build to a Linux `amd64` builder or CI
  job rather than depending on QNAP-native compilation
- document a faster package-development loop that unpacks a built QPKG and re-runs `sh qinstall.sh` while iterating on package install logic instead of rebuilding the archive for every `package_routines` change
- use `qbuild --exclude` or equivalent packaging filters so development metadata does not leak into the QPKG payload
- verify the packaged binary matches the NAS CPU architecture and ABI expectations
- verify the package lifecycle from install to upgrade to uninstall on the target NAS
- document that the QNAP reverse proxy UI may require clicking away from
  `Network Access` in the left sidebar and then back again before managed JSON
  changes become visible in the interface

### Stage 11: End-To-End Validation

Scope:

- verify all acceptance criteria from installation through uninstall
- validate both success paths and failure-handling behavior

Deliverables:

- validation checklist mapped to the acceptance criteria
- test notes for single-port, multi-port, unsupported, and failure-recovery cases

Exit criteria:

- acceptance criteria are satisfied on the target NAS
- operational retry, notice deduplication, recovery notice, and cleanup behavior are confirmed

Task list:

- test a container with one host-published TCP port and verify automatic backend selection
- test a single published port that does not return a valid HTTP response and verify it is not published
- test a container with multiple host-published TCP ports where only one returns a parseable HTTP response and verify automatic backend selection
- test a container with multiple host-published TCP ports where more than one returns a parseable HTTP response and verify the label requirement
- test a container with no suitable host-published TCP port and verify actionable misconfiguration handling
- test a container with valid non-colliding aliases and verify each alias becomes its own managed reverse proxy entry and publishes through mDNS successfully
- test a container published only on a non-loopback host IP and verify it is rejected as unsupported
- test duplicate hostname and alias claims and verify no conflicting mapping is published
- test alias collisions and verify only the colliding aliases are skipped while valid aliases still route and publish
- test existing primary mDNS collision on a non-NAS IP and verify both mDNS publication and reverse proxy publication are refused with a user-visible notice unless exact-match ownership is adopted
- test reverse proxy config generation, `scan_config`, validation, and reload success paths
- test JSON write path: verify a managed entry appears in `/etc/config/reverseproxy/reverseproxy.json` with `(managed)` in the `name` field, correct `server_name`, `host_name: localhost`, discovered `local` profile `access` id, `des_port`, `port: 80`, `qnap_docker_mdns_managed: true`, and the expected `qnap_docker_mdns_key`
- test Apache render path: verify each generated managed `VirtualHost` includes `/etc/reverseproxy/access/<discovered-local-access-id>.conf`
- test alias expansion: verify a container with two valid aliases produces three managed JSON entries with distinct stable ownership keys and three corresponding generated `VirtualHost` entries
- test next-ID assignment: verify the JSON entry's `id` is `max(existing_ids) + 1`
- test generated Apache + JSON consistency: after reconciliation, verify the generated `80.conf` entry and the JSON entry have matching hostnames, ports, and discovered `local` access profile
- test `scan_config` regeneration: trigger QNAP's `scan_config`, verify `80.conf` is regenerated from JSON, then trigger daemon reconciliation and verify the managed JSON entries still produce the expected `VirtualHost` output
- test dated backup retention: make repeated substantive JSON changes, verify dated backup files are created, and verify the oldest backups are pruned once `max_backups` is exceeded
- test 64-rule limit: fill `reverseproxy.json` with 64 entries, add another managed container, verify a user-visible notice is emitted and the rule is not added
- test mDNS publication failure after successful reverse proxy reload and verify the route stays active while re-announcement retries continue
- test Avahi publication, resolution, cleanup, and restart reconciliation
- test upgrade behavior and verify existing reverse proxy config remains in place while mDNS announcements are re-announced after daemon restart
- test container replacement, stop, destroy, and rename cases that make a container ineligible and verify reverse proxy and mDNS cleanup occurs
- test notice deduplication for repeated failures
- test recovery notices using `notice_log_tool --severity 5` for both misconfiguration and operational recovery paths
- test daemon restart during an open problem and verify deduplicated notice and recovery behavior remain correct
- test duplicate daemon start attempts and verify the second start is refused cleanly
- test crash recovery and verify a stale lock-file path without a held `flock` does not block restart
- test operational failure retry behavior for Docker, reload, LAN IP discovery, and Avahi domains
- document validation results against the acceptance criteria

### Recommended Build Order

1. Stage 1: Platform Integration
2. Stage 2: State Model And Label Parsing
3. Stage 3: Docker Discovery
4. Stage 4: Reverse Proxy Rendering
5. Stage 5: Safe Config Reconciliation
6. Stage 6: Reverse Proxy Reload
7. Stage 7: mDNS Publication
8. Stage 8: Notifications, Logging, And Retry
9. Stage 9: Event Loop
10. Stage 10: QPKG Packaging
11. Stage 11: End-To-End Validation

---

## Future Enhancements

- HTTPS VirtualHosts
- Let's Encrypt integration
- Wildcard DNS support
- Web UI
- Container Station integration
- Per-container authentication
- Per-container TLS settings
- Metrics endpoint

## Implementation

Implementation should be organized as small, testable packages around:

- Docker discovery and label parsing
- desired managed-rule expansion from containers to hostname-specific JSON entries
- reverse-proxy JSON rendering, merge, backup, and rollback
- reload and validation command execution
- mDNS publisher supervision
- notification, retry, and persisted runtime state

The implementation should keep pure decision logic separate from QNAP side-effect adapters where practical so unit tests can cover the reconciliation rules without requiring a live NAS.

### Unit Tests

Unit tests should cover at minimum:

- label parsing, hostname normalization, and alias splitting
- multi-port HTTP probing and port-selection decisions
- deterministic collision resolution across primary hostnames and aliases
- expansion of one container into one primary managed rule plus one rule per surviving alias
- managed JSON ownership-marker handling, including entries that lack managed markers
- next-ID allocation, 64-rule-limit enforcement, and no-op detection
- dated backup creation decisions, retention pruning, and rollback-source selection
- reconciliation diffing between desired managed rules and existing `reverseproxy.json`
- retry scheduling, notice deduplication, and recovery-notice transitions
- Avahi helper adoption and ownership filtering based on observable `hostname + IP` metadata

Unit tests should run in normal local development without a NAS. QNAP-specific command behavior, `scan_config`, and end-to-end reverse proxy generation should remain covered by Stage 1 and Stage 11 validation on target hardware.

---

## Acceptance Criteria

1. Install QPKG.
2. Enable daemon.
3. Launch container with exactly one host-published TCP port:

```yaml
labels:
  qnap-docker-mdns.enable: "true"
```

If probing multiple host-published TCP ports finds more than one HTTP-capable endpoint, add:

```yaml
labels:
  qnap-docker-mdns.port: "3000"
```

4. Configuration automatically generated for the selected backend port.
5. Generated reverse proxy backend targets `localhost:<host-published-port>`.
6. QNAP-generated `VirtualHost` entries are written to `/etc/reverseproxy/extra/80.conf` from daemon-managed JSON entries after `scan_config` runs.
7. Each generated managed `VirtualHost` includes `/etc/reverseproxy/access/<discovered-local-access-id>.conf` so the built-in `local` access profile is always applied.
8. A matching JSON entry exists in `/etc/config/reverseproxy/reverseproxy.json` with `"name": "<primary-hostname> (managed)"` so the QNAP UI Rule Name column shows the managed label.
9. The matching JSON entry sets `"access": <discovered-local-access-id>` so QNAP's generated config also uses the built-in `local` access profile.
10. A matching JSON entry stores `"qnap_docker_mdns_managed": true` and the expected stable `"qnap_docker_mdns_key"` so the daemon can identify owned rules without relying on `/var/run` state.
11. A newly created managed JSON entry uses the next available sequential ID after the maximum existing ID in the file, while an existing owned managed rule preserves its previously assigned ID across reconciliation.
12. QNAP reverse proxy reloaded.
13. `avahi-publish-address -a` is started for each published hostname and supported NAS LAN IPv4 address.
14. `http://<container>.local` works.
15. A valid non-colliding alias routes successfully through its own generated managed `VirtualHost` entry.
16. `avahi-browse` shows the expected published hostname/address advertisements.
17. Repeated `avahi-resolve-host-name <container>.local` checks resolve to advertised NAS LAN IPv4 addresses.
18. Repeated `avahi-resolve-host-name <alias>.local` checks resolve to advertised NAS LAN IPv4 addresses for a valid non-colliding alias.
19. Stopping a managed container removes the matching JSON entry, regenerates the derived `VirtualHost`, and stops the matching Avahi publisher.
20. Stopping, destroying, renaming, or replacing a managed container with an ineligible container instance removes the matching JSON entry, regenerates the derived `VirtualHost`, and stops the matching Avahi publishers.
21. A single published port that does not return a parseable HTTP response to the configured probe is not treated as a candidate port.
22. A container with multiple host-published TCP ports but exactly one parseable HTTP endpoint is published automatically using that endpoint.
23. A container with more than one parseable HTTP-capable candidate port is ignored until `qnap-docker-mdns.port` is set.
24. An enabled container with no suitable loopback-reachable host-published TCP port is ignored.
25. A container published only on a non-loopback host IP is treated as unsupported and is not proxied.
26. Managed hostname and alias collisions resolve deterministically by stable normalized alphanumeric ordering, with later conflicting names skipped.
27. An alias that collides with another managed hostname or alias is skipped while the remaining non-conflicting hostnames and aliases continue to publish.
28. A primary hostname that collides with an unmanaged reverse proxy `ServerName` or `ServerAlias` is not published.
29. An alias that collides with an unmanaged reverse proxy `ServerName` or `ServerAlias` is skipped while the remaining non-conflicting hostnames and aliases continue to publish.
30. A primary hostname that already resolves through mDNS to an address outside the selected NAS LAN IPv4 address set is not published in either mDNS or the generated reverse proxy configuration unless exact-match ownership is adopted, and it produces a user-visible notice.
31. An alias that already resolves through mDNS to an address outside the selected NAS LAN IPv4 address set is skipped while the remaining non-conflicting hostnames and aliases continue to publish, and the skipped alias produces a user-visible notice.
32. After an external `scan_config`, the daemon's next reconciliation confirms the managed JSON entries still produce the expected `VirtualHost` entries without duplicate ServerName mappings.
33. The generated Apache output and the JSON entries are always consistent after reconciliation (same hostnames, same ports, same discovered `local` access profile).
34. Adding a managed rule when `reverseproxy.json` already has 64 entries produces a user-visible notice and the rule is not added.
35. Each substantive change to `reverseproxy.json` creates one dated backup of the prior file contents, and backup retention is capped by `max_backups`.
36. A primary hostname blocked by an unmanaged reverse proxy `ServerName` or `ServerAlias`, and an alias skipped for that reason, each produce a visible QNAP notice through `notice_log_tool --severity 4`.
37. An actionable misconfiguration produces one visible QNAP notice through `notice_log_tool --severity 4`.
38. An operational failure produces both a visible QNAP notice through `notice_log_tool --severity 3` and a syslog entry through `logger`.
39. Repeated reconciliation while the same failure persists does not spam duplicate notifications.
40. Clearing a previously reported failure produces one notice-level QNAP recovery notice through `notice_log_tool --severity 5`.
41. Daemon restart during an open problem does not re-emit the same failure as a new notice and still allows the matching recovery notice later.
42. Operational failures are retried with backoff and stop retrying after recovery.
43. mDNS publication failure after a successful reverse proxy reload leaves the HTTP route active while de-announcement and re-announcement retries continue.
44. A duplicate daemon start is refused while the active daemon instance holds the advisory lock.
45. Startup reconciliation may adopt an exact-match user-generated `avahi-publish-address` helper whose advertised `hostname + IP` pair matches current desired state only after recovering that pair from process-observable helper metadata; once adopted, that exact-match advertisement becomes service-managed.
46. Stale lock-file paths without a held advisory lock do not block daemon restart after a crash.
47. Upgrade stops the daemon, replaces the packaged binary, default assets, and `config.local.yaml.sample`, preserves `config.local.yaml`, and restarts without removing the existing managed reverse proxy config first; startup reconciliation re-announces mDNS entries.
48. Uninstall removes only daemon-managed reverse proxy and mDNS state (JSON entries + regenerated Apache output + Avahi publishers) and leaves user-managed entries intact.

---

## Recommended Implementation Language

Go

Reasons:

- Static binaries
- Native Docker SDK
- Easy mDNS support
- Small memory footprint
- Easy deployment on QNAP

QNAP notes:

- QNAP does not need to ship Go at runtime; the QPKG should ship prebuilt native binaries.
- Build for the target NAS architecture, typically `linux/amd64` for x86_64 models and `linux/arm64` or `linux/arm` for ARM models.
- For x86_64 NAS targets, a developer working on a `darwin/arm64` Mac should
  cross-compile the daemon with `GOOS=linux GOARCH=amd64` rather than building
  on the NAS.
- Prefer static builds with `CGO_ENABLED=0` unless a required dependency forces CGO.
- Recommended default command for an x86_64 QNAP build:

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o dist/qnap-docker-mdnsd-amd64 ./cmd/qnap-docker-mdnsd
```

- Pure-Go builds should work directly from Apple Silicon macOS because Go can
  cross-compile from `darwin/arm64` to `linux/amd64` without matching the host
  CPU architecture.
- If a future dependency requires CGO, build the Linux `amd64` artifact in a
  Linux `amd64` container, VM, or CI runner instead of assuming cross-CGO will
  be reliable from macOS.
- Treat Go support on QNAP as native-binary compatibility, not as an installed platform runtime.
- Do not rely on the system `Python 2.7` interpreter for the daemon implementation.
- Expect to build the QPKG with QDK tooling on the development machine or CI, then
  use an unpacked `qinstall.sh` loop for faster iteration on package-install logic.
