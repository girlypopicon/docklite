package docker

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
)

// Site containers publish their port on 127.0.0.1 only, and nginx's proxy_pass names that port.
// A random port (what "0" asks for) is re-drawn every time the container starts, so nginx goes stale on
// any restart. Instead each site gets a fixed port from a range, stored in the container's own settings,
// so it never changes. Existing containers that still have a random port are kept in line by the
// upstream sync in the agent.

const defaultPortRange = "20000-29999"

// portAllocMu serializes "choose a port + create the container", so two sites made at once can't pick the same one.
var portAllocMu sync.Mutex

// PortRange is the span of host ports DockLite hands out to sites (DOCKLITE_PORT_RANGE, default 20000-29999).
func PortRange() (lo, hi int) {
	v := strings.TrimSpace(os.Getenv("DOCKLITE_PORT_RANGE"))
	if v == "" {
		v = defaultPortRange
	}
	parts := strings.SplitN(v, "-", 2)
	if len(parts) == 2 {
		a, e1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		b, e2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		if e1 == nil && e2 == nil && a >= 1024 && b <= 65535 && a < b {
			return a, b
		}
	}
	return 20000, 29999
}

// pickPort returns the lowest port in [lo, hi] that is not in used. Pure, so it can be tested.
func pickPort(lo, hi int, used map[int]bool) (int, bool) {
	for p := lo; p <= hi; p++ {
		if !used[p] {
			return p, true
		}
	}
	return 0, false
}

func portListening(p int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
	if err != nil {
		return true
	}
	_ = l.Close()
	return false
}

// usedHostPorts is every host port a container (running or stopped) is configured to publish, so we never
// hand out a port a stopped container would claim again when it starts.
func (c *Client) usedHostPorts(ctx context.Context) (map[int]bool, error) {
	used := map[int]bool{}
	list, err := c.Client.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}
	for _, item := range list {
		info, err := c.Client.ContainerInspect(ctx, item.ID)
		if err != nil || info.HostConfig == nil {
			continue
		}
		for _, bindings := range info.HostConfig.PortBindings {
			for _, b := range bindings {
				if n, err := strconv.Atoi(b.HostPort); err == nil && n > 0 {
					used[n] = true
				}
			}
		}
	}
	return used, nil
}

// allocateHostPort picks a free port in the range. The caller must hold portAllocMu until the container exists.
func (c *Client) allocateHostPort(ctx context.Context) (int, error) {
	used, err := c.usedHostPorts(ctx)
	if err != nil {
		return 0, err
	}
	lo, hi := PortRange()
	for p := lo; p <= hi; p++ {
		if used[p] {
			continue
		}
		if portListening(p) {
			used[p] = true
			continue
		}
		return p, nil
	}
	return 0, fmt.Errorf("no free port left in %d-%d", lo, hi)
}

// WatchSiteStarts sends on the returned channel whenever a DockLite-managed container starts, until ctx ends.
// Events are coalesced: a burst (e.g. a reboot starting twenty sites) produces a short run of signals.
func (c *Client) WatchSiteStarts(ctx context.Context) <-chan struct{} {
	out := make(chan struct{}, 1)
	go func() {
		defer close(out)
		backoff := time.Second
		for ctx.Err() == nil {
			args := filters.NewArgs(
				filters.Arg("type", "container"),
				filters.Arg("event", "start"),
				filters.Arg("label", "docklite.managed=true"),
			)
			msgs, errs := c.Client.Events(ctx, events.ListOptions{Filters: args})
		loop:
			for {
				select {
				case <-ctx.Done():
					return
				case <-msgs:
					backoff = time.Second
					select {
					case out <- struct{}{}:
					default:
					}
				case <-errs:
					break loop
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
		}
	}()
	return out
}
