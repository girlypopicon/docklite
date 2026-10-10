package handlers

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"docklite-agent/internal/demo"
)

// Keeps nginx pointed at the port each site container is really on. A site container publishes its port on
// 127.0.0.1:<port> and nginx's proxy_pass names that port; if they ever differ, visitors get a 502. This runs
// when the agent starts, whenever Docker starts a managed container (a reboot, a Docker restart, a crash and
// automatic restart, a manual `docker restart`), and once a minute as a backstop. It only ever changes the
// port number inside a proxy_pass that already points at a local upstream, in the server block that names
// the site, and nginx is only reloaded if `nginx -t` accepts the change.

var upstreamSyncMu sync.Mutex

type upstreamBlock struct {
	File  string
	Names []string
	Ports []int
}

type upstreamFix struct {
	File   string `json:"file"`
	Domain string `json:"domain"`
	Old    int    `json:"old"`
	New    int    `json:"new"`
}

// parseUpstreamBlocks reads the helper's output: FILE <TAB> server names <TAB> proxy ports, one server block per line.
func parseUpstreamBlocks(out string) []upstreamBlock {
	var blocks []upstreamBlock
	for _, line := range strings.Split(out, "\n") {
		cols := strings.Split(line, "\t")
		if len(cols) < 3 || strings.TrimSpace(cols[0]) == "" {
			continue
		}
		b := upstreamBlock{File: strings.TrimSpace(cols[0])}
		for _, n := range strings.Fields(cols[1]) {
			b.Names = append(b.Names, strings.ToLower(n))
		}
		for _, p := range strings.Fields(cols[2]) {
			if n, err := strconv.Atoi(p); err == nil {
				b.Ports = append(b.Ports, n)
			}
		}
		blocks = append(blocks, b)
	}
	return blocks
}

// planUpstreamFixes decides what to change. desired maps a site's domain to the port its container is on now.
// Blocks with more than one distinct upstream (e.g. /api elsewhere) are left alone: we can't tell which to move.
//
// moved maps a port some container used to be on to the port it is on now. It catches blocks whose server_name is an
// alias of the site (a domain served by another site's container), which a name match can't.
func planUpstreamFixes(blocks []upstreamBlock, desired map[string]int, moved map[int]int) (fixes []upstreamFix, seen map[string]bool) {
	seen = map[string]bool{}
	done := map[string]bool{}
	for _, b := range blocks {
		matched := false
		for _, name := range b.Names {
			domain := strings.TrimPrefix(name, "www.")
			want, ok := desired[domain]
			if !ok {
				continue
			}
			seen[domain] = true
			matched = true
			distinct := map[int]bool{}
			for _, p := range b.Ports {
				distinct[p] = true
			}
			if len(distinct) != 1 {
				break
			}
			old := b.Ports[0]
			if old == want {
				break
			}
			key := b.File + "|" + domain + "|" + strconv.Itoa(old)
			if !done[key] {
				done[key] = true
				fixes = append(fixes, upstreamFix{File: b.File, Domain: domain, Old: old, New: want})
			}
			break
		}
		if matched || len(b.Names) == 0 {
			continue
		}
		// No container claims any of this block's names. If its only upstream is a port a container has since left,
		// follow that container.
		distinct := map[int]bool{}
		for _, p := range b.Ports {
			distinct[p] = true
		}
		if len(distinct) != 1 {
			continue
		}
		old := b.Ports[0]
		if now, ok := moved[old]; ok && now != old {
			name := strings.TrimPrefix(b.Names[0], "www.")
			key := b.File + "|" + name + "|" + strconv.Itoa(old)
			if !done[key] && name != "_" && !strings.ContainsAny(name, "*~") {
				done[key] = true
				fixes = append(fixes, upstreamFix{File: b.File, Domain: name, Old: old, New: now})
			}
		}
	}
	return fixes, seen
}

type sitePort struct {
	Name   string
	Domain string
	Port   int
}

// runningSitePorts lists every running managed site container with the host port it is published on right now.
func (h *Handlers) runningSitePorts(ctx context.Context) []sitePort {
	var out []sitePort
	containers, err := h.docker.ListContainers(ctx, false)
	if err != nil {
		return nil
	}
	for _, c := range containers {
		if c.State != "running" || c.Labels["docklite.managed"] != "true" {
			continue
		}
		domain := strings.ToLower(c.Labels["docklite.domain"])
		if domain == "" || c.Labels["docklite.type"] == "postgres" {
			continue
		}
		internal := 80
		if p, err := strconv.Atoi(c.Labels["docklite.internal_port"]); err == nil && p > 0 {
			internal = p
		}
		hostPort, err := h.getContainerHostPort(ctx, c.ID, internal)
		if err != nil || hostPort <= 0 {
			continue
		}
		out = append(out, sitePort{Name: c.Name, Domain: domain, Port: hostPort})
	}
	return out
}

// desiredUpstreams maps each site's domain to the port its container is on now. A domain claimed by more than
// one running container is skipped (we can't know which is right).
func desiredUpstreams(sites []sitePort) map[string]int {
	out := map[string]int{}
	clash := map[string]bool{}
	for _, s := range sites {
		if _, dup := out[s.Domain]; dup {
			clash[s.Domain] = true
		}
		out[s.Domain] = s.Port
	}
	for d := range clash {
		delete(out, d)
	}
	return out
}

// movedPorts: for containers whose port changed since we last looked, old port -> new port. A port that some
// running container is on now is never a "from" (it is in use, not abandoned).
func movedPorts(remembered map[string]int, sites []sitePort) map[int]int {
	now := map[string]int{}
	inUse := map[int]bool{}
	for _, s := range sites {
		now[s.Name] = s.Port
		inUse[s.Port] = true
	}
	moved := map[int]int{}
	for name, old := range remembered {
		if cur, ok := now[name]; ok && cur != old && !inUse[old] {
			moved[old] = cur
		}
	}
	return moved
}

const upstreamMemoryFile = "data/upstream-ports.json"

func loadPortMemory() map[string]int {
	m := map[string]int{}
	if data, err := os.ReadFile(upstreamMemoryFile); err == nil {
		_ = json.Unmarshal(data, &m)
	}
	return m
}

func savePortMemory(m map[string]int) {
	if data, err := json.Marshal(m); err == nil {
		_ = os.MkdirAll(filepath.Dir(upstreamMemoryFile), 0o750)
		_ = os.WriteFile(upstreamMemoryFile, data, 0o640)
	}
}

// SyncUpstreams fixes every stale proxy port it can find. It returns what it changed, and the domains that
// have a running container but no nginx server block at all (the caller may want to create one).
func (h *Handlers) SyncUpstreams(ctx context.Context) (fixed []upstreamFix, missing []string, err error) {
	upstreamSyncMu.Lock()
	defer upstreamSyncMu.Unlock()
	sites := h.runningSitePorts(ctx)
	if len(sites) == 0 {
		return nil, nil, nil
	}
	desired := desiredUpstreams(sites)
	remembered := loadPortMemory()
	moved := movedPorts(remembered, sites)
	listing, err := runRootHelper(nil, "nginx-upstreams")
	if err != nil {
		return nil, nil, err
	}
	fixes, seen := planUpstreamFixes(parseUpstreamBlocks(string(listing)), desired, moved)
	// Remember where everything is now, so the next change (even across a reboot) can be followed.
	for _, s := range sites {
		remembered[s.Name] = s.Port
	}
	savePortMemory(remembered)
	for d := range desired {
		if !seen[d] {
			missing = append(missing, d)
		}
	}
	for _, f := range fixes {
		msg, ferr := runRootHelper(nil, "nginx-fix-port", f.File, f.Domain, strconv.Itoa(f.Old), strconv.Itoa(f.New))
		detail := map[string]any{"file": f.File, "old": f.Old, "new": f.New, "ok": ferr == nil}
		if ferr != nil {
			detail["error"] = strings.TrimSpace(string(msg))
			log.Printf("upstream sync: could not fix %s in %s: %s", f.Domain, f.File, strings.TrimSpace(string(msg)))
		} else {
			fixed = append(fixed, f)
			log.Printf("upstream sync: %s now proxies to port %d (was %d)", f.Domain, f.New, f.Old)
		}
		auditSystem("nginx.upstream-sync", f.Domain, detail)
	}
	return fixed, missing, nil
}

// RunUpstreamSync runs the sync for the life of the agent.
func (h *Handlers) RunUpstreamSync(ctx context.Context) {
	if demo.On || h.docker == nil {
		return
	}
	starts := h.docker.WatchSiteStarts(ctx)
	run := func() {
		c, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		if _, _, err := h.SyncUpstreams(c); err != nil {
			log.Printf("upstream sync: %v", err)
		}
	}
	first := time.NewTimer(20 * time.Second) // let Docker finish starting everything after a boot
	defer first.Stop()
	tick := time.NewTicker(60 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-first.C:
			run()
		case <-tick.C:
			run()
		case _, ok := <-starts:
			if !ok {
				starts = nil
				continue
			}
			// A container started: give Docker a moment to publish its port, then sync once for the whole burst.
			if !first.Stop() {
				select {
				case <-first.C:
				default:
				}
			}
			first.Reset(3 * time.Second)
		}
	}
}
