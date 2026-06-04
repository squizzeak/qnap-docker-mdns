## ADDED Requirements

### Requirement: JSON file read
The daemon SHALL read `/etc/config/reverseproxy/reverseproxy.json` on each reconciliation.
The daemon SHALL parse all entries, separating managed (by embedded ownership marker) from unmanaged.

#### Scenario: File read succeeds
- **WHEN** reconciliation starts
- **THEN** the daemon SHALL read the JSON file
- **THEN** it SHALL identify owned entries by `qnap_docker_mdns_managed: true`

#### Scenario: File read fails
- **WHEN** the JSON file cannot be read
- **THEN** it SHALL be treated as an operational failure
- **THEN** the daemon SHALL retry with backoff

### Requirement: Dated backup creation
The daemon SHALL create a dated backup of the current `reverseproxy.json` before writing substantive changes.
Backups SHALL follow the naming pattern: `reverseproxy.json.qnap-docker-mdns.<YYYYMMDD-HHMMSS>.bak`.

#### Scenario: Backup created before write
- **WHEN** the rendered JSON differs from the current file content
- **THEN** the daemon SHALL create a dated backup in the same directory
- **THEN** the daemon SHALL write the new JSON atomically

#### Scenario: No backup for no-op
- **WHEN** the rendered JSON matches the current file content
- **THEN** the daemon SHALL skip backup and write

### Requirement: Backup retention
The daemon SHALL retain at most `backups.max_backups` dated backup files (default 100).
Oldest backups SHALL be pruned after a successful new backup.

#### Scenario: Retention enforced
- **WHEN** a new backup is created and the count exceeds `max_backups`
- **THEN** the daemon SHALL delete the oldest backups until the count equals `max_backups`

### Requirement: Atomic JSON write
The daemon SHALL write the updated JSON atomically.
The daemon SHALL merge managed entries with unmanaged entries, preserving all unmanaged entries.

#### Scenario: Atomic write succeeds
- **WHEN** writing the updated JSON
- **THEN** the daemon SHALL write to a temporary file then rename into place

#### Scenario: Unmanaged entries preserved
- **WHEN** merging managed entries into the file
- **THEN** all entries without `qnap_docker_mdns_managed: true` SHALL be preserved

### Requirement: scan_config execution
After writing JSON, the daemon SHALL run `/etc/init.d/reverse_proxy.sh scan_config`.

#### Scenario: scan_config succeeds
- **WHEN** `scan_config` completes with exit code 0
- **THEN** the daemon SHALL proceed to validate and reload

#### Scenario: scan_config fails
- **WHEN** `scan_config` returns non-zero
- **THEN** the daemon SHALL restore the dated backup and retry

### Requirement: Generated Apache config validation
The daemon SHALL validate the generated Apache config using the configured `validate_command`.

#### Scenario: Validation passes
- **WHEN** the validation command exits with code 0
- **THEN** the daemon SHALL proceed to reload

#### Scenario: Validation fails
- **WHEN** the validation command exits non-zero
- **THEN** the daemon SHALL restore the dated backup
- **THEN** the daemon SHALL re-run `scan_config` with the restored state

### Requirement: Generated output verification
After `scan_config`, the daemon SHALL inspect the generated per-port Apache files to confirm expected VirtualHost entries.

#### Scenario: Expected entries present
- **WHEN** inspecting the generated per-port file
- **THEN** the daemon SHALL verify expected `ServerName`, backend port, and access-profile include exist

#### Scenario: Expected entries missing
- **WHEN** expected entries are not in the generated file
- **THEN** the daemon SHALL treat it as an operational failure
- **THEN** the daemon SHALL restore the dated backup

### Requirement: Uninstall reconciliation
On uninstall, the daemon SHALL remove only managed JSON entries from `reverseproxy.json`.
After removing entries, the daemon SHALL run `scan_config` to regenerate per-port files.

#### Scenario: Uninstall removes managed entries
- **WHEN** uninstall is triggered
- **THEN** only entries with `qnap_docker_mdns_managed: true` are removed
- **THEN** unmanaged entries are untouched
