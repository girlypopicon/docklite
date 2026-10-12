package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// RestoreResult says what a restore did, in terms the dashboard and CLI can show.
type RestoreResult struct {
	Files        int      `json:"files"`
	Bytes        int64    `json:"bytes"`
	SafetyBackup string   `json:"safety_backup,omitempty"` // archive of what was there before, if anything was
	Skipped      []string `json:"skipped,omitempty"`       // entries that were left out, with the reason
}

// ArchiveInfo is what InspectSiteArchive learns by reading a backup without changing anything.
type ArchiveInfo struct {
	Folder  string   `json:"folder"` // the top-level folder the files were backed up under (the old folder name)
	Files   int      `json:"files"`
	Bytes   int64    `json:"bytes"` // uncompressed size
	Skipped []string `json:"skipped,omitempty"`
}

// cleanEntry turns a tar entry name into a path relative to the site folder. It returns ok=false for names that
// would land outside it (absolute paths, "..") and for the top-level folder entry itself (rel == "").
func cleanEntry(name string) (top, rel string, ok bool) {
	name = strings.TrimPrefix(filepath.ToSlash(name), "./")
	if name == "" || strings.HasPrefix(name, "/") {
		return "", "", false
	}
	clean := path.Clean(name)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", "", false
	}
	parts := strings.SplitN(clean, "/", 2)
	if len(parts) == 1 {
		return parts[0], "", true
	}
	return parts[0], parts[1], true
}

// InspectSiteArchive reads a site backup end to end and reports what's in it. It rejects an archive that is damaged, empty,
// or holds more than one top-level folder, so a restore never starts from something it can't finish.
func InspectSiteArchive(archivePath string) (*ArchiveInfo, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("can't open the backup: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("the backup file isn't a valid archive: %w", err)
	}
	tr := tar.NewReader(gz)
	info := &ArchiveInfo{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("the backup file is damaged: %w", err)
		}
		top, rel, ok := cleanEntry(hdr.Name)
		if !ok {
			return nil, fmt.Errorf("the backup holds an unsafe path (%q), so it won't be restored", hdr.Name)
		}
		if info.Folder == "" {
			info.Folder = top
		} else if top != info.Folder {
			return nil, fmt.Errorf("the backup isn't a single site folder (found %q and %q)", info.Folder, top)
		}
		if _, err := io.Copy(io.Discard, tr); err != nil {
			return nil, fmt.Errorf("the backup file is damaged: %w", err)
		}
		switch hdr.Typeflag {
		case tar.TypeReg:
			info.Files++
			info.Bytes += hdr.Size
		case tar.TypeDir:
		case tar.TypeSymlink:
			if !symlinkStaysInside(rel, hdr.Linkname) {
				info.Skipped = append(info.Skipped, fmt.Sprintf("%s (link leaves the site folder)", rel))
			}
		default:
			info.Skipped = append(info.Skipped, fmt.Sprintf("%s (unsupported file type)", rel))
		}
	}
	if info.Folder == "" {
		return nil, fmt.Errorf("the backup is empty")
	}
	return info, nil
}

// symlinkStaysInside reports whether a link at rel (inside the site folder) points somewhere inside the site folder.
func symlinkStaysInside(rel, target string) bool {
	if rel == "" || path.IsAbs(target) {
		return false
	}
	resolved := path.Clean(path.Join(path.Dir(rel), target))
	return resolved != ".." && !strings.HasPrefix(resolved, "../")
}

// RestoreSiteArchive replaces the contents of siteDir with those of a site backup.
//
// Safety, in order: the archive is read through first; what is in siteDir now is backed up to safetyDir; the backup is
// unpacked into a staging folder next to siteDir; and only then are the contents swapped, in place, because a running
// container has siteDir itself mounted and the folder must keep its identity. If the swap fails part way the old
// contents are put back. Nothing is deleted until the new contents are in place.
func RestoreSiteArchive(ctx context.Context, archivePath, siteDir, safetyDir string) (*RestoreResult, error) {
	rep := reporterFrom(ctx)
	rep.Phase(kindVerifying, "Checking the backup", 0)
	info, err := InspectSiteArchive(archivePath)
	if err != nil {
		return nil, err
	}
	if st, err := os.Stat(siteDir); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("the site folder %s doesn't exist; create the site first", siteDir)
	}

	parent := filepath.Dir(siteDir)
	if err := enoughSpace(parent, info.Bytes); err != nil {
		return nil, err
	}

	res := &RestoreResult{Files: info.Files, Bytes: info.Bytes, Skipped: info.Skipped}

	// 1. Keep what is there now (unless it is empty).
	if hasContents(siteDir) {
		if err := os.MkdirAll(safetyDir, 0o755); err != nil {
			return nil, err
		}
		safety := filepath.Join(safetyDir, fmt.Sprintf("before-restore-%s-%s.tar.gz", sanitizeFilename(filepath.Base(siteDir)), time.Now().UTC().Format("20060102-150405")))
		rep.Phase(kindWorking, "Saving what is there now", 0)
		if _, err := writeSiteArchive(ctx, siteDir, safety, rep); err != nil {
			_ = os.Remove(safety)
			return nil, fmt.Errorf("couldn't make the safety copy, so nothing was changed: %w", err)
		}
		res.SafetyBackup = safety
	}

	// 2. Unpack into a staging folder on the same filesystem as the site.
	stamp := time.Now().UTC().Format("20060102-150405")
	stage := filepath.Join(parent, ".restore-"+filepath.Base(siteDir)+"-"+stamp)
	if err := os.Mkdir(stage, 0o770); err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	rep.Phase(kindWorking, "Unpacking the backup", info.Bytes)
	if err := extractInto(ctx, archivePath, stage, rep); err != nil {
		return nil, fmt.Errorf("couldn't unpack the backup, so nothing was changed: %w", err)
	}

	// 3. Swap: old contents aside, new contents in; undo if it fails.
	rep.Phase(kindWorking, "Putting the files in place", 0)
	aside := filepath.Join(stage, ".old")
	if err := os.Mkdir(aside, 0o770); err != nil {
		return nil, err
	}
	if err := swapContents(siteDir, stage, aside); err != nil {
		return nil, err
	}
	return res, nil
}

// swapContents moves every entry of dir into aside, then every entry of stage (except aside) into dir. On failure it
// moves everything back. Renames only, so it is quick and never copies.
func swapContents(dir, stage, aside string) error {
	old, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var moved []string
	undo := func() {
		// remove whatever new entries made it in, then bring the old ones back
		if cur, err := os.ReadDir(dir); err == nil {
			for _, e := range cur {
				_ = os.RemoveAll(filepath.Join(dir, e.Name()))
			}
		}
		for _, n := range moved {
			_ = os.Rename(filepath.Join(aside, n), filepath.Join(dir, n))
		}
	}
	for _, e := range old {
		if err := os.Rename(filepath.Join(dir, e.Name()), filepath.Join(aside, e.Name())); err != nil {
			undo()
			return fmt.Errorf("couldn't move the current files aside, so nothing was changed: %w", err)
		}
		moved = append(moved, e.Name())
	}
	fresh, err := os.ReadDir(stage)
	if err != nil {
		undo()
		return err
	}
	for _, e := range fresh {
		if filepath.Join(stage, e.Name()) == aside {
			continue
		}
		if err := os.Rename(filepath.Join(stage, e.Name()), filepath.Join(dir, e.Name())); err != nil {
			undo()
			return fmt.Errorf("couldn't put the restored files in place; the old files were put back: %w", err)
		}
	}
	return nil
}

// extractInto unpacks a site backup into dest, dropping the archive's top-level folder name. Paths that would escape
// dest are never written. Files and folders end up readable and writable by the docklite group, like everything else
// DockLite creates in a site folder.
func extractInto(ctx context.Context, archivePath, dest string, rep *Reporter) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	type dirTime struct {
		path string
		mod  time.Time
	}
	var dirs []dirTime
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		_, rel, ok := cleanEntry(hdr.Name)
		if !ok {
			return fmt.Errorf("unsafe path %q", hdr.Name)
		}
		if rel == "" {
			continue
		}
		target := filepath.Join(dest, filepath.FromSlash(rel))
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o770); err != nil {
				return err
			}
			_ = os.Chmod(target, (os.FileMode(hdr.Mode).Perm()|0o770)|os.ModeSetgid)
			dirs = append(dirs, dirTime{target, hdr.ModTime})
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o770); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o660)
			if err != nil {
				return err
			}
			n, err := io.Copy(out, tr)
			rep.Add(n)
			if cerr := out.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				return err
			}
			_ = os.Chmod(target, os.FileMode(hdr.Mode).Perm()|0o660)
			_ = os.Chtimes(target, hdr.ModTime, hdr.ModTime)
		case tar.TypeSymlink:
			if !symlinkStaysInside(rel, hdr.Linkname) {
				continue // reported by InspectSiteArchive
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o770); err != nil {
				return err
			}
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		}
	}
	for i := len(dirs) - 1; i >= 0; i-- { // deepest first, so a parent's time isn't reset by its children
		_ = os.Chtimes(dirs[i].path, dirs[i].mod, dirs[i].mod)
	}
	return nil
}

func hasContents(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) > 0
}

// enoughSpace refuses to start when the disk can't hold the unpacked files plus some headroom.
func enoughSpace(dir string, need int64) error {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return nil // can't tell; don't block
	}
	free := int64(st.Bavail) * int64(st.Bsize)
	want := need + need/10 + 64<<20
	if free < want {
		return fmt.Errorf("not enough free disk space to restore: needs about %d MB, %d MB free", want>>20, free>>20)
	}
	return nil
}

// ReadManifest loads the manifest that sits beside a backup file. It returns nil (and no error) when there is none, as
// for backups made before manifests existed or files copied in by hand.
func ReadManifest(artifactPath string) (*Manifest, error) {
	data, err := os.ReadFile(ManifestPathForArtifact(artifactPath))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("the backup's manifest is damaged: %w", err)
	}
	return &m, nil
}

// CheckAgainstManifest compares a backup file with the size and SHA-256 its manifest recorded when it was made, so a file
// that was altered or corrupted on disk is caught before anything is restored from it.
func CheckAgainstManifest(artifactPath string) error {
	m, err := ReadManifest(artifactPath)
	if err != nil {
		return err
	}
	if m == nil {
		return nil
	}
	for _, f := range m.Files {
		if f.Name != filepath.Base(artifactPath) {
			continue
		}
		size, sum, err := hashFile(artifactPath)
		if err != nil {
			return err
		}
		if size != f.Bytes || sum != f.Sha256 {
			return fmt.Errorf("this backup file doesn't match what was recorded when it was made, so it was changed or damaged and won't be used")
		}
	}
	return nil
}
