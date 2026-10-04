package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"docklite-agent/internal/backup"
	"docklite-agent/internal/store"
)

func (h *Handlers) Backups(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.listBackups(w, r)
	case http.MethodDelete:
		h.deleteBackup(w, r)
	case http.MethodPost:
		h.cleanupBackups(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handlers) BackupExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		TargetType    string `json:"target_type"`
		TargetID      int64  `json:"target_id"`
		Delivery      string `json:"delivery"`
		RetentionDays *int   `json:"retention_days"`
		Notes         string `json:"notes"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.TargetType == "" || body.TargetID <= 0 {
		writeError(w, http.StatusBadRequest, "missing required fields: target_type, target_id")
		return
	}

	delivery := strings.TrimSpace(body.Delivery)
	if delivery == "" {
		delivery = "download"
	}
	if delivery != "download" && delivery != "local" {
		writeError(w, http.StatusBadRequest, "invalid delivery")
		return
	}

	if body.TargetType == "site" {
		if !isAdminRole(r) {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
	} else if body.TargetType == "database" {
		if !isAdminRole(r) {
			userID, ok := readUserIDFromContext(r)
			if !ok || userID <= 0 {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			allowed, err := h.store.HasDatabaseAccess(userID, body.TargetID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !allowed {
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}
		}
	} else {
		writeError(w, http.StatusBadRequest, "invalid target_type")
		return
	}

	dest, err := h.ensureLocalDestination()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// One backup of the same thing at a time: a second click (or a second
	// person) gets pointed at the one already running instead of racing it.
	if runningID, running := backup.DefaultTracker.RunningFor(body.TargetType, body.TargetID); running {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":     "A backup of this is already running.",
			"backup_id": runningID,
		})
		return
	}

	record := store.BackupRecord{
		JobID:       nil,
		Destination: dest.ID,
		TargetType:  body.TargetType,
		TargetID:    body.TargetID,
		BackupPath:  "",
		SizeBytes:   0,
		Status:      "in_progress",
	}
	backupID, err := h.store.CreateBackup(record)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	reporter := backup.DefaultTracker.Start(backupID, body.TargetType, body.TargetID)
	if reporter == nil {
		// Lost a race with another request between the check and the start.
		message := "A backup of this is already running."
		_ = h.store.UpdateBackupStatus(backupID, "failed", &message, nil, nil)
		writeError(w, http.StatusConflict, message)
		return
	}

	go h.runExport(reporter, backupID, dest, body.TargetType, body.TargetID, delivery, body.RetentionDays, body.Notes)

	// 202: accepted, still working. The page polls /api/backups/progress.
	writeJSON(w, http.StatusAccepted, map[string]any{"backup_id": backupID, "delivery": delivery})
}

// runExport does the backup in the background and reports into the tracker;
// whatever happens, the backup record and the tracker end up agreeing.
func (h *Handlers) runExport(rep *backup.Reporter, backupID int64, dest *store.BackupDestination, targetType string, targetID int64, delivery string, retentionDays *int, notes string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			message := fmt.Sprintf("The backup stopped unexpectedly: %v", recovered)
			_ = h.store.UpdateBackupStatus(backupID, "failed", &message, nil, nil)
			rep.Fail(message)
		}
	}()

	ctx, cancel := context.WithTimeout(backup.WithReporter(context.Background(), rep), 6*time.Hour)
	defer cancel()

	subDir := "downloads"
	if delivery == "local" {
		if targetType == "site" {
			subDir = "sites"
		} else {
			subDir = "databases"
		}
	}

	var artifact *backup.ArtifactResult
	var err error
	switch targetType {
	case "site":
		artifact, err = backup.CreateSiteBackup(ctx, h.store, h.backupBaseDir, subDir, targetID, notes)
	case "database":
		artifact, err = backup.CreateDatabaseBackup(ctx, h.store, h.docker, h.backupBaseDir, subDir, targetID, notes)
	}
	if err != nil {
		message := backup.FriendlyError(err)
		_ = h.store.UpdateBackupStatus(backupID, "failed", &message, nil, nil)
		rep.Fail(message)
		return
	}

	_ = h.store.UpdateBackupStatus(backupID, "success", nil, &artifact.Size, &artifact.Path)

	if delivery == "download" {
		rep.SetDownloadURL("/api/backups/local/download?file=" + url.QueryEscape(artifact.RelativePath))
		// A download copy is temporary: it and its record go after an hour.
		time.AfterFunc(time.Hour, func() {
			_ = backup.RemoveBackupArtifacts(artifact.Path)
			_ = h.store.DeleteBackup(backupID)
		})
	} else if retentionDays != nil && *retentionDays > 0 {
		cutoff := time.Now().UTC().AddDate(0, 0, -*retentionDays).Format("2006-01-02 15:04:05")
		if records, err := h.store.ListOldBackups(dest.ID, cutoff); err == nil {
			for _, record := range records {
				if record.ID == backupID {
					continue
				}
				_ = backup.RemoveBackupArtifacts(record.BackupPath)
				_ = h.store.DeleteBackup(record.ID)
			}
		}
	}
	rep.Succeed(artifact, artifact.Verified)
}

// BackupProgress reports how a backup is going. While it runs, the live
// numbers come from the tracker; afterwards (or after a restart) from the
// saved record.
//
//	GET /api/backups/progress?id=<backup id>
func (h *Handlers) BackupProgress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "missing required parameter: id")
		return
	}
	record, err := h.store.GetBackupByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if record == nil {
		writeError(w, http.StatusNotFound, "backup not found")
		return
	}
	if !h.canAccessBackupRecord(r, record) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	if snapshot, ok := backup.DefaultTracker.Snapshot(id); ok {
		writeJSON(w, http.StatusOK, snapshot)
		return
	}

	// Not tracked (finished long ago, or the agent restarted): describe the record.
	status := backup.StatusSuccess
	phase := "Done"
	percent := float64(100)
	errorMessage := ""
	switch record.Status {
	case "failed":
		status, phase, percent = backup.StatusFailed, "Failed", 0
	case "in_progress":
		status, phase, percent = backup.StatusFailed, "Interrupted", 0
		errorMessage = "This backup didn't finish."
	}
	if record.ErrorMessage != nil {
		errorMessage = *record.ErrorMessage
	}
	writeJSON(w, http.StatusOK, backup.Progress{
		BackupID:   record.ID,
		TargetType: record.TargetType,
		TargetID:   record.TargetID,
		Status:     status,
		Phase:      phase,
		Percent:    percent,
		Size:       record.SizeBytes,
		Error:      errorMessage,
		Warnings:   []string{},
		FileName:   filepath.Base(record.BackupPath),
		StartedAt:  record.CreatedAt,
	})
}

// canAccessBackupRecord applies the same rule as exporting: admins can see
// everything; others only backups of databases they have access to.
func (h *Handlers) canAccessBackupRecord(r *http.Request, record *store.BackupRecord) bool {
	if isAdminRole(r) {
		return true
	}
	userID, ok := readUserIDFromContext(r)
	if !ok || userID <= 0 || record.TargetType != "database" {
		return false
	}
	allowed, err := h.store.HasDatabaseAccess(userID, record.TargetID)
	return err == nil && allowed
}

func (h *Handlers) BackupDestinations(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	switch r.Method {
	case http.MethodGet:
		destinations, err := h.store.GetBackupDestinations()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"destinations": destinations})
	case http.MethodPost:
		var body struct {
			Name    string          `json:"name"`
			Type    string          `json:"type"`
			Config  json.RawMessage `json:"config"`
			Enabled *int            `json:"enabled"`
		}
		if err := readJSON(w, r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Name == "" || body.Type == "" || len(body.Config) == 0 {
			writeError(w, http.StatusBadRequest, "missing required fields: name, type, config")
			return
		}
		if body.Type != "local" {
			writeError(w, http.StatusBadRequest, "only local destinations are supported")
			return
		}
		enabled := 1
		if body.Enabled != nil {
			enabled = *body.Enabled
		}
		id, err := h.store.CreateBackupDestination(body.Name, body.Type, string(body.Config), enabled)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "message": "Backup destination created successfully"})
	case http.MethodPut:
		var body struct {
			ID      int64           `json:"id"`
			Name    *string         `json:"name"`
			Type    *string         `json:"type"`
			Config  json.RawMessage `json:"config"`
			Enabled *int            `json:"enabled"`
		}
		if err := readJSON(w, r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.ID <= 0 {
			writeError(w, http.StatusBadRequest, "missing required field: id")
			return
		}
		params := make(map[string]any)
		if body.Name != nil {
			params["name"] = *body.Name
		}
		if body.Type != nil {
			if *body.Type != "local" {
				writeError(w, http.StatusBadRequest, "only local destinations are supported")
				return
			}
			params["type"] = *body.Type
		}
		if body.Config != nil {
			params["config"] = string(body.Config)
		}
		if body.Enabled != nil {
			params["enabled"] = *body.Enabled
		}
		if err := h.store.UpdateBackupDestination(body.ID, params); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"message": "Backup destination updated successfully"})
	case http.MethodDelete:
		idStr := r.URL.Query().Get("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "missing required parameter: id")
			return
		}
		if err := h.store.DeleteBackupDestination(id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"message": "Backup destination deleted successfully"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handlers) BackupJobs(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	switch r.Method {
	case http.MethodGet:
		destID := r.URL.Query().Get("destination_id")
		if destID != "" {
			id, err := strconv.ParseInt(destID, 10, 64)
			if err != nil || id <= 0 {
				writeError(w, http.StatusBadRequest, "invalid destination_id")
				return
			}
			jobs, err := h.store.GetBackupJobsByDestination(id)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
			return
		}
		jobs, err := h.store.GetBackupJobs()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
	case http.MethodPost:
		var body struct {
			DestinationID int64  `json:"destination_id"`
			TargetType    string `json:"target_type"`
			TargetID      *int64 `json:"target_id"`
			Frequency     string `json:"frequency"`
			RetentionDays *int   `json:"retention_days"`
			Enabled       *int   `json:"enabled"`
		}
		if err := readJSON(w, r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.DestinationID == 0 || body.TargetType == "" || body.Frequency == "" {
			writeError(w, http.StatusBadRequest, "missing required fields: destination_id, target_type, frequency")
			return
		}
		retention := 30
		if body.RetentionDays != nil {
			retention = *body.RetentionDays
		}
		enabled := 1
		if body.Enabled != nil {
			enabled = *body.Enabled
		}
		id, err := h.store.CreateBackupJob(body.DestinationID, body.TargetType, body.TargetID, body.Frequency, retention, enabled)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "message": "Backup job created successfully"})
	case http.MethodPut:
		var body struct {
			ID            int64   `json:"id"`
			DestinationID *int64  `json:"destination_id"`
			TargetType    *string `json:"target_type"`
			TargetID      *int64  `json:"target_id"`
			Frequency     *string `json:"frequency"`
			RetentionDays *int    `json:"retention_days"`
			Enabled       *int    `json:"enabled"`
		}
		if err := readJSON(w, r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.ID <= 0 {
			writeError(w, http.StatusBadRequest, "missing required field: id")
			return
		}
		params := make(map[string]any)
		if body.DestinationID != nil {
			params["destination_id"] = *body.DestinationID
		}
		if body.TargetType != nil {
			params["target_type"] = *body.TargetType
		}
		if body.TargetID != nil {
			params["target_id"] = *body.TargetID
		}
		if body.Frequency != nil {
			params["frequency"] = *body.Frequency
		}
		if body.RetentionDays != nil {
			params["retention_days"] = *body.RetentionDays
		}
		if body.Enabled != nil {
			params["enabled"] = *body.Enabled
		}
		if err := h.store.UpdateBackupJob(body.ID, params); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"message": "Backup job updated successfully"})
	case http.MethodDelete:
		idStr := r.URL.Query().Get("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "missing required parameter: id")
			return
		}
		if err := h.store.DeleteBackupJob(id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"message": "Backup job deleted successfully"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handlers) BackupHistory(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	backups, err := h.store.ListBackups()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, record := range backups {
		if record.BackupPath != "" {
			_ = backup.RemoveBackupArtifacts(record.BackupPath)
		}
	}
	deleted, err := h.store.ClearBackupHistory()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "Backup history cleared", "deleted": deleted})
}

func (h *Handlers) BackupTrigger(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		JobID int64 `json:"job_id"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.JobID <= 0 {
		writeError(w, http.StatusBadRequest, "missing required field: job_id")
		return
	}
	go func() {
		_ = backup.TriggerJob(context.Background(), h.store, h.docker, h.backupBaseDir, body.JobID)
	}()
	writeJSON(w, http.StatusOK, map[string]any{"message": "Backup job triggered successfully"})
}

func (h *Handlers) LocalBackups(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	switch r.Method {
	case http.MethodGet:
		files, err := h.listBackupFiles()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		sort.Slice(files, func(i, j int) bool {
			return files[i]["modified_at"].(string) > files[j]["modified_at"].(string)
		})
		writeJSON(w, http.StatusOK, map[string]any{"path": h.backupBaseDir, "files": files})
	case http.MethodDelete:
		fileName := r.URL.Query().Get("file")
		if fileName == "" {
			writeError(w, http.StatusBadRequest, "file name is required")
			return
		}
		resolvedPath, ok := h.resolveBackupPath(fileName)
		if !ok {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		if err := os.Remove(resolvedPath); err != nil {
			if os.IsNotExist(err) {
				writeError(w, http.StatusNotFound, "file not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		_ = os.Remove(backup.ManifestPathForArtifact(resolvedPath))
		writeJSON(w, http.StatusOK, map[string]any{"message": "Backup file deleted successfully"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handlers) LocalBackupDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	fileName := r.URL.Query().Get("file")
	if fileName == "" {
		writeError(w, http.StatusBadRequest, "file name is required")
		return
	}
	resolvedPath, ok := h.resolveBackupPath(fileName)
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if !isAdminRole(r) {
		userID, ok := readUserIDFromContext(r)
		if !ok || userID <= 0 {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		record, err := h.store.GetBackupByPath(resolvedPath)
		if err != nil || record == nil {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		if record.TargetType != "database" {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		allowed, err := h.store.HasDatabaseAccess(userID, record.TargetID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		if !allowed {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
	}
	info, err := os.Stat(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	file, err := os.Open(resolvedPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer file.Close()

	w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(resolvedPath)+"\"")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func (h *Handlers) resolveBackupPath(fileName string) (string, bool) {
	cleaned := filepath.Clean(fileName)
	if cleaned == "." || cleaned == "/" || strings.HasPrefix(cleaned, "..") {
		return "", false
	}
	if filepath.IsAbs(cleaned) {
		return "", false
	}
	target := filepath.Join(h.backupBaseDir, cleaned)
	rel, err := filepath.Rel(h.backupBaseDir, target)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return target, true
}

func (h *Handlers) listBackups(w http.ResponseWriter, r *http.Request) {
	jobIDStr := r.URL.Query().Get("job_id")
	targetType := r.URL.Query().Get("target_type")
	targetIDStr := r.URL.Query().Get("target_id")

	if jobIDStr != "" {
		jobID, err := strconv.ParseInt(jobIDStr, 10, 64)
		if err != nil || jobID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid job_id")
			return
		}
		backups, err := h.store.GetBackupsByJob(jobID, 50)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"backups": backups})
		return
	}

	if targetType != "" && targetIDStr != "" {
		targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
		if err != nil || targetID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid target_id")
			return
		}
		backups, err := h.store.GetBackupsByTarget(targetType, targetID, 50)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"backups": backups})
		return
	}

	backups, err := h.store.GetBackups(100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"backups": backups})
}

func (h *Handlers) deleteBackup(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "missing required parameter: id")
		return
	}
	record, err := h.store.GetBackupByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if record == nil {
		writeError(w, http.StatusNotFound, "backup not found")
		return
	}
	if record.BackupPath != "" {
		_ = backup.RemoveBackupArtifacts(record.BackupPath)
	}
	if err := h.store.DeleteBackup(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "Backup deleted successfully"})
}

func (h *Handlers) cleanupBackups(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DestinationID int64 `json:"destination_id"`
		RetentionDays int   `json:"retention_days"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.DestinationID == 0 || body.RetentionDays == 0 {
		writeError(w, http.StatusBadRequest, "missing required fields: destination_id, retention_days")
		return
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -body.RetentionDays).Format("2006-01-02 15:04:05")
	records, err := h.store.ListOldBackups(body.DestinationID, cutoff)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var deleted int64
	for _, record := range records {
		if record.BackupPath != "" {
			_ = backup.RemoveBackupArtifacts(record.BackupPath)
		}
		if err := h.store.DeleteBackup(record.ID); err == nil {
			deleted++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "Old backups cleaned up successfully", "deleted": deleted})
}

func (h *Handlers) ensureLocalDestination() (*store.BackupDestination, error) {
	destinations, err := h.store.GetBackupDestinations()
	if err != nil {
		return nil, err
	}
	for _, dest := range destinations {
		if dest.Type == "local" {
			return &dest, nil
		}
	}
	config := map[string]string{"path": h.backupBaseDir}
	payload, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	id, err := h.store.CreateBackupDestination("Local Server", "local", string(payload), 1)
	if err != nil {
		return nil, err
	}
	return h.store.GetBackupDestinationByID(id)
}

func (h *Handlers) listBackupFiles() ([]map[string]any, error) {
	baseDir := filepath.Clean(h.backupBaseDir)
	if _, err := os.Stat(baseDir); err != nil {
		if os.IsNotExist(err) {
			return []map[string]any{}, nil
		}
		return nil, err
	}
	var files []map[string]any
	err := filepath.WalkDir(baseDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == "_tmp" {
				return filepath.SkipDir
			}
			rel, relErr := filepath.Rel(baseDir, path)
			if relErr != nil {
				return filepath.SkipDir
			}
			depth := strings.Count(rel, string(filepath.Separator))
			if rel != "." && depth >= 2 {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(baseDir, path)
		if relErr != nil || strings.HasPrefix(rel, "..") {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if !strings.HasPrefix(rel, "sites/") && !strings.HasPrefix(rel, "databases/") && !strings.HasPrefix(rel, "downloads/") {
			return nil
		}
		if !strings.HasSuffix(rel, ".tar.gz") && !strings.HasSuffix(rel, ".dump.gz") {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return nil
		}
		files = append(files, map[string]any{
			"name":        rel,
			"size":        info.Size(),
			"modified_at": info.ModTime().UTC().Format(time.RFC3339),
		})
		return nil
	})
	return files, err
}
