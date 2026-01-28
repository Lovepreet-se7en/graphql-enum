package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle         = lipgloss.NewStyle().MarginLeft(2).Bold(true).Foreground(lipgloss.Color("#FF06B7"))
	selectedItemStyle  = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("#00BFFF"))
	selectedDescStyle  = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("#666666"))
	queryStyle         = lipgloss.NewStyle().Margin(1).Padding(1).Border(lipgloss.RoundedBorder()).Background(lipgloss.Color("#1a1a1a"))
	helpStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
	statusMessageStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Bold(true)
)

type item struct {
	title       string
	description string
	index       int
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.description }
func (i item) FilterValue() string { return i.title + i.description }

func (m Model) View() string {
	if m.showQuery {
		return m.renderQueryView()
	}
	return m.renderListView()
}

func (m Model) renderListView() string {
	var b strings.Builder
	b.WriteString(m.list.View())
	if m.statusMsg != "" {
		b.WriteString("\n")
		b.WriteString(statusMessageStyle.Render("✓ " + m.statusMsg))
	}
	return b.String()
}

func (m Model) renderQueryView() string {
	var b strings.Builder
	
	b.WriteString(titleStyle.Render("Generated GraphQL Query"))
	b.WriteString("\n\n")
	b.WriteString(queryStyle.Render(m.selectedQuery.Query))
	b.WriteString("\n\n")
	
	// Variables section
	if len(m.selectedQuery.Variables) > 0 {
		b.WriteString(lipgloss.NewStyle().Bold(true).Render("Variables:"))
		b.WriteString("\n")
		for k, v := range m.selectedQuery.Variables {
			b.WriteString(fmt.Sprintf("  $%s: %v\n", k, v))
		}
		b.WriteString("\n")
	}
	
	b.WriteString(helpStyle.Render("c: copy • s: save • q: back • ctrl+c: quit"))
	
	if m.statusMsg != "" {
		b.WriteString("\n")
		b.WriteString(statusMessageStyle.Render(m.statusMsg))
	}
	
	return b.String()
}
