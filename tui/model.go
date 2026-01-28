package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/lovepreet-se7en/graphql-enum/internal/generator"
	"github.com/lovepreet-se7en/graphql-enum/internal/schema"
)

type Model struct {
	list          list.Model
	paths         []schema.GraphQLPath
	queries       map[int]generator.GeneratedQuery
	selectedQuery generator.GeneratedQuery
	showQuery     bool
	width         int
	height        int
	schema        *schema.Schema
	targetType    string
	statusMsg     string
}

func NewModel(paths []schema.GraphQLPath, scm *schema.Schema, target string) Model {
	gen := generator.New(scm, "/tmp")
	queries, _ := gen.GenerateAll(paths)
	
	items := make([]list.Item, len(paths))
	queryMap := make(map[int]generator.GeneratedQuery)
	
	for i, q := range queries {
		queryMap[i] = q
		items[i] = item{
			title:       q.Description,
			description: q.Path,
			index:       i,
		}
	}
	
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = selectedItemStyle
	delegate.Styles.SelectedDesc = selectedDescStyle
	
	l := list.New(items, delegate, 0, 0)
	l.Title = "🔍 GraphQL Path Enumerator - " + target
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	l.Styles.HelpStyle = helpStyle
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "view query")),
			key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy")),
			key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "save all")),
			key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		}
	}
	
	return Model{
		list:       l,
		paths:      paths,
		queries:    queryMap,
		schema:     scm,
		targetType: target,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}
