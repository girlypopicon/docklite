package handlers

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"
)

func (h *Handlers) DBDebug(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	tables, err := h.queryStringColumn(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}


	schemaAndData := make([]map[string]any, 0, len(tables))
	for _, tableName := range tables {
		schemaRows, err := h.queryRows(`PRAGMA table_info(` + quoteIdent(tableName) + `)`)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		columnNames := make([]string, 0)
		for _, row := range schemaRows {
			name, _ := row["name"].(string)
			if name == "" {
				continue
			}
			if isSecretColumn(name) {
				continue
			}
			columnNames = append(columnNames, name)
		}

		countRow, err := h.queryRow(`SELECT COUNT(*) as count FROM ` + quoteIdent(tableName))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		var data []map[string]any
		if len(columnNames) > 0 {
			columnList := make([]string, 0, len(columnNames))
			for _, column := range columnNames {
				columnList = append(columnList, quoteIdent(column))
			}
			data, err = h.queryRows(`SELECT ` + strings.Join(columnList, ", ") + ` FROM ` + quoteIdent(tableName) + ` LIMIT 10`)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal server error")
				return
			}
		}

		schemaAndData = append(schemaAndData, map[string]any{
			"name":   tableName,
			"schema": schemaRows,
			"count":  countRow["count"],
			"data":   data,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"dbInfo": schemaAndData})
}

func (h *Handlers) Debug(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if os.Getenv("ENABLE_DEBUG_PAGES") != "true" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	debugInfo := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"database": map[string]any{
			"status":  "unknown",
			"error":   nil,
			"details": map[string]any{},
		},
		"docker": map[string]any{
			"status":  "unknown",
			"error":   nil,
			"details": map[string]any{},
		},
		"authentication": map[string]any{
			"status":  "unknown",
			"error":   nil,
			"details": map[string]any{},
		},
	}

	if err := h.fillDebugDatabase(debugInfo); err != nil {
		return
	}
	if err := h.fillDebugDocker(r.Context(), debugInfo); err != nil {
		return
	}
	h.fillDebugAuth(r, debugInfo)

	overall := "unhealthy"
	dbStatus := debugInfo["database"].(map[string]any)["status"]
	dockerStatus := debugInfo["docker"].(map[string]any)["status"]
	if dbStatus == "connected" && dockerStatus == "connected" {
		overall = "healthy"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": overall,
		"debug":  debugInfo,
	})
}

func (h *Handlers) fillDebugDatabase(debugInfo map[string]any) error {
	info := debugInfo["database"].(map[string]any)

	countRow, err := h.queryRow(`SELECT COUNT(*) as count FROM users`)
	if err != nil {
		info["status"] = "error"
		info["error"] = err.Error()
		info["details"] = map[string]any{"path": h.store.Path}
		return nil
	}

	tables, err := h.queryRows(`SELECT name FROM sqlite_master WHERE type='table'`)
	if err != nil {
		info["status"] = "error"
		info["error"] = err.Error()
		info["details"] = map[string]any{"path": h.store.Path}
		return nil
	}

	info["status"] = "connected"
	info["details"] = map[string]any{
		"path":      h.store.Path,
		"userCount": countRow["count"],
		"tables":    tables,
	}
	return nil
}

func (h *Handlers) fillDebugDocker(ctx context.Context, debugInfo map[string]any) error {
	info := debugInfo["docker"].(map[string]any)
	ctx, cancel := dockerContext(ctx)
	defer cancel()

	containers, err := h.docker.ListContainers(ctx, true)
	if err != nil {
		info["status"] = "error"
		info["error"] = err.Error()
		info["details"] = map[string]any{"socketPath": "/var/run/docker.sock"}
		return nil
	}

	sample := make([]map[string]any, 0)
	for i, container := range containers {
		if i >= 3 {
			break
		}
		id := container.ID
		if len(id) > 12 {
			id = id[:12]
		}
		sample = append(sample, map[string]any{
			"id":     id,
			"name":   container.Name,
			"status": container.Status,
			"state":  container.State,
		})
	}

	info["status"] = "connected"
	info["details"] = map[string]any{
		"containerCount": len(containers),
		"containers":     sample,
	}
	return nil
}

func (h *Handlers) fillDebugAuth(r *http.Request, debugInfo map[string]any) {
	info := debugInfo["authentication"].(map[string]any)
	userID, ok := readUserIDFromContext(r)
	role, _ := readUserRoleFromContext(r)
	info["status"] = "authenticated"
	if !ok {
		info["status"] = "not_authenticated"
	}
	info["details"] = map[string]any{
		"hasSession": ok,
		"user": func() any {
			if !ok {
				return nil
			}
			return map[string]any{
				"userId":   userID,
				"role":     role,
				"isAdmin":  role == "admin" || role == "super_admin",
				"username": "",
			}
		}(),
	}
}

func (h *Handlers) queryStringColumn(query string) ([]string, error) {
	rows, err := h.store.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		results = append(results, value)
	}
	return results, rows.Err()
}

func (h *Handlers) queryRow(query string, args ...any) (map[string]any, error) {
	rows, err := h.queryRows(query, args...)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return map[string]any{}, nil
	}
	return rows[0], nil
}

func (h *Handlers) queryRows(query string, args ...any) ([]map[string]any, error) {
	rows, err := h.store.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	results := make([]map[string]any, 0)
	for rows.Next() {
		raw := make([]any, len(cols))
		dest := make([]any, len(cols))
		for i := range raw {
			dest[i] = &raw[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			value := raw[i]
			if bytes, ok := value.([]byte); ok {
				row[col] = string(bytes)
			} else {
				row[col] = value
			}
		}
		results = append(results, row)
	}
	return results, rows.Err()
}

func quoteIdent(value string) string {
	escaped := strings.ReplaceAll(value, `"`, `""`)
	return `"` + escaped + `"`
}

// isSecretColumn reports whether a column holds credentials and must never
// be shown in the system database viewer (password and token hashes, the
// Cloudflare API token, and anything named like them in future tables).
func isSecretColumn(name string) bool {
	lower := strings.ToLower(name)
	for _, marker := range []string{"password", "hash", "token", "secret", "key", "fingerprint", "credential"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
