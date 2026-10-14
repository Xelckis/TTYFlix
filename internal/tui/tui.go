package tui

import (
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/ssh"
)

type item struct {
	title, desc, path string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

func getMediaItems(dir string) []list.Item {
	var items []list.Item
	entries, err := os.ReadDir(dir)
	if err != nil {
		return items
	}

	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".mp4") {
			items = append(items, item{
				title: e.Name(),
				desc:  "Local: " + filepath.Join(dir, e.Name()),
				path:  filepath.Join(dir, e.Name()),
			})
		}
	}
	return items
}

type model struct {
	sess     ssh.Session
	list     list.Model
	selected string
}

func TeaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	pty, _, _ := s.Pty()
	width := pty.Window.Width
	height := pty.Window.Height
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}

	items := getMediaItems("./media")

	l := list.New(items, list.NewDefaultDelegate(), width, height-4)
	l.Title = "TTYFlix - Catálogo de Filmes"

	l.Styles.Title = lipgloss.NewStyle().
		Background(lipgloss.Color("57")).
		Foreground(lipgloss.Color("231")).
		Bold(true).
		Padding(0, 1)

	m := model{
		sess: s,
		list: l,
	}

	return m, []tea.ProgramOption{}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-4)
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			if i, ok := m.list.SelectedItem().(item); ok {
				m.selected = i.path
				m.sess.Context().SetValue("selected_video", i.path)
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() tea.View {
	if m.selected != "" {
		return tea.NewView("")
	}

	tunnelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("0")).
		Background(lipgloss.Color("214")).
		Bold(true).
		Padding(0, 1).
		MarginBottom(1)

	tunnelMsg := tunnelStyle.Render("⚠️  ÁUDIO: Conecte via túnel -> ssh -p 2222 -L 8080:localhost:8080 live@localhost")

	v := tea.NewView(tunnelMsg + "\n" + m.list.View())
	v.AltScreen = true
	return v
}
