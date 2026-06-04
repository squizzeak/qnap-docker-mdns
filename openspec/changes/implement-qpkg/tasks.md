## 1. Project Setup And Go Module

- [x] 1.1 Initialize Go module at `qnap-docker-mdns/`
- [x] 1.2 Add dependencies: `github.com/docker/docker`, `gopkg.in/yaml.v3`
- [x] 1.3 Create directory structure: `cmd/qnap-docker-mdnsd/`, `internal/` sub-packages
- [x] 1.4 Add `Makefile` with targets for build, cross-compile, test, lint
- [x] 1.5 Add `.gitignore` for Go binaries, dist/, qpkg artifacts

## 2. Configuration Loading

- [x] 2.1 Define `Config` struct with all YAML fields from the design
- [x] 2.2 Implement default config at `$QPKG_ROOT/config.yaml`
- [x] 2.3 Implement runtime override loading from `config.local.yaml`
- [x] 2.4 Implement config merge (defaults + overrides)
- [x] 2.5 Write unit tests for config parsing and merge

## 3. Docker Discovery

- [x] 3.1 Implement Docker socket connection via `/var/run/docker.sock`
- [x] 3.2 Implement container listing and inspection
- [x] 3.3 Extract container labels, names, and published port bindings
- [x] 3.4 Filter port bindings to loopback-reachable TCP ports only
- [x] 3.5 Implement HTTP probe routine for candidate port validation
- [x] 3.6 Implement port auto-selection logic (single candidate, multiple candidates, none)
- [x] 3.7 Implement label parsing for `qnap-docker-mdns.*` labels
- [x] 3.8 Implement hostname normalization and alias splitting
- [x] 3.9 Write unit tests for label parsing, port selection, and probe logic

## 4. State Model And Container Classification

- [x] 4.1 Define backend data structures (ContainerName, Hostnames, Port, Status)
- [x] 4.2 Implement container classification: valid, actionable misconfiguration, operational failure
- [x] 4.3 Implement stable normalized alphanumeric ordering for containers
- [x] 4.4 Implement desired-state registry building from discovered containers
- [x] 4.5 Write unit tests for classification and ordering

## 5. Reverse Proxy Rendering

- [x] 5.1 Implement JSON rule rendering with QNAP `reverseproxy.json` format
- [x] 5.2 Implement access profile discovery from `access.json` by `name: "local"`
- [x] 5.3 Implement `access: 0` fallback when local access profile cannot be identified
- [x] 5.4 Implement next-ID assignment from existing entries (`max_id + 1`)
- [x] 5.5 Implement owned entry update by matching `qnap_docker_mdns_key`
- [x] 5.6 Implement alias expansion: one container → multiple JSON entries
- [x] 5.7 Implement managed entry marking with `qnap_docker_mdns_managed` and `qnap_docker_mdns_key`
- [x] 5.8 Implement hostname collision detection against managed and unmanaged entries
- [x] 5.9 Implement deterministic collision resolution with stable ordering
- [x] 5.10 Implement 64-rule limit enforcement
- [x] 5.11 Implement stable output ordering to avoid unnecessary churn
- [x] 5.12 Write unit tests for all rendering, collision, and expansion logic

## 6. Config Reconciliation

- [x] 6.1 Implement JSON file read/write with atomic replacement
- [x] 6.2 Implement dated backup creation before substantive changes
- [x] 6.3 Implement backup retention and pruning (`max_backups`)
- [x] 6.4 Implement managed/unmanaged entry merge (preserve unmanaged)
- [x] 6.5 Implement stale managed entry removal by ownership key
- [x] 6.6 Implement `scan_config` execution after JSON writes
- [x] 6.7 Implement Apache config validation command execution
- [x] 6.8 Implement reverse proxy reload command execution
- [x] 6.9 Implement rollback: restore dated backup and re-run `scan_config` on failure
- [x] 6.10 Implement generated Apache output verification after `scan_config`
- [x] 6.11 Implement uninstall reconciliation (remove managed entries, run scan_config)
- [x] 6.12 Implement no-op detection (skip backup/write when content unchanged)
- [x] 6.13 Write unit tests for merge, backup, rollback, and no-op logic

## 7. Reverse Proxy Reload

- [x] 7.1 Implement configurable reload and validate command execution
- [x] 7.2 Capture exit status, stderr, and timing for reload attempts
- [x] 7.3 Classify reload failures as operational failures
- [x] 7.4 Connect reload failures to rollback logic
- [x] 7.5 Write unit tests for command execution wrapper

## 8. mDNS Publication

- [x] 8.1 Implement NAS LAN IPv4 discovery (enumerate non-loopback addresses)
- [x] 8.2 Implement address filtering for LAN-client-usable addresses
- [x] 8.3 Implement `avahi-publish-address` subprocess launcher
- [x] 8.4 Implement child PID tracking per `hostname + IP` pair
- [x] 8.5 Implement publisher stop/termination logic
- [x] 8.6 Implement mDNS collision detection (resolve existing advertisements)
- [x] 8.7 Handle primary hostname collision: skip both mDNS and proxy
- [x] 8.8 Handle alias collision: skip alias only, continue publishing rest
- [x] 8.9 Implement startup reconciliation: inspect running helpers, adopt or terminate
- [x] 8.10 Handle mDNS failure after successful proxy reload (keep proxy route)
- [x] 8.11 Write unit tests for address discovery, collision detection, and helper adoption

## 9. Notifications, Logging, And Retry

- [x] 9.1 Implement `notice_log_tool` wrapper for severity 3 (error) and 4 (warning)
- [x] 9.2 Implement `notice_log_tool` wrapper for severity 5 (recovery notice)
- [x] 9.3 Implement `logger` wrapper for syslog with severity mapping
- [x] 9.4 Implement notice deduplication keyed by problem signature
- [x] 9.5 Implement open-problem tracking and recovery notice emission
- [x] 9.6 Implement open-problem persistence to `/var/run/qnap-docker-mdns/notice-state.json`
- [x] 9.7 Implement recovery of persisted state on daemon restart
- [x] 9.8 Implement per-domain retry state with exponential backoff and jitter
- [x] 9.9 Implement backoff reset on successful reconciliation
- [x] 9.10 Write unit tests for deduplication, retry scheduling, and state persistence

## 10. Event Loop

- [x] 10.1 Implement Docker event subscription (start, stop, die, destroy, rename)
- [x] 10.2 Implement event debounce with configurable window (default 500ms)
- [x] 10.3 Implement periodic full-rescan timer with startup jitter (default 5 min)
- [x] 10.4 Implement reconciliation coordinator merging event + scheduled triggers
- [x] 10.5 Ensure overlapping reconciliation collapses into latest desired state
- [x] 10.6 Implement advisory `flock` single-instance guard
- [x] 10.7 Implement lock-file metadata (PID, startup time)
- [x] 10.8 Handle duplicate start refusal in daemon and control script
- [x] 10.9 Wire all components into the main reconciliation loop

## 11. QPKG Packaging

- [x] 11.1 Create `qpkg.cfg` with package metadata
- [x] 11.2 Create `shared/qnap-docker-mdns.sh` control script
- [x] 11.3 Implement `start` command with disabled-QPKG guard and lock check
- [x] 11.4 Implement `stop` command
- [x] 11.5 Implement `restart` command
- [x] 11.6 Implement `remove` uninstall reconciliation
- [x] 11.7 Implement upgrade behavior: stop, replace binary, preserve config, restart
- [x] 11.8 Create default `config.yaml` with documented defaults
- [x] 11.9 Create `config.local.yaml.sample` with commented override instructions
- [x] 11.10 Add icons/ directory with placeholder icons
- [x] 11.11 Document cross-compilation workflow in `Makefile`
- [x] 11.12 Document QPKG install/upgrade/uninstall flow
- [x] 11.13 Document QNAP reverse proxy UI refresh quirk

## 12. Stage 1: Platform Integration (On-Target Validation)

- [x] 12.1 Inspect `/etc/reverseproxy/extra/80.conf` and confirm expected include behavior
- [x] 12.2 Inspect `/etc/config/reverseproxy/access.json` and `/etc/reverseproxy/access/*.conf`
- [x] 12.3 Verify built-in `local` access-profile discovery by `name: "local"`
- [ ] 12.4 Test fallback behavior when `local` profile is not identified (`access: 0`)
- [x] 12.5 Test candidate reload commands and record working defaults
- [x] 12.6 Test candidate validate commands and record working defaults
- [ ] 12.7 Test custom JSON ownership-marker fields survive UI saves and `scan_config`
- [x] 12.8 Verify `/var/run/docker.sock` access from QPKG service context
- [x] 12.9 Verify `notice_log_tool`, `logger`, `avahi-publish-address`, `ip route` availability
- [x] 12.10 Capture QTS-version-specific deviations for configurable settings

## 13. Stage 11: End-To-End Validation

- [ ] 13.1 Test single-port container auto-detection and publication
- [ ] 13.2 Test HTTP probe: non-HTTP port not published
- [ ] 13.3 Test multi-port with exactly one HTTP-capable endpoint
- [ ] 13.4 Test multi-port with multiple HTTP-capable endpoints requiring label
- [ ] 13.5 Test alias expansion and mDNS publication
- [ ] 13.6 Test non-loopback binding rejection
- [ ] 13.7 Test hostname/alias collision resolution
- [ ] 13.8 Test mDNS collision with external address
- [ ] 13.9 Test JSON write, `scan_config`, validation, and reload success path
- [ ] 13.10 Test backup creation and retention pruning
- [ ] 13.11 Test 64-rule limit enforcement
- [ ] 13.12 Test rollback on validation/reload failure
- [ ] 13.13 Test mDNS failure after proxy reload (keep route active)
- [ ] 13.14 Test notice deduplication and recovery notices
- [ ] 13.15 Test daemon restart during open problem
- [ ] 13.16 Test duplicate start refusal
- [ ] 13.17 Test crash recovery (stale lock does not block restart)
- [ ] 13.18 Test upgrade preserves config and re-announces mDNS
- [ ] 13.19 Test uninstall removes managed config and stops publishers
