package notify

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/squizzeak/qnap-docker-mdns/internal/config"
)

func TestProblemStateOpenClose(t *testing.T) {
	dir := t.TempDir()
	ps := NewProblemState(filepath.Join(dir, "state.json"))

	sig := "test:container"
	if ps.IsOpen(sig) {
		t.Error("expected not open initially")
	}

	ps.Open(sig)
	if !ps.IsOpen(sig) {
		t.Error("expected open after Open()")
	}

	ps.Close(sig)
	if ps.IsOpen(sig) {
		t.Error("expected closed after Close()")
	}
}

func TestProblemStatePersistence(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")

	ps1 := NewProblemState(statePath)
	ps1.Open("test:container")

	ps2 := NewProblemState(statePath)
	if !ps2.IsOpen("test:container") {
		t.Error("expected state to persist across instances")
	}
}

func TestProblemStateAllOpen(t *testing.T) {
	dir := t.TempDir()
	ps := NewProblemState(filepath.Join(dir, "state.json"))
	ps.Open("a")
	ps.Open("b")

	all := ps.AllOpen()
	if len(all) != 2 {
		t.Errorf("expected 2 open problems, got %d", len(all))
	}
}

func TestRetryStateInitial(t *testing.T) {
	rs := NewRetryState()
	cfg := config.RetryConfig{
		ImmediateRetries: 1,
		InitialBackoff:   config.Duration{Duration: 5 * time.Second},
		MaxBackoff:       config.Duration{Duration: 5 * time.Minute},
		JitterPercent:    20,
	}

	should, backoff := rs.ShouldRetry("docker", cfg)
	if !should {
		t.Error("expected should retry")
	}
	if backoff != 0 {
		t.Errorf("expected immediate retry (0 backoff), got %v", backoff)
	}
}

func TestRetryStateBackoff(t *testing.T) {
	rs := NewRetryState()
	cfg := config.RetryConfig{
		ImmediateRetries: 0,
		InitialBackoff:   config.Duration{Duration: 5 * time.Second},
		MaxBackoff:       config.Duration{Duration: 5 * time.Minute},
		JitterPercent:    0,
	}

	_, backoff1 := rs.ShouldRetry("test", cfg)
	if backoff1 < 4*time.Second || backoff1 > 6*time.Second {
		t.Errorf("expected ~5s backoff, got %v", backoff1)
	}
}

func TestRetryStateReset(t *testing.T) {
	rs := NewRetryState()
	cfg := config.RetryConfig{
		ImmediateRetries: 0,
		InitialBackoff:   config.Duration{Duration: 5 * time.Second},
		MaxBackoff:       config.Duration{Duration: 5 * time.Minute},
		JitterPercent:    0,
	}

	rs.ShouldRetry("reset-test", cfg)
	rs.Reset("reset-test")

	// After reset, should start fresh with initial backoff
	_, backoff := rs.ShouldRetry("reset-test", cfg)
	if backoff < 4*time.Second || backoff > 6*time.Second {
		t.Errorf("expected ~5s initial backoff after reset, got %v", backoff)
	}
}

func TestProblemSignature(t *testing.T) {
	sig := ProblemSignature("docker", "grafana")
	if sig != "docker:grafana" {
		t.Errorf("unexpected signature: %s", sig)
	}
}

func TestNotifyFunctionsDoNotPanic(t *testing.T) {
	NotifyMisconfig("test", "test message")
	NotifyFailure("test failure")
	NotifyRecovery("test recovery")
	LogErr("test error")
	LogWarn("test warning")
	LogInfo("test info")
}

func TestReloadFailureDetail(t *testing.T) {
	detail := ReloadFailureDetail("apache_proxy -k graceful", 1, "syntax error")
	if detail == "" {
		t.Error("expected non-empty detail")
	}
}
