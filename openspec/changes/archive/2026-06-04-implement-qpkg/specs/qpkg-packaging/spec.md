## ADDED Requirements

### Requirement: QPKG metadata
The QPKG SHALL include a `qpkg.cfg` file with package metadata and service hooks.
The control script entrypoint SHALL be `shared/qnap-docker-mdns.sh`.

#### Scenario: Package installed
- **WHEN** the QPKG is installed
- **THEN** `qpkg.cfg` SHALL define the package name, version, author, and service script

### Requirement: Service control script
The control script SHALL support `start`, `stop`, `restart`, and `remove` commands.
The script SHALL follow the QDK disabled-package guard convention.

#### Scenario: Daemon starts
- **WHEN** `start` is called
- **THEN** the script SHALL check if the QPKG is enabled
- **THEN** the script SHALL launch `qnap-docker-mdnsd` with the config path

#### Scenario: Daemon stops
- **WHEN** `stop` is called
- **THEN** the script SHALL terminate the daemon process

#### Scenario: Duplicate start guarded
- **WHEN** `start` is called and the daemon is already running
- **THEN** the script SHALL detect the lock and refuse
- **THEN** the script SHALL report the existing instance metadata

### Requirement: Configuration loading
The daemon SHALL load defaults from `$QPKG_ROOT/config.yaml` then overlay runtime overrides from `$QPKG_ROOT/config.local.yaml`.

#### Scenario: Defaults loaded
- **WHEN** the daemon starts
- **THEN** it SHALL read `config.yaml` for default settings

#### Scenario: Runtime overrides applied
- **WHEN** `config.local.yaml` exists
- **THEN** its values SHALL override the defaults

### Requirement: Upgrade behavior
Upgrade SHALL stop the daemon, replace the binary and packaged defaults, preserve `config.local.yaml`, and restart.

#### Scenario: Upgrade preserves config
- **WHEN** the QPKG is upgraded
- **THEN** `config.local.yaml` SHALL NOT be overwritten
- **THEN** `config.local.yaml.sample` SHALL be replaced
- **THEN** the daemon SHALL restart and re-announce mDNS entries

### Requirement: Uninstall cleanup
Uninstall SHALL remove daemon-managed JSON entries, run `scan_config`, and stop mDNS publishers.

#### Scenario: Managed config removed
- **WHEN** the QPKG is uninstalled
- **THEN** the control script SHALL remove entries with `qnap_docker_mdns_managed: true` from `reverseproxy.json`
- **THEN** the script SHALL run `scan_config`
- **THEN** the script SHALL stop all daemon-managed Avahi publishers

### Requirement: Cross-compilation workflow
The project SHALL include a `Makefile` with targets for local builds, cross-compilation, and QPKG assembly.

#### Scenario: Cross-compile for QNAP
- **WHEN** building for x86_64 QNAP from a development machine
- **THEN** `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build` SHALL produce the daemon binary
- **THEN** the binary SHALL be placed in the `shared/` directory for packaging
