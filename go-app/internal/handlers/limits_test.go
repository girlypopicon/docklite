package handlers

import (
	"testing"

	"docklite-agent/internal/docker"
)

func TestStoredLimitsRoundTrip(t *testing.T) {
	h := testHandlers(t)
	if h.StoredLimits("docklite-site-a-com") != nil {
		t.Fatal("nothing saved yet")
	}
	_ = h.store.SetSetting(limitsSettingPrefix+"docklite-site-a-com", `{"cpus":0.5,"memoryMB":256,"pids":100}`)
	l := h.StoredLimits("docklite-site-a-com")
	if l == nil || *l != (docker.Limits{CPUs: 0.5, MemoryMB: 256, Pids: 100}) {
		t.Fatalf("got %+v", l)
	}
	_ = h.store.SetSetting(limitsSettingPrefix+"docklite-site-a-com", "")
	if h.StoredLimits("docklite-site-a-com") != nil {
		t.Fatal("cleared limits must read as none")
	}
}
