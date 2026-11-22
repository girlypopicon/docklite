package handlers

import (
	"context"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"docklite-agent/internal/store"
)

// A SQLite database is a file, so nothing announces it: a site's own code just opens "board.db" and it exists. DockLite
// finds them by looking inside each site's shared folders for files that really are SQLite databases, and gives each one
// a card, exactly as if it had been made through "New box". It only reads (the first 16 bytes of each candidate) and
// never follows links, so a site cannot point the scan at files outside its own folder.

var sqliteExts = map[string]bool{".db": true, ".sqlite": true, ".sqlite3": true, ".db3": true}

// Folders that are full of other people's files; a database in there is almost never the site's own.
var sqliteSkipDirs = map[string]bool{
	"node_modules": true, ".git": true, ".next": true, ".nuxt": true, "vendor": true, ".cache": true,
	"__pycache__": true, ".venv": true, "venv": true, ".npm": true, ".bun": true, ".svn": true,
}

const (
	sqliteScanMaxDepth   = 6
	sqliteScanMaxEntries = 50000
	sqliteScanMaxFound   = 200
)

type sqliteFile struct {
	ContainerPath string // where the site sees it
	Size          int64
}

var sqliteMagic = []byte("SQLite format 3\x00")

func isSQLiteFile(p string) bool {
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, len(sqliteMagic))
	if _, err := io.ReadFull(f, head); err != nil {
		return false
	}
	return string(head) == string(sqliteMagic)
}

// scanSQLiteFiles walks one shared folder (host path source, seen by the site as dest) for SQLite databases.
func scanSQLiteFiles(source, dest string) []sqliteFile {
	var found []sqliteFile
	entries := 0
	root := filepath.Clean(source)
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		entries++
		if entries > sqliteScanMaxEntries || len(found) >= sqliteScanMaxFound {
			return filepath.SkipAll
		}
		rel, _ := filepath.Rel(root, p)
		if d.IsDir() {
			if p != root && (sqliteSkipDirs[d.Name()] || strings.Count(rel, string(filepath.Separator)) >= sqliteScanMaxDepth) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() { // links and devices are never followed
			return nil
		}
		if !sqliteExts[strings.ToLower(filepath.Ext(d.Name()))] {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() < 100 {
			// An empty file is a valid empty database, but a site that never wrote to it has nothing to show yet.
			return nil
		}
		if !isSQLiteFile(p) {
			return nil
		}
		found = append(found, sqliteFile{ContainerPath: path.Join(dest, filepath.ToSlash(rel)), Size: info.Size()})
		return nil
	})
	return found
}

var (
	sqliteScanMu   sync.Mutex
	sqliteScanLast time.Time
)

type sqliteDiscovery struct {
	Added    []string `json:"added"`
	Existing int      `json:"existing"`
}

// discoverSQLiteDatabases registers every SQLite file it finds in the sites' folders that isn't registered yet or
// removed on purpose. Scans are limited to one per 20 seconds unless forced, because the dashboard asks often.
func (h *Handlers) discoverSQLiteDatabases(ctx context.Context, force bool) (*sqliteDiscovery, error) {
	sqliteScanMu.Lock()
	defer sqliteScanMu.Unlock()
	if !force && time.Since(sqliteScanLast) < 20*time.Second {
		return &sqliteDiscovery{}, nil
	}
	sqliteScanLast = time.Now()

	containers, err := h.docker.VisibleContainers(ctx, true)
	if err != nil {
		return nil, err
	}
	live := map[string]bool{}
	for _, c := range containers {
		live[c.ID] = true
	}
	ignored, _ := h.store.IgnoredSQLiteFiles()
	records, err := h.store.ListDatabases()
	if err != nil {
		return nil, err
	}
	known := map[string]bool{} // "container id\x00path"
	for _, r := range records {
		if r.Type == "sqlite" {
			known[r.ContainerID+"\x00"+r.DBPath] = true
		}
	}

	out := &sqliteDiscovery{Added: []string{}}
	for _, c := range containers {
		domain := c.Labels["docklite.domain"]
		if c.Labels["docklite.managed"] != "true" || domain == "" {
			continue
		}
		cname := ""
		if len(c.Names) > 0 {
			cname = strings.TrimPrefix(c.Names[0], "/")
		}
		for _, m := range c.Mounts {
			if m.Type != "bind" || m.Source == "" || !strings.HasPrefix(m.Destination, "/") {
				continue
			}
			// Only the sites' own folders: a bind mount of anything else is not DockLite's to look through.
			if !strings.HasPrefix(filepath.Clean(m.Source), filepath.Clean(siteBaseDir)+string(filepath.Separator)) {
				continue
			}
			for _, f := range scanSQLiteFiles(m.Source, m.Destination) {
				if known[c.ID+"\x00"+f.ContainerPath] {
					out.Existing++
					continue
				}
				if ignored[cname+"\x00"+f.ContainerPath] {
					continue
				}
				name := h.freeSQLiteName(domain, f.ContainerPath, c.ID, live)
				rec, err := h.store.UpsertDatabase(name, "sqlite", c.ID, 0, f.ContainerPath)
				if err != nil {
					continue
				}
				known[c.ID+"\x00"+f.ContainerPath] = true
				if sid, err := strconv.ParseInt(c.Labels["docklite.site.id"], 10, 64); err == nil && sid > 0 {
					_ = h.store.SetDatabaseSite(rec.ID, sid)
				}
				out.Added = append(out.Added, name)
			}
		}
	}
	return out, nil
}

// freeSQLiteName picks the name for a found file: the file's own name ("board"), then with the site in front
// ("stellagauthier_com_board"), then numbered. A name already used by a record on a live site is not reused, because
// a record is looked up by name; a record on a site that no longer exists is, so a recreated site keeps its database.
func (h *Handlers) freeSQLiteName(domain, containerPath, containerID string, live map[string]bool) string {
	base := sanitizeDatabaseName(strings.TrimSuffix(path.Base(containerPath), path.Ext(containerPath)))
	if base == "" {
		base = "sqlite"
	}
	free := func(name string) bool {
		rec, _ := h.store.GetDatabaseByName(name)
		if rec == nil {
			return true
		}
		if rec.Type == "sqlite" && rec.ContainerID == containerID && rec.DBPath == containerPath {
			return true
		}
		return rec.Type == "sqlite" && !live[rec.ContainerID] && rec.DBPath == containerPath
	}
	candidates := []string{base, sanitizeDatabaseName(domain) + "_" + base}
	for _, n := range candidates {
		if free(n) {
			return n
		}
	}
	for i := 2; ; i++ {
		n := candidates[1] + "_" + strconv.Itoa(i)
		if free(n) {
			return n
		}
	}
}

// DatabaseScan is POST /api/databases/scan: look for SQLite databases in the sites now, and say what was added.
func (h *Handlers) DatabaseScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !isAdminRole(r) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	ctx, cancel := dockerContext(r.Context())
	defer cancel()
	res, err := h.discoverSQLiteDatabases(ctx, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// sqliteHostSize is the size of a SQLite database's file (with its write-ahead log), or -1 when it can't be found.
func (h *Handlers) sqliteHostSize(ctx context.Context, rec *store.DatabaseRecord) int64 {
	if rec.ContainerID == "" || rec.DBPath == "" {
		return -1
	}
	info, err := h.docker.InspectContainer(ctx, rec.ContainerID)
	if err != nil {
		return -1
	}
	host, err := hostPathForContainerFile(mountPairs(info.Mounts), rec.DBPath)
	if err != nil {
		return -1
	}
	st, err := os.Stat(host)
	if err != nil {
		return -1
	}
	size := st.Size()
	if w, err := os.Stat(host + "-wal"); err == nil {
		size += w.Size()
	}
	return size
}
