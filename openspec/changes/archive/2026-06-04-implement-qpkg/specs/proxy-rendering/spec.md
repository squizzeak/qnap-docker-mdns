## ADDED Requirements

### Requirement: JSON rule rendering
The daemon SHALL render one managed JSON entry per primary hostname and alias.
Each entry SHALL be written to the `list` object in QNAP's `reverseproxy.json` format.

#### Scenario: Primary hostname rendered
- **WHEN** a container has a valid primary hostname
- **THEN** the daemon SHALL render one JSON entry with `server_name` set to that hostname
- **THEN** the entry SHALL have `name` set to `<primary-hostname> (managed)`

#### Scenario: Alias rendered as separate entry
- **WHEN** a container has non-colliding aliases
- **THEN** the daemon SHALL render one JSON entry per alias
- **THEN** each alias entry SHALL have its own distinct `qnap_docker_mdns_key`

### Requirement: JSON entry fields
Each managed JSON entry SHALL include: `id`, `name`, `protocol`, `des_protocol`, `server_name`, `hsts`, `host_name`, `access`, `port`, `des_port`, `proxy_timeout`, `header`, `qnap_docker_mdns_managed`, `qnap_docker_mdns_key`.

#### Scenario: Entry field values
- **WHEN** a managed entry is rendered
- **THEN** `host_name` SHALL be `localhost`
- **THEN** `protocol` and `des_protocol` SHALL be `http`
- **THEN** `port` SHALL be the Apache listening port (default 80)
- **THEN** `des_port` SHALL be the selected host-published backend port
- **THEN** `proxy_timeout` SHALL be 60
- **THEN** `hsts` SHALL be false
- **THEN** `header` SHALL be an empty array
- **THEN** `qnap_docker_mdns_managed` SHALL be `true`
- **THEN** `qnap_docker_mdns_key` SHALL be a unique stable key for this managed rule

### Requirement: Access profile assignment
The daemon SHALL set `access` to the discovered numeric ID of QNAP's built-in `local` profile.
When the built-in `local` profile cannot be identified, the daemon SHALL set `access` to 0.

#### Scenario: Local profile discovered
- **WHEN** `/etc/config/reverseproxy/access.json` contains an entry with `name: "local"`
- **THEN** the daemon SHALL use that entry's numeric `id` for the `access` field

#### Scenario: Local profile not found
- **WHEN** no entry with `name: "local"` exists in `access.json`
- **THEN** the daemon SHALL set `access` to 0
- **THEN** the daemon SHALL log the fallback

### Requirement: ID assignment
The daemon SHALL assign sequential IDs for new managed entries starting from `max(existing_ids) + 1`.
Existing owned entries SHALL preserve their assigned IDs across reconciliation.

#### Scenario: New entry gets next ID
- **WHEN** creating a new managed entry
- **THEN** the daemon SHALL scan all entries in `reverseproxy.json` for the maximum `id`
- **THEN** the new entry SHALL use `max_id + 1`

#### Scenario: Existing entry preserves ID
- **WHEN** updating an existing owned entry matched by `qnap_docker_mdns_key`
- **THEN** the daemon SHALL keep the original `id` value

### Requirement: Hostname collision detection
The daemon SHALL detect hostname collisions before rendering against both managed and unmanaged `ServerName`/`ServerAlias` entries.

#### Scenario: Collision with managed hostname
- **WHEN** two enabled containers claim the same primary hostname
- **THEN** the lexically first container in stable normalized ordering keeps the hostname
- **THEN** the later container's hostname SHALL be skipped

#### Scenario: Collision with alias
- **WHEN** a primary hostname collides with another container's alias
- **THEN** the lexically first claim in stable normalized ordering wins
- **THEN** the later conflicting name SHALL be skipped

#### Scenario: Collision with unmanaged entry
- **WHEN** a requested hostname matches an unmanaged `ServerName` or `ServerAlias`
- **THEN** the primary hostname SHALL be skipped as actionable misconfiguration

### Requirement: 64-rule limit enforcement
The daemon SHALL respect QNAP's 64-rule limit in `reverseproxy.json`.

#### Scenario: Limit reached
- **WHEN** adding a new managed rule would exceed 64 total entries
- **THEN** the daemon SHALL NOT add the rule
- **THEN** the daemon SHALL emit a user-visible notice
