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
}

func NewReconciler(dockerClient *dockerpkg.Client, publisher *mdns.Publisher, proxyMgr *proxy.Manager, cfg *ConfigAdapter) *Reconciler {
	return &Reconciler{
		dockerClient: dockerClient,
		publisher:    publisher,
		proxyMgr:     proxyMgr,
		cfg:          cfg,
		triggerCh:    make(chan struct{}, 1),
		stopCh:       make(chan struct{}),
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
				fmt.Printf("qnap-docker-mdns: docker event error: %v\n", err)
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

	jitter := time.Duration(rand.Int63n(int64(r.cfg.FullRescanInterval())))
	select {
	case <-r.stopCh:
		return
	case <-time.After(jitter):
	}

	ticker := time.NewTicker(r.cfg.FullRescanInterval())
	defer ticker.Stop()

	r.Reconcile(ctx)

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

	fmt.Println("qnap-docker-mdns: reconciliation starting")

	containers, err := r.dockerClient.ListRunningContainers(ctx)
	if err != nil {
		fmt.Printf("qnap-docker-mdns: container list error: %v\n", err)
		return
	}

	reg := state.BuildRegistry(containers, r.cfg.DomainSuffix(),
		func(port uint16) bool {
			return r.dockerClient.ProbePort(ctx, port)
		}, 0, false)

	current, err := proxy.ReadJSON(r.cfg.JSONPath())
	if err != nil {
		fmt.Printf("qnap-docker-mdns: read json error: %v\n", err)
		return
	}

	accessID, _ := proxy.DiscoverLocalAccessProfile(r.cfg.AccessProfilePath())

	var desired []proxy.DesiredRule
	for _, b := range reg.Backends {
		if b.Status != state.StatusValid {
			continue
		}
		for _, hostname := range b.Hostnames {
			desired = append(desired, proxy.DesiredRule{
				ContainerName: b.ContainerName,
				Hostname:      hostname,
				Port:          b.Port,
				ListenPort:    r.cfg.ListenPort(),
				AccessID:      accessID,
			})
		}
	}

	updated := proxy.RenderAndMerge(current, desired, accessID)

	result := r.proxyMgr.Sync(current, updated, r.cfg)
	if !result.Success {
		fmt.Printf("qnap-docker-mdns: proxy sync failed: %s\n", result.Error)
		return
	}

	addresses, err := mdns.DiscoverLANAddresses()
	if err != nil {
		fmt.Printf("qnap-docker-mdns: LAN address discovery failed: %v\n", err)
		return
	}

	publishMap := make(map[string][]string)
	for _, d := range desired {
		publishMap[d.Hostname] = addresses
	}

	if err := r.publisher.Reconcile(publishMap, nil); err != nil {
		fmt.Printf("qnap-docker-mdns: mDNS reconciliation error: %v\n", err)
	}

	fmt.Println("qnap-docker-mdns: reconciliation complete")
}
