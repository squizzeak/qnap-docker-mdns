package notify

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/squizzeak/qnap-docker-mdns/internal/config"
)

const (
	appName     = "qnap-docker-mdns"
	userAdmin   = "admin"
	facilityApp = "13"
)

// eventType maps to log_tool -t: 0=Information, 1=Warning, 2=Error
const (
	eventInfo    = "0"
	eventWarning = "1"
	eventError   = "2"
)

type ProblemState struct {
	mu      sync.Mutex
	open    map[string]bool
	statePath string
}

func NewProblemState(statePath string) *ProblemState {
	ps := &ProblemState{
		open:      make(map[string]bool),
		statePath: statePath,
	}
	ps.load()
	return ps
}

func (ps *ProblemState) IsOpen(signature string) bool {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.open[signature]
}

func (ps *ProblemState) Open(signature string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.open[signature] = true
	ps.save()
}

func (ps *ProblemState) Close(signature string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	delete(ps.open, signature)
	ps.save()
}

func (ps *ProblemState) AllOpen() map[string]bool {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	result := make(map[string]bool)
	for k, v := range ps.open {
		result[k] = v
	}
	return result
}

func (ps *ProblemState) load() {
	data, err := os.ReadFile(ps.statePath)
	if err != nil {
		return
	}
	var open map[string]bool
	if err := json.Unmarshal(data, &open); err != nil {
		return
	}
	ps.open = open
}

func (ps *ProblemState) save() {
	data, err := json.Marshal(ps.open)
	if err != nil {
		return
	}
	os.WriteFile(ps.statePath, data, 0644)
}

func NotifyMisconfig(containerName, detail string) {
	msg := fmt.Sprintf("[qnap-docker-mdns] container %s: %s", containerName, detail)
	noticeLogTool(msg, "4")
	eventLogTool(eventWarning, msg)
}

func NotifyFailure(detail string) {
	msg := fmt.Sprintf("[qnap-docker-mdns] %s", detail)
	noticeLogTool(msg, "3")
	eventLogTool(eventError, msg)
}

func NotifyRecovery(detail string) {
	msg := fmt.Sprintf("[qnap-docker-mdns] %s", detail)
	noticeLogTool(msg, "5")
	eventLogTool(eventInfo, msg)
}

func NotifyAudit(detail string) {
	msg := fmt.Sprintf("[qnap-docker-mdns] %s", detail)
	noticeLogTool(msg, "5")
	eventLogTool(eventInfo, msg)
}

func LogErr(msg string) {
	logger(msg, "daemon.err")
}

func LogWarn(msg string) {
	logger(msg, "daemon.warning")
}

func LogInfo(msg string) {
	logger(msg, "daemon.notice")
}

func ReloadFailureDetail(cmd string, exitCode int, stderr string) string {
	return fmt.Sprintf("reverse proxy reload failed: exit_status=%d command=%s", exitCode, cmd)
}

func noticeLogTool(msg, severity string) {
	// Uses the QNAP notice_log_tool binary for popup notifications.
	// In tests or non-QNAP environments, this is a no-op.
	go func() {
		cmd := exec.Command("notice_log_tool",
			"-E", msg,
			"-t", severity,
			"-u", userAdmin,
			"-i", appName,
			"-y", facilityApp,
			"-N", "qnap-docker-mdns",
			"-S", "2",
			"-g", "MSG_QDM_001",
		)
		cmd.Run()
	}()
}

func eventLogTool(eventType, msg string) {
	// Uses the QNAP log_tool binary for persistent entries in qulog notification center.
	// In tests or non-QNAP environments, this is a no-op.
	go func() {
		cmd := exec.Command("log_tool",
			"-t", eventType,
			"-u", userAdmin,
			"-m", appName,
			"-a", msg,
		)
		cmd.Run()
	}()
}

func logger(msg, priority string) {
	go func() {
		cmd := exec.Command("logger", "-t", appName, "-p", priority, msg)
		cmd.Run()
	}()
}

func ProblemSignature(domain, containerName string) string {
	return fmt.Sprintf("%s:%s", domain, containerName)
}

type RetryState struct {
	mu            sync.Mutex
	domains       map[string]*domainRetry
}

type domainRetry struct {
	currentBackoff time.Duration
	attempts       int
}

func NewRetryState() *RetryState {
	return &RetryState{
		domains: make(map[string]*domainRetry),
	}
}

func (rs *RetryState) ShouldRetry(domain string, cfg config.RetryConfig) (bool, time.Duration) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	d, exists := rs.domains[domain]
	if !exists {
		d = &domainRetry{currentBackoff: 0}
		rs.domains[domain] = d
	}

	d.attempts++

	if d.attempts <= cfg.ImmediateRetries {
		return true, 0
	}

	if d.currentBackoff == 0 {
		d.currentBackoff = cfg.InitialBackoff.Duration
	} else {
		d.currentBackoff = time.Duration(float64(d.currentBackoff) * 2)
		if d.currentBackoff > cfg.MaxBackoff.Duration {
			d.currentBackoff = cfg.MaxBackoff.Duration
		}
	}

	jitter := time.Duration(float64(d.currentBackoff) * (float64(cfg.JitterPercent) / 100.0) * (rand.Float64()*2 - 1))
	backoff := d.currentBackoff + jitter

	return true, backoff
}

func (rs *RetryState) Reset(domain string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	delete(rs.domains, domain)
}
