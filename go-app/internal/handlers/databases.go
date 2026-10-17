package handlers

import (
	"docklite-agent/internal/docker"
	"net/http"
	"regexp"
	"strings"
)

type createDatabaseRequest struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Port        *int   `json:"port"`
	ContainerID string `json:"container_id"`
	DBPath      string `json:"db_path"`
}

func (h *Handlers) Databases(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listDatabases(w, r)
	case http.MethodPost:
		h.createDatabase(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handlers) listDatabases(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	ctx, cancel := dockerContext(r.Context())
	defer cancel()

	databases, err := h.docker.ListDatabases(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"databases": databases})
}

func (h *Handlers) createDatabase(w http.ResponseWriter, r *http.Request) {
	var req createDatabaseRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	sanitized := sanitizeDatabaseName(name)
	if sanitized == "" {
		writeError(w, http.StatusBadRequest, "invalid database name")
		return
	}

	ctx, cancel := dockerContext(r.Context())
	defer cancel()

	if req.Type == "sqlite" {
		if req.ContainerID == "" {
			writeError(w, http.StatusBadRequest, "container_id is required for sqlite")
			return
		}
		if req.DBPath == "" {
			writeError(w, http.StatusBadRequest, "db_path is required for sqlite")
			return
		}

		// Verify container exists
		_, err := h.docker.InspectContainer(ctx, req.ContainerID)
		if err != nil {
			writeError(w, http.StatusNotFound, "container not found")
			return
		}

		record, err := h.store.UpsertDatabase(sanitized, "sqlite", req.ContainerID, 0, req.DBPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"database": map[string]any{
				"id":            record.ID,
				"name":          record.Name,
				"type":          "sqlite",
				"container_id":  record.ContainerID,
				"db_path":       record.DBPath,
				"postgres_port": 0,
			},
			"connection": map[string]any{
				"type": "sqlite",
				"path": req.DBPath,
				"host": "localhost",
			},
		})
		return
	}

	engine, ok := docker.EngineFor(req.Type)
	if !ok {
		writeError(w, http.StatusBadRequest, "unsupported database type (use postgres, mariadb, mongodb, redis or sqlite)")
		return
	}

	port := 0
	if req.Port != nil {
		port = *req.Port
		if port < 1 || port > 65535 {
			writeError(w, http.StatusBadRequest, "invalid port")
			return
		}
	}

	id, assignedPort, username, password, err := h.docker.CreateDatabaseOfType(ctx, engine.Type, sanitized, req.Username, req.Password, port)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if _, err := h.store.UpsertDatabase(sanitized, engine.Type, id, assignedPort, ""); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	host := docker.ContainerNameFor(sanitized)
	writeJSON(w, http.StatusCreated, map[string]any{
		"database": map[string]any{
			"id":       id,
			"name":     sanitized,
			"type":     engine.Type,
			"port":     assignedPort,
			"username": username,
			"password": password,
		},
		"connection": map[string]any{
			"type":     engine.Type,
			"host":     "localhost",
			"port":     assignedPort,
			"database": sanitized,
			"username": username,
			"password": password,
			"url":      engine.ConnectionURL("localhost", assignedPort, sanitized, username, password),
			// From another container on DockLite's network (a site, for example): the container's name and the engine's own port.
			"container_host": host,
			"container_port": engine.Port,
			"container_url":  engine.ConnectionURL(host, engine.Port, sanitized, username, password),
		},
	})
}

func sanitizeDatabaseName(name string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	sanitized := re.ReplaceAllString(name, "_")
	return strings.Trim(sanitized, "_")
}
