## ADDED Requirements

### Requirement: Docker socket connection
The daemon SHALL connect to the Docker daemon through `/var/run/docker.sock` using the Docker API client.
The daemon SHALL use `github.com/docker/docker/client` for all Docker API interactions.

#### Scenario: Socket connection established
- **WHEN** the daemon starts
- **THEN** it SHALL open a connection to `/var/run/docker.sock`
- **THEN** it SHALL verify the connection is functional by listing containers

#### Scenario: Socket unavailable
- **WHEN** `/var/run/docker.sock` is not accessible
- **THEN** the daemon SHALL treat it as an operational failure
- **THEN** the daemon SHALL retry with backoff

### Requirement: Container discovery
The daemon SHALL list all running containers on every reconciliation pass.
The daemon SHALL inspect each container's labels, name, and published port bindings.

#### Scenario: Running containers listed
- **WHEN** reconciliation starts
- **THEN** the daemon SHALL query all running containers from Docker
- **THEN** the daemon SHALL collect labels, container names, exposed ports, and host-published TCP bindings for each container

#### Scenario: Container inspection failure
- **WHEN** a container cannot be inspected
- **THEN** the daemon SHALL skip that container
- **THEN** the daemon SHALL log the failure with container identifier

### Requirement: Label parsing
The daemon SHALL parse `qnap-docker-mdns.*` labels from each container.
Supported labels: `qnap-docker-mdns.enable`, `qnap-docker-mdns.port`, `qnap-docker-mdns.hostname`, `qnap-docker-mdns.aliases`.

#### Scenario: Enable label parsed
- **WHEN** a container has `qnap-docker-mdns.enable=true`
- **THEN** the daemon SHALL treat the container as eligible for publication

#### Scenario: Enable label false
- **WHEN** a container has `qnap-docker-mdns.enable=false` or missing
- **THEN** the daemon SHALL skip the container

#### Scenario: Port label parsed
- **WHEN** `qnap-docker-mdns.port` is set
- **THEN** the daemon SHALL validate it matches a host-published TCP port
- **THEN** the daemon SHALL require that port to pass HTTP probing before use

#### Scenario: Invalid port label
- **WHEN** `qnap-docker-mdns.port` does not match any host-published TCP port
- **THEN** the daemon SHALL treat it as actionable misconfiguration
- **THEN** the daemon SHALL emit a user-visible notice

#### Scenario: Custom hostname label
- **WHEN** `qnap-docker-mdns.hostname` is set
- **THEN** the daemon SHALL normalize it to lowercase FQDN under the configured suffix
- **THEN** the daemon SHALL use it as the primary hostname

#### Scenario: Default hostname
- **WHEN** `qnap-docker-mdns.hostname` is not set
- **THEN** the daemon SHALL use `<container-name>.local` as the default hostname

#### Scenario: Aliases label parsed
- **WHEN** `qnap-docker-mdns.aliases` is set
- **THEN** the daemon SHALL split by comma and normalize each alias
- **THEN** each non-colliding alias SHALL become its own managed reverse proxy entry

### Requirement: Port binding filtering
The daemon SHALL filter host-published TCP port bindings to those reachable through `localhost` on the NAS.

#### Scenario: Zero-bind host port accepted
- **WHEN** a port binding uses `0.0.0.0` host address
- **THEN** the daemon SHALL accept it as reachable through `localhost`

#### Scenario: Loopback bind accepted
- **WHEN** a port binding uses `127.0.0.1` host address
- **THEN** the daemon SHALL accept it as reachable through `localhost`

#### Scenario: Non-loopback bind rejected
- **WHEN** a port binding uses a specific non-loopback IPv4 address
- **THEN** the daemon SHALL reject it as unsupported
- **THEN** the daemon SHALL treat it as actionable misconfiguration

#### Scenario: IPv6-only bind rejected
- **WHEN** a port binding uses IPv6 only
- **THEN** the daemon SHALL reject it as unsupported

### Requirement: HTTP endpoint probing
The daemon SHALL probe candidate host-published TCP ports through `localhost` with a short HTTP request.
A candidate port MUST return a parseable HTTP response to `GET /` or `HEAD /` over plain HTTP.

#### Scenario: HTTP probe succeeds
- **WHEN** a host-published TCP port returns a parseable HTTP response
- **THEN** the daemon SHALL mark it as a candidate for backend selection

#### Scenario: HTTP probe fails
- **WHEN** a host-published TCP port returns connection refusal, timeout, TLS-only response, or non-HTTP response
- **THEN** the daemon SHALL NOT mark it as a candidate

#### Scenario: Probe timeout applies
- **WHEN** probing a port
- **THEN** the daemon SHALL use a short configurable timeout (default 2s)

### Requirement: Port selection
The daemon SHALL select the backend port based on probe results and label configuration.

#### Scenario: Single candidate auto-selected
- **WHEN** exactly one candidate port is available after probing
- **THEN** the daemon SHALL use it automatically

#### Scenario: Multiple candidates require label
- **WHEN** more than one candidate port exists
- **THEN** the daemon SHALL require `qnap-docker-mdns.port` to be set
- **THEN** if not set, it SHALL be treated as actionable misconfiguration

#### Scenario: No candidates skip container
- **WHEN** zero candidate ports are available
- **THEN** the daemon SHALL skip the container
- **THEN** the daemon SHALL treat it as actionable misconfiguration
