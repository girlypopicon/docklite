package handlers

import (
	"context"
	"errors"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"docklite-agent/internal/models"
)

func (h *Handlers) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	ctx, cancel := dockerContext(r.Context())
	defer cancel()

	status, err := collectStatus(ctx, h)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func collectStatus(ctx context.Context, h *Handlers) (*models.StatusResponse, error) {
	hostname := hostName()
	memoryTotal, memoryFree := readMemory()
	uptime := readUptime()
	cpuUsage, _ := readCPUUsage()
	diskUsage := readDiskUsage()
	networkStats := readNetworkStats()

	info, err := h.docker.Client.Info(ctx)
	if err != nil {
		return nil, err
	}
	version, err := h.docker.Client.ServerVersion(ctx)
	if err != nil {
		return nil, err
	}

	return &models.StatusResponse{
		Hostname:       hostname,
		Platform:       runtime.GOOS,
		Arch:           runtime.GOARCH,
		CPUs:           runtime.NumCPU(),
		TotalMemory:    memoryTotal,
		FreeMemory:     memoryFree,
		Uptime:         uptime,
		DockerVersion:  version.Version,
		ContainerCount: info.Containers,
		ImageCount:     info.Images,
		CPUUsage:       cpuUsage,
		DiskUsage:      diskUsage,
		NetworkStats:   networkStats,
	}, nil
}

func readMemory() (uint64, uint64) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	var total, free uint64
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch strings.TrimSuffix(fields[0], ":") {
		case "MemTotal":
			total = parseKiB(fields[1])
		case "MemAvailable":
			free = parseKiB(fields[1])
		}
	}
	return total, free
}

func parseKiB(value string) uint64 {
	number, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0
	}
	return number * 1024
}

func readUptime() float64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	uptime, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return uptime
}

func readCPUUsage() (float64, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, err
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return 0, errors.New("missing cpu data")
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 5 {
		return 0, errors.New("invalid cpu data")
	}
	var values []uint64
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return 0, err
		}
		values = append(values, value)
	}
	var total uint64
	for _, value := range values {
		total += value
	}
	idle := values[3]
	if total == 0 {
		return 0, nil
	}
	usage := (float64(total-idle) / float64(total)) * 100
	return usage, nil
}

func readDiskUsage() models.DiskUsage {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		return models.DiskUsage{}
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	used := total - free
	return models.DiskUsage{Total: total, Free: free, Used: used}
}

func readNetworkStats() models.NetworkStats {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return models.NetworkStats{}
	}
	lines := strings.Split(string(data), "\n")
	var rxTotal uint64
	var txTotal uint64
	for _, line := range lines[2:] {
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		iface := strings.TrimSuffix(fields[0], ":")
		if iface == "lo" {
			continue
		}
		rx, _ := strconv.ParseUint(fields[1], 10, 64)
		tx, _ := strconv.ParseUint(fields[9], 10, 64)
		rxTotal += rx
		txTotal += tx
	}
	return models.NetworkStats{RxBytes: rxTotal, TxBytes: txTotal}
}
