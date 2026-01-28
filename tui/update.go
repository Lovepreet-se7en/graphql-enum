package tui

import (
	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/lovepreet-se7en/graphql-enum/internal/generator"
	"os"
	"path/filepath"
)

type statusMsg string

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-4)
		return m, nil
		
	case tea.KeyMsg:
		if m.showQuery {
			return m.handleQueryViewKeys(msg)
		}
		return m.handleListKeys(msg)
		
	case statusMsg:
		m.statusMsg = string(msg)
		return m, nil
	}
	
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) handleListKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "enter":
		if i, ok := m.list.SelectedItem().(item); ok {
			m.selectedQuery = m.queries[i.index]
			m.showQuery = true
		}
	case "s":
		return m, m.saveAll()
	case "e":
		return m, m.exportSelected()
	}
	
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) handleQueryViewKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.showQuery = false
		m.statusMsg = ""
		return m, nil
	case "c":
		clipboard.WriteAll(m.selectedQuery.Query)
		return m, func() tea.Msg { return statusMsg("Copied to clipboard!") }
	case "s":
		filename := filepath.Join(".", m.selectedQuery.FileName)
		os.WriteFile(filename, []byte(m.selectedQuery.Query), 0644)
		return m, func() tea.Msg { return statusMsg("Saved to " + filename) }
	}
	return m, nil
}

func (m Model) saveAll() tea.Cmd {
	var gen *generator.Generator
	var queries []generator.GeneratedQuery

	return func() tea.Msg {
		gen = generator.New(m.schema, "./queries")
		queries = make([]generator.GeneratedQuery, 0, len(m.queries))
		for _, q := range m.queries {
			queries = append(queries, q)
		}
		gen.SaveToFiles(queries)
		return statusMsg("Saved all queries to ./queries/")
	}
}

func (m Model) exportSelected() tea.Cmd {
	return func() tea.Msg {
		// Export logic here
		return statusMsg("Exported!")
	}
}
