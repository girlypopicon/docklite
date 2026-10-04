package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newReporter returns a reporter wired to a fresh tracker and a way to read its state.
func newReporter(t *testing.T, id int64) (*Reporter, func() Progress) {
	t.Helper()
	tracker := &Tracker{runs: map[int64]*Progress{}}
	rep := tracker.Start(id, "site", 1)
	if rep == nil {
		t.Fatal("could not start a progress entry")
	}
	return rep, func() Progress {
		p, ok := tracker.Snapshot(id)
		if !ok {
			t.Fatal("no progress entry")
		}
		return p
	}
}

func makeSite(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "example.com")
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(root, "assets", "img"), 0o755))
	must(os.MkdirAll(filepath.Join(root, "empty-dir"), 0o755))
	must(os.WriteFile(filepath.Join(root, "index.html"), []byte("<h1>hello</h1>"), 0o644))
	must(os.WriteFile(filepath.Join(root, "assets", "app.js"), bytes.Repeat([]byte("x"), 100_000), 0o644))
	must(os.WriteFile(filepath.Join(root, "secret.env"), []byte("TOKEN=1"), 0o600))
	must(os.Symlink("index.html", filepath.Join(root, "home")))
	// A link pointing outside the folder must be stored as a link, never followed.
	must(os.Symlink("/etc/hostname", filepath.Join(root, "outside")))
	return root
}

func readArchive(t *testing.T, path string) map[string]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	entries := map[string]string{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return entries
		}
		if err != nil {
			t.Fatal(err)
		}
		switch hdr.Typeflag {
		case tar.TypeSymlink:
			entries[hdr.Name] = "-> " + hdr.Linkname
		case tar.TypeDir:
			entries[hdr.Name] = "dir"
		default:
			data, _ := io.ReadAll(tr)
			entries[hdr.Name] = string(data)
		}
	}
}

func TestSiteArchiveRoundTripVerifyAndProgress(t *testing.T) {
	site := makeSite(t)
	dest := filepath.Join(t.TempDir(), "out.tar.gz")
	rep, state := newReporter(t, 1)

	entries, err := writeSiteArchive(context.Background(), site, dest, rep)
	if err != nil {
		t.Fatal(err)
	}
	if entries != 9 { // 4 dirs? root, assets, img, empty-dir + 5 non-dirs
		// root, assets, assets/img, empty-dir (4) + index.html, app.js, secret.env, home, outside (5)
		t.Fatalf("expected 9 entries, got %d", entries)
	}
	if got := state(); got.Files != entries || got.BytesDone < 100_000 {
		t.Fatalf("progress should count files and bytes, got %+v", got)
	}

	sha, size, err := verifySiteArchive(dest, entries, rep)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(dest)
	sum := sha256.Sum256(raw)
	if sha != hex.EncodeToString(sum[:]) || size != int64(len(raw)) {
		t.Fatal("verification must hash the whole file")
	}

	got := readArchive(t, dest)
	if got["example.com/index.html"] != "<h1>hello</h1>" {
		t.Fatalf("content wrong: %v", got)
	}
	if got["example.com/home"] != "-> index.html" || got["example.com/outside"] != "-> /etc/hostname" {
		t.Fatalf("links must be stored, not followed: %v", got)
	}
	if _, ok := got["example.com/empty-dir/"]; !ok {
		t.Fatalf("empty folders must survive: %v", got)
	}
}

func TestSiteArchiveSkipsUnreadableFilesWithAWarning(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read everything")
	}
	site := makeSite(t)
	locked := filepath.Join(site, "locked.txt")
	if err := os.WriteFile(locked, []byte("nope"), 0o000); err != nil {
		t.Fatal(err)
	}
	rep, state := newReporter(t, 2)
	if _, err := writeSiteArchive(context.Background(), site, filepath.Join(t.TempDir(), "o.tar.gz"), rep); err != nil {
		t.Fatalf("an unreadable file must not fail the whole backup: %v", err)
	}
	got := state()
	if got.WarningCount != 1 || !strings.Contains(got.Warnings[0], "locked.txt") {
		t.Fatalf("expected one warning naming the file, got %+v", got.Warnings)
	}
}

func TestVerifyCatchesDamagedAndIncompleteArchives(t *testing.T) {
	site := makeSite(t)
	dir := t.TempDir()
	good := filepath.Join(dir, "good.tar.gz")
	rep, _ := newReporter(t, 3)
	entries, err := writeSiteArchive(context.Background(), site, good, rep)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(good)

	// Truncated: the end of the file is missing.
	truncated := filepath.Join(dir, "truncated.tar.gz")
	_ = os.WriteFile(truncated, raw[:len(raw)/2], 0o644)
	if _, _, err := verifySiteArchive(truncated, entries, rep); err == nil {
		t.Fatal("a truncated archive must fail verification")
	}

	// Bit rot in the middle.
	corrupt := append([]byte{}, raw...)
	corrupt[len(corrupt)/2] ^= 0xFF
	corruptPath := filepath.Join(dir, "corrupt.tar.gz")
	_ = os.WriteFile(corruptPath, corrupt, 0o644)
	if _, _, err := verifySiteArchive(corruptPath, entries, rep); err == nil {
		t.Fatal("a corrupted archive must fail verification")
	}

	// Right file, but not as many entries as were written.
	if _, _, err := verifySiteArchive(good, entries+3, rep); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("a wrong entry count must read as incomplete, got %v", err)
	}

	// Not an archive at all.
	notArchive := filepath.Join(dir, "x.tar.gz")
	_ = os.WriteFile(notArchive, []byte("hello"), 0o644)
	if _, _, err := verifySiteArchive(notArchive, 0, rep); err == nil {
		t.Fatal("plain text must fail verification")
	}
}

func gz(t *testing.T, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "db.dump.gz")
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, _ = w.Write(content)
	_ = w.Close()
	_ = os.WriteFile(path, buf.Bytes(), 0o644)
	return path
}

func TestVerifyDatabaseDumpNeedsAPostgresSignature(t *testing.T) {
	rep, _ := newReporter(t, 4)
	if _, _, err := verifyDatabaseDump(gz(t, append([]byte("PGDMP"), bytes.Repeat([]byte{1}, 1000)...)), rep); err != nil {
		t.Fatalf("a real-looking dump should pass: %v", err)
	}
	for name, content := range map[string][]byte{
		"empty":        {},
		"wrong format": []byte("-- plain sql dump\nCREATE TABLE x();"),
		"too short":    []byte("PG"),
	} {
		if _, _, err := verifyDatabaseDump(gz(t, content), rep); err == nil {
			t.Errorf("%s: must be rejected, an empty or wrong dump is not a backup", name)
		}
	}
}

func TestTrackerOneRunPerTargetAndPercentPhases(t *testing.T) {
	tracker := &Tracker{runs: map[int64]*Progress{}}
	rep := tracker.Start(10, "database", 5)
	if rep == nil {
		t.Fatal("first start must succeed")
	}
	if tracker.Start(11, "database", 5) != nil {
		t.Fatal("a second backup of the same target must be refused while one runs")
	}
	if tracker.Start(12, "database", 6) == nil {
		t.Fatal("a different target is independent")
	}
	if id, ok := tracker.RunningFor("database", 5); !ok || id != 10 {
		t.Fatalf("RunningFor: %d %v", id, ok)
	}

	snap := func() Progress { p, _ := tracker.Snapshot(10); return p }
	rep.Phase(kindWorking, "Compressing", 200)
	rep.Add(100)
	if got := snap().Percent; got != 47.5 { // 5 + 85*0.5
		t.Fatalf("halfway through working should be 47.5%%, got %v", got)
	}
	rep.Phase(kindWorking, "Exporting", 0)
	if snap().Percent != -1 {
		t.Fatal("an unknown total must report -1 (indeterminate)")
	}
	rep.Phase(kindVerifying, "Checking", 100)
	rep.Add(50)
	if got := snap().Percent; got != 95 {
		t.Fatalf("halfway through verifying should be 95%%, got %v", got)
	}

	rep.Succeed(&ArtifactResult{Path: "/b/site-x.tar.gz", Size: 9, Sha256: "abc"}, true)
	done := snap()
	if done.Status != StatusSuccess || done.Percent != 100 || done.FileName != "site-x.tar.gz" || !done.Verified {
		t.Fatalf("success state wrong: %+v", done)
	}
	if _, ok := tracker.RunningFor("database", 5); ok {
		t.Fatal("a finished backup must free its target")
	}
	if tracker.Start(13, "database", 5) == nil {
		t.Fatal("the target can be backed up again after it finishes")
	}
}

func TestNilReporterIsHarmless(t *testing.T) {
	var rep *Reporter
	rep.Phase(kindWorking, "x", 1)
	rep.Add(1)
	rep.Warn("w")
	rep.SetFiles(1)
	rep.Fail("f")
	rep.Succeed(nil, false)
}

func TestFriendlyError(t *testing.T) {
	for in, want := range map[string]string{
		"write /x: no space left on device":              "disk is full",
		"open /var/backups/x: permission denied":         "isn't allowed to write",
		"site folder not found: /var/www/sites/a/b":      "files weren't found",
		"Error response: container abc is not running":   "isn't running",
		"password authentication failed for user x":      "couldn't log in",
		"context deadline exceeded":                      "too long",
		"the dump is empty or isn't a PostgreSQL backup": "discarded",
		"something nobody has seen before":               "something nobody has seen before",
	} {
		if got := FriendlyError(errors.New(in)); !strings.Contains(got, want) {
			t.Errorf("%q -> %q (want it to mention %q)", in, got, want)
		}
	}
}
