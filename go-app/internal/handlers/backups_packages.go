package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"docklite-agent/internal/backup"
	"docklite-agent/internal/store"
)

// backupTargetDB is a database as the backup picker shows it.
type backupTargetDB struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	SiteID *int64 `json:"site_id"`
}

type backupTargetSite struct {
	ID        int64            `json:"id"`
	Domain    string           `json:"domain"`
	UserID    int64            `json:"user_id"`
	Databases []backupTargetDB `json:"databases"`
}

// BackupTargets lists what can go into a backup: every site with the databases that belong to it, and every database
// (with the site it belongs to, if any). Non-admins see only the databases they have access to.
func (h *Handlers) BackupTargets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	h.syncDatabasesFromDocker(r.Context())
	admin := isAdminRole(r)
	if admin {
		h.linkSQLiteDatabases()
	}

	links, err := h.store.DatabaseSiteLinks()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var dbs []store.DatabaseRecord
	if admin {
		dbs, err = h.store.ListDatabases()
	} else {
		userID, ok := readUserIDFromContext(r)
		if !ok || userID <= 0 {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		dbs, err = h.store.ListDatabasesByUser(userID)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	outDBs := make([]backupTargetDB, 0, len(dbs))
	bySite := map[int64][]backupTargetDB{}
	for _, db := range dbs {
		t := backupTargetDB{ID: db.ID, Name: db.Name, Type: db.Type}
		if t.Type == "" {
			t.Type = "postgres"
		}
		if siteID, ok := links[db.ID]; ok {
			id := siteID
			t.SiteID = &id
			bySite[siteID] = append(bySite[siteID], t)
		}
		outDBs = append(outDBs, t)
	}

	outSites := []backupTargetSite{}
	if admin {
		sites, err := h.store.ListSites()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		for _, s := range sites {
			linked := bySite[s.ID]
			if linked == nil {
				linked = []backupTargetDB{}
			}
			outSites = append(outSites, backupTargetSite{ID: s.ID, Domain: s.Domain, UserID: s.UserID, Databases: linked})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"sites": outSites, "databases": outDBs, "is_admin": admin})
}

// syncDatabasesFromDocker records database containers DockLite doesn't know about yet, like the Databases page does, so
// the picker shows every database. Best effort.
func (h *Handlers) syncDatabasesFromDocker(ctx context.Context) {
	if h.docker == nil {
		return
	}
	ctx, cancel := dockerContext(ctx)
	defer cancel()
	found, err := h.docker.ListDatabases(ctx)
	if err != nil {
		return
	}
	for _, db := range found {
		_, _ = h.store.UpsertDatabase(db.Name, db.Type, db.ID, db.Port, "")
	}
}

// linkSQLiteDatabases links each SQLite database to the site whose container it lives in, unless it is linked already.
// Best effort.
func (h *Handlers) linkSQLiteDatabases() {
	dbs, err := h.store.ListDatabases()
	if err != nil {
		return
	}
	links, err := h.store.DatabaseSiteLinks()
	if err != nil {
		return
	}
	sites, err := h.store.ListSites()
	if err != nil {
		return
	}
	siteByContainer := map[string]int64{}
	for _, s := range sites {
		if s.ContainerID != nil && *s.ContainerID != "" {
			siteByContainer[*s.ContainerID] = s.ID
		}
	}
	for _, db := range dbs {
		if db.Type != "sqlite" || db.ContainerID == "" {
			continue
		}
		if _, linked := links[db.ID]; linked {
			continue
		}
		if siteID, ok := siteByContainer[db.ContainerID]; ok {
			_ = h.store.SetDatabaseSite(db.ID, siteID)
		}
	}
}

// BackupLinks says which site a database belongs to, so "this site and its databases" knows what to include.
// POST {"database_id": 3, "site_id": 7}; site_id 0 unlinks it.
func (h *Handlers) BackupLinks(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		DatabaseID int64 `json:"database_id"`
		SiteID     int64 `json:"site_id"`
	}
	if err := readJSON(w, r, &body); err != nil || body.DatabaseID <= 0 {
		writeError(w, http.StatusBadRequest, "database_id is required")
		return
	}
	if db, err := h.store.GetDatabaseByID(body.DatabaseID); err != nil || db == nil {
		writeError(w, http.StatusNotFound, "database not found")
		return
	}
	if body.SiteID > 0 {
		if site, err := h.store.GetSiteByID(body.SiteID); err != nil || site == nil {
			writeError(w, http.StatusNotFound, "site not found")
			return
		}
	}
	if err := h.store.SetDatabaseSite(body.DatabaseID, body.SiteID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// BackupProgress reports how a backup is going: GET /api/backups/progress?id=<backup id>. Once the agent no longer has
// it in memory (finished over an hour ago, or the agent restarted) the history row answers instead.
func (h *Handlers) BackupProgress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	if p, ok := backup.DefaultTracker.Snapshot(id); ok {
		writeJSON(w, http.StatusOK, p)
		return
	}
	rec, err := h.store.GetBackupByID(id)
	if err != nil || rec == nil {
		writeError(w, http.StatusNotFound, "backup not found")
		return
	}
	p := backup.Progress{BackupID: id, Status: rec.Status, Percent: 100, Size: rec.SizeBytes, StartedAt: rec.CreatedAt}
	if rec.Status == "in_progress" {
		p.Status, p.Percent = backup.StatusRunning, -1
	}
	if rec.ErrorMessage != nil {
		if rec.Status == "failed" {
			p.Error = *rec.ErrorMessage
		} else {
			p.Warnings = []string{*rec.ErrorMessage}
			p.WarningCount = 1
		}
	}
	writeJSON(w, http.StatusOK, p)
}

// selectionAllowed checks a non-admin may back up a selection: only databases they have access to.
func (h *Handlers) selectionAllowed(r *http.Request, sel backup.Selection) (bool, error) {
	if isAdminRole(r) {
		return true, nil
	}
	if sel.AllSites || sel.AllDatabases || len(sel.Sites) > 0 || len(sel.SiteDatabases) > 0 || len(sel.Groups) > 0 || len(sel.Containers) > 0 {
		return false, nil
	}
	userID, ok := readUserIDFromContext(r)
	if !ok || userID <= 0 {
		return false, nil
	}
	for _, id := range sel.Databases {
		allowed, err := h.store.HasDatabaseAccess(userID, id)
		if err != nil || !allowed {
			return false, err
		}
	}
	return true, nil
}

// startPackage makes a package in the background, with a progress bar the dashboard polls. jobKey keeps one schedule
// from running twice at once (0 for a backup made by hand). When wait is set it returns only once the backup is done.
func (h *Handlers) startPackage(job *store.BackupJob, dest store.BackupDestination, sel backup.Selection, subDir, notes string, download, wait bool) (int64, *backup.PackageResult, error) {
	backupID, err := backup.StartPackageRecord(h.store, h.docker, job, dest, sel)
	if err != nil {
		return 0, nil, err
	}
	trackType, trackID := "package", backupID
	if job != nil {
		trackType, trackID = "job", job.ID
	}
	rep := backup.DefaultTracker.Start(backupID, trackType, trackID)
	if rep == nil {
		msg := "this schedule is already running"
		_ = h.store.UpdateBackupStatus(backupID, "failed", &msg, nil, nil)
		return backupID, nil, fmt.Errorf("%s", msg)
	}
	type outcome struct {
		res *backup.PackageResult
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		ctx := backup.WithReporter(context.Background(), rep)
		res, err := backup.FinishPackage(ctx, h.store, h.docker, h.backupBaseDir, backupID, job, dest, sel, subDir, notes)
		if err != nil {
			rep.Fail(err.Error())
		} else {
			if download {
				rep.SetDownloadURL("/api/backups/local/download?file=" + url.QueryEscape(res.RelativePath))
				time.AfterFunc(time.Hour, func() {
					_ = backup.RemoveBackupArtifacts(res.Path)
					_ = h.store.DeleteBackup(backupID)
				})
			}
			rep.Succeed(&res.ArtifactResult, res.Verified)
		}
		done <- outcome{res, err}
	}()
	if !wait {
		return backupID, nil, nil
	}
	o := <-done
	return backupID, o.res, o.err
}
