package handlers

import (
	"math"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type serverStatsResponse struct {
	Hostname       string            `json:"hostname"`
	Platform       string            `json:"platform"`
	Arch           string            `json:"arch"`
	CPUs           int               `json:"cpus"`
	TotalMemory    uint64            `json:"totalMemory"`
	FreeMemory     uint64            `json:"freeMemory"`
	Uptime         float64           `json:"uptime"`
	DockerVersion  string            `json:"dockerVersion"`
	ContainerCount int               `json:"containerCount"`
	ImageCount     int               `json:"imageCount"`
	CPUUsage       float64           `json:"cpuUsage"`
	DiskUsage      map[string]uint64 `json:"diskUsage"`
	NetworkStats   map[string]uint64 `json:"networkStats"`
}

func (h *Handlers) ServerStats(w http.ResponseWriter, r *http.Request) {
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

	info, err := h.docker.Client.Info(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	version, err := h.docker.Client.ServerVersion(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	hostname, _ := os.Hostname()
	totalMemory, freeMemory := readMemory()
	uptime := readUptime()
	cpuUsage := sampleCPUUsage()
	diskUsage := readDiskUsage()
	networkStats := readNetworkStats()

	diskPercent := uint64(0)
	if diskUsage.Total > 0 {
		diskPercent = uint64(math.Round(float64(diskUsage.Used) / float64(diskUsage.Total) * 100))
	}

	writeJSON(w, http.StatusOK, serverStatsResponse{
		Hostname:       hostname,
		Platform:       runtime.GOOS,
		Arch:           runtime.GOARCH,
		CPUs:           runtime.NumCPU(),
		TotalMemory:    totalMemory,
		FreeMemory:     freeMemory,
		Uptime:         uptime,
		DockerVersion:  version.Version,
		ContainerCount: info.ContainersRunning,
		ImageCount:     info.Images,
		CPUUsage:       cpuUsage,
		DiskUsage: map[string]uint64{
			"total":      diskUsage.Total,
			"used":       diskUsage.Used,
			"free":       diskUsage.Free,
			"percentage": diskPercent,
		},
		NetworkStats: map[string]uint64{
			"received":    networkStats.RxBytes,
			"transmitted": networkStats.TxBytes,
		},
	})
}

func sampleCPUUsage() float64 {
	idle1, total1, ok := readCPUStat()
	if !ok {
		return loadAvgCPUUsage()
	}
	time.Sleep(100 * time.Millisecond)
	idle2, total2, ok := readCPUStat()
	if !ok {
		return loadAvgCPUUsage()
	}
	idleDelta := float64(idle2 - idle1)
	totalDelta := float64(total2 - total1)
	if totalDelta <= 0 {
		return 0
	}
	usage := 100 - (100 * idleDelta / totalDelta)
	return math.Round(usage*10) / 10
}

func readCPUStat() (idle uint64, total uint64, ok bool) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return 0, 0, false
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 5 {
		return 0, 0, false
	}
	var values []uint64
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return 0, 0, false
		}
		values = append(values, value)
	}
	for _, value := range values {
		total += value
	}
	idle = values[3]
	return idle, total, true
}

func loadAvgCPUUsage() float64 {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	load, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	cpus := float64(runtime.NumCPU())
	if cpus == 0 {
		return 0
	}
	usage := (load / cpus) * 100
	if usage > 100 {
		usage = 100
	}
	return math.Round(usage*10) / 10
}
