package api

import "testing"

func TestDemoBlocked(t *testing.T) {
	for _, p := range []string{"/api/network/overview", "/api/server/logs", "/api/server/services/action", "/api/system/update/run", "/api/debug"} {
		if !demoBlocked(p) {
			t.Errorf("%s should be blocked in demo mode", p)
		}
	}
	for _, p := range []string{"/api/containers", "/api/server/stats", "/api/databases", "/api/backups", "/api/system/update/status"} {
		if demoBlocked(p) {
			t.Errorf("%s should stay available in demo mode", p)
		}
	}
}
