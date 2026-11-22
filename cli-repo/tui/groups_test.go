package tui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"docklite-tui/api"
)

type call struct{ Method, Path, Body string }

func fakeAgent(t *testing.T) (*httptest.Server, *[]call) {
	var calls []call
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		calls = append(calls, call{r.Method, r.URL.RequestURI(), string(b)})
		w.Write([]byte(`{"success":true,"group":{"id":9,"name":"x","members":[]}}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func groupModel(t *testing.T) (Model, *[]call) {
	srv, calls := fakeAgent(t)
	m := newTestModel()
	m.client.SetBaseURL(srv.URL)
	m.currentScreen, m.currentView = ScreenContainers, ViewDetails
	m.containers = []api.Container{*siteBox("static")}
	m.groups = []api.Group{{ID: 1, Name: "Client work", Members: []string{"other"}}, {ID: 2, Name: "Mine", Linked: true, Members: []string{"docklite-site-a-com"}}}
	m.selectedContainer = &m.containers[0]
	m.updateFilteredContainers()
	return m, calls
}

func send(t *testing.T, m Model, k tea.KeyMsg) (Model, tea.Cmd) {
	next, cmd := m.handleMainPanelInput(k)
	return next.(Model), cmd
}

func run(cmd tea.Cmd) groupDoneMsg {
	if cmd == nil {
		return groupDoneMsg{Err: io.EOF}
	}
	return cmd().(groupDoneMsg)
}

func TestMoveBoxToAnExistingGroup(t *testing.T) {
	m, calls := groupModel(t)
	m.actionCursor = len(m.detailActions()) - 2 // Move to group
	m, _ = send(t, m, keyEnter())
	if m.mg.mode != mgPick {
		t.Fatalf("should open the list")
	}
	if !strings.Contains(m.renderMoveGroup(), "← here now") || !strings.Contains(m.renderMoveGroup(), "Take it out") {
		t.Fatalf("list: %s", m.renderMoveGroup())
	}
	m.mg.cursor = 0 // Client work
	m, cmd := send(t, m, keyEnter())
	msg := run(cmd)
	if msg.Err != nil || len(*calls) != 1 || (*calls)[0].Method != "POST" || (*calls)[0].Path != "/api/groups/1/members" || !strings.Contains((*calls)[0].Body, `"container_id":"cid1"`) {
		t.Fatalf("calls %+v err %v", *calls, msg.Err)
	}
	nm, _ := m.Update(msg)
	m = nm.(Model)
	if m.currentView != ViewList || !strings.Contains(m.notice, "moved to Client work") {
		t.Fatalf("after: %v %q", m.currentView, m.notice)
	}
}

func TestMoveBoxToANewLinkedGroup(t *testing.T) {
	m, calls := groupModel(t)
	m.actionCursor = len(m.detailActions()) - 2
	m, _ = send(t, m, keyEnter())
	m.mg.cursor = len(m.groups) // "A new group..."
	m, _ = send(t, m, keyEnter())
	if m.mg.mode != mgName {
		t.Fatalf("should ask for a name")
	}
	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Shop stuff")})
	m, _ = send(t, m, keyEnter())
	if m.mg.mode != mgLink {
		t.Fatalf("should ask about linking")
	}
	m, cmd := send(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if msg := run(cmd); msg.Err != nil {
		t.Fatal(msg.Err)
	}
	var body map[string]any
	_ = json.Unmarshal([]byte((*calls)[0].Body), &body)
	if (*calls)[0].Path != "/api/groups" || body["name"] != "Shop stuff" || body["linked"] != true || body["container_ids"].([]any)[0] != "cid1" {
		t.Fatalf("sent %+v", (*calls)[0])
	}
}

func TestTakeABoxOutOfItsGroup(t *testing.T) {
	m, calls := groupModel(t)
	m.actionCursor = len(m.detailActions()) - 2
	m, _ = send(t, m, keyEnter())
	choices := m.moveChoices()
	m.mg.cursor = len(choices) - 1
	if choices[m.mg.cursor] != "out" {
		t.Fatalf("choices %v", choices)
	}
	m, cmd := send(t, m, keyEnter())
	run(cmd)
	if (*calls)[0].Method != "DELETE" || (*calls)[0].Path != "/api/groups/2/members?container_id=cid1" {
		t.Fatalf("sent %+v", (*calls)[0])
	}
}

func TestGroupsViewLinksUnlinksRenamesAndDeletes(t *testing.T) {
	m, calls := groupModel(t)
	m.currentView = ViewGroups
	out := m.renderGroupsView()
	if !strings.Contains(out, "Client work") || !strings.Contains(out, "yes: its boxes can talk") {
		t.Fatalf("view: %s", out)
	}
	// Enter on a group toggles it
	_, cmd := send(t, m, keyEnter())
	run(cmd)
	if (*calls)[0].Method != "PUT" || (*calls)[0].Path != "/api/groups/1" || !strings.Contains((*calls)[0].Body, `"linked":true`) {
		t.Fatalf("link: %+v", (*calls)[0])
	}
	m.gv.cursor = 1 // Mine is linked: Enter unlinks
	_, cmd = send(t, m, keyEnter())
	run(cmd)
	if !strings.Contains((*calls)[1].Body, `"linked":false`) {
		t.Fatalf("unlink: %+v", (*calls)[1])
	}
	// rename
	m.gv.cursor = 0
	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m.gv.input = "Clients"
	_, cmd = send(t, m, keyEnter())
	run(cmd)
	if !strings.Contains((*calls)[2].Body, `"name":"Clients"`) {
		t.Fatalf("rename: %+v", (*calls)[2])
	}
	// delete needs a second Enter; another key cancels
	m.gv.mode = gvList
	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if m.gv.mode != gvDelete {
		t.Fatalf("d should ask")
	}
	m2, cmd := send(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if cmd != nil || m2.gv.mode != gvList {
		t.Fatalf("another key should cancel")
	}
	_, cmd = send(t, m, keyEnter())
	run(cmd)
	if (*calls)[3].Method != "DELETE" || (*calls)[3].Path != "/api/groups/1" {
		t.Fatalf("delete: %+v", (*calls)[3])
	}
}

func TestGKeyOpensGroupsFromTheList(t *testing.T) {
	m, _ := groupModel(t)
	m.currentView = ViewList
	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if m.currentView != ViewGroups {
		t.Fatalf("view %v", m.currentView)
	}
}
