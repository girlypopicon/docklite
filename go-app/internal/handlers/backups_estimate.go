package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"docklite-agent/internal/backup"
	"docklite-agent/internal/store"
)

// How much smaller a package is than what goes into it. Site files are a mix of text (which gzip shrinks a lot) and
// images or archives (which it can't); database dumps are text. These are rough, and the dashboard says "about".
const (
	siteCompressionRatio = 0.6
	dbCompressionRatio   = 0.35
)

type estimateItem struct {
	Name  string `json:"name"`
	Kind  string `json:"kind"` // "site" or the database type
	Bytes int64  `json:"bytes"`
	Known bool   `json:"known"` // false when the size couldn't be read (a stopped database, a file database)
}

// BackupEstimate answers "how big would this backup be?" for a selection, without making it. POST {"selection": {...}}.
func (h *Handlers) BackupEstimate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Selection backup.Selection `json:"selection"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Selection.Empty() {
		writeJSON(w, http.StatusOK, map[string]any{"items": []estimateItem{}, "raw_bytes": 0, "estimate_bytes": 0, "free_bytes": h.backupFreeBytes()})
		return
	}
	if allowed, err := h.selectionAllowed(r, body.Selection); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	} else if !allowed {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	resolved, err := body.Selection.Resolve(h.store)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	ctx, cancel := dockerContext(r.Context())
	defer cancel()

	users, passwords := map[string]string{}, map[string]string{}
	if len(resolved.Databases) > 0 {
		if found, err := h.docker.ListDatabases(ctx); err == nil {
			for _, d := range found {
				users[d.ID], passwords[d.ID] = d.Username, d.Password
			}
		}
	}

	items := []estimateItem{}
	var siteBytes, dbBytes float64
	unknown := 0
	for _, site := range resolved.Sites {
		n := backup.TreeSize(site.CodePath)
		items = append(items, estimateItem{Name: site.Domain, Kind: "site", Bytes: n, Known: site.CodePath != ""})
		siteBytes += float64(n)
	}
	// Databases are asked in parallel: each is a command run inside its container.
	dbItems := make([]estimateItem, len(resolved.Databases))
	var wg sync.WaitGroup
	for i, db := range resolved.Databases {
		wg.Add(1)
		go func(i int, db store.DatabaseRecord) {
			defer wg.Done()
			n, ok := h.databaseSizeBytes(ctx, db, users[db.ContainerID], passwords[db.ContainerID])
			dbItems[i] = estimateItem{Name: db.Name, Kind: db.Type, Bytes: n, Known: ok}
		}(i, db.DatabaseRecord)
	}
	wg.Wait()
	for _, it := range dbItems {
		if !it.Known {
			unknown++
		}
		items = append(items, it)
		dbBytes += float64(it.Bytes)
	}
	raw := int64(siteBytes + dbBytes)
	estimate := int64(siteBytes*siteCompressionRatio + dbBytes*dbCompressionRatio)
	writeJSON(w, http.StatusOK, map[string]any{
		"items":          items,
		"raw_bytes":      raw,
		"estimate_bytes": estimate,
		"unknown":        unknown,
		"free_bytes":     h.backupFreeBytes(),
	})
}

// backupFreeBytes is the space left where local backups are written (0 when it can't be read).
func (h *Handlers) backupFreeBytes() int64 {
	var st syscall.Statfs_t
	if h.backupBaseDir == "" || syscall.Statfs(h.backupBaseDir, &st) != nil {
		return 0
	}
	return int64(st.Bavail) * int64(st.Bsize)
}

// databaseSizeBytes is how much data a database holds, when it can be read (it must be running, with a known login).
func (h *Handlers) databaseSizeBytes(ctx context.Context, db store.DatabaseRecord, user, pass string) (int64, bool) {
	if db.ContainerID == "" || db.Type == "sqlite" || pass == "" || (user == "" && db.Type != "redis") {
		return 0, false
	}
	if db.Type == "" || db.Type == "postgres" {
		out, err := h.docker.ExecPostgres(ctx, db.ContainerID, user, db.Name, pass, "SELECT pg_database_size('"+strings.ReplaceAll(db.Name, "'", "")+"')", false)
		if err != nil {
			return 0, false
		}
		n, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
		return n, err == nil
	}
	n, err := h.docker.EngineSize(ctx, db.ContainerID, db.Type, db.Name, user, pass)
	return n, err == nil
}
