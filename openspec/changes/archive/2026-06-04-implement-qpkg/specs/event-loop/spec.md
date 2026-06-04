## ADDED Requirements

### Requirement: Docker event subscription
The daemon SHALL subscribe to Docker lifecycle events: `start`, `stop`, `die`, `destroy`, `rename`.

#### Scenario: Container start event
- **WHEN** a container start event is received
- **THEN** the daemon SHALL trigger reconciliation after debounce

#### Scenario: Container stop/die/destroy event
- **WHEN** a container stop, die, or destroy event is received for a tracked container
- **THEN** the daemon SHALL trigger reconciliation after debounce
- **THEN** the daemon SHALL re-inspect container state from Docker API (not trust event payload alone)

#### Scenario: Container rename event
- **WHEN** a container rename event is received
- **THEN** the daemon SHALL trigger reconciliation after debounce

### Requirement: Event debouncing
The daemon SHALL debounce rapid event bursts with a configurable window (default 500ms).

#### Scenario: Events debounced
- **WHEN** multiple Docker events arrive within the debounce window
- **THEN** the daemon SHALL wait for the debounce timer to expire after the last event
- **THEN** the daemon SHALL trigger one reconciliation pass

### Requirement: Periodic full rescan
The daemon SHALL run a periodic full container rescan even without Docker events.

#### Scenario: Periodic rescan triggers
- **WHEN** the rescan interval elapses (default 5 minutes)
- **THEN** the daemon SHALL run a full reconciliation
- **THEN** the daemon SHALL apply startup jitter to the first interval

### Requirement: Reconciliation coordinator
The daemon SHALL merge event-triggered and scheduled reconciliation into a single coordinator.
Overlapping reconciliation requests SHALL collapse into the latest desired state.

#### Scenario: Serial reconciliation
- **WHEN** reconciliation is in progress and a new trigger arrives
- **THEN** the new trigger SHALL be queued
- **THEN** the daemon SHALL run one final reconciliation after the current one completes

### Requirement: Single-instance lock
The daemon SHALL acquire an advisory exclusive `flock` on the lock file for its lifetime.

#### Scenario: Lock acquired
- **WHEN** the daemon starts
- **THEN** it SHALL open and lock `/var/run/qnap-docker-mdns/daemon.lock` with `flock`

#### Scenario: Duplicate start refused
- **WHEN** a second daemon instance attempts to start
- **THEN** `flock` acquisition SHALL fail
- **THEN** the second instance SHALL exit

#### Scenario: Crash recovery
- **WHEN** the daemon crashes
- **THEN** the kernel SHALL release the `flock`
- **THEN** a new instance SHALL be able to start without manual cleanup
