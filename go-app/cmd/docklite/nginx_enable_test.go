package main

import "testing"

func TestDisabledButRunning(t *testing.T) {
	sites := []nginxSiteRow{
		{Domain: "a.com", Enabled: true, HasConfig: true},
		{Domain: "b.com", Enabled: false, HasConfig: true},  // running, switched off
		{Domain: "c.com", Enabled: false, HasConfig: false}, // running, no file
		{Domain: "d.com", Enabled: false, HasConfig: true},  // switched off on purpose: its container is stopped
	}
	off, noFile := disabledButRunning(sites, map[string]bool{"a.com": true, "b.com": true, "c.com": true})
	if len(off) != 1 || off[0] != "b.com" || len(noFile) != 1 || noFile[0] != "c.com" {
		t.Fatalf("off=%v noFile=%v", off, noFile)
	}
}
