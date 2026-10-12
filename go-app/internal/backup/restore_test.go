package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		return "<missing>"
	}
	return string(b)
}

// buildTar writes a gzip'd tar with the given entries (name -> content; names ending in / are folders; "link:x" content makes a symlink to x).
func buildTar(t *testing.T, path string, entries [][2]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		switch {
		case strings.HasSuffix(e[0], "/"):
			_ = tw.WriteHeader(&tar.Header{Name: e[0], Typeflag: tar.TypeDir, Mode: 0o755})
		case strings.HasPrefix(e[1], "link:"):
			_ = tw.WriteHeader(&tar.Header{Name: e[0], Typeflag: tar.TypeSymlink, Linkname: strings.TrimPrefix(e[1], "link:"), Mode: 0o777})
		default:
			_ = tw.WriteHeader(&tar.Header{Name: e[0], Typeflag: tar.TypeReg, Size: int64(len(e[1])), Mode: 0o644})
			_, _ = tw.Write([]byte(e[1]))
		}
	}
	_ = tw.Close()
	_ = gz.Close()
}

func TestRestoreRoundTrip(t *testing.T) {
	root := t.TempDir()
	site := filepath.Join(root, "sites", "u", "example.com")
	writeFile(t, filepath.Join(site, "index.html"), "original")
	writeFile(t, filepath.Join(site, "css", "a.css"), "body{}")
	archive := filepath.Join(root, "backup.tar.gz")
	if _, err := writeSiteArchive(context.Background(), site, archive, nil); err != nil {
		t.Fatal(err)
	}

	// the site changes after the backup: a file edited, one added, one removed
	writeFile(t, filepath.Join(site, "index.html"), "broken edit")
	writeFile(t, filepath.Join(site, "new.txt"), "added later")
	_ = os.Remove(filepath.Join(site, "css", "a.css"))
	inodeBefore := mustInode(t, site)

	res, err := RestoreSiteArchive(context.Background(), archive, site, filepath.Join(root, "safety"))
	if err != nil {
		t.Fatal(err)
	}
	if readFile(t, filepath.Join(site, "index.html")) != "original" || readFile(t, filepath.Join(site, "css", "a.css")) != "body{}" {
		t.Fatal("the files weren't restored")
	}
	if readFile(t, filepath.Join(site, "new.txt")) != "<missing>" {
		t.Fatal("a file added after the backup should be gone after restoring to it")
	}
	if mustInode(t, site) != inodeBefore {
		t.Fatal("the site folder itself must keep its identity (a container has it mounted)")
	}
	if res.SafetyBackup == "" || !strings.Contains(res.SafetyBackup, "before-restore-") {
		t.Fatalf("expected a safety backup, got %+v", res)
	}
	// the safety copy holds the state from before the restore, so a restore can be undone
	safeInfo, err := InspectSiteArchive(res.SafetyBackup)
	if err != nil || safeInfo.Files != 2 { // index.html (broken edit) + new.txt
		t.Fatalf("safety backup should hold the pre-restore files: %+v %v", safeInfo, err)
	}
	// no staging folders left behind
	left, _ := filepath.Glob(filepath.Join(root, "sites", "u", ".restore-*"))
	if len(left) != 0 {
		t.Fatalf("staging left behind: %v", left)
	}
	// group can write what was restored
	if st, _ := os.Stat(filepath.Join(site, "index.html")); st.Mode().Perm()&0o060 != 0o060 {
		t.Fatalf("restored files should be group read/write, got %v", st.Mode())
	}
}

func mustInode(t *testing.T, p string) uint64 {
	t.Helper()
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return statIno(st)
}

func TestRestoreRejectsUnsafeAndDamagedArchives(t *testing.T) {
	root := t.TempDir()
	site := filepath.Join(root, "site")
	writeFile(t, filepath.Join(site, "keep.txt"), "keep me")
	cases := map[string][][2]string{
		"parent traversal": {{"site/", ""}, {"site/../../evil.txt", "x"}},
		"absolute path":    {{"/etc/passwd", "x"}},
		"two top folders":  {{"a/x.txt", "1"}, {"b/y.txt", "2"}},
		"empty":            {},
	}
	for name, entries := range cases {
		p := filepath.Join(root, strings.ReplaceAll(name, " ", "-")+".tar.gz")
		buildTar(t, p, entries)
		if _, err := RestoreSiteArchive(context.Background(), p, site, filepath.Join(root, "safety")); err == nil {
			t.Fatalf("%s: should have been refused", name)
		}
		if readFile(t, filepath.Join(site, "keep.txt")) != "keep me" {
			t.Fatalf("%s: a refused restore must not touch the site", name)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "evil.txt")); err == nil {
		t.Fatal("traversal wrote outside the site")
	}
	// not an archive at all / truncated
	junk := filepath.Join(root, "junk.tar.gz")
	_ = os.WriteFile(junk, []byte("not a backup"), 0o644)
	if _, err := RestoreSiteArchive(context.Background(), junk, site, filepath.Join(root, "safety")); err == nil {
		t.Fatal("junk should be refused")
	}
	good := filepath.Join(root, "good.tar.gz")
	buildTar(t, good, [][2]string{{"site/", ""}, {"site/a.txt", strings.Repeat("data", 5000)}})
	b, _ := os.ReadFile(good)
	_ = os.WriteFile(filepath.Join(root, "cut.tar.gz"), b[:len(b)/2], 0o644)
	if _, err := RestoreSiteArchive(context.Background(), filepath.Join(root, "cut.tar.gz"), site, filepath.Join(root, "safety")); err == nil {
		t.Fatal("a truncated archive should be refused")
	}
	if readFile(t, filepath.Join(site, "keep.txt")) != "keep me" {
		t.Fatal("site was touched by a refused restore")
	}
}

func TestRestoreSkipsLinksThatLeaveTheSite(t *testing.T) {
	root := t.TempDir()
	site := filepath.Join(root, "site")
	writeFile(t, filepath.Join(site, "x"), "x")
	p := filepath.Join(root, "b.tar.gz")
	buildTar(t, p, [][2]string{{"site/", ""}, {"site/ok.txt", "ok"}, {"site/good-link", "link:ok.txt"}, {"site/evil-link", "link:/etc/passwd"}, {"site/up-link", "link:../../outside"}})
	res, err := RestoreSiteArchive(context.Background(), p, site, filepath.Join(root, "safety"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(site, "good-link")); err != nil {
		t.Fatal("a link inside the site should be restored")
	}
	for _, n := range []string{"evil-link", "up-link"} {
		if _, err := os.Lstat(filepath.Join(site, n)); err == nil {
			t.Fatalf("%s must not be restored", n)
		}
	}
	if len(res.Skipped) != 2 {
		t.Fatalf("both skipped links should be reported: %v", res.Skipped)
	}
}

func TestSwapContentsUndoesOnFailure(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "site")
	writeFile(t, filepath.Join(dir, "a.txt"), "A")
	writeFile(t, filepath.Join(dir, "sub", "b.txt"), "B")
	stage := filepath.Join(root, "stage") // does not exist: reading the new files fails after the old ones were moved aside
	aside := filepath.Join(root, "aside")
	_ = os.MkdirAll(aside, 0o755)
	if err := swapContents(dir, stage, aside); err == nil {
		t.Fatal("expected a failure")
	}
	if readFile(t, filepath.Join(dir, "a.txt")) != "A" || readFile(t, filepath.Join(dir, "sub", "b.txt")) != "B" {
		t.Fatal("the old files must be back after a failed swap")
	}
}

func TestCheckAgainstManifest(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "site-example.com-1.tar.gz")
	buildTar(t, p, [][2]string{{"example.com/", ""}, {"example.com/a.txt", "hello"}})
	if err := CheckAgainstManifest(p); err != nil {
		t.Fatalf("no manifest should be fine: %v", err)
	}
	size, sum, _ := hashFile(p)
	m := Manifest{Files: []ManifestFile{{Name: filepath.Base(p), Bytes: size, Sha256: sum}}}
	if _, err := writeManifest(root, root, filepath.Base(p), m); err != nil {
		t.Fatal(err)
	}
	if err := CheckAgainstManifest(p); err != nil {
		t.Fatalf("matching manifest: %v", err)
	}
	b, _ := os.ReadFile(p)
	b[len(b)-12] ^= 0xff
	_ = os.WriteFile(p, b, 0o644)
	if err := CheckAgainstManifest(p); err == nil {
		t.Fatal("a changed file must be caught")
	}
}
