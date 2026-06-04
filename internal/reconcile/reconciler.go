package reconcile

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/docker/docker/api/types/events"
	dockerpkg "github.com/squizzeak/qnap-docker-mdns/internal/docker"
	"github.com/squizzeak/qnap-docker-mdns/internal/mdns"
	"github.com/squizzeak/qnap-docker-mdns/internal/notify"
	"github.com/squizzeak/qnap-docker-mdns/internal/proxy"
	"github.com/squizzeak/qnap-docker-mdns/internal/state"
)



type Reconciler struct {
	mu           sync.Mutex
	dockerClient *dockerpkg.Client
	publisher    *mdns.Publisher
	proxyMgr     *proxy.Manager
	cfg          *ConfigAdapter
	triggerCh    chan struct{}
	stopCh       chan struct{}
	wg           sync.WaitGroup
	problemState *notify.ProblemState
	retryState   *notify.RetryState
}

func NewReconciler(dockerClient *dockerpkg.Client, publisher *mdns.Publisher, proxyMgr *proxy.Manager, cfg *ConfigAdapter) *Reconciler {
	return &Reconciler{
		dockerClient: dockerClient,
		publisher:    publisher,
		proxyMgr:     proxyMgr,
		cfg:          cfg,
		triggerCh:    make(chan struct{}, 1),
		stopCh:       make(chan struct{}),
		problemState: notify.NewProblemState(cfg.NoticeStateFile()),
		retryState:   notify.NewRetryState(),
	}
}

func (r *Reconciler) Start(ctx context.Context) {
	r.wg.Add(2)
	go r.eventLoop(ctx)
	go r.periodicLoop(ctx)
}

func (r *Reconciler) Stop() {
	close(r.stopCh)
	r.wg.Wait()
}

func (r *Reconciler) Trigger() {
	select {
	case r.triggerCh <- struct{}{}:
	default:
	}
}

func (r *Reconciler) eventLoop(ctx context.Context) {
	defer r.wg.Done()

	eventCh, errCh := r.dockerClient.Events(ctx)

	var timer *time.Timer
	for {
		select {
		case <-r.stopCh:
			return
		case <-ctx.Done():
			return
		case err := <-errCh:
			if err != nil {
				notify.LogErr(fmt.Sprintf("docker event error: %v", err))
			}
		case event := <-eventCh:
			if isTrackedEvent(event) {
				if timer != nil {
					timer.Stop()
				}
				timer = time.AfterFunc(r.cfg.DebounceWindow(), func() {
					r.Reconcile(ctx)
				})
			}
		}
	}
}

func isTrackedEvent(event events.Message) bool {
	switch event.Action {
	case "start", "stop", "die", "destroy", "rename":
		return true
	}
	return false
}

func (r *Reconciler) periodicLoop(ctx context.Context) {
	defer r.wg.Done()

	r.Reconcile(ctx)

	jitter := time.Duration(rand.Int63n(int64(r.cfg.FullRescanInterval())))
	select {
	case <-r.stopCh:
		return
	case <-time.After(jitter):
	}

	ticker := time.NewTicker(r.cfg.FullRescanInterval())
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.Reconcile(ctx)
		}
	}
}

func (r *Reconciler) Reconcile(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()

	notify.LogInfo("reconciliation starting")

	containers, err := r.dockerClient.ListRunningContainers(ctx)
	if err != nil {
		sig := notify.ProblemSignature("docker-list", "*")
		if !r.problemState.IsOpen(sig) {
			notify.NotifyFailure(fmt.Sprintf("container list failed: %v", err))
			notify.LogErr(fmt.Sprintf("container list failed: %v", err))
			r.problemState.Open(sig)
		}
		return
	}
	r.problemState.Close(notify.ProblemSignature("docker-list", "*"))

	reg := state.BuildRegistry(containers, r.cfg.DomainSuffix(),
		func(port uint16) bool {
			return r.dockerClient.ProbePort(ctx, port)
		}, 0, false)

	for _, b := range reg.Backends {
		if b.Status == state.StatusMisconfig {
			notify.NotifyMisconfig(b.ContainerName, b.StatusReason)
		}
	}

	current, err := proxy.ReadJSON(r.cfg.JSONPath())
	if err != nil {
		sig := notify.ProblemSignature("read-json", "*")
		if !r.problemState.IsOpen(sig) {
			notify.NotifyFailure(fmt.Sprintf("read reverseproxy.json: %v", err))
			notify.LogErr(fmt.Sprintf("read reverseproxy.json: %v", err))
			r.problemState.Open(sig)
		}
		return
	}
	r.problemState.Close(notify.ProblemSignature("read-json", "*"))

	accessID, _ := proxy.DiscoverLocalAccessProfile(r.cfg.AccessProfilePath())

	addresses, err := mdns.DiscoverLANAddresses()
	hasLAN := err == nil

	nasAddrSet := make(map[string]bool)
	if hasLAN {
		for _, a := range addresses {
			nasAddrSet[a] = true
		}
	}

	var desired []proxy.DesiredRule
	for _, b := range reg.Backends {
		if b.Status != state.StatusValid {
			continue
		}

		primaryHostname := b.Hostnames[0]
		aliases := b.Hostnames[1:]

		isPrimaryExternal, _ := mdns.IsHostnamePublishedByExternal(primaryHostname, addresses)
		if isPrimaryExternal {
			notify.NotifyMisconfig(b.ContainerName, fmt.Sprintf(
				"primary hostname %q collides with external mDNS, skipping entire container", primaryHostname))
			continue
		}

		if proxy.IsUnmanagedServerName(current, primaryHostname, r.cfg.AccessProfilePath()) {
			notify.NotifyMisconfig(b.ContainerName, fmt.Sprintf(
				"primary hostname %q collides with unmanaged proxy entry, skipping", primaryHostname))
			continue
		}

		desired = append(desired, proxy.DesiredRule{
			ContainerName: b.ContainerName,
			Hostname:      primaryHostname,
			Port:          b.Port,
			ListenPort:    r.cfg.ListenPort(),
			AccessID:      accessID,
		})

		for _, alias := range aliases {
			isAliasExternal, _ := mdns.IsHostnamePublishedByExternal(alias, addresses)
			if isAliasExternal {
				notify.NotifyMisconfig(b.ContainerName, fmt.Sprintf(
					"alias %q collides with external mDNS, skipping alias", alias))
				continue
			}

			if proxy.IsUnmanagedServerName(current, alias, r.cfg.AccessProfilePath()) {
				notify.NotifyMisconfig(b.ContainerName, fmt.Sprintf(
					"alias %q collides with unmanaged proxy entry, skipping", alias))
				continue
			}

			desired = append(desired, proxy.DesiredRule{
				ContainerName: b.ContainerName,
				Hostname:      alias,
				Port:          b.Port,
				ListenPort:    r.cfg.ListenPort(),
				AccessID:      accessID,
				IsAlias:       true,
			})
		}
	}

	updated := proxy.RenderAndMerge(current, desired, accessID)

	result := r.proxyMgr.Sync(current, updated, r.cfg)
	sig := notify.ProblemSignature("proxy-sync", "*")
	if !result.Success {
		if !r.problemState.IsOpen(sig) {
			notify.NotifyFailure(fmt.Sprintf("proxy sync failed: %s", result.Error))
			notify.LogErr(fmt.Sprintf("proxy sync failed: %s", result.Error))
			r.problemState.Open(sig)
		}
		return
	}
	r.problemState.Close(sig)

	for _, d := range desired {
		if !proxy.EntryExists(current, d.ContainerName, d.Hostname) {
			notify.NotifyAudit(fmt.Sprintf(
				"reverse proxy entry created: %s → %s:%d (container: %s)",
				d.Hostname, "localhost", d.Port, d.ContainerName))
		}
	}

	if !hasLAN {
		sig := notify.ProblemSignature("lan-addrs", "*")
		if !r.problemState.IsOpen(sig) {
			notify.NotifyFailure(fmt.Sprintf("LAN address discovery: %v", err))
			r.problemState.Open(sig)
		}
		return
	}
	r.problemState.Close(notify.ProblemSignature("lan-addrs", "*"))

	primaryAddr := addresses[0]
	publishMap := make(map[string][]string, len(desired))
	for _, d := range desired {
		publishMap[d.Hostname] = []string{primaryAddr}
	}

	if merr := r.publisher.Reconcile(publishMap, nil); merr != nil {
		notify.LogErr(fmt.Sprintf("mDNS reconciliation: %v", merr))
	}

	notify.LogInfo("reconciliation complete")
}
