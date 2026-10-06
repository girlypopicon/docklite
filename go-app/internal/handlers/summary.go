package handlers

import (
	"net/http"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/volume"
)

type SummaryResponse struct {
	ContainersRunning int `json:"containers_running"`
	ContainersStopped int `json:"containers_stopped"`
	ImagesCount       int `json:"images_count"`
	VolumesCount      int `json:"volumes_count"`
}

func (h *Handlers) Summary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx, cancel := dockerContext(r.Context())
	defer cancel()

	// Get containers
	containers, err := h.docker.VisibleContainers(ctx, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Count running and stopped containers
	running := 0
	stopped := 0
	for _, c := range containers {
		if c.State == "running" {
			running++
		} else {
			stopped++
		}
	}

	// Get images count
	images, err := h.docker.Client.ImageList(ctx, image.ListOptions{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Get volumes count
	volumesResp, err := h.docker.Client.VolumeList(ctx, volume.ListOptions{Filters: filters.Args{}})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	summary := SummaryResponse{
		ContainersRunning: running,
		ContainersStopped: stopped,
		ImagesCount:       len(images),
		VolumesCount:      len(volumesResp.Volumes),
	}

	writeJSON(w, http.StatusOK, summary)
}
