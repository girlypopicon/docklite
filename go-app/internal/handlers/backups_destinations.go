package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"docklite-agent/internal/backup"
	"docklite-agent/internal/store"
)

// incomingDir is where this server keeps backups that other DockLite servers send it ("server" destinations).
const incomingDir = "incoming"

// prepareDestinationConfig checks a destination's settings before they are saved and returns the JSON to store. old is
// the saved config when editing (a blank secret keeps the stored one). For an SFTP server whose key isn't pinned yet it
// connects once to read the key and saves its fingerprint, returned so the person can compare it.
func (h *Handlers) prepareDestinationConfig(ctx context.Context, typ, old, next string) (string, string, error) {
	if old != "" {
		next = backup.MergeConfig(old, next)
	}
	cfg := backup.ParseDestConfig(next)
	if typ == "local" && cfg.Path == "" && !cfg.Encrypt {
		// an empty local config keeps meaning "this agent's own backup folder"
		return next, "", nil
	}
	if err := cfg.Validate(typ); err != nil {
		return "", "", err
	}
	if typ == "server" {
		if srv, err := h.store.GetRemoteServer(cfg.ServerID); err != nil || srv == nil {
			return "", "", fmt.Errorf("that server isn't connected; add it on the Servers page first")
		}
	}
	hostKey := ""
	if typ == "sftp" {
		oldCfg := backup.ParseDestConfig(old)
		if cfg.HostKey != "" && (oldCfg.Host != cfg.Host || oldCfg.Port != cfg.Port) && cfg.HostKey == oldCfg.HostKey {
			cfg.HostKey = "" // a different server: its key is not the saved one
		}
		if cfg.HostKey == "" {
			c, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			fp, err := backup.ScanHostKey(c, cfg.Host, cfg.Port)
			if err != nil {
				return "", "", err
			}
			cfg.HostKey, hostKey = fp, fp
		}
	}
	b, err := json.Marshal(cfg)
	return string(b), hostKey, err
}

// BackupDestinationTest (POST /api/backups/destinations/test) checks that a destination works: it connects, writes a
// tiny file and removes it. Send {id} for a saved destination, or {type, config} for one that is still a form (with an
// id as well, blank secrets are taken from the saved one).
func (h *Handlers) BackupDestinationTest(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		ID     int64           `json:"id"`
		Type   string          `json:"type"`
		Config json.RawMessage `json:"config"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	typ, config := body.Type, string(body.Config)
	if body.ID > 0 {
		saved, err := h.store.GetBackupDestinationByID(body.ID)
		if err != nil || saved == nil {
			writeError(w, http.StatusNotFound, "destination not found")
			return
		}
		typ = saved.Type
		if config == "" {
			config = saved.Config
		} else {
			config = backup.MergeConfig(saved.Config, config)
		}
	}
	fail := func(err error) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
	}
	if typ == "local" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "A folder on this server."})
		return
	}
	prepared, hostKey, err := h.prepareDestinationConfig(r.Context(), typ, "", config)
	if err != nil {
		fail(err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	remote, err := backup.OpenRemote(h.store, storeDest(typ, prepared))
	if err != nil {
		fail(err)
		return
	}
	msg, err := remote.Test(ctx)
	if err != nil {
		fail(err)
		return
	}
	out := map[string]any{"ok": true, "message": msg}
	if hostKey != "" {
		out["host_key"] = hostKey
	}
	writeJSON(w, http.StatusOK, out)
}

// BackupKeys (POST /api/backups/keys) makes a new backup key. The private half is in this answer and nowhere else:
// DockLite keeps only the public half, which can lock backups but not open them.
func (h *Handlers) BackupKeys(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	recipient, identity, err := backup.GenerateKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(r, "backup.key.generate", recipient, nil)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"recipient": recipient,
		"identity":  identity,
		"warning":   "Save the private key somewhere safe now (a password manager). DockLite does not keep it, and without it these backups can never be opened.",
	})
}

// BackupFetch (POST /api/backups/fetch {backup_id, identity?}) brings a backup that is on another destination, or
// locked, to this server as a plain package in the downloads folder, so it can be inspected and restored like any other.
func (h *Handlers) BackupFetch(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		BackupID int64  `json:"backup_id"`
		Identity string `json:"identity"`
	}
	if err := readJSON(w, r, &body); err != nil || body.BackupID <= 0 {
		writeError(w, http.StatusBadRequest, "backup_id is required")
		return
	}
	rec, err := h.store.GetBackupByID(body.BackupID)
	if err != nil || rec == nil || rec.Status != "success" || rec.BackupPath == "" {
		writeError(w, http.StatusNotFound, "that backup isn't available")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
	defer cancel()
	path, err := backup.FetchRecord(ctx, h.store, h.backupBaseDir, *rec, body.Identity)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	// The plain copy is only here to restore from; it goes away after an hour.
	time.AfterFunc(time.Hour, func() { _ = os.Remove(path) })
	rel, _ := filepath.Rel(h.backupBaseDir, path)
	h.audit(r, "backup.fetch", strconv.FormatInt(rec.ID, 10), nil)
	writeJSON(w, http.StatusOK, map[string]any{"file": filepath.ToSlash(rel), "message": "The backup is ready to restore. The copy on this server is removed after an hour."})
}

// incomingName keeps what another server sends to a plain file name in the incoming folder.
func incomingName(raw string) (string, bool) {
	base := filepath.Base(filepath.FromSlash(strings.TrimSpace(raw)))
	name := backup.SanitizeFilename(base)
	if !backup.IsPackage(name) && !backup.IsPackage(strings.TrimSuffix(name, backup.EncryptedExt)) {
		return "", false
	}
	return name, true
}

// BackupIncoming is how another DockLite server stores its backups here (a "server" destination there):
//
//	GET    /api/backups/incoming               what is stored (also how the other side tests the connection)
//	POST   /api/backups/incoming?name=…        body: the file
//	GET    /api/backups/incoming/file?name=…   the file back
//	DELETE /api/backups/incoming?name=…
func (h *Handlers) BackupIncoming(w http.ResponseWriter, r *http.Request) {
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}
	dir := filepath.Join(h.backupBaseDir, incomingDir)
	isFile := strings.HasSuffix(strings.TrimRight(r.URL.Path, "/"), "/file")
	switch {
	case r.Method == http.MethodGet && !isFile:
		entries, _ := os.ReadDir(dir)
		files := []map[string]any{}
		for _, e := range entries {
			if info, err := e.Info(); err == nil && !e.IsDir() && !strings.HasSuffix(e.Name(), ".partial") {
				files = append(files, map[string]any{"name": e.Name(), "size": info.Size()})
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"files": files})
	case r.Method == http.MethodGet && isFile:
		name, ok := incomingName(r.URL.Query().Get("name"))
		if !ok {
			writeError(w, http.StatusBadRequest, "that isn't a backup file name")
			return
		}
		http.ServeFile(w, r, filepath.Join(dir, name))
	case r.Method == http.MethodPost:
		name, ok := incomingName(r.URL.Query().Get("name"))
		if !ok {
			writeError(w, http.StatusBadRequest, "that isn't a backup file name (it should end in .litpkg or .litpkg.age)")
			return
		}
		if r.ContentLength <= 0 {
			writeError(w, http.StatusLengthRequired, "the backup's size is needed")
			return
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		tmp := filepath.Join(dir, name+".partial")
		out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		n, err := io.Copy(out, r.Body)
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err == nil && n != r.ContentLength {
			err = fmt.Errorf("only %d of %d bytes arrived", n, r.ContentLength)
		}
		if err == nil {
			err = os.Rename(tmp, filepath.Join(dir, name))
		}
		if err != nil {
			_ = os.Remove(tmp)
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.audit(r, "backup.incoming", name, map[string]any{"bytes": n})
		writeJSON(w, http.StatusOK, map[string]any{"name": name, "bytes": n})
	case r.Method == http.MethodDelete:
		name, ok := incomingName(r.URL.Query().Get("name"))
		if !ok {
			writeError(w, http.StatusBadRequest, "that isn't a backup file name")
			return
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		_ = os.Remove(backup.ManifestPathForArtifact(filepath.Join(dir, name)))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// storeDest wraps a type and settings as a destination that isn't saved (for testing it).
func storeDest(typ, config string) store.BackupDestination {
	return store.BackupDestination{Type: typ, Config: config, Name: typ}
}
