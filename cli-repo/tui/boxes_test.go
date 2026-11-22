package tui

import (
	"strings"
	"testing"

	"docklite-tui/api"
)

func testBoxes() ([]api.Container, []api.Group) {
	mk := func(name, domain, typ string) api.Container {
		l := map[string]string{"docklite.managed": "true", "docklite.type": typ}
		if domain != "" {
			l["docklite.domain"] = domain
		}
		return api.Container{ID: "abcdef1234567890", Name: name, Image: "node:20-alpine", Status: "Up 2 days", Labels: l}
	}
	return []api.Container{
			mk("docklite-site-solo-example-com", "solo.example.com", "node"),
			mk("docklite-site-a-example-com", "a.example.com", "node"),
			mk("docklite-site-b-example-com", "b.example.com", "static"),
			mk("docklite-db-shop", "", "postgres"),
		}, []api.Group{
			{ID: 1, Name: "Client work", Members: []string{"docklite-site-a-example-com", "docklite-site-b-example-com", "docklite-db-shop"}},
		}
}

func TestBoxTitleUsesTheSiteAddress(t *testing.T) {
	containers, _ := testBoxes()
	if got := boxTitle(containers[0]); got != "solo.example.com" {
		t.Fatalf("got %q", got)
	}
	if got := boxTitle(containers[3]); got != "shop" {
		t.Fatalf("db container: got %q", got)
	}
	if got := boxKind(containers[3]); got != "PostgreSQL DB" {
		t.Fatalf("kind: got %q", got)
	}
}

func TestGroupedBoxesAllShowTogether(t *testing.T) {
	containers, groups := testBoxes()
	m := Model{containers: containers, groups: groups, width: 120, height: 40, currentScreen: ScreenContainers}
	m.updateFilteredContainers()
	if len(m.filteredContainers) != 4 {
		t.Fatalf("got %d containers, want all 4", len(m.filteredContainers))
	}
	order := []string{}
	for _, c := range m.filteredContainers {
		order = append(order, c.Name)
	}
	if order[0] != "docklite-site-a-example-com" || order[3] != "docklite-site-solo-example-com" {
		t.Fatalf("group members should come first, in the group's order: %v", order)
	}
	out := m.renderContainers()
	for _, want := range []string{"Client work (3)", "Not in a group (1)", "a.example.com", "b.example.com", "solo.example.com", "shop", "PostgreSQL DB"} {
		if !strings.Contains(out, want) {
			t.Errorf("list is missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "docklite-sit") {
		t.Errorf("raw container names should not be the headline:\n%s", out)
	}
}

func TestSearchFindsByGroupAndAddress(t *testing.T) {
	containers, groups := testBoxes()
	m := Model{containers: containers, groups: groups}
	m.searchQuery = "client work"
	m.updateFilteredContainers()
	if len(m.filteredContainers) != 3 {
		t.Fatalf("group search: %d", len(m.filteredContainers))
	}
	m.searchQuery = "solo.example"
	m.updateFilteredContainers()
	if len(m.filteredContainers) != 1 {
		t.Fatalf("address search: %d", len(m.filteredContainers))
	}
}

func TestDatabaseConnectURLPerEngine(t *testing.T) {
	for typ, want := range map[string]string{
		"postgres": "postgres://u:p@localhost:5432/n",
		"mariadb":  "mysql://u:p@localhost:5432/n",
		"mongodb":  "mongodb://u:p@localhost:5432/n",
		"redis":    "redis://:p@localhost:5432",
	} {
		if got := dbConnectURL(api.Database{Type: typ, Username: "u", Password: "p", Port: 5432, Name: "n"}); got != want {
			t.Errorf("%s: got %q want %q", typ, got, want)
		}
	}
}

func TestUpdateScreenShowsStateAndAsksTwice(t *testing.T) {
	m := Model{width: 120, height: 40, currentScreen: ScreenSettings, focusArea: FocusMain, x: extraState{setView: 1},
		updStatus: &api.UpdateStatus{Version: "1.14.0", LatestVersion: "1.15.0", UpdateAvailable: true}}
	if out := m.renderUpdate(); !strings.Contains(out, "1.15.0") || !strings.Contains(out, "web dashboard") {
		t.Fatalf("update screen: %s", out)
	}
	next, cmd := m.handleMainPanelInput(keyEnter())
	m = next.(Model)
	if cmd != nil || !m.updConfirm {
		t.Fatalf("first Enter should only ask to confirm")
	}
	next, cmd = m.handleMainPanelInput(keyEnter())
	m = next.(Model)
	if cmd == nil || !m.updRunning {
		t.Fatalf("second Enter should start the update")
	}
}
