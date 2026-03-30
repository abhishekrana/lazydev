package ui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/abhishek-rana/lazydk/internal/ui/components"
	"github.com/abhishek-rana/lazydk/internal/ui/theme"
	"github.com/abhishek-rana/lazydk/pkg/messages"
)

// TabModel is the interface each tab must implement.
type TabModel interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (TabModel, tea.Cmd)
	View() string
	Title() string
	SetSize(width, height int)
}

// RootModel is the top-level Bubble Tea model.
type RootModel struct {
	tabs      []TabModel
	tabBar    components.TabBar
	statusBar components.StatusBar
	activeTab int
	width     int
	height    int
	ready     bool
}

// NewRootModel creates the root model with the given tabs.
func NewRootModel(tabs []TabModel) RootModel {
	titles := make([]string, len(tabs))
	for i, t := range tabs {
		titles[i] = t.Title()
	}

	return RootModel{
		tabs:      tabs,
		tabBar:    components.NewTabBar(titles),
		statusBar: components.NewStatusBar(),
	}
}

// Init initializes the root model and all tabs.
func (m RootModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, tab := range m.tabs {
		cmds = append(cmds, tab.Init())
	}
	return tea.Batch(cmds...)
}

// Update handles messages for the root model.
func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.tabBar.Width = m.width
		m.statusBar.Width = m.width

		contentHeight := m.contentHeight()
		for i := range m.tabs {
			m.tabs[i].SetSize(m.width, contentHeight)
		}
		return m, nil

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, theme.Keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, theme.Keys.TabNext):
			m.activeTab = (m.activeTab + 1) % len(m.tabs)
			m.tabBar.ActiveTab = m.activeTab
			return m, nil
		case key.Matches(msg, theme.Keys.TabPrev):
			m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
			m.tabBar.ActiveTab = m.activeTab
			return m, nil
		}

	case messages.SwitchTabMsg:
		if msg.Tab >= 0 && msg.Tab < len(m.tabs) {
			m.activeTab = msg.Tab
			m.tabBar.ActiveTab = m.activeTab
		}
		return m, nil

	case messages.DiscoveryResultMsg:
		ctx := ""
		if msg.DockerAvailable {
			ctx += "docker"
		}
		if msg.KubeAvailable {
			if ctx != "" {
				ctx += " | "
			}
			ctx += msg.KubeContext
		}
		m.statusBar.Context = ctx
	}

	// Delegate to active tab.
	if m.activeTab >= 0 && m.activeTab < len(m.tabs) {
		var cmd tea.Cmd
		m.tabs[m.activeTab], cmd = m.tabs[m.activeTab].Update(msg)
		return m, cmd
	}

	return m, nil
}

// View renders the root model.
func (m RootModel) View() tea.View {
	if !m.ready {
		v := tea.NewView("Starting lazydk...")
		v.AltScreen = true
		return v
	}

	tabBarView := m.tabBar.View()
	statusBarView := m.statusBar.View()

	var contentView string
	if m.activeTab >= 0 && m.activeTab < len(m.tabs) {
		contentView = m.tabs[m.activeTab].View()
	}

	contentStyle := lipgloss.NewStyle().
		Width(m.width).
		Height(m.contentHeight())

	v := tea.NewView(lipgloss.JoinVertical(
		lipgloss.Left,
		tabBarView,
		contentStyle.Render(contentView),
		statusBarView,
	))
	v.AltScreen = true
	return v
}

func (m RootModel) contentHeight() int {
	h := m.height - 3
	if h < 1 {
		h = 1
	}
	return h
}
