package handlers

import (
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

// BackupExport backs up a selection of sites and databases into one .litpkg, kept on the server ("local") or offered as
// a download for an hour ("download"). It answers straight away with the backup's id; the dashboard then polls
// /api/backups/progress. With "wait": true it answers when the backup is done (the CLI does this).
//
// The older single-target form (target_type + target_id or domain, optionally with_databases) still works and makes a
// package of that one site or database.
func (h *Handlers) BackupExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Selection     *backup.Selection `json:"selection"`
		TargetType    string            `json:"target_type"`
		TargetID      int64             `json:"target_id"`
		Domain        string            `json:"domain"` // alternative to target_id for a site: its domain name
		WithDatabases bool              `json:"with_databases"`
		Delivery      string            `json:"delivery"`
		RetentionDays *int              `json:"retention_days"`
		Notes         string            `json:"notes"`
		Wait          bool              `json:"wait"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	sel, status, err := h.exportSelection(body.Selection, body.TargetType, body.TargetID, body.Domain, body.WithDatabases)
	if err != nil {
		writeError(w, status, err.Error())
		return
	}
	if allowed, err := h.selectionAllowed(r, sel); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	} else if !allowed {
		writeError(w, http.StatusForbidden, "forbidden")
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

	dest, err := h.ensureLocalDestination()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// A backup made by hand always goes in this agent's own backup folder, where the list and downloads look.
	dest.Config = ""
	subDir := backup.PackageSubDir
	if delivery == "download" {
		subDir = "downloads"
	}
	backupID, res, err := h.startPackage(nil, *dest, sel, subDir, body.Notes, delivery == "download", body.Wait)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if delivery == "local" && body.RetentionDays != nil && *body.RetentionDays > 0 {
		cutoff := time.Now().UTC().AddDate(0, 0, -*body.RetentionDays).Format("2006-01-02 15:04:05")
		if records, err := h.store.ListOldBackups(dest.ID, cutoff); err == nil {
			for _, record := range records {
				_ = backup.RemoveBackupArtifacts(record.BackupPath)
				_ = h.store.DeleteBackup(record.ID)
			}
		}
	}

	out := map[string]any{"backup_id": backupID, "progress_url": "/api/backups/progress?id=" + strconv.FormatInt(backupID, 10)}
	if res != nil {
		out["file"] = res.RelativePath
		out["size"] = res.Size
		out["description"] = res.Description
		out["problems"] = res.Problems
		if delivery == "download" {
			out["download_url"] = "/api/backups/local/download?file=" + url.QueryEscape(res.RelativePath)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// exportSelection turns an export request into a selection, accepting the older single-target form too.
func (h *Handlers) exportSelection(sel *backup.Selection, targetType string, targetID int64, domain string, withDatabases bool) (backup.Selection, int, error) {
	if sel != nil {
		if sel.Empty() {
			return backup.Selection{}, http.StatusBadRequest, fmt.Errorf("nothing was selected to back up")
		}
		return *sel, 0, nil
	}
	if targetType == "" && domain != "" {
		targetType = "site"
	}
	if targetType == "site" && targetID <= 0 && domain != "" {
		site, err := h.store.GetSiteByDomain(strings.ToLower(strings.TrimSpace(domain)))
		if err != nil || site == nil {
			return backup.Selection{}, http.StatusNotFound, fmt.Errorf("there is no site called %s", domain)
		}
		targetID = site.ID
	}
	switch {
	case targetType == "all-sites":
		return backup.Selection{AllSites: true, AllDatabases: withDatabases}, 0, nil
	case targetType == "all-databases":
		return backup.Selection{AllDatabases: true}, 0, nil
	case targetID <= 0:
		return backup.Selection{}, http.StatusBadRequest, fmt.Errorf("missing required fields: selection, or target_type and target_id")
	case targetType == "site":
		out := backup.Selection{Sites: []int64{targetID}}
		if withDatabases {
			out.SiteDatabases = []int64{targetID}
		}
		return out, 0, nil
	case targetType == "database":
		return backup.Selection{Databases: []int64{targetID}}, 0, nil
	}
	return backup.Selection{}, http.StatusBadRequest, fmt.Errorf("invalid target_type")
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
			writeJSON(w, http.StatusOK, map[string]any{"jobs": h.describeJobs(jobs)})
			return
		}
		jobs, err := h.store.GetBackupJobs()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"jobs": h.describeJobs(jobs)})
	case http.MethodPost:
		var body struct {
			DestinationID int64             `json:"destination_id"`
			Selection     *backup.Selection `json:"selection"` // what to back up; replaces target_type/target_id
			TargetType    string            `json:"target_type"`
			TargetID      *int64            `json:"target_id"`
			Domain        string            `json:"domain"` // alternative to target_id for a site
			Frequency     string            `json:"frequency"`
			RetentionDays *int              `json:"retention_days"`
			Enabled       *int              `json:"enabled"`
		}
		if err := readJSON(w, r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.TargetType == "site" && body.TargetID == nil && body.Domain != "" {
			site, err := h.store.GetSiteByDomain(strings.ToLower(strings.TrimSpace(body.Domain)))
			if err != nil || site == nil {
				writeError(w, http.StatusNotFound, "there is no site called "+body.Domain)
				return
			}
			body.TargetID = &site.ID
		}
		selection := ""
		if body.Selection != nil {
			if body.Selection.Empty() {
				writeError(w, http.StatusBadRequest, "nothing was selected to back up")
				return
			}
			var id int64
			body.TargetType, id = body.Selection.LegacyTarget(true)
			body.TargetID = nil
			if id > 0 {
				body.TargetID = &id
			}
			selection = body.Selection.JSON()
		}
		if body.DestinationID == 0 {
			if dest, err := h.ensureLocalDestination(); err == nil {
				body.DestinationID = dest.ID
			}
		}
		if body.DestinationID == 0 || body.TargetType == "" || body.Frequency == "" {
			writeError(w, http.StatusBadRequest, "missing required fields: destination_id, target_type, frequency")
			return
		}
		if _, err := backup.ParseSchedule(body.Frequency); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
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
		id, err := h.store.CreateBackupJob(body.DestinationID, body.TargetType, body.TargetID, body.Frequency, retention, enabled, selection)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "message": "Backup job created successfully"})
	case http.MethodPut:
		var body struct {
			ID            int64             `json:"id"`
			DestinationID *int64            `json:"destination_id"`
			Selection     *backup.Selection `json:"selection"`
			TargetType    *string           `json:"target_type"`
			TargetID      *int64            `json:"target_id"`
			Frequency     *string           `json:"frequency"`
			RetentionDays *int              `json:"retention_days"`
			Enabled       *int              `json:"enabled"`
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
		if body.Selection != nil {
			if body.Selection.Empty() {
				writeError(w, http.StatusBadRequest, "nothing was selected to back up")
				return
			}
			targetType, id := body.Selection.LegacyTarget(true)
			params["target_type"] = targetType
			params["target_id"] = nil
			if id > 0 {
				params["target_id"] = id
			}
			params["selection"] = body.Selection.JSON()
		}
		if body.Frequency != nil {
			if _, err := backup.ParseSchedule(*body.Frequency); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
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
	job, err := h.store.GetBackupJobByID(body.JobID)
	if err != nil || job == nil {
		writeError(w, http.StatusNotFound, "schedule not found")
		return
	}
	dest, err := h.store.GetBackupDestinationByID(job.Destination)
	if err != nil || dest == nil {
		writeError(w, http.StatusNotFound, "the schedule's destination is gone")
		return
	}
	sel, err := backup.SelectionForJob(*job)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	backupID, _, err := h.startPackage(job, *dest, sel, "", "", false, false)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "Backup job triggered successfully", "backup_id": backupID})
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
		if !strings.HasPrefix(rel, "sites/") && !strings.HasPrefix(rel, "databases/") && !strings.HasPrefix(rel, "downloads/") && !strings.HasPrefix(rel, backup.PackageSubDir+"/") {
			return nil
		}
		if !strings.HasSuffix(rel, ".tar.gz") && !strings.HasSuffix(rel, ".dump.gz") && !backup.IsPackage(rel) {
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

// jobView is a backup job plus its schedule and what it backs up in plain words, so the dashboard and terminal needn't
// parse the frequency text or the selection.
type jobView struct {
	store.BackupJob
	ScheduleText  string            `json:"schedule_text"`
	ScheduleValid bool              `json:"schedule_valid"`
	KeepLast      int               `json:"keep_last"`
	What          string            `json:"what"`           // "example.com + 2 databases"
	SelectionData *backup.Selection `json:"selection_data"` // the selection, also for schedules made before packages
}

func (h *Handlers) describeJobs(jobs []store.BackupJob) []jobView {
	out := make([]jobView, 0, len(jobs))
	for _, j := range jobs {
		v := jobView{BackupJob: j}
		if sel, err := backup.SelectionForJob(j); err == nil {
			v.SelectionData = &sel
			if res, err := sel.Resolve(h.store); err == nil {
				v.What = sel.Describe(res)
			}
		}
		if sched, err := backup.ParseSchedule(j.Frequency); err == nil {
			v.ScheduleText, v.ScheduleValid, v.KeepLast = sched.Describe(), true, sched.KeepLast
		} else {
			v.ScheduleText = "unreadable schedule (" + j.Frequency + ")"
		}
		out = append(out, v)
	}
	return out
}
