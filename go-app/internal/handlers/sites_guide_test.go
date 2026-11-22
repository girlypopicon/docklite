package handlers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureSitesGuideWritesAndRespectsOwnFiles(t *testing.T) {
	old := siteBaseDir
	siteBaseDir = t.TempDir()
	defer func() { siteBaseDir = old }()

	(&Handlers{}).EnsureSitesGuide()
	b, err := os.ReadFile(filepath.Join(siteBaseDir, "CLAUDE.md"))
	if err != nil || !strings.Contains(string(b), "docklite databases create") {
		t.Fatalf("guide not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(siteBaseDir, "AGENTS.md")); err != nil {
		t.Fatal("AGENTS.md missing")
	}

	// A file someone wrote themselves is never replaced.
	own := filepath.Join(siteBaseDir, "CLAUDE.md")
	os.WriteFile(own, []byte("my own notes"), 0o644)
	(&Handlers{}).EnsureSitesGuide()
	if b, _ := os.ReadFile(own); string(b) != "my own notes" {
		t.Fatal("overwrote a file that was not DockLite's")
	}

	// An older DockLite guide is refreshed.
	os.WriteFile(own, []byte(sitesGuideMarker+"\nold text"), 0o644)
	(&Handlers{}).EnsureSitesGuide()
	if b, _ := os.ReadFile(own); string(b) != sitesGuide {
		t.Fatal("old guide not refreshed")
	}
}
