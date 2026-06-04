## ADDED Requirements

### Requirement: NAS LAN IPv4 discovery
The daemon SHALL discover supported non-loopback IPv4 addresses on the NAS for mDNS publication.

#### Scenario: LAN addresses discovered
- **WHEN** reconciliation starts
- **THEN** the daemon SHALL enumerate non-loopback IPv4 addresses on NAS interfaces
- **THEN** the daemon SHALL filter to addresses intended for LAN client access

#### Scenario: No LAN addresses found
- **WHEN** no suitable non-loopback IPv4 address is found
- **THEN** the daemon SHALL treat this as an operational failure
- **THEN** the daemon SHALL retry with backoff

### Requirement: Avahi publisher lifecycle
The daemon SHALL launch one `avahi-publish-address -a` subprocess per hostname and discovered IPv4 address pair.
The daemon SHALL track child PIDs and the hostname/IP each PID owns.

#### Scenario: Publisher started
- **WHEN** a hostname needs to be published to a LAN IPv4 address
- **THEN** the daemon SHALL launch `avahi-publish-address -a <hostname> <ip>`
- **THEN** the daemon SHALL record the PID, hostname, and IP in its state

#### Scenario: Publisher stopped
- **WHEN** a container is removed or hostname changes
- **THEN** the daemon SHALL terminate the matching `avahi-publish-address` process
- **THEN** the daemon SHALL remove it from tracked state

### Requirement: mDNS collision detection
The daemon SHALL detect existing mDNS advertisements before publishing.

#### Scenario: Primary hostname collision with external address
- **WHEN** a requested primary hostname resolves to an address outside the NAS LAN IPv4 set
- **THEN** the daemon SHALL refuse to publish that hostname in both mDNS and reverse proxy
- **THEN** the daemon SHALL emit a user-visible notice

#### Scenario: Alias collision with external address
- **WHEN** a requested alias resolves to an address outside the NAS LAN IPv4 set
- **THEN** the daemon SHALL skip only that colliding alias
- **THEN** the daemon SHALL continue publishing non-conflicting names

### Requirement: Startup reconciliation
On daemon startup, the daemon SHALL inspect running `avahi-publish-address` helpers to recover advertised hostname/IP pairs.

#### Scenario: Orphaned publisher adopted
- **WHEN** a running helper advertises a `hostname + IP` pair matching current desired state
- **THEN** the daemon SHALL adopt it and track it as service-managed

#### Scenario: Orphaned publisher terminated
- **WHEN** a running helper advertises a `hostname + IP` pair not in the desired state
- **THEN** the daemon SHALL terminate the orphaned process

### Requirement: mDNS failure handling
The daemon SHALL preserve reverse proxy routes when only mDNS publication fails.

#### Scenario: mDNS fails after proxy reload
- **WHEN** mDNS publication fails after a successful reverse proxy reload
- **THEN** the daemon SHALL keep the proxy route active
- **THEN** the daemon SHALL retry mDNS publication with backoff
