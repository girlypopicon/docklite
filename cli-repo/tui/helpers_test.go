package tui

import tea "github.com/charmbracelet/bubbletea"

func keyEnter() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyEnter} }

func keyEsc() tea.KeyMsg        { return tea.KeyMsg{Type: tea.KeyEsc} }
func runeKey(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }
