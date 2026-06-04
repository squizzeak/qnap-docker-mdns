# qnap-docker-mdns

A daemon for QNAP NAS devices that automatically discovers Docker containers and exposes them through the QNAP reverse proxy with mDNS (`.local` hostnames), so you can reach every container at `<name>.local` from any device on your LAN.

## What it does

- **Watches** Docker for container start/stop events.
- **Probes** each enabled container's ports to find a working HTTP endpoint.
- **Adds** a reverse-proxy entry to QNAP's built-in reverse proxy (the same engine that powers the Web GUI).
- **Publishes** an mDNS hostname via Avahi so the container is reachable at `<container>.local`.
- **Keeps things tidy**: removes entries when containers stop, creates backups before every change, emits QNAP system notices for problems.

## How it works

```
Docker event → label inspection → HTTP port probe
                                      │
                                      ▼
                         QNAP reverse proxy JSON
                                      │
                         scan_config (regenerates Apache config)
                                      │
                         validate → reload (graceful restart)
                                      │
                         mDNS publish (avahi-publish-address)
```

The daemon reconciles current Docker state against the reverse-proxy configuration on a loop: every 500 ms when a container changes, plus a full rescan every 5 minutes.  Built-in collision detection prevents conflicts with manually-configured reverse-proxy entries and other mDNS publishers.

## Docker labels

Enable a container by adding labels to its Docker configuration:

| Label                          | Required | Description                                                                          |
| ------------------------------ | -------- | ------------------------------------------------------------------------------------ |
| `qnap-docker-mdns.enable`      | **Yes**  | Set to `"true"` to enable this container                                             |
| `qnap-docker-mdns.hostname`    | No       | Custom hostname (e.g. `"my-app"`).  Defaults to `<container-name>.local`.            |
| `qnap-docker-mdns.port`        | No       | Explicit container port.  Required only when the container exposes multiple HTTP ports. |
| `qnap-docker-mdns.aliases`     | No       | Comma-separated extra hostnames (e.g. `"web,stats"` → `web.local`, `stats.local`).   |

### Example (Docker Compose)

```yaml
services:
  my-app:
    image: my-app:latest
    ports:
      - "8080:80"
    labels:
      qnap-docker-mdns.enable: "true"
      qnap-docker-mdns.hostname: "my-app"
      qnap-docker-mdns.aliases: "api,admin"
```

This would make the container reachable at `my-app.local`, `api.local`, and `admin.local`.

### Port detection

The daemon probes every loopback-bound TCP port the container exposes (except 443/tcp).  Ports that respond with a valid HTTP response are considered candidates.  If exactly one candidate is found it is selected automatically.  If multiple HTTP ports are available you must add the `qnap-docker-mdns.port` label to disambiguate — otherwise the daemon will emit a misconfiguration notice.

### Access control

All generated reverse-proxy entries use the `local` access profile (LAN-only by default).  The daemon reads `/etc/config/reverseproxy/access.json` at startup to discover the profile ID.

## Installation

### Prerequisites (on your QNAP NAS)

1. Open **App Center** → **Settings** (gear icon) → **General**.
2. Enable **"Allow installation of applications without a valid digital signature"**.
3. Click **Apply**.

### Build & install from your Mac/Linux machine

```bash
# Clone the repository
git clone https://github.com/squizzeak/qnap-docker-mdns.git
cd qnap-docker-mdns

# Build the QPKG in a container, upload, install, and enable — all in one step
make install NAS_HOST=qnap.local
```

`make install` does four things:
1. Cross-compiles the Go binary for Linux/amd64.
2. Builds the `.qpkg` package inside a Podman/Docker container (Ubuntu + QDK).
3. Uploads the `.qpkg` to your NAS via `scp`.
4. Runs `qpkg_cli -m` (install) followed by `qpkg_cli --enable`.

Override the NAS address if needed: `make install NAS_HOST=192.168.1.100`.

### Requirements on your build machine

- **Go** 1.25+ (for cross-compilation).
- **Podman** or **Docker** (for the QDK build container).
- If you're on Apple Silicon: QEMU binfmt_misc must be enabled for amd64 emulation.  Test it with:
  ```bash
  podman machine ssh -- sudo podman run --rm --arch amd64 alpine echo ok
  ```

### Individual Makefile targets

| Target            | Description                                                       |
| ----------------- | ----------------------------------------------------------------- |
| `container-image` | Build the QDK + Go builder image.                                 |
| `container-build` | Cross-compile + build the `.qpkg`.                                |
| `container-sign`  | Build + add code signing signature (needs `QNAP_CODESIGNING_TOKEN`). |
| `container-shell` | Interactive shell inside the builder container.                   |
| `install`         | Build, upload, install, and enable on the NAS.                    |
| `cross-build`     | Cross-compile the Go binary only.                                 |
| `build`           | Build for the host platform (for local development).              |
| `test`            | Run the test suite.                                               |
| `lint`            | Run `go vet`.                                                     |
| `help`            | Show all targets.                                                 |

For a full list run `make help`.

### Manual install on the NAS

If you prefer to install manually:

```bash
# Build the QPKG first
make container-build

# Copy to the NAS
scp build/qnap-docker-mdns_1.0.0.qpkg admin@qnap.local:/tmp/

# On the NAS (via SSH)
ssh admin@qnap.local
qpkg_cli -m /tmp/qnap-docker-mdns_1.0.0.qpkg -A
qpkg_cli --enable qnap-docker-mdns
```

## Configuration

The daemon reads `config.yaml` for defaults and optionally overrides it with `config.local.yaml` (both in the QPKG install directory).  You can customise any setting:

```yaml
# config.local.yaml — only include settings you want to change
domain_suffix: local              # default domain for hostnames

reconcile:
  debounce: 500ms                 # wait after a container event before reconciling
  full_rescan_interval: 5m        # full rescan frequency

retry:
  immediate_retries: 1            # immediate retries after a failure
  initial_backoff: 5s
  max_backoff: 5m
  jitter_percent: 20

probe_timeout: 2s                 # HTTP probe timeout

backups:
  max_backups: 100                # max reverse-proxy JSON backups to retain

reverse_proxy:
  json_db: /etc/config/reverseproxy/reverseproxy.json
  conf_dir: /etc/reverseproxy/extra
  access_profile_name: local
```

Configuration files are layered: `config.yaml` provides defaults, `config.local.yaml` provides user overrides.  The local file is preserved across QPKG upgrades.

### Reload notes

After the daemon writes new reverse-proxy entries the QNAP Web GUI does **not** automatically refresh.  To see the changes in App Center → Network Access, click away from the section in the left sidebar and then click back.

## Using it

After installation, enable any Docker container by adding `qnap-docker-mdns.enable=true` as a label:

```bash
docker update --label-add qnap-docker-mdns.enable=true my-container
```

The daemon detects the change within 500 ms, probes the container's ports, and publishes the hostname.  Visit `http://<container-name>.local` from any device on your LAN.

### Verifying it works

```bash
# On the NAS — check the daemon is running
ssh admin@qnap.local qpkg_cli -s qnap-docker-mdns --output 2

# Check reverse-proxy entries
ssh admin@qnap.local cat /etc/config/reverseproxy/reverseproxy.json

# Resolve the mDNS hostname (from any machine with Avahi/Bonjour)
avahi-resolve-host-name tautulli.local
```

### Lifecycle

```bash
# Stop the daemon
ssh admin@qnap.local /etc/init.d/qnap-docker-mdns.sh stop

# Start it again
ssh admin@qnap.local /etc/init.d/qnap-docker-mdns.sh start

# Restart
ssh admin@qnap.local /etc/init.d/qnap-docker-mdns.sh restart

# Uninstall
ssh admin@qnap.local qpkg_cli -R qnap-docker-mdns
```

The daemon cleans up its mDNS publishers and reverse-proxy entries on shutdown.  The `remove` handler in the control script also backs up and strips managed entries from the JSON database.

## Troubleshooting

**"Allow unsigned applications" is disabled**

The `qpkg_cli -m` command will fail with exit code -1.  Enable the setting in App Center → Settings → General.

**Container appears but no hostname is published**

- The container must expose at least one loopback-bound TCP port (other than 443).
- If the container exposes multiple HTTP ports, add the `qnap-docker-mdns.port` label.
- The daemon runs an HTTP probe — the container must be serving HTTP within the probe timeout (2 s by default).

**Hostname resolves but the page doesn't load**

The QNAP reverse proxy listens on port 80 (HTTP) only.  Your container's internal port is proxied — make sure the container is actually listening inside.

**"64-rule limit" notice**

QNAP's reverse proxy engine has a practical limit of 64 rules.  If you exceed this, the daemon will not add new entries and will emit a warning notice.

**Changes not visible in the Web GUI**

Click away from "Network Access" in the left sidebar and then click back — the GUI does not auto-refresh after external JSON changes.

**mDNS `Publish("service-name", 0x10) failed: Local name collision`**

This is normal on QNAP devices where two Avahi daemons are running.  The daemon uses the `-R` flag (no reverse lookups) to avoid collisions with the second daemon's PTR records.

## Code signing

QTS 5.x enforces code signing for App Center packages.  Without a valid signature the QPKG installs only when the "Allow unsigned" setting is enabled.

To produce a signed QPKG you need a `QNAP_CODESIGNING_TOKEN` from the [QNAP Developer Partner](https://www.qnap.com/) portal:

```bash
export QNAP_CODESIGNING_TOKEN=...
make container-sign
```

The `qbuild --add-code-signing` step contacts QNAP's signing server to embed a CMS signature that QTS 5.x trusts.
