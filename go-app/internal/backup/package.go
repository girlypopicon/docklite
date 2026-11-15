package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"docklite-agent/internal/docker"
	"docklite-agent/internal/store"
)

// A .litpkg is one gzipped tar holding any mix of sites and databases:
//
//	sites/<domain>/files/...     a site's folder, as it is on disk (its .dkl included)
//	databases/<name>.<ext>       a database dump, as the engine's own tool writes it (.pgdump, .sql, .archive, .rdb, .sqlite)
//	manifest.json                what is in the package; written last
//
// It unpacks with plain `tar -xzf`, so nothing about it needs DockLite to read.
const (
	PackageExt     = ".litpkg" // what DockLite writes; see packageExts for what it reads
	PackageSubDir  = "packages"
	packageFormat  = "docklite-package"
	packageVersion = 1
	manifestEntry  = "manifest.json"
)

type PackageManifest struct {
	Format      string            `json:"format"`
	Version     int               `json:"version"`
	ID          string            `json:"id"`
	CreatedAt   string            `json:"created_at"`
	Description string            `json:"description"`
	Selection   Selection         `json:"selection"`
	Sites       []PackageSite     `json:"sites"`
	Databases   []PackageDatabase `json:"databases"`
	Problems    []string          `json:"problems,omitempty"` // things that were selected but left out, and why
	Notes       string            `json:"notes,omitempty"`
}

type PackageSite struct {
	ID       int64  `json:"id"`
	Domain   string `json:"domain"`
	Template string `json:"template,omitempty"`
	Folder   string `json:"folder"` // where the files are inside the package
	Files    int    `json:"files"`  // entries (files, folders, links)
}

type PackageDatabase struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Engine string `json:"engine"`
	Site   string `json:"site,omitempty"` // the domain of the site it belongs to
	File   string `json:"file"`           // where the dump is inside the package
	Bytes  int64  `json:"bytes"`
	Sha256 string `json:"sha256"`
}

// PackageResult is a finished package and what didn't make it in.
type PackageResult struct {
	ArtifactResult
	Description string
	Problems    []string
	Sites       int
	Databases   int
}

// CreatePackage backs up everything a selection names into one .litpkg in baseDir/subDir.
//
// One broken site or database doesn't stop the rest: it is left out and listed in Problems (and in the package's
// manifest). Only when nothing at all could be backed up does it fail. Like every other backup, the finished file is
// read back from start to end before it gets its real name.
func CreatePackage(ctx context.Context, st *store.SQLiteStore, dockerClient *docker.Client, baseDir, subDir string, sel Selection, notes string) (*PackageResult, error) {
	rep := reporterFrom(ctx)
	rep.Phase(kindCounting, "Working out what to back up", 0)
	if sel.Empty() {
		return nil, fmt.Errorf("nothing was selected to back up")
	}
	resolved, err := sel.Resolve(st)
	if err != nil {
		return nil, err
	}
	if len(resolved.Sites) == 0 && len(resolved.Databases) == 0 {
		if len(resolved.Missing) > 0 {
			return nil, fmt.Errorf("nothing left to back up: %s", strings.Join(resolved.Missing, ", "))
		}
		return nil, fmt.Errorf("there are no sites or databases to back up yet")
	}

	destDir := filepath.Join(baseDir, subDir)
	tmpDir := filepath.Join(baseDir, "_tmp")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, err
	}

	m := PackageManifest{
		Format:      packageFormat,
		Version:     packageVersion,
		ID:          generateID(),
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		Description: sel.Describe(resolved),
		Selection:   sel,
		Sites:       []PackageSite{},
		Databases:   []PackageDatabase{},
		Problems:    append([]string{}, resolved.Missing...),
		Notes:       notes,
	}
	problem := func(msg string) {
		m.Problems = append(m.Problems, msg)
		rep.Warn(msg)
	}

	// 1. Dump the databases to temporary files first and check each one, so a dump that fails or comes out empty is
	// left out instead of ending up in the package looking fine.
	type dumped struct {
		info PackageDatabase
		path string
	}
	var dumps []dumped
	defer func() {
		for _, d := range dumps {
			_ = os.Remove(d.path)
		}
	}()
	usedNames := map[string]bool{}
	for i, db := range resolved.Databases {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rep.Phase(kindWorking, fmt.Sprintf("Exporting database %s (%d of %d)", db.Name, i+1, len(resolved.Databases)), 0)
		engine := db.Type
		if engine == "" {
			engine = "postgres"
		}
		name := sanitizeFilename(db.Name)
		if usedNames[name] {
			name = fmt.Sprintf("%s-%d", name, db.ID)
		}
		usedNames[name] = true
		tmp := filepath.Join(tmpDir, m.ID+"-"+name+dumpExt(engine)+".partial")
		size, sum, err := dumpDatabaseTo(ctx, dockerClient, db.DatabaseRecord, engine, tmp, rep)
		if err != nil {
			_ = os.Remove(tmp)
			problem(fmt.Sprintf("Database %s was left out: %v", db.Name, err))
			continue
		}
		dumps = append(dumps, dumped{path: tmp, info: PackageDatabase{
			ID: db.ID, Name: db.Name, Engine: engine, Site: db.SiteDomain,
			File: "databases/" + name + dumpExt(engine), Bytes: size, Sha256: sum,
		}})
	}

	// 2. Pack the site folders and the dumps into one archive.
	var total int64
	for _, site := range resolved.Sites {
		total += treeSize(site.CodePath)
	}
	for _, d := range dumps {
		total += d.info.Bytes
	}
	filename := uniqueName(destDir, fmt.Sprintf("%s-%s", packageSlug(sel, resolved), time.Now().UTC().Format("20060102-150405")), PackageExt)
	tempPath := filepath.Join(tmpDir, m.ID+"-"+filename+".partial") // the id keeps two runs started together apart
	finalPath := filepath.Join(destDir, filename)

	entries, err := writePackage(ctx, tempPath, resolved.Sites, total, rep, &m, problem, func(tw *tar.Writer) (int, error) {
		n := 0
		for _, d := range dumps {
			if err := addFile(tw, d.path, d.info.File, rep); err != nil {
				return n, err
			}
			m.Databases = append(m.Databases, d.info)
			n++
		}
		return n, nil
	})
	if err != nil {
		_ = os.Remove(tempPath)
		return nil, err
	}
	if len(m.Sites) == 0 && len(m.Databases) == 0 {
		_ = os.Remove(tempPath)
		return nil, fmt.Errorf("nothing could be backed up: %s", strings.Join(m.Problems, "; "))
	}

	// 3. Read it back, then give it its real name.
	sha, size, err := verifySiteArchive(tempPath, entries, rep)
	if err != nil {
		_ = os.Remove(tempPath)
		return nil, err
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		_ = os.Remove(tempPath)
		return nil, err
	}
	side := Manifest{
		ID:        m.ID,
		Type:      "package",
		CreatedAt: m.CreatedAt,
		Files:     []ManifestFile{{Name: filepath.Base(finalPath), Bytes: size, Sha256: sha}},
		Notes:     notes,
	}
	if len(m.Sites) == 1 {
		side.Source.SiteID, side.Source.Domain = &m.Sites[0].ID, m.Sites[0].Domain
	}
	manifestPath, err := writeManifest(tmpDir, destDir, filename, side)
	if err != nil {
		return nil, err
	}
	relative, _ := filepath.Rel(baseDir, finalPath)
	return &PackageResult{
		ArtifactResult: ArtifactResult{
			Verified: true, Path: finalPath, Size: size, Sha256: sha,
			ManifestPath: manifestPath, RelativePath: relative,
		},
		Description: m.Description,
		Problems:    m.Problems,
		Sites:       len(m.Sites),
		Databases:   len(m.Databases),
	}, nil
}

// writePackage writes the archive: every site folder, then whatever addDumps adds, then the manifest. It returns the
// number of tar entries, for the read-back check.
func writePackage(ctx context.Context, dest string, sites []store.SiteRecord, total int64, rep *Reporter, m *PackageManifest, problem func(string), addDumps func(*tar.Writer) (int, error)) (int, error) {
	out, err := os.Create(dest)
	if err != nil {
		return 0, err
	}
	defer out.Close()
	gz := gzip.NewWriter(out)
	tw := tar.NewWriter(gz)

	rep.Phase(kindWorking, "Packing", total)
	entries := 0
	for _, site := range sites {
		if site.CodePath == "" {
			problem(fmt.Sprintf("Site %s was left out: it has no folder", site.Domain))
			continue
		}
		if err := checkFolder(site.CodePath); err != nil {
			problem(fmt.Sprintf("Site %s was left out: %v", site.Domain, err))
			continue
		}
		folder := "sites/" + sanitizeFilename(site.Domain) + "/files"
		n, err := addTree(ctx, tw, site.CodePath, folder, rep)
		entries += n
		if err != nil {
			return 0, err
		}
		m.Sites = append(m.Sites, PackageSite{ID: site.ID, Domain: site.Domain, Template: site.TemplateType, Folder: folder, Files: n})
	}
	n, err := addDumps(tw)
	entries += n
	if err != nil {
		return 0, err
	}
	payload, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return 0, err
	}
	if err := tw.WriteHeader(&tar.Header{Name: manifestEntry, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(payload)), ModTime: time.Now()}); err != nil {
		return 0, err
	}
	if _, err := tw.Write(payload); err != nil {
		return 0, err
	}
	entries++
	rep.SetFiles(entries)

	if err := tw.Close(); err != nil {
		return 0, err
	}
	if err := gz.Close(); err != nil {
		return 0, err
	}
	if err := out.Sync(); err != nil {
		return 0, err
	}
	return entries, out.Close()
}

// addFile copies one file on disk into the archive under name.
func addFile(tw *tar.Writer, path, name string, rep *Reporter) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	if err := tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o600, Size: fi.Size(), ModTime: fi.ModTime()}); err != nil {
		return err
	}
	_, err = io.CopyN(tw, progressReader{r: f, rep: rep}, fi.Size())
	return err
}

// dumpDatabaseTo writes a complete dump of one database to path, uncompressed (the package is compressed as a whole),
// then reads it back to check it is a whole dump of its engine. It returns the dump's size and SHA-256.
func dumpDatabaseTo(ctx context.Context, dockerClient *docker.Client, db store.DatabaseRecord, engine, path string, rep *Reporter) (int64, string, error) {
	if db.ContainerID == "" {
		return 0, "", fmt.Errorf("it has no container")
	}
	if dockerClient == nil {
		return 0, "", fmt.Errorf("Docker isn't available")
	}
	var cmd, env []string
	if engine == "sqlite" {
		if db.DBPath == "" {
			return 0, "", fmt.Errorf("its file location isn't known")
		}
		cmd = []string{"cat", db.DBPath}
	} else {
		user, pass, err := resolveDatabaseCredentials(ctx, dockerClient, db.ContainerID)
		if err != nil {
			return 0, "", err
		}
		p := ""
		if pass != nil {
			p = *pass
		}
		if cmd, env, err = DumpCommand(engine, db.Name, user, p); err != nil {
			return 0, "", err
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, "", err
	}
	if err := dockerClient.ExecCommandToWriter(ctx, db.ContainerID, cmd, env, progressWriter{w: f, rep: rep}); err != nil {
		f.Close()
		return 0, "", err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return 0, "", err
	}
	if err := f.Close(); err != nil {
		return 0, "", err
	}
	return checkDumpFile(path, engine)
}

// checkDumpFile reads a dump back and checks it looks like a whole dump of its engine.
func checkDumpFile(path, engine string) (int64, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()
	hasher := sha256.New()
	counter := &countWriter{}
	r := io.TeeReader(f, io.MultiWriter(hasher, counter))
	if engine == "sqlite" {
		head := make([]byte, 16)
		n, _ := io.ReadFull(r, head)
		if !bytes.Equal(head[:n], []byte("SQLite format 3\x00")) {
			return 0, "", fmt.Errorf("the file is empty or isn't an SQLite database")
		}
		if _, err := io.Copy(io.Discard, r); err != nil {
			return 0, "", err
		}
	} else if err := checkDump(r, engine); err != nil {
		return 0, "", err
	}
	return counter.n, hex.EncodeToString(hasher.Sum(nil)), nil
}

type countWriter struct{ n int64 }

func (c *countWriter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}

// uniqueName is base+ext, or base-2+ext and so on when two backups of the same thing land in the same second, so one
// never replaces the other.
func uniqueName(dir, base, ext string) string {
	name := base + ext
	for i := 2; ; i++ {
		if _, err := os.Lstat(filepath.Join(dir, name)); os.IsNotExist(err) {
			return name
		}
		name = fmt.Sprintf("%s-%d%s", base, i, ext)
	}
}

// packageSlug starts the file name, so the backups folder reads sensibly: example.com-..., db-shop-..., everything-...
func packageSlug(sel Selection, r *Resolved) string {
	switch {
	case sel.AllSites && sel.AllDatabases:
		return "everything"
	case sel.AllSites:
		return "all-sites"
	case sel.AllDatabases && len(r.Sites) == 0:
		return "all-databases"
	case len(r.Sites) == 1:
		return sanitizeFilename(r.Sites[0].Domain)
	case len(r.Sites) == 0 && len(r.Databases) == 1:
		return "db-" + sanitizeFilename(r.Databases[0].Name)
	}
	return "docklite"
}

// PackageInfo is what InspectPackage learns by reading a package through without changing anything.
type PackageInfo struct {
	Manifest PackageManifest `json:"manifest"`
	Bytes    int64           `json:"bytes"` // uncompressed size
	Entries  int             `json:"entries"`
}

// packageExts are the file endings IsPackage accepts: today's, the long spelling, and .dklpkg from before 1.5.
var packageExts = []string{PackageExt, ".litepkg", ".dklpkg"}

// IsPackage reports whether a backup file is a package (.litpkg, or an older .litepkg / .dklpkg).
func IsPackage(path string) bool {
	for _, ext := range packageExts {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

// InspectPackage reads a whole package, so a damaged one is caught, and returns its manifest.
func InspectPackage(path string) (*PackageInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("can't open the backup: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("the backup file isn't a valid archive: %w", err)
	}
	tr := tar.NewReader(gz)
	info := &PackageInfo{}
	found := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("the backup file is damaged: %w", err)
		}
		info.Entries++
		if hdr.Name == manifestEntry {
			data, err := io.ReadAll(io.LimitReader(tr, 16<<20))
			if err != nil {
				return nil, fmt.Errorf("the backup file is damaged: %w", err)
			}
			if err := json.Unmarshal(data, &info.Manifest); err != nil {
				return nil, fmt.Errorf("the package's manifest is damaged: %w", err)
			}
			found = true
		}
		if _, err := io.Copy(io.Discard, tr); err != nil {
			return nil, fmt.Errorf("the backup file is damaged: %w", err)
		}
		if hdr.Typeflag == tar.TypeReg {
			info.Bytes += hdr.Size
		}
	}
	if !found || info.Manifest.Format != packageFormat {
		return nil, fmt.Errorf("this isn't a DockLite package (it has no manifest)")
	}
	return info, nil
}

// SiteFolder is where a site's files are inside a package, or "" if the package doesn't hold that site.
func (m *PackageManifest) SiteFolder(domain string) string {
	for _, s := range m.Sites {
		if strings.EqualFold(s.Domain, domain) {
			return s.Folder
		}
	}
	return ""
}
