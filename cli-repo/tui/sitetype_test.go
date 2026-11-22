package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"docklite-tui/api"
)

func siteBox(typ string) *api.Container {
	return &api.Container{ID: "cid1", Name: "docklite-site-a-com", Labels: map[string]string{"docklite.managed": "true", "docklite.domain": "a.com", "docklite.type": typ}}
}

func TestChangeSiteTypeOnlyOffersItForWebsites(t *testing.T) {
	m := newTestModel()
	m.selectedContainer = siteBox("static")
	if got := m.detailActions(); got[len(got)-4] != "Change site type" || got[len(got)-3] != "Forwarded domains" || got[len(got)-1] != "Delete" {
		t.Fatalf("a website should offer it before Move to group and Delete: %v", got)
	}
	m.selectedContainer = &api.Container{Name: "db", Labels: map[string]string{"docklite.managed": "true", "docklite.type": "postgres"}}
	for _, a := range m.detailActions() {
		if a == "Change site type" {
			t.Fatalf("a database must not offer it")
		}
	}
}

func TestChangeSiteTypeFlowSendsTheRequest(t *testing.T) {
	var got map[string]any
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{"success":true,"warning":"no index.js"}`))
	}))
	defer srv.Close()

	m := newTestModel()
	m.client.SetBaseURL(srv.URL)
	m.currentScreen, m.currentView = ScreenContainers, ViewDetails
	m.selectedContainer = siteBox("static")
	m.actionCursor = len(m.detailActions()) - 4 // Change site type

	press := func(k tea.KeyMsg) tea.Cmd {
		next, cmd := m.handleMainPanelInput(k)
		m = next.(Model)
		return cmd
	}
	press(keyEnter())
	if m.ctMode != 1 || siteTypes[m.ctCursor].id != "static" {
		t.Fatalf("should open on the current kind: mode %d cursor %d", m.ctMode, m.ctCursor)
	}
	press(tea.KeyMsg{Type: tea.KeyDown})
	press(tea.KeyMsg{Type: tea.KeyDown}) // node
	press(keyEnter())
	if m.ctMode != 2 {
		t.Fatalf("node asks for a port, mode %d", m.ctMode)
	}
	press(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("8080")})
	press(keyEnter())
	if m.ctMode != 3 || !strings.Contains(m.renderChangeType(), "Press Enter to switch to Node.js app") {
		t.Fatalf("should ask to confirm: %s", m.renderChangeType())
	}
	cmd := press(keyEnter())
	if cmd == nil {
		t.Fatal("confirming should send the request")
	}
	msg := cmd().(siteTypeDoneMsg)
	if msg.Err != nil || path != "/api/containers/cid1/template" || got["template_type"] != "node" || got["port"] != float64(8080) {
		t.Fatalf("sent %v to %s, err %v", got, path, msg.Err)
	}
	nm, _ := m.Update(msg)
	m = nm.(Model)
	if m.currentView != ViewList || !strings.Contains(m.notice, "now runs as Node.js app") || !strings.Contains(m.notice, "no index.js") {
		t.Fatalf("after: view %v notice %q", m.currentView, m.notice)
	}
}

func TestChangeSiteTypeToSameKindDoesNothing(t *testing.T) {
	m := newTestModel()
	m.currentScreen, m.currentView = ScreenContainers, ViewDetails
	m.selectedContainer = siteBox("php")
	m.ctMode, m.ctCursor = 1, siteTypeIndex("php")
	next, cmd := m.handleChangeTypeInput(keyEnter())
	if cmd != nil || next.(Model).ctMode != 0 {
		t.Fatalf("same kind should just close")
	}
}
