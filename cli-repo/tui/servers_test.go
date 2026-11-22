package tui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPickingAnotherServerSendsCallsThroughTheHub(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	m := newTestModel()
	m.client.SetBaseURL(srv.URL)
	m.currentScreen = ScreenSettings
	m.x.setView = 5
	m.x.servers = []serverRow{{ID: 3, Name: "smoll", URL: "https://smoll.example"}}
	m.containers = append(m.containers, *siteBox("static"))

	next, cmd := m.handleServersInput(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	next, cmd = m.handleServersInput(keyEnter())
	m = next.(Model)
	if m.x.viewing != "smoll" || m.client.Via != "/api/servers/3/proxy" || len(m.containers) != 0 || m.x.setView != 0 || cmd == nil {
		t.Fatalf("viewing %q via %q containers %d view %d", m.x.viewing, m.client.Via, len(m.containers), m.x.setView)
	}
	if !strings.Contains(m.renderSidebar(), "smoll") {
		t.Fatal("the menu should say which server is shown")
	}
	// calls go through the hub; the hub's own server list never does
	_, _ = m.client.GetContainers()
	_ = m.client.GetJSON("/api/servers", &struct{}{})
	if len(paths) < 2 || paths[0] != "/api/servers/3/proxy/containers" || paths[len(paths)-1] != "/api/servers" {
		t.Fatalf("paths %v", paths)
	}

	// back to this server
	m.currentScreen, m.x.setView, m.x.serverCursor = ScreenSettings, 5, 0
	next, _ = m.handleServersInput(keyEnter())
	m = next.(Model)
	if m.client.Via != "" || m.x.viewing != "" {
		t.Fatal("This server should clear the redirect")
	}
}

func TestServersScreenExplainsWhenThereAreNone(t *testing.T) {
	m := newTestModel()
	m.currentScreen = ScreenSettings
	m.x.setView = 5
	out := m.renderServers()
	for _, want := range []string{"This server", "docklite share", "docklite servers add"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(m.renderSettings(), "Servers") {
		t.Fatal("Settings should list Servers")
	}
}
