## ADDED Requirements

### Requirement: QNAP notice for actionable misconfiguration
The daemon SHALL use `notice_log_tool --severity 4` for configuration issues requiring administrator action.

#### Scenario: Misconfiguration notice emitted
- **WHEN** a container has `qnap-docker-mdns.enable=true` but ambiguous or invalid configuration
- **THEN** the daemon SHALL call `notice_log_tool --severity 4 --user admin --serviceName qnap-docker-mdns --facility 13`
- **THEN** the notice message SHALL identify the container and the specific misconfiguration

### Requirement: QNAP notice for operational failure
The daemon SHALL use `notice_log_tool --severity 3` for operational failures affecting reconciliation.

#### Scenario: Operational failure notice emitted
- **WHEN** a command fails (reload, scan_config, Docker API, etc.)
- **THEN** the daemon SHALL call `notice_log_tool --severity 3 --user admin --serviceName qnap-docker-mdns --facility 13`
- **THEN** the notice SHALL include the failing command and exit status

### Requirement: QNAP recovery notice
The daemon SHALL use `notice_log_tool --severity 5` when a previously reported problem is resolved.

#### Scenario: Recovery notice emitted
- **WHEN** a previously failed operation succeeds on a subsequent reconciliation
- **THEN** the daemon SHALL call `notice_log_tool --severity 5 --user admin --serviceName qnap-docker-mdns --facility 13`
- **THEN** the notice SHALL reference the resolved problem

### Requirement: Notice deduplication
The daemon SHALL NOT emit duplicate failure notices for the same active problem.
The daemon SHALL track open problems per failure domain.

#### Scenario: Duplicate failure suppressed
- **WHEN** the same failure persists across multiple reconciliation cycles
- **THEN** the daemon SHALL emit the failure notice only once

#### Scenario: Recovery after failure
- **WHEN** a tracked failure is resolved
- **THEN** the daemon SHALL emit exactly one recovery notice
- **THEN** the daemon SHALL clear the open problem state

### Requirement: Syslog logging
The daemon SHALL use `logger` for operational failure logging.

#### Scenario: Operational failure logged
- **WHEN** a command fails
- **THEN** the daemon SHALL call `logger -t qnap-docker-mdns -p daemon.err` with failure details

#### Scenario: Retry scheduling logged
- **WHEN** a retryable failure occurs
- **THEN** the daemon SHALL call `logger -t qnap-docker-mdns -p daemon.notice` with backoff details

### Requirement: Open-problem persistence
The daemon SHALL persist open-problem state to `/var/run/qnap-docker-mdns/notice-state.json`.

#### Scenario: Problem state persisted
- **WHEN** a failure notice is emitted
- **THEN** the problem signature SHALL be written to the notice state file

#### Scenario: State recovered on restart
- **WHEN** the daemon restarts
- **THEN** it SHALL read the persisted notice state
- **THEN** it SHALL NOT re-emit failure notices for problems already open

### Requirement: Retry with exponential backoff
The daemon SHALL retry operational failures with exponential backoff and jitter.
Failure domains: Docker watch, config write/validate/reload, LAN IP discovery, Avahi operations.

#### Scenario: Immediate retry then backoff
- **WHEN** an operational failure occurs
- **THEN** the daemon SHALL retry immediately once
- **THEN** if still failing, backoff SHALL start at 5 seconds
- **THEN** backoff SHALL increase exponentially with 20% jitter
- **THEN** backoff SHALL cap at 5 minutes

#### Scenario: Backoff reset on success
- **WHEN** a previously failing operation succeeds
- **THEN** the retry state for that domain SHALL reset
