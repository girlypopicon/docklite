package docker

import (
	"context"
	"docklite-agent/internal/demo"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"docklite-agent/internal/models"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
)

const (
	dockliteNetworkName = "docklite_network"
	staticImageName     = "nginx:alpine"
	phpImageName        = "webdevops/php-nginx:8.2-alpine"
	nodeImageName       = "node:20-alpine"
)

// VisibleContainers lists containers this DockLite instance should see. A demo instance
// sees only demo containers and a real one never sees them; every listing goes through here.
func (c *Client) VisibleContainers(ctx context.Context, all bool) ([]types.Container, error) {
	list, err := c.Client.ContainerList(ctx, container.ListOptions{All: all})
	if err != nil {
		return nil, err
	}
	out := list[:0:0]
	for _, item := range list {
		if (item.Labels[demo.Label] == "1") == demo.On {
			out = append(out, item)
		}
	}
	return out, nil
}

func (c *Client) ListContainers(ctx context.Context, all bool) ([]models.ContainerInfo, error) {
	containers, err := c.VisibleContainers(ctx, all)
	if err != nil {
		return nil, err
	}
	results := make([]models.ContainerInfo, 0, len(containers))
	for _, item := range containers {
		name := ""
		if len(item.Names) > 0 {
			name = strings.TrimPrefix(item.Names[0], "/")
		}
		uptime := "-"
		if item.State == "running" && item.Created > 0 {
			// Time since the last start (not creation), falling back to creation.
			since := time.Unix(item.Created, 0)
			if info, err := c.Client.ContainerInspect(ctx, item.ID); err == nil && info.State != nil {
				if started, err := time.Parse(time.RFC3339Nano, info.State.StartedAt); err == nil && started.Year() > 1 {
					since = started
				}
			}
			uptime = formatUptime(time.Since(since))
		}
		results = append(results, models.ContainerInfo{
			ID:      item.ID,
			Name:    name,
			Image:   item.Image,
			Created: item.Created,
			State:   item.State,
			Status:  item.Status,
			Uptime:  uptime,
			Ports:   formatPorts(item.Ports),
			Labels:  item.Labels,
		})
	}
	return results, nil
}

func (c *Client) StartContainer(ctx context.Context, id string) error {
	return c.Client.ContainerStart(ctx, id, container.StartOptions{})
}

func (c *Client) StopContainer(ctx context.Context, id string) error {
	return c.Client.ContainerStop(ctx, id, container.StopOptions{})
}

func (c *Client) RestartContainer(ctx context.Context, id string) error {
	return c.Client.ContainerRestart(ctx, id, container.StopOptions{})
}

func (c *Client) ContainerLogs(ctx context.Context, id string, tail string) (string, error) {
	options := container.LogsOptions{ShowStdout: true, ShowStderr: true, Tail: tail}
	reader, err := c.Client.ContainerLogs(ctx, id, options)
	if err != nil {
		return "", err
	}
	defer reader.Close()
	bytes, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (c *Client) ContainerStats(ctx context.Context, id string) (*models.ContainerStats, error) {
	stats, err := c.Client.ContainerStatsOneShot(ctx, id)
	if err != nil {
		return nil, err
	}
	defer stats.Body.Close()
	decoded, err := decodeStats(stats)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func decodeStats(response types.ContainerStats) (*models.ContainerStats, error) {
	var stats types.StatsJSON
	if err := json.NewDecoder(response.Body).Decode(&stats); err != nil {
		return nil, err
	}
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	cpuPercent := 0.0
	if systemDelta > 0 && cpuDelta > 0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(len(stats.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}
	memoryUsage := stats.MemoryStats.Usage
	memoryLimit := stats.MemoryStats.Limit
	memoryPercent := 0.0
	if memoryLimit > 0 {
		memoryPercent = (float64(memoryUsage) / float64(memoryLimit)) * 100.0
	}
	return &models.ContainerStats{
		CPUUsage:    cpuPercent,
		MemoryUsage: memoryUsage,
		MemoryLimit: memoryLimit,
		MemoryPct:   memoryPercent,
	}, nil
}

func (c *Client) InspectContainer(ctx context.Context, id string) (types.ContainerJSON, error) {
	return c.Client.ContainerInspect(ctx, id)
}

func (c *Client) RemoveContainer(ctx context.Context, id string) error {
	return c.Client.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
}

func (c *Client) CreateSiteContainer(ctx context.Context, domain string, templateType string, includeWww bool, sitePath string, port int, siteID int64, userID int64, folderID *int64) (string, error) {
	if templateType == "" {
		templateType = "static"
	}
	if err := c.ensureNetwork(ctx, dockliteNetworkName); err != nil {
		return "", err
	}

	// Node containers run as the agent's UID:GID so files they write into the
	// bind-mount stay owned by 'docklite' on the host.
	// Static (nginx) and PHP containers must start as root (port 80 / supervisord),
	// so we leave their User field empty.
	agentUser := fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())

	imageName := staticImageName
	internalPort := 80
	bindTarget := "/usr/share/nginx/html"
	readOnly := true
	var env []string
	var cmd []string
	workingDir := ""
	containerUser := ""

	switch templateType {
	case "php":
		imageName = phpImageName
		bindTarget = "/app"
		readOnly = false
		env = []string{
			"WEB_DOCUMENT_ROOT=/app",
			"PHP_DISPLAY_ERRORS=1",
			"PHP_MEMORY_LIMIT=256M",
			"PHP_MAX_EXECUTION_TIME=300",
			"PHP_POST_MAX_SIZE=50M",
			"PHP_UPLOAD_MAX_FILESIZE=50M",
		}
	case "node":
		imageName = nodeImageName
		bindTarget = "/app"
		readOnly = false
		internalPort = port
		if internalPort <= 0 {
			internalPort = 3000
		}
		env = []string{
			"NODE_ENV=production",
			fmt.Sprintf("PORT=%d", internalPort),
			"HOST=0.0.0.0",
		}
		cmd = []string{"node", "index.js"}
		workingDir = "/app"
		containerUser = agentUser
	case "static":
	default:
		return "", fmt.Errorf("unsupported template type: %s", templateType)
	}

	if err := c.pullImage(ctx, imageName); err != nil {
		return "", err
	}

	sanitized := sanitizeDomain(domain)
	labels := buildSiteLabels(domain, includeWww, templateType, internalPort, siteID, userID, folderID)
	portKey := nat.Port(fmt.Sprintf("%d/tcp", internalPort))
	bindMode := "ro"
	if !readOnly {
		bindMode = "rw"
	}

	config := &container.Config{
		Image:        imageName,
		Labels:       labels,
		ExposedPorts: nat.PortSet{portKey: struct{}{}},
		Env:          env,
		Cmd:          cmd,
		WorkingDir:   workingDir,
		User:         containerUser,
	}
	portAllocMu.Lock()
	defer portAllocMu.Unlock()
	hostPortValue := "0"
	if p, perr := c.allocateHostPort(ctx); perr == nil {
		hostPortValue = strconv.Itoa(p)
	}
	hostConfig := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:%s:%s", sitePath, bindTarget, bindMode),
		},
		PortBindings: nat.PortMap{
			// Loopback only: sites are reached through nginx, never directly. A fixed port (see ports.go) so nginx
			// never goes stale; "0" (random) is only the fallback if no port in the range is free.
			portKey: []nat.PortBinding{{HostIP: "127.0.0.1", HostPort: hostPortValue}},
		},
		RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
		NetworkMode:   container.NetworkMode(dockliteNetworkName),
	}

	containerName := fmt.Sprintf("docklite-site-%s", sanitized)
	resp, err := c.Client.ContainerCreate(ctx, config, hostConfig, nil, nil, containerName)
	if err != nil && strings.Contains(err.Error(), "Conflict.") {
		containerName = fmt.Sprintf("docklite-site-%s-%d", sanitized, time.Now().Unix())
		resp, err = c.Client.ContainerCreate(ctx, config, hostConfig, nil, nil, containerName)
	}
	if err != nil {
		return "", err
	}
	if err := c.Client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", err
	}
	return resp.ID, nil
}

// PrewarmImages pulls the standard site images in the background so they are
// cached before the first container creation request arrives. This prevents
// long image pulls from blocking an HTTP handler and causing proxy timeouts.
func (c *Client) PrewarmImages(ctx context.Context) {
	images := []string{staticImageName, phpImageName, nodeImageName}
	for _, img := range images {
		go func(name string) {
			pullCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
			defer cancel()
			if err := c.pullImage(pullCtx, name); err != nil {
				// Non-fatal — the image will be pulled on first use instead.
				_ = err
			}
		}(img)
	}
}

func (c *Client) pullImage(ctx context.Context, imageName string) error {
	// Check if image already exists locally to avoid a network round-trip.
	if c.imageExists(ctx, imageName) {
		return nil
	}
	// Use an independent long-timeout context so the pull is not bound to
	// the lifetime of an incoming HTTP request.
	pullCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	reader, err := c.Client.ImagePull(pullCtx, imageName, image.PullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()
	_, _ = io.Copy(io.Discard, reader)
	return nil
}

func (c *Client) imageExists(ctx context.Context, imageName string) bool {
	images, err := c.Client.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return false
	}
	for _, img := range images {
		for _, tag := range img.RepoTags {
			if tag == imageName || tag == imageName+":latest" {
				return true
			}
		}
	}
	return false
}

func (c *Client) ensureNetwork(ctx context.Context, name string) error {
	networks, err := c.Client.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		return err
	}
	for _, network := range networks {
		if network.Name == name {
			return nil
		}
	}
	_, err = c.Client.NetworkCreate(ctx, name, network.CreateOptions{})
	return err
}

func sanitizeDomain(domain string) string {
	var b strings.Builder
	for _, r := range domain {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "site"
	}
	return strings.ToLower(result)
}

func formatUptime(duration time.Duration) string {
	seconds := int64(duration.Seconds())
	if seconds < 0 {
		seconds = 0
	}
	minutes := seconds / 60
	hours := minutes / 60
	days := hours / 24

	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hours%24)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes%60)
	case minutes > 0:
		return fmt.Sprintf("%dm", minutes)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}

func formatPorts(ports []types.Port) string {
	if len(ports) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(ports))
	for _, port := range ports {
		if port.PublicPort == 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%d->%d", port.PublicPort, port.PrivatePort))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

func buildSiteLabels(domain string, includeWww bool, templateType string, internalPort int, siteID int64, userID int64, folderID *int64) map[string]string {
	folderValue := ""
	if folderID != nil {
		folderValue = fmt.Sprintf("%d", *folderID)
	}
	labels := map[string]string{
		"docklite.managed":       "true",
		"docklite.site.id":       fmt.Sprintf("%d", siteID),
		"docklite.domain":        domain,
		"docklite.type":          templateType,
		"docklite.user.id":       fmt.Sprintf("%d", userID),
		"docklite.folder.id":     folderValue,
		"docklite.include_www":   boolToLabel(includeWww),
		"docklite.internal_port": fmt.Sprintf("%d", internalPort),
	}
	if demo.On {
		labels[demo.Label] = "1"
	}
	return labels
}

func boolToLabel(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
