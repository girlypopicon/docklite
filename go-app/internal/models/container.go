package models

type ContainerInfo struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	Created       int64             `json:"created"`
	State         string            `json:"state"`
	Status        string            `json:"status"`
	Uptime        string            `json:"uptime,omitempty"`
	Ports         string            `json:"ports,omitempty"`
	Labels        map[string]string `json:"labels"`
	OwnerUsername string            `json:"owner_username,omitempty"`
	// Unmanaged is set on a container DockLite did not create (no docklite.managed label). Admins see these so they can be
	// grouped, started and stopped like any box.
	Unmanaged bool      `json:"unmanaged,omitempty"`
	Style     *BoxStyle `json:"style,omitempty"` // card colour and icon the owner picked
}

// BoxStyle is how a box's card looks. Empty means the default (a colour by type, the box icon).
type BoxStyle struct {
	Color string `json:"color,omitempty"`
	Icon  string `json:"icon,omitempty"`
}

type ContainerStats struct {
	CPUUsage    float64 `json:"cpuUsage"`
	MemoryUsage uint64  `json:"memoryUsage"`
	MemoryLimit uint64  `json:"memoryLimit"`
	MemoryPct   float64 `json:"memoryPct"`
}
