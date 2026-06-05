package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/squizzeak/qnap-docker-mdns/internal/config"
	dockerpkg "github.com/squizzeak/qnap-docker-mdns/internal/docker"
	"github.com/squizzeak/qnap-docker-mdns/internal/mdns"
	"github.com/squizzeak/qnap-docker-mdns/internal/notify"
	"github.com/squizzeak/qnap-docker-mdns/internal/proxy"
	"github.com/squizzeak/qnap-docker-mdns/internal/reconcile"
	"github.com/squizzeak/qnap-docker-mdns/internal/state"
)

func main() {
	configPath := flag.String("config", "", "Path to config.yaml")
	flag.Parse()

	cfg := config.DefaultConfig()
	if *configPath != "" {
		defPath := filepath.Join(filepath.Dir(*configPath), "config.yaml")
		overPath := filepath.Join(filepath.Dir(*configPath), "config.local.yaml")
		loaded, err := config.LoadMerged(defPath, overPath)
		if err == nil {
			cfg = loaded
		}
	}

	if err := os.MkdirAll(cfg.State.RuntimeDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "runtime dir: %v\n", err)
		os.Exit(1)
	}

	lock := state.NewLock(cfg.State.LockFile)
	acquired, err := lock.Acquire()
	if err != nil {
		fmt.Fprintf(os.Stderr, "lock error: %v\n", err)
		os.Exit(1)
	}
	if !acquired {
		pid, _ := lock.ReadOwner()
		fmt.Fprintf(os.Stderr, "daemon already running (pid %d)\n", pid)
		os.Exit(1)
	}
	defer lock.Release()

	dockerClient, err := dockerpkg.NewClient(cfg.Docker.Socket, cfg.ProbeTimeout.Duration)
	if err != nil {
		fmt.Fprintf(os.Stderr, "docker: %v\n", err)
		os.Exit(1)
	}
	defer dockerClient.Close()

	publisher := mdns.NewPublisher()

	// Adopt existing avahi-publish-address processes from previous runs
	// so we don't spawn duplicate mDNS publishers across restarts.
	if helpers, err := mdns.FindAdoptedHelpers(); err == nil {
		publisher.Adopt(helpers)
	}

	proxyMgr := proxy.NewManager()
	adapter := &reconcile.ConfigAdapter{Config: cfg}

	reconciler := reconcile.NewReconciler(dockerClient, publisher, proxyMgr, adapter)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	notify.NotifyInfo("process started")
	reconciler.Start(ctx)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("qnap-docker-mdns: shutting down")
	if err := reconciler.Shutdown(); err != nil {
		notify.NotifyFailure(fmt.Sprintf("shutdown cleanup failed: %v", err))
		fmt.Fprintf(os.Stderr, "shutdown cleanup failed: %v\n", err)
		return
	}
	notify.NotifyInfo("process stopped; all reverse proxies removed")
}
