package handlers

import (
	"strings"
	"testing"
)

func TestAgentPort(t *testing.T) {
	for in, want := range map[string]int{":3000": 3000, "127.0.0.1:9100": 9100, "": 3000, ":abc": 3000} {
		if got := agentPort(in); got != want {
			t.Errorf("agentPort(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestPanelVhostPointsAtTheAgent(t *testing.T) {
	cfg := panelVhostConfig("smoll.docklite.net", 3000)
	for _, want := range []string{"server_name smoll.docklite.net;", "proxy_pass http://127.0.0.1:3000;", "listen 80;"} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config is missing %q", want)
		}
	}
	if nginxConfigHasSSL(cfg) {
		t.Error("a fresh panel vhost has no certificate yet")
	}
}
