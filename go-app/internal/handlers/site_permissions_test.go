package handlers

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"docklite-agent/internal/testhelpers"
)

func TestNewSiteFoldersAreGroupWritableDespiteUmask(t *testing.T) {
	old := syscall.Umask(0o022) // the usual server default, which silently removes group write
	defer syscall.Umask(old)
	base := t.TempDir()
	prev := siteBaseDir
	siteBaseDir = base
	defer func() { siteBaseDir = prev }()

	testhelpers.AssertNoError(t, ensureSiteDirectory("admin", "glowrom.com"))
	for _, p := range []string{filepath.Join(base, "admin"), filepath.Join(base, "admin", "glowrom.com")} {
		st, err := os.Stat(p)
		testhelpers.AssertNoError(t, err)
		if st.Mode().Perm()&0o070 != 0o070 {
			t.Errorf("%s: group needs read/write/enter, got %v", p, st.Mode())
		}
		if st.Mode()&os.ModeSetgid == 0 {
			t.Errorf("%s: setgid bit missing, new files wouldn't keep the docklite group", p)
		}
		if st.Mode().Perm()&0o002 != 0 {
			t.Errorf("%s must never be world-writable", p)
		}
	}
	// a default file made by DockLite is group-writable too
	f := filepath.Join(base, "admin", "glowrom.com", "index.html")
	testhelpers.AssertNoError(t, ensureFile(f, "<h1>hi</h1>"))
	st, _ := os.Stat(f)
	if st.Mode().Perm()&0o060 != 0o060 {
		t.Errorf("default file should be group read/write, got %v", st.Mode())
	}
}

func TestFixSitePermissionsOnlyAddsGroupAccess(t *testing.T) {
	root := t.TempDir()
	testhelpers.AssertNoError(t, os.MkdirAll(filepath.Join(root, "assets", "img"), 0o755))
	testhelpers.AssertNoError(t, os.WriteFile(filepath.Join(root, "index.html"), []byte("x"), 0o644))
	testhelpers.AssertNoError(t, os.WriteFile(filepath.Join(root, "run.sh"), []byte("#!/bin/sh"), 0o755))
	testhelpers.AssertNoError(t, os.WriteFile(filepath.Join(root, "secret.key"), []byte("k"), 0o600))
	outside := filepath.Join(t.TempDir(), "target")
	testhelpers.AssertNoError(t, os.WriteFile(outside, []byte("t"), 0o600))
	testhelpers.AssertNoError(t, os.Symlink(outside, filepath.Join(root, "link")))

	changed, _ := fixSitePermissions(root)
	if changed == 0 {
		t.Fatal("expected changes")
	}
	mode := func(p string) os.FileMode { st, _ := os.Stat(p); return st.Mode() }
	testhelpers.AssertEqual(t, mode(filepath.Join(root, "index.html")).Perm(), os.FileMode(0o664))
	testhelpers.AssertEqual(t, mode(filepath.Join(root, "run.sh")).Perm(), os.FileMode(0o775))     // stays executable
	testhelpers.AssertEqual(t, mode(filepath.Join(root, "secret.key")).Perm(), os.FileMode(0o660)) // never opens it to "other"
	d := mode(filepath.Join(root, "assets", "img"))
	if d.Perm() != 0o775 || d&os.ModeSetgid == 0 {
		t.Errorf("folders should be 2775, got %v", d)
	}
	testhelpers.AssertEqual(t, mode(outside).Perm(), os.FileMode(0o600)) // a link must never change its target

	again, _ := fixSitePermissions(root)
	testhelpers.AssertEqual(t, again, 0) // running it twice changes nothing
}

func TestMkdirAllSharedSharesEveryNewFolder(t *testing.T) {
	old := syscall.Umask(0o022)
	defer syscall.Umask(old)
	root := t.TempDir()
	testhelpers.AssertNoError(t, mkdirAllShared(filepath.Join(root, "a", "b", "c")))
	for _, p := range []string{"a", "a/b", "a/b/c"} {
		st, _ := os.Stat(filepath.Join(root, p))
		if st.Mode().Perm() != 0o775 {
			t.Errorf("%s should be 775, got %v", p, st.Mode().Perm())
		}
	}
}
