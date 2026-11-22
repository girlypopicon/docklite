package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestForwardFlowSendsDomainAndMode(t *testing.T) {
	var got map[string]any
	var path, method string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, method = r.URL.Path, r.Method
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{"success":true,"group":"a.com"}`))
	}))
	defer srv.Close()

	m := newTestModel()
	m.client.SetBaseURL(srv.URL)
	m.currentScreen, m.currentView = ScreenContainers, ViewDetails
	m.selectedContainer = siteBox("static")
	m.actionCursor = len(m.detailActions()) - 3 // Forwarded domains

	press := func(k tea.KeyMsg) tea.Cmd {
		next, cmd := m.handleMainPanelInput(k)
		m = next.(Model)
		return cmd
	}
	press(keyEnter())
	if m.fw.mode != fwDomain {
		t.Fatalf("should ask for the domain, mode %d", m.fw.mode)
	}
	for _, r := range "Old-Name.com" {
		press(runeKey(r))
	}
	press(keyEnter())
	if m.fw.mode != fwHow || m.fw.domain != "old-name.com" {
		t.Fatalf("mode %d domain %q", m.fw.mode, m.fw.domain)
	}
	press(tea.KeyMsg{Type: tea.KeyDown}) // proxy
	press(keyEnter())
	cmd := press(keyEnter())
	if cmd == nil {
		t.Fatal("confirming should send the request")
	}
	done := cmd().(forwardDoneMsg)
	if done.Err != nil || method != "POST" || path != "/api/containers/cid1/forward" || got["domain"] != "old-name.com" || got["mode"] != "proxy" {
		t.Fatalf("%v %s %s %v", done.Err, method, path, got)
	}
}

func TestForwardRejectsAnObviouslyBadDomain(t *testing.T) {
	m := newTestModel()
	m.currentScreen, m.currentView = ScreenContainers, ViewDetails
	m.selectedContainer = siteBox("static")
	m.fw = forwardState{mode: fwDomain, domain: "nodots"}
	next, _ := m.handleForwardInput(keyEnter())
	if next.(Model).fw.mode != fwDomain || next.(Model).err == "" {
		t.Fatal("a domain with no dot should be refused with a message")
	}
}
