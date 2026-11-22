package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func foldModel() Model {
	containers, groups := testBoxes()
	m := Model{containers: containers, groups: groups, width: 120, height: 40, currentScreen: ScreenContainers, focusArea: FocusMain}
	m.updateFilteredContainers()
	return m
}

func TestGroupsStartOpenWithHeadings(t *testing.T) {
	m := foldModel()
	// "Client work" heading + 3 members, "Not in a group" heading + 1
	if len(m.rows) != 6 || !m.rows[0].header || m.rows[0].count != 3 || !m.rows[4].header || m.rows[4].group != "" {
		t.Fatalf("rows: %+v", m.rows)
	}
}

func TestLeftFoldsAndRightOpensAGroup(t *testing.T) {
	m := foldModel()
	next, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyLeft})
	m = next.(Model)
	if len(m.rows) != 3 { // heading, "Not in a group" heading + 1
		t.Fatalf("folded rows: %d", len(m.rows))
	}
	next, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	if len(m.rows) != 6 {
		t.Fatalf("opened rows: %d", len(m.rows))
	}
}

func TestEnterOnHeadingTogglesAndOnContainerOpensIt(t *testing.T) {
	m := foldModel()
	next, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if !m.collapsed["Client work"] || m.currentView != ViewList {
		t.Fatal("enter on a heading should fold it")
	}
	m.containersCursor = 1 // "Not in a group" heading
	m.containersCursor = 2 // solo container
	next, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.currentView != ViewDetails || m.selectedContainer == nil || m.selectedContainer.Name != "docklite-site-solo-example-com" {
		t.Fatalf("view %v sel %+v", m.currentView, m.selectedContainer)
	}
}

func TestLeftOnAContainerJumpsToItsHeading(t *testing.T) {
	m := foldModel()
	m.containersCursor = 2
	next, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyLeft})
	if next.(Model).containersCursor != 0 {
		t.Fatalf("cursor %d", next.(Model).containersCursor)
	}
}
