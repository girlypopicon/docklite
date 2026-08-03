package handlers

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
)

type serverOverviewResponse struct {
	Hostname  string         `json:"hostname"`
	OS        string         `json:"os"`
	OSVersion string         `json:"osVersion"`
	Kernel    string         `json:"kernel"`
	Arch      string         `json:"arch"`
	CPUCount  int            `json:"cpuCount"`
	Uptime    float64        `json:"uptime"`
	LoadAvg   loadAvg        `json:"loadAvg"`
	Memory    memorySummary  `json:"memory"`
	Disk      diskSummary    `json:"disk"`
	TimeSync  timeSyncStatus `json:"timeSync"`
	ClockISO  string         `json:"clockIso"`
}

type loadAvg struct {
	One     float64 `json:"one"`
	Five    float64 `json:"five"`
	Fifteen float64 `json:"fifteen"`
}

type memorySummary struct {
	Total uint64 `json:"total"`
	Free  uint64 `json:"free"`
}

type diskSummary struct {
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
	Free  uint64 `json:"free"`
}

type timeSyncStatus struct {
	Status   string `json:"status"`
	Detail   string `json:"detail"`
	Timezone string `json:"timezone"`
}

type serverUpdatesResponse struct {
	PendingUpdates  int             `json:"pendingUpdates"`
	SecurityUpdates int             `json:"securityUpdates"`
	RebootRequired  bool            `json:"rebootRequired"`
	AutoUpdates     autoUpdateState `json:"autoUpdates"`
	Source          string          `json:"source"`
}

type autoUpdateState struct {
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type serviceStatus struct {
	Name             string `json:"name"`
	Kind             string `json:"kind"`
	Status           string `json:"status"`
	Detail           string `json:"detail"`
	StartedAt        string `json:"startedAt"`
	RestartSupported bool   `json:"restartSupported"`
	ReloadSupported  bool   `json:"reloadSupported"`
	LogsSupported    bool   `json:"logsSupported"`
}

type serverServicesResponse struct {
	Docker            serviceStatus  `json:"docker"`
	Docklite          *serviceStatus `json:"docklite"`
	DockliteSecondary *serviceStatus `json:"dockliteSecondary"`
	Proxy             *serviceStatus `json:"proxy"`
}

type mountUsage struct {
	Filesystem string `json:"filesystem"`
	Type       string `json:"type"`
	Size       uint64 `json:"size"`
	Used       uint64 `json:"used"`
	Available  uint64 `json:"available"`
	UsePercent int    `json:"usePercent"`
	Mountpoint string `json:"mountpoint"`
}

type dockerUsage struct {
	ImageCount      int   `json:"imageCount"`
	ContainerCount  int   `json:"containerCount"`
	VolumeCount     int   `json:"volumeCount"`
	BuildCacheCount int   `json:"buildCacheCount"`
	ImageSize       int64 `json:"imageSize"`
	ContainerSize   int64 `json:"containerSize"`
	VolumeSize      int64 `json:"volumeSize"`
	BuildCacheSize  int64 `json:"buildCacheSize"`
	TotalSize       int64 `json:"totalSize"`
}

type dockerVolumeInfo struct {
	Name      string `json:"name"`
	Driver    string `json:"driver"`
	Mount     string `json:"mount"`
	Size      int64  `json:"size"`
	RefCount  int64  `json:"refCount"`
	CreatedAt string `json:"createdAt"`
}

type serverStorageResponse struct {
	Mounts  []mountUsage       `json:"mounts"`
	Docker  dockerUsage        `json:"docker"`
	Volumes []dockerVolumeInfo `json:"volumes"`
}

type serverSecurityResponse struct {
	SSH          sshStatus        `json:"ssh"`
	SudoUsers    []string         `json:"sudoUsers"`
	FailedLogins failedLoginStats `json:"failedLogins"`
}

type sshStatus struct {
	Status string   `json:"status"`
	Unit   string   `json:"unit"`
	Ports  []string `json:"ports"`
}

type failedLoginStats struct {
	Count  int    `json:"count"`
	Latest string `json:"latest"`
	Source string `json:"source"`
}

type serverLogsResponse struct {
	Logs string `json:"logs"`
}

type serviceActionRequest struct {
	Service string `json:"service"`
	Action  string `json:"action"`
}

type storagePruneRequest struct {
	Target string `json:"target"`
}

type storagePruneResponse struct {
	Reclaimed uint64 `json:"reclaimed"`
	Deleted   int    `json:"deleted"`
}

func (h *Handlers) ServerOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	hostname, _ := os.Hostname()
	osName, osVersion := readOSRelease()
	kernel, _ := runCommandTimeout(2*time.Second, "uname", "-r")
	uptime := readUptime()
	totalMemory, freeMemory := readMemory()
	diskUsage := readDiskUsage()
	load := readLoadAvg()
	timeSync := readTimeSync()

	writeJSON(w, http.StatusOK, serverOverviewResponse{
		Hostname:  hostname,
		OS:        osName,
		OSVersion: osVersion,
		Kernel:    kernel,
		Arch:      runtime.GOARCH,
		CPUCount:  runtime.NumCPU(),
		Uptime:    uptime,
		LoadAvg:   load,
		Memory: memorySummary{
			Total: totalMemory,
			Free:  freeMemory,
		},
		Disk: diskSummary{
			Total: diskUsage.Total,
			Used:  diskUsage.Used,
			Free:  diskUsage.Free,
		},
		TimeSync: timeSync,
		ClockISO: time.Now().Format(time.RFC3339),
	})
}

func (h *Handlers) ServerUpdates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	pending, security, source := readAptUpdates()
	rebootRequired := fileExists("/var/run/reboot-required")
	autoUpdates := readUnattendedStatus()

	writeJSON(w, http.StatusOK, serverUpdatesResponse{
		PendingUpdates:  pending,
		SecurityUpdates: security,
		RebootRequired:  rebootRequired,
		AutoUpdates:     autoUpdates,
		Source:          source,
	})
}

func (h *Handlers) ServerServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	ctx, cancel := dockerContext(r.Context())
	defer cancel()

	dockerStatus := serviceStatus{
		Name:             "Docker Engine",
		Kind:             "docker",
		Status:           "unknown",
		Detail:           "",
		RestartSupported: false,
		ReloadSupported:  false,
		LogsSupported:    false,
	}
	if info, err := h.docker.Client.Info(ctx); err == nil {
		version, _ := h.docker.Client.ServerVersion(ctx)
		dockerStatus.Status = "running"
		dockerStatus.Detail = fmt.Sprintf("v%s, %d containers", version.Version, info.ContainersRunning)
	} else {
		dockerStatus.Status = "unavailable"
		dockerStatus.Detail = err.Error()
	}

	primary, secondary := h.detectDockliteService(ctx)
	proxy := h.detectProxyService(ctx)

	writeJSON(w, http.StatusOK, serverServicesResponse{
		Docker:            dockerStatus,
		Docklite:          primary,
		DockliteSecondary: secondary,
		Proxy:             proxy,
	})
}

func (h *Handlers) ServerServiceAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	var payload serviceActionRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	action := strings.ToLower(strings.TrimSpace(payload.Action))
	service := strings.ToLower(strings.TrimSpace(payload.Service))
	if action != "restart" && action != "reload" {
		writeError(w, http.StatusBadRequest, "invalid action")
		return
	}
	if service == "" {
		writeError(w, http.StatusBadRequest, "missing service")
		return
	}

	ctx, cancel := dockerContext(r.Context())
	defer cancel()

	switch service {
	case "docklite":
		if err := h.performDockliteAction(ctx, action); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	case "proxy", "traefik":
		if err := h.performProxyAction(ctx, action); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "unknown service")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *Handlers) ServerStorage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	mounts := readMountUsage()
	dockerUsage, volumes := h.readDockerUsage(r.Context())

	writeJSON(w, http.StatusOK, serverStorageResponse{
		Mounts:  mounts,
		Docker:  dockerUsage,
		Volumes: volumes,
	})
}

func (h *Handlers) ServerStoragePrune(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	var payload storagePruneRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx, cancel := dockerContext(r.Context())
	defer cancel()

	target := strings.ToLower(strings.TrimSpace(payload.Target))
	switch target {
	case "images":
		report, err := h.docker.Client.ImagesPrune(ctx, filters.NewArgs())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, storagePruneResponse{
			Reclaimed: report.SpaceReclaimed,
			Deleted:   len(report.ImagesDeleted),
		})
	case "build-cache":
		report, err := h.docker.Client.BuildCachePrune(ctx, types.BuildCachePruneOptions{All: true})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, storagePruneResponse{
			Reclaimed: report.SpaceReclaimed,
			Deleted:   len(report.CachesDeleted),
		})
	default:
		writeError(w, http.StatusBadRequest, "unknown prune target")
	}
}

func (h *Handlers) ServerSecurity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	ssh := readSSHStatus()
	sudoUsers := readSudoUsers()
	failed := readFailedLogins()

	writeJSON(w, http.StatusOK, serverSecurityResponse{
		SSH:          ssh,
		SudoUsers:    sudoUsers,
		FailedLogins: failed,
	})
}

func (h *Handlers) ServerLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	target := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("target")))
	tail := parseTail(r.URL.Query().Get("tail"), 200)

	var logs string
	var err error
	switch target {
	case "system":
		logs, err = readSystemLogs(tail)
	case "docklite":
		logs, err = h.readDockliteLogs(r.Context(), tail)
	case "proxy":
		logs, err = h.readProxyLogs(r.Context(), tail)
	default:
		writeError(w, http.StatusBadRequest, "unknown log target")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, serverLogsResponse{Logs: logs})
}

func (h *Handlers) ServerDiagnostics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	path, name, err := h.createDiagnosticsBundle(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", name))
	http.ServeFile(w, r, path)
}

func readOSRelease() (string, string) {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return runtime.GOOS, ""
	}
	values := map[string]string{}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, `"'`)
		values[key] = value
	}
	name := values["NAME"]
	version := values["VERSION_ID"]
	if version == "" {
		version = values["VERSION"]
	}
	if name == "" {
		name = values["PRETTY_NAME"]
	}
	return name, version
}

func readLoadAvg() loadAvg {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return loadAvg{}
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return loadAvg{}
	}
	one, _ := strconv.ParseFloat(fields[0], 64)
	five, _ := strconv.ParseFloat(fields[1], 64)
	fifteen, _ := strconv.ParseFloat(fields[2], 64)
	return loadAvg{One: one, Five: five, Fifteen: fifteen}
}

func readTimeSync() timeSyncStatus {
	status := timeSyncStatus{Status: "unknown", Detail: "unavailable"}
	if !commandExists("timedatectl") {
		return status
	}
	output, err := runCommandTimeout(2*time.Second, "timedatectl", "show", "-p", "NTPSynchronized", "-p", "NTP", "-p", "Timezone")
	if err != nil && output == "" {
		return status
	}
	fields := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		fields[parts[0]] = parts[1]
	}
	ntpSync := strings.ToLower(fields["NTPSynchronized"])
	ntp := strings.ToLower(fields["NTP"])
	status.Timezone = fields["Timezone"]
	switch {
	case ntpSync == "yes":
		status.Status = "synced"
		status.Detail = "ntp synchronized"
	case ntp == "no":
		status.Status = "disabled"
		status.Detail = "ntp disabled"
	case ntpSync == "no":
		status.Status = "unsynced"
		status.Detail = "ntp not synchronized"
	default:
		status.Status = "unknown"
		status.Detail = "unknown"
	}
	return status
}

func readAptUpdates() (int, int, string) {
	if !commandExists("apt-get") {
		return 0, 0, "unsupported"
	}
	output, err := runCommandTimeout(6*time.Second, "apt-get", "-s", "upgrade")
	if err != nil && output == "" {
		return 0, 0, "apt-get error"
	}
	pending := 0
	security := 0
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Inst ") {
			continue
		}
		pending++
		if strings.Contains(line, "security") {
			security++
		}
	}
	return pending, security, "apt-get -s upgrade"
}

func readUnattendedStatus() autoUpdateState {
	state := autoUpdateState{Status: "unknown", Detail: "systemctl missing"}
	if !commandExists("systemctl") {
		return state
	}
	output, err := runCommandTimeout(2*time.Second, "systemctl", "is-enabled", "unattended-upgrades")
	output = strings.TrimSpace(output)
	if err != nil && output == "" {
		return autoUpdateState{Status: "unknown", Detail: err.Error()}
	}
	switch output {
	case "enabled":
		return autoUpdateState{Status: "enabled", Detail: "unattended-upgrades enabled"}
	case "disabled":
		return autoUpdateState{Status: "disabled", Detail: "unattended-upgrades disabled"}
	case "static":
		return autoUpdateState{Status: "static", Detail: "unattended-upgrades static"}
	case "masked":
		return autoUpdateState{Status: "masked", Detail: "unattended-upgrades masked"}
	case "not-found":
		return autoUpdateState{Status: "not-installed", Detail: "unattended-upgrades not found"}
	default:
		if strings.Contains(output, "not-found") {
			return autoUpdateState{Status: "not-installed", Detail: "unattended-upgrades not found"}
		}
		if output == "" {
			output = "unknown"
		}
		return autoUpdateState{Status: output, Detail: "unattended-upgrades status"}
	}
}

func readSSHStatus() sshStatus {
	status := sshStatus{Status: "unknown", Unit: "", Ports: readSSHPorts()}
	if !commandExists("systemctl") {
		return status
	}
	for _, unit := range []string{"ssh", "sshd"} {
		output, err := runCommandTimeout(2*time.Second, "systemctl", "is-active", unit)
		output = strings.TrimSpace(output)
		if err != nil && output == "" {
			continue
		}
		if output == "active" {
			status.Status = "active"
			status.Unit = unit
			return status
		}
		if output != "" && output != "unknown" && output != "inactive" {
			status.Status = output
			status.Unit = unit
			return status
		}
		if output == "inactive" && status.Unit == "" {
			status.Status = "inactive"
			status.Unit = unit
		}
	}
	return status
}

func readSSHPorts() []string {
	ports := []string{}
	data, err := os.ReadFile("/etc/ssh/sshd_config")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) >= 2 && strings.EqualFold(fields[0], "Port") {
				ports = append(ports, fields[1])
			}
		}
	}
	if len(ports) == 0 {
		ports = []string{"22"}
	}
	return ports
}

func readSudoUsers() []string {
	users := map[string]struct{}{}
	data, err := os.ReadFile("/etc/group")
	if err != nil {
		return []string{}
	}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) < 4 {
			continue
		}
		group := parts[0]
		if group != "sudo" && group != "wheel" {
			continue
		}
		for _, user := range strings.Split(parts[3], ",") {
			user = strings.TrimSpace(user)
			if user != "" {
				users[user] = struct{}{}
			}
		}
	}
	list := make([]string, 0, len(users))
	for user := range users {
		list = append(list, user)
	}
	sort.Strings(list)
	return list
}

func readFailedLogins() failedLoginStats {
	candidates := []string{"/var/log/auth.log", "/var/log/secure"}
	for _, path := range candidates {
		if !fileExists(path) {
			continue
		}
		lines, err := readTailLines(path, 2000)
		if err != nil {
			continue
		}
		count := 0
		latest := ""
		for _, line := range lines {
			if strings.Contains(line, "Failed password") || strings.Contains(line, "authentication failure") {
				count++
				latest = line
			}
		}
		return failedLoginStats{
			Count:  count,
			Latest: latest,
			Source: filepath.Base(path),
		}
	}
	return failedLoginStats{Count: 0, Latest: "", Source: "unavailable"}
}

func readTailLines(path string, maxLines int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	size := stat.Size()
	if size == 0 {
		return []string{}, nil
	}
	readSize := int64(64 * 1024)
	if size < readSize {
		readSize = size
	}
	_, err = file.Seek(-readSize, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	buffer := make([]byte, readSize)
	if _, err := io.ReadFull(file, buffer); err != nil && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	content := strings.Split(string(buffer), "\n")
	if len(content) > maxLines {
		content = content[len(content)-maxLines:]
	}
	return content, nil
}

func readMountUsage() []mountUsage {
	if !commandExists("df") {
		return []mountUsage{}
	}
	output, err := runCommandTimeout(3*time.Second, "df", "-P", "-T", "-B1")
	if err != nil && output == "" {
		return []mountUsage{}
	}
	lines := strings.Split(output, "\n")
	if len(lines) <= 1 {
		return []mountUsage{}
	}
	mounts := make([]mountUsage, 0)
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}
		size, _ := strconv.ParseUint(fields[2], 10, 64)
		used, _ := strconv.ParseUint(fields[3], 10, 64)
		avail, _ := strconv.ParseUint(fields[4], 10, 64)
		percent := strings.TrimSuffix(fields[5], "%")
		percentVal, _ := strconv.Atoi(percent)
		mounts = append(mounts, mountUsage{
			Filesystem: fields[0],
			Type:       fields[1],
			Size:       size,
			Used:       used,
			Available:  avail,
			UsePercent: percentVal,
			Mountpoint: fields[6],
		})
	}
	sort.Slice(mounts, func(i, j int) bool {
		return mounts[i].Mountpoint < mounts[j].Mountpoint
	})
	return mounts
}

func (h *Handlers) readDockerUsage(ctx context.Context) (dockerUsage, []dockerVolumeInfo) {
	dockerCtx, cancel := dockerContext(ctx)
	defer cancel()

	usage, err := h.docker.Client.DiskUsage(dockerCtx, types.DiskUsageOptions{})
	if err != nil {
		return dockerUsage{}, []dockerVolumeInfo{}
	}

	var imageSize int64
	for _, img := range usage.Images {
		imageSize += img.Size
	}
	var containerSize int64
	for _, ctr := range usage.Containers {
		containerSize += ctr.SizeRootFs
	}
	var volumeSize int64
	volumes := make([]dockerVolumeInfo, 0, len(usage.Volumes))
	for _, vol := range usage.Volumes {
		size := int64(0)
		refCount := int64(-1)
		if vol.UsageData != nil {
			size = vol.UsageData.Size
			refCount = vol.UsageData.RefCount
		}
		volumeSize += size
		volumes = append(volumes, dockerVolumeInfo{
			Name:      vol.Name,
			Driver:    vol.Driver,
			Mount:     vol.Mountpoint,
			Size:      size,
			RefCount:  refCount,
			CreatedAt: vol.CreatedAt,
		})
	}
	var buildCacheSize int64
	for _, cache := range usage.BuildCache {
		buildCacheSize += cache.Size
	}

	return dockerUsage{
			ImageCount:      len(usage.Images),
			ContainerCount:  len(usage.Containers),
			VolumeCount:     len(usage.Volumes),
			BuildCacheCount: len(usage.BuildCache),
			ImageSize:       imageSize,
			ContainerSize:   containerSize,
			VolumeSize:      volumeSize,
			BuildCacheSize:  buildCacheSize,
			TotalSize:       imageSize + containerSize + volumeSize + buildCacheSize,
		},
		volumes
}

func (h *Handlers) detectDockliteService(ctx context.Context) (*serviceStatus, *serviceStatus) {
	var containerStatus *serviceStatus
	containers, err := h.docker.Client.ContainerList(ctx, container.ListOptions{All: true})
	if err == nil {
		for i := range containers {
			c := containers[i]
			if isDockliteContainer(c) {
				status := h.containerServiceStatus(ctx, c, "DockLite API")
				containerStatus = &status
				break
			}
		}
	}

	systemdStatus, ok := systemdUnitStatus([]string{"docklite-agent.service", "docklite.service", "docklite-web.service"})
	if !ok {
		systemdStatus = nil
	}

	if containerStatus != nil {
		return containerStatus, systemdStatus
	}
	if systemdStatus != nil {
		return systemdStatus, nil
	}
	return nil, nil
}

func (h *Handlers) detectProxyService(ctx context.Context) *serviceStatus {
	containers, err := h.docker.Client.ContainerList(ctx, container.ListOptions{All: true})
	if err == nil {
		for i := range containers {
			c := containers[i]
			if isTraefikContainer(c) {
				status := h.containerServiceStatus(ctx, c, "Traefik Proxy")
				status.RestartSupported = true
				status.ReloadSupported = false
				status.LogsSupported = true
				return &status
			}
		}
	}

	out, err := exec.Command("systemctl", "is-active", "nginx").Output()
	if err == nil {
		state := strings.TrimSpace(string(out))
		detail := ""
		if verOut, verErr := exec.Command("nginx", "-v").CombinedOutput(); verErr == nil {
			detail = strings.TrimSpace(string(verOut))
		}
		return &serviceStatus{
			Name:             "Nginx",
			Kind:             "systemd",
			Status:           state,
			Detail:           detail,
			RestartSupported: true,
			ReloadSupported:  true,
			LogsSupported:    true,
		}
	}

	return nil
}

func (h *Handlers) containerServiceStatus(ctx context.Context, c types.Container, displayName string) serviceStatus {
	status := serviceStatus{
		Name:             displayName,
		Kind:             "container",
		Status:           c.State,
		Detail:           c.Status,
		RestartSupported: true,
		ReloadSupported:  false,
		LogsSupported:    true,
	}
	inspect, err := h.docker.Client.ContainerInspect(ctx, c.ID)
	if err == nil && inspect.State != nil {
		status.StartedAt = inspect.State.StartedAt
		if inspect.State.Health != nil && inspect.State.Health.Status != "" {
			status.Detail = fmt.Sprintf("%s (health: %s)", status.Detail, inspect.State.Health.Status)
		}
	}
	return status
}

func isDockliteContainer(c types.Container) bool {
	name := ""
	if len(c.Names) > 0 {
		name = strings.TrimPrefix(c.Names[0], "/")
	}
	lowerName := strings.ToLower(name)
	if strings.Contains(lowerName, "docklite-db") || strings.Contains(lowerName, "docklite_db") {
		return false
	}
	if c.Labels != nil {
		if c.Labels["docklite.type"] == "postgres" || c.Labels["docklite.database"] != "" {
			return false
		}
		if role := c.Labels["com.docklite.role"]; role == "api" || role == "web" {
			return true
		}
		if role := c.Labels["docklite.role"]; role == "api" || role == "web" {
			return true
		}
	}
	if strings.Contains(lowerName, "docklite_api") ||
		strings.Contains(lowerName, "docklite-api") ||
		strings.Contains(lowerName, "docklite-agent") ||
		strings.Contains(lowerName, "docklite-web") {
		return true
	}
	return false
}

func isTraefikContainer(c types.Container) bool {
	name := ""
	if len(c.Names) > 0 {
		name = strings.TrimPrefix(c.Names[0], "/")
	}
	lowerName := strings.ToLower(name)
	if lowerName == "docklite_traefik" || strings.Contains(lowerName, "traefik") {
		return true
	}
	if strings.Contains(strings.ToLower(c.Image), "traefik") {
		return true
	}
	if c.Labels != nil {
		if role := c.Labels["com.docklite.role"]; role == "proxy" {
			return true
		}
		if role := c.Labels["docklite.role"]; role == "proxy" {
			return true
		}
	}
	return false
}

func systemdUnitStatus(candidates []string) (*serviceStatus, bool) {
	if !commandExists("systemctl") {
		return nil, false
	}
	for _, unit := range candidates {
		output, err := runCommandTimeout(2*time.Second, "systemctl", "show", unit, "--no-page", "--property=LoadState,ActiveState,SubState,ActiveEnterTimestamp")
		if err != nil && output == "" {
			continue
		}
		fields := map[string]string{}
		for _, line := range strings.Split(output, "\n") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			fields[parts[0]] = parts[1]
		}
		if strings.TrimSpace(fields["LoadState"]) == "not-found" {
			continue
		}
		status := serviceStatus{
			Name:             strings.TrimSuffix(unit, ".service"),
			Kind:             "systemd",
			Status:           strings.TrimSpace(fields["ActiveState"]),
			Detail:           strings.TrimSpace(fields["SubState"]),
			StartedAt:        strings.TrimSpace(fields["ActiveEnterTimestamp"]),
			RestartSupported: true,
			ReloadSupported:  false,
			LogsSupported:    true,
		}
		return &status, true
	}
	return nil, false
}

func (h *Handlers) performDockliteAction(ctx context.Context, action string) error {
	containers, err := h.docker.Client.ContainerList(ctx, container.ListOptions{All: true})
	if err == nil {
		for i := range containers {
			if isDockliteContainer(containers[i]) {
				if action == "restart" {
					return h.docker.Client.ContainerRestart(ctx, containers[i].ID, container.StopOptions{})
				}
				return fmt.Errorf("docklite reload not supported")
			}
		}
	}

	unitStatus, ok := systemdUnitStatus([]string{"docklite-agent.service", "docklite.service", "docklite-web.service"})
	if ok && unitStatus != nil {
		unit := unitStatus.Name + ".service"
		if action == "restart" {
			_, err := runCommandTimeout(6*time.Second, "systemctl", "restart", unit)
			return err
		}
		if action == "reload" {
			_, err := runCommandTimeout(6*time.Second, "systemctl", "reload", unit)
			return err
		}
	}
	return fmt.Errorf("docklite service not found")
}

func (h *Handlers) performProxyAction(ctx context.Context, action string) error {
	containers, err := h.docker.Client.ContainerList(ctx, container.ListOptions{All: true})
	if err == nil {
		for i := range containers {
			if isTraefikContainer(containers[i]) {
				if action == "restart" {
					return h.docker.Client.ContainerRestart(ctx, containers[i].ID, container.StopOptions{})
				}
				if action == "reload" {
					return h.docker.Client.ContainerKill(ctx, containers[i].ID, "HUP")
				}
			}
		}
	}

	if _, lookErr := exec.LookPath("nginx"); lookErr == nil {
		if action == "reload" {
			_, err := runCommandTimeout(6*time.Second, "sudo", "nginx", "-s", "reload")
			return err
		}
		if action == "restart" {
			_, err := runCommandTimeout(6*time.Second, "sudo", "systemctl", "restart", "nginx")
			return err
		}
	}

	return fmt.Errorf("proxy service not found")
}

func (h *Handlers) readDockliteLogs(ctx context.Context, tail int) (string, error) {
	containers, err := h.docker.Client.ContainerList(ctx, container.ListOptions{All: true})
	if err == nil {
		for i := range containers {
			if isDockliteContainer(containers[i]) {
				dockerCtx, cancel := dockerContext(ctx)
				defer cancel()
				return h.docker.ContainerLogs(dockerCtx, containers[i].ID, strconv.Itoa(tail))
			}
		}
	}
	unitStatus, ok := systemdUnitStatus([]string{"docklite-agent.service", "docklite.service", "docklite-web.service"})
	if ok && unitStatus != nil {
		unit := unitStatus.Name + ".service"
		output, err := runCommandTimeout(4*time.Second, "journalctl", "-u", unit, "-n", strconv.Itoa(tail), "--no-pager")
		if err != nil && output == "" {
			return "", err
		}
		return output, nil
	}
	return "", fmt.Errorf("docklite logs unavailable")
}

func (h *Handlers) readProxyLogs(ctx context.Context, tail int) (string, error) {
	containers, err := h.docker.Client.ContainerList(ctx, container.ListOptions{All: true})
	if err == nil {
		for i := range containers {
			if isTraefikContainer(containers[i]) {
				dockerCtx, cancel := dockerContext(ctx)
				defer cancel()
				return h.docker.ContainerLogs(dockerCtx, containers[i].ID, strconv.Itoa(tail))
			}
		}
	}

	if commandExists("journalctl") {
		output, jErr := runCommandTimeout(4*time.Second, "journalctl", "-u", "nginx", "-n", strconv.Itoa(tail), "--no-pager")
		if jErr == nil || output != "" {
			return output, nil
		}
	}

	errorLog := "/var/log/nginx/error.log"
	if data, fErr := exec.Command("tail", "-n", strconv.Itoa(tail), errorLog).Output(); fErr == nil {
		return string(data), nil
	}

	return "", fmt.Errorf("proxy logs unavailable")
}

func readSystemLogs(tail int) (string, error) {
	if commandExists("journalctl") {
		output, err := runCommandTimeout(4*time.Second, "journalctl", "-n", strconv.Itoa(tail), "--no-pager")
		if err != nil && output == "" {
			return "", err
		}
		return output, nil
	}
	for _, path := range []string{"/var/log/syslog", "/var/log/messages"} {
		if !fileExists(path) {
			continue
		}
		lines, err := readTailLines(path, tail)
		if err != nil {
			return "", err
		}
		return strings.Join(lines, "\n"), nil
	}
	return "", fmt.Errorf("system logs unavailable")
}

func (h *Handlers) createDiagnosticsBundle(ctx context.Context) (string, string, error) {
	baseDir := filepath.Join(h.backupBaseDir, "diagnostics")
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return "", "", err
	}

	name := fmt.Sprintf("diagnostics-%s.tar.gz", time.Now().Format("20060102-150405"))
	path := filepath.Join(baseDir, name)

	file, err := os.Create(path)
	if err != nil {
		return "", "", err
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return "", "", err
	}

	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)

	addTextToTar(tw, "system/hostname.txt", func() string {
		hostname, _ := os.Hostname()
		return hostname
	}())
	addTextToTar(tw, "system/os-release.txt", readFileSafe("/etc/os-release"))
	addTextToTar(tw, "system/uname.txt", readCommandSafe(2*time.Second, "uname", "-a"))
	addTextToTar(tw, "system/uptime.txt", fmt.Sprintf("uptime_seconds=%.0f\n", readUptime()))
	addTextToTar(tw, "system/loadavg.txt", readFileSafe("/proc/loadavg"))
	addTextToTar(tw, "system/df.txt", readCommandSafe(4*time.Second, "df", "-h"))

	if info, err := h.docker.Client.Info(ctx); err == nil {
		if payload, err := json.MarshalIndent(info, "", "  "); err == nil {
			addTextToTar(tw, "docker/info.json", string(payload))
		}
	}
	if version, err := h.docker.Client.ServerVersion(ctx); err == nil {
		if payload, err := json.MarshalIndent(version, "", "  "); err == nil {
			addTextToTar(tw, "docker/version.json", string(payload))
		}
	}

	if logs, err := readSystemLogs(400); err == nil {
		addTextToTar(tw, "logs/system.log", logs)
	}
	if logs, err := h.readDockliteLogs(ctx, 400); err == nil {
		addTextToTar(tw, "logs/docklite.log", logs)
	}
	if logs, err := h.readProxyLogs(ctx, 400); err == nil {
		addTextToTar(tw, "logs/proxy.log", logs)
	}

	if err := tw.Close(); err != nil {
		_ = gz.Close()
		_ = file.Close()
		return "", "", err
	}
	if err := gz.Close(); err != nil {
		_ = file.Close()
		return "", "", err
	}
	if err := file.Close(); err != nil {
		return "", "", err
	}

	cleanupOldDiagnostics(baseDir, 5)

	return path, name, nil
}

func addTextToTar(tw *tar.Writer, name, content string) {
	if tw == nil {
		return
	}
	if content == "" {
		content = "unavailable\n"
	}
	payload := []byte(content)
	header := &tar.Header{
		Name:    name,
		Mode:    0o600,
		Size:    int64(len(payload)),
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(header); err != nil {
		return
	}
	_, _ = tw.Write(payload)
}

func readFileSafe(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("unable to read %s: %v\n", path, err)
	}
	return string(data)
}

func readCommandSafe(timeout time.Duration, name string, args ...string) string {
	output, err := runCommandTimeout(timeout, name, args...)
	if err != nil && output == "" {
		return fmt.Sprintf("command %s failed: %v\n", name, err)
	}
	return output + "\n"
}

func cleanupOldDiagnostics(dir string, keep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type fileInfo struct {
		name    string
		modTime time.Time
	}
	files := make([]fileInfo, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, fileInfo{name: entry.Name(), modTime: info.ModTime()})
	}
	if len(files) <= keep {
		return
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.After(files[j].modTime)
	})
	for _, file := range files[keep:] {
		_ = os.Remove(filepath.Join(dir, file.name))
	}
}

func parseTail(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	if parsed > 2000 {
		return 2000
	}
	return parsed
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func runCommandTimeout(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(output))
	if err != nil {
		return result, err
	}
	return result, nil
}

type servicePortsResponse struct {
	AgentAddr  string `json:"agentAddr"`
	AgentPort  int    `json:"agentPort"`
	WebURL     string `json:"webUrl"`
	WebPort    int    `json:"webPort"`
	Headless   bool   `json:"headless"`
}

func (h *Handlers) ServicePorts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	agentPort := 3000
	addr := h.listenAddr
	if strings.HasPrefix(addr, ":") {
		if p, err := strconv.Atoi(addr[1:]); err == nil {
			agentPort = p
		}
	}

	webPort := 0
	webURL := h.nextjsURL
	headless := webURL == ""
	if !headless {
		if p, err := strconv.Atoi(strings.TrimPrefix(webURL[strings.LastIndex(webURL, ":")+1:], "/")); err == nil {
			webPort = p
		}
	}

	writeJSON(w, http.StatusOK, servicePortsResponse{
		AgentAddr: addr,
		AgentPort: agentPort,
		WebURL:    webURL,
		WebPort:   webPort,
		Headless:  headless,
	})
}
