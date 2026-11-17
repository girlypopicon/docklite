package handlers

import (
	"compress/gzip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"

	"docklite-agent/internal/store"
)

// A SQLite database is one file inside a site, so it has no server, no username and no password. These handlers
// browse and edit it straight from the site's folder on the host, using the same request and answer shapes as the
// Postgres table editor, so the dashboard's editor works for it unchanged (minus the login).

type mountPair struct{ Source, Dest string }

// hostPathForContainerFile finds where a path inside a container really is on the host, through the container's bind
// mounts. The longest matching mount wins; a path that tries to climb out of its mount with ".." is refused.
func hostPathForContainerFile(mounts []mountPair, containerPath string) (string, error) {
	clean := path.Clean("/" + strings.TrimSpace(containerPath))
	best := -1
	for i, m := range mounts {
		dest := path.Clean(m.Dest)
		if m.Source == "" || dest == "." {
			continue
		}
		if clean == dest || strings.HasPrefix(clean, strings.TrimSuffix(dest, "/")+"/") {
			if best < 0 || len(dest) > len(path.Clean(mounts[best].Dest)) {
				best = i
			}
		}
	}
	if best < 0 {
		return "", fmt.Errorf("%s is not inside a folder this site shares with DockLite, so DockLite can't open it", containerPath)
	}
	rel := strings.TrimPrefix(clean, path.Clean(mounts[best].Dest))
	host := filepath.Join(mounts[best].Source, filepath.FromSlash(rel))
	if host != mounts[best].Source && !strings.HasPrefix(host, filepath.Clean(mounts[best].Source)+string(filepath.Separator)) {
		return "", fmt.Errorf("that path leaves the site's folder")
	}
	return host, nil
}

// mountPairs turns a container's mounts into source/destination pairs.
func mountPairs(in []types.MountPoint) []mountPair {
	out := make([]mountPair, 0, len(in))
	for _, m := range in {
		out = append(out, mountPair{Source: m.Source, Dest: m.Destination})
	}
	return out
}

// sqlitePathHelp names the folders of a site that DockLite can reach, for the error a wrong path gets.
func sqlitePathHelp(mounts []mountPair) string {
	var dests []string
	for _, m := range mounts {
		if m.Source != "" && strings.HasPrefix(m.Dest, "/") && m.Dest != "/" {
			dests = append(dests, m.Dest)
		}
	}
	if len(dests) == 0 {
		return ""
	}
	return " Start the path with " + strings.Join(dests, " or ") + " (for example " + strings.TrimSuffix(dests[0], "/") + "/data/db.sqlite)."
}

// prepareSQLiteFile makes sure the database file exists on the host, so a new database can be opened straight
// away (an empty SQLite file is a valid, empty database). It never touches a file that is already there.
func prepareSQLiteFile(mounts []mountPair, containerPath string) error {
	host, err := hostPathForContainerFile(mounts, containerPath)
	if err != nil {
		return fmt.Errorf("%v.%s", strings.TrimSuffix(err.Error(), "."), sqlitePathHelp(mounts))
	}
	if _, err := os.Stat(host); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(host), 0o777); err != nil {
		return fmt.Errorf("couldn't make the folder for the database: %v", err)
	}
	f, err := os.OpenFile(host, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o666)
	if err != nil {
		return fmt.Errorf("couldn't make the database file: %v", err)
	}
	_ = f.Close()
	_ = os.Chmod(host, 0o666)
	return nil
}

// openSQLiteDatabase opens the file a SQLite database record points at. The caller closes it.
func (h *Handlers) openSQLiteDatabase(ctx context.Context, rec *store.DatabaseRecord) (*sql.DB, string, error) {
	if rec.ContainerID == "" || rec.DBPath == "" {
		return nil, "", fmt.Errorf("this database isn't linked to a site file")
	}
	info, err := h.docker.InspectContainer(ctx, rec.ContainerID)
	if err != nil {
		return nil, "", fmt.Errorf("couldn't look at the site this database lives in: %v", err)
	}
	host, err := hostPathForContainerFile(mountPairs(info.Mounts), rec.DBPath)
	if err != nil {
		return nil, "", err
	}
	st, err := os.Stat(host)
	if err != nil {
		return nil, "", fmt.Errorf("the file %s doesn't exist yet. It appears once the site first writes to it", rec.DBPath)
	}
	if !st.Mode().IsRegular() {
		return nil, "", fmt.Errorf("%s is not a file", rec.DBPath)
	}
	db, err := sql.Open("sqlite", "file:"+(&url.URL{Path: host}).EscapedPath()+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, "", err
	}
	db.SetMaxOpenConns(1)
	return db, host, nil
}

func sqliteQuoteIdent(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }

type sqliteColumn struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Default  any    `json:"default"`
	Key      string `json:"key"`
}

func sqliteColumns(db *sql.DB, table string) ([]sqliteColumn, error) {
	rows, err := db.Query("PRAGMA table_info(" + sqliteQuoteIdent(table) + ")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := []sqliteColumn{}
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		c := sqliteColumn{Name: name, Type: strings.ToLower(typ), Nullable: notnull == 0 && pk == 0}
		if c.Type == "" {
			c.Type = "any"
		}
		if dflt.Valid {
			c.Default = dflt.String
		}
		if pk > 0 {
			c.Key = "PRIMARY KEY"
		}
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

// sqliteSchema lists the tables and their columns, like the Postgres schema answer.
func sqliteSchema(db *sql.DB) ([]map[string]any, error) {
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			return nil, err
		}
		names = append(names, n)
	}
	rows.Close()
	out := make([]map[string]any, 0, len(names))
	for _, n := range names {
		cols, err := sqliteColumns(db, n)
		if err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"name": n, "columns": cols})
	}
	return out, nil
}

// sqliteRowsToMaps reads every remaining row as a map, turning byte blobs into text so they can be sent as JSON.
func sqliteRowsToMaps(rows *sql.Rows) ([]string, []map[string]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}
	out := []map[string]any{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		row := make(map[string]any, len(cols))
		for i, c := range cols {
			if b, ok := vals[i].([]byte); ok {
				row[c] = string(b)
			} else {
				row[c] = vals[i]
			}
		}
		out = append(out, row)
	}
	return cols, out, rows.Err()
}

// sqliteTable returns the first 100 rows of a table with its columns.
func sqliteTable(db *sql.DB, table string) ([]sqliteColumn, []map[string]any, error) {
	cols, err := sqliteColumns(db, table)
	if err != nil {
		return nil, nil, err
	}
	if len(cols) == 0 {
		return nil, nil, fmt.Errorf("there is no table called %s", table)
	}
	rows, err := db.Query("SELECT * FROM " + sqliteQuoteIdent(table) + " LIMIT 100")
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	_, data, err := sqliteRowsToMaps(rows)
	return cols, data, err
}

// sqliteRunSQL runs what was typed: a query answers with rows, anything else with how many rows it changed.
func sqliteRunSQL(db *sql.DB, text string) (map[string]any, error) {
	stmt := normalizeSQL(text)
	lower := strings.ToLower(stmt)
	if strings.HasPrefix(lower, "select") || strings.HasPrefix(lower, "with") || strings.HasPrefix(lower, "pragma") {
		rows, err := db.Query(stmt)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		cols, data, err := sqliteRowsToMaps(rows)
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "select", "rows": data, "columns": cols}, nil
	}
	res, err := db.Exec(stmt)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	return map[string]any{"type": "command", "output": fmt.Sprintf("OK, %d rows changed", n)}, nil
}

// sqliteUpdateRows sets the given columns on rows picked by their id.
func sqliteUpdateRows(db *sql.DB, table string, rows []map[string]any) error {
	for _, row := range rows {
		id, ok := row["id"]
		if !ok {
			return fmt.Errorf("each row must include an id field")
		}
		var sets []string
		var args []any
		for col, val := range row {
			if col == "id" {
				continue
			}
			if !identifierPattern.MatchString(col) {
				return fmt.Errorf("invalid column name")
			}
			sets = append(sets, sqliteQuoteIdent(col)+" = ?")
			args = append(args, val)
		}
		if len(sets) == 0 {
			continue
		}
		args = append(args, id)
		if _, err := db.Exec("UPDATE "+sqliteQuoteIdent(table)+" SET "+strings.Join(sets, ", ")+` WHERE "id" = ?`, args...); err != nil {
			return err
		}
	}
	return nil
}

// sqliteRoute handles /api/databases/{id}/{query,schema,table,update-rows,download} for a SQLite database. None of
// them needs a username or password: the file itself is the database.
func (h *Handlers) sqliteRoute(w http.ResponseWriter, r *http.Request, rec *store.DatabaseRecord, action string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if action == "download" {
		h.sqliteDownload(w, r, rec)
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		// The dashboard's editor sends a username and password with every request; a file has neither, so they are ignored.
		Username string           `json:"username"`
		Password string           `json:"password"`
		Table    string           `json:"table"`
		SQL      string           `json:"sql"`
		Rows     []map[string]any `json:"rows"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ctx, cancel := dockerContext(r.Context())
	defer cancel()
	db, _, err := h.openSQLiteDatabase(ctx, rec)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer db.Close()

	fail := func(err error) { writeError(w, http.StatusBadRequest, err.Error()) }
	switch action {
	case "schema":
		tables, err := sqliteSchema(db)
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"tables": tables})
	case "table":
		if !identifierPattern.MatchString(body.Table) {
			writeError(w, http.StatusBadRequest, "invalid table name")
			return
		}
		cols, data, err := sqliteTable(db, body.Table)
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"columns": cols, "rows": data})
	case "query":
		if strings.TrimSpace(body.SQL) == "" {
			writeError(w, http.StatusBadRequest, "sql is required")
			return
		}
		out, err := sqliteRunSQL(db, body.SQL)
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	case "update-rows":
		if !identifierPattern.MatchString(body.Table) {
			writeError(w, http.StatusBadRequest, "invalid table name")
			return
		}
		if err := sqliteUpdateRows(db, body.Table, body.Rows); err != nil {
			fail(err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true})
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

// sqliteDownload sends the database file itself (a copy that is consistent even while the site is writing), optionally gzipped.
func (h *Handlers) sqliteDownload(w http.ResponseWriter, r *http.Request, rec *store.DatabaseRecord) {
	if !isAdminRole(r) {
		userID, err := readUserID(r)
		if err != nil || userID == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		allowed, err := h.store.HasDatabaseAccess(*userID, rec.ID)
		if err != nil || !allowed {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Gzip     bool   `json:"gzip"`
	}
	_ = readJSON(w, r, &body)
	ctx, cancel := dockerContext(r.Context())
	defer cancel()
	db, host, err := h.openSQLiteDatabase(ctx, rec)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer db.Close()

	// VACUUM INTO writes a clean, complete copy even if the file is in the middle of being written to.
	tmp, err := os.CreateTemp("", "docklite-sqlite-*.db")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not prepare the download")
		return
	}
	tmpPath := tmp.Name()
	tmp.Close()
	os.Remove(tmpPath) // VACUUM INTO needs the target not to exist
	defer os.Remove(tmpPath)
	if _, err := db.Exec("VACUUM INTO ?", tmpPath); err != nil {
		writeError(w, http.StatusInternalServerError, "could not make a copy of the database: "+err.Error())
		return
	}
	f, err := os.Open(tmpPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read the copy")
		return
	}
	defer f.Close()

	name := strings.TrimSuffix(filepath.Base(host), filepath.Ext(host))
	if name == "" {
		name = rec.Name
	}
	stamp := time.Now().Format("20060102-150405")
	if body.Gzip {
		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s.sqlite.gz"`, name, stamp))
		gz := gzip.NewWriter(w)
		_, _ = io.Copy(gz, f)
		_ = gz.Close()
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s.sqlite"`, name, stamp))
	_, _ = io.Copy(w, f)
}
