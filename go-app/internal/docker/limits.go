package docker

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
)

// Limits caps what one container may use. Zero means "no limit" for that resource.
type Limits struct {
	CPUs     float64 `json:"cpus"`     // e.g. 0.5 = half a processor core
	MemoryMB int64   `json:"memoryMB"` // memory including swap is held to this (no swapping past it)
	Pids     int64   `json:"pids"`     // most processes/threads at once; stops a fork bomb
}

// None reports whether nothing is limited.
func (l Limits) None() bool { return l.CPUs <= 0 && l.MemoryMB <= 0 && l.Pids <= 0 }

// HostResources is what this server has, so limits can be checked and shown as a share of it.
type HostResources struct {
	CPUs     int   `json:"cpus"`
	MemoryMB int64 `json:"memoryMB"`
}

func Host() HostResources {
	h := HostResources{CPUs: runtime.NumCPU()}
	if f, err := os.Open("/proc/meminfo"); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if strings.HasPrefix(sc.Text(), "MemTotal:") {
				fields := strings.Fields(sc.Text())
				if len(fields) >= 2 {
					kb, _ := strconv.ParseInt(fields[1], 10, 64)
					h.MemoryMB = kb / 1024
				}
				break
			}
		}
	}
	return h
}

// Validate says what is wrong with l in plain words, or returns nil.
func (l Limits) Validate(host HostResources) error {
	if l.CPUs < 0 || l.MemoryMB < 0 || l.Pids < 0 {
		return fmt.Errorf("limits can't be negative")
	}
	if l.CPUs > 0 && l.CPUs < 0.05 {
		return fmt.Errorf("CPU: use at least 0.05 of a core (or leave it empty for no limit)")
	}
	if host.CPUs > 0 && l.CPUs > float64(host.CPUs) {
		return fmt.Errorf("CPU: this server only has %d core(s)", host.CPUs)
	}
	if l.MemoryMB > 0 && l.MemoryMB < 16 {
		return fmt.Errorf("memory: use at least 16 MB (most sites need much more)")
	}
	if host.MemoryMB > 0 && l.MemoryMB > host.MemoryMB {
		return fmt.Errorf("memory: this server only has %d MB", host.MemoryMB)
	}
	if l.Pids > 0 && l.Pids < 16 {
		return fmt.Errorf("processes: use at least 16 (or leave it empty for no limit)")
	}
	if l.Pids > 4194304 {
		return fmt.Errorf("processes: that is more than Linux allows")
	}
	return nil
}

// Resources turns limits into Docker's settings (used when a container is created).
func (l Limits) Resources() container.Resources {
	r := container.Resources{}
	if l.CPUs > 0 {
		r.NanoCPUs = int64(l.CPUs * 1e9)
	}
	if l.MemoryMB > 0 {
		r.Memory = l.MemoryMB << 20
		r.MemorySwap = r.Memory // same number: no swap on top
	}
	if l.Pids > 0 {
		p := l.Pids
		r.PidsLimit = &p
	}
	return r
}

// updateResources is the same for a running container. Docker's update treats 0 as "leave as is", so "no limit" is
// spelled out: all the server's cores, all its memory with unlimited swap, and -1 processes.
func (l Limits) updateResources(host HostResources) container.Resources {
	r := l.Resources()
	if l.CPUs <= 0 {
		r.NanoCPUs = int64(host.CPUs) * 1e9
	}
	if l.MemoryMB <= 0 {
		r.Memory = host.MemoryMB << 20
		r.MemorySwap = -1
	}
	if l.Pids <= 0 {
		m := int64(-1)
		r.PidsLimit = &m
	}
	return r
}

// LimitsProvider tells container creation what limits a container of that name should get (nil = none), so a
// container that is re-created (a transfer, a type switch) keeps its limits.
type LimitsProvider func(containerName string) *Limits

// SetLimitsProvider installs the provider; set once at startup.
func (c *Client) SetLimitsProvider(p LimitsProvider) { c.limitsFor = p }

func (c *Client) applyStoredLimits(name string, hc *container.HostConfig) {
	if c.limitsFor == nil {
		return
	}
	if l := c.limitsFor(name); l != nil && !l.None() {
		hc.Resources = l.Resources()
	}
}

// UpdateLimits changes a container's limits right away, without restarting it.
func (c *Client) UpdateLimits(ctx context.Context, id string, l Limits) error {
	_, err := c.Client.ContainerUpdate(ctx, id, container.UpdateConfig{Resources: l.updateResources(Host())})
	return err
}

// CurrentLimits reads a container's limits from Docker.
func (c *Client) CurrentLimits(ctx context.Context, id string) (Limits, error) {
	info, err := c.Client.ContainerInspect(ctx, id)
	if err != nil {
		return Limits{}, err
	}
	var l Limits
	if info.HostConfig == nil {
		return l, nil
	}
	hr := Host()
	hc := info.HostConfig
	if hc.NanoCPUs > 0 && (hr.CPUs == 0 || hc.NanoCPUs < int64(hr.CPUs)*1e9) {
		l.CPUs = float64(hc.NanoCPUs) / 1e9
	}
	if hc.Memory > 0 && (hr.MemoryMB == 0 || hc.Memory>>20 < hr.MemoryMB) {
		l.MemoryMB = hc.Memory >> 20
	}
	if hc.PidsLimit != nil && *hc.PidsLimit > 0 {
		l.Pids = *hc.PidsLimit
	}
	return l, nil
}
