package proxy

import (
	"os"
	"os/exec"
	"strings"
	"time"
)

type ExecResult struct {
	ExitCode int
	Stderr   string
	Duration time.Duration
}

func RunCommand(command string) ExecResult {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return ExecResult{ExitCode: -1, Stderr: "empty command"}
	}

	name := parts[0]
	args := parts[1:]
	start := time.Now()

	cmd := exec.Command(name, args...)
	stderr := new(strings.Builder)
	cmd.Stderr = stderr
	cmd.Env = append(os.Environ(), "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")

	err := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return ExecResult{
		ExitCode: exitCode,
		Stderr:   strings.TrimSpace(stderr.String()),
		Duration: duration,
	}
}

func RunScanConfig() ExecResult {
	return RunCommand("/etc/init.d/reverse_proxy.sh scan_config")
}

func RunValidate(validateCommand string) ExecResult {
	return RunCommand(validateCommand)
}

func RunReload(reloadCommand string) ExecResult {
	return RunCommand(reloadCommand)
}
