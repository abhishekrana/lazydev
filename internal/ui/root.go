package ui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/abhishek-rana/lazydev/internal/ui/components"
	"github.com/abhishek-rana/lazydev/internal/ui/theme"
	"github.com/abhishek-rana/lazydev/pkg/messages"
)

// TabModel is the interface each tab must implement.
type TabModel interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (TabModel, tea.Cmd)
	View() string
	Title() string
	SetSize(width, height int)
}

// Notifier is an optional interface tabs can implement to show status bar messages.
type Notifier interface {
	Notification() string
}

// RootModel is the top-level Bubble Tea model.
type RootModel struct {
	tabs       []TabModel
	tabBar     components.TabBar
	statusBar  components.StatusBar
	help       components.HelpOverlay
	cmdPalette components.CmdPalette
	activeTab  int
	width      int
	height     int
	ready      bool
}

// NewRootModel creates the root model with the given tabs.
func NewRootModel(tabs []TabModel) RootModel {
	titles := make([]string, len(tabs))
	for i, t := range tabs {
		titles[i] = t.Title()
	}

	return RootModel{
		tabs:       tabs,
		tabBar:     components.NewTabBar(titles),
		statusBar:  components.NewStatusBar(),
		help:       components.NewHelpOverlay(),
		cmdPalette: components.NewCmdPalette(),
	}
}

// Init initializes the root model and all tabs.
func (m RootModel) Init() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.tabs))
	for _, tab := range m.tabs {
		cmds = append(cmds, tab.Init())
	}
	return tea.Batch(cmds...)
}

// Update handles messages for the root model.
func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Help overlay intercepts all input when visible.
	if m.help.Visible() {
		cmd := m.help.Update(msg)
		return m, cmd
	}

	// Command palette intercepts all input when visible.
	if m.cmdPalette.Visible() {
		cmd := m.cmdPalette.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.tabBar.Width = m.width
		m.statusBar.Width = m.width
		m.help.SetSize(m.width, m.height)
		m.cmdPalette.SetWidth(m.width)

		contentHeight := m.contentHeight()
		for i := range m.tabs {
			m.tabs[i].SetSize(m.width, contentHeight)
		}
		return m, nil

	case tea.MouseClickMsg:
		mouse := msg.Mouse()
		// Tab bar click (row 0 or 1, within the tab bar area).
		if mouse.Y <= 1 {
			x := 0
			for i, tab := range m.tabs {
				tabWidth := len(tab.Title()) + 4 // padding
				if mouse.X >= x && mouse.X < x+tabWidth {
					m.activeTab = i
					m.tabBar.ActiveTab = m.activeTab
					return m, nil
				}
				x += tabWidth
			}
		}

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, theme.Keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, theme.Keys.Help):
			m.help.Toggle()
			return m, nil
		case key.Matches(msg, theme.Keys.Command):
			m.cmdPalette.Show(m.executeCommand)
			return m, nil
		case key.Matches(msg, theme.Keys.TabNext):
			m.activeTab = (m.activeTab + 1) % len(m.tabs)
			m.tabBar.ActiveTab = m.activeTab
			return m, nil
		case key.Matches(msg, theme.Keys.TabPrev):
			m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
			m.tabBar.ActiveTab = m.activeTab
			return m, nil
		case msg.String() == "1", msg.String() == "2", msg.String() == "3", msg.String() == "4":
			idx := int(msg.String()[0] - '1')
			if idx >= 0 && idx < len(m.tabs) {
				m.activeTab = idx
				m.tabBar.ActiveTab = m.activeTab
			}
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

	// Broadcast data messages to all tabs so each tab receives its own async results.
	// This is needed because Init() fires Cmds for all tabs, but Update() normally
	// only routes to the active tab.
	switch msg.(type) {
	case messages.LogBatchMsg, messages.ContainerListMsg, messages.ResourceStatsMsg,
		messages.ContainerActionMsg, messages.ContainerInspectMsg,
		messages.LogStreamErrorMsg, messages.ExecFinishedMsg, messages.ScaleMsg,
		messages.LogExportedMsg:
		var cmds []tea.Cmd
		for i := range m.tabs {
			var cmd tea.Cmd
			m.tabs[i], cmd = m.tabs[i].Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)
	}

	// Delegate to active tab.
	if m.activeTab >= 0 && m.activeTab < len(m.tabs) {
		var cmd tea.Cmd
		m.tabs[m.activeTab], cmd = m.tabs[m.activeTab].Update(msg)
		return m, cmd
	}

	return m, nil
}

// executeCommand handles commands from the command palette.
func (m RootModel) executeCommand(cmd string, args []string) tea.Cmd {
	switch strings.ToLower(cmd) {
	case "quit", "q":
		return tea.Quit
	case "tab":
		if len(args) > 0 {
			for i, tab := range m.tabs {
				if strings.EqualFold(tab.Title(), args[0]) {
					return func() tea.Msg { return messages.SwitchTabMsg{Tab: i} }
				}
			}
		}
	case "docker":
		return func() tea.Msg { return messages.SwitchTabMsg{Tab: 0} }
	case "k8s", "kubernetes":
		return func() tea.Msg { return messages.SwitchTabMsg{Tab: 1} }
	case "logs":
		for i, tab := range m.tabs {
			if tab.Title() == "All Logs" {
				idx := i
				return func() tea.Msg { return messages.SwitchTabMsg{Tab: idx} }
			}
		}
	case "dashboard":
		for i, tab := range m.tabs {
			if tab.Title() == "Dashboard" {
				idx := i
				return func() tea.Msg { return messages.SwitchTabMsg{Tab: idx} }
			}
		}
	case "help":
		m.help.Toggle()
	}
	return nil
}

// View renders the root model.
func (m RootModel) View() tea.View {
	if !m.ready {
		v := tea.NewView("Starting lazydev...")
		v.AltScreen = true
		return v
	}

	// Help overlay covers everything.
	if m.help.Visible() {
		v := tea.NewView(m.help.View())
		v.AltScreen = true
		return v
	}

	// Pull notification from active tab if it implements Notifier.
	m.statusBar.Message = ""
	if m.activeTab >= 0 && m.activeTab < len(m.tabs) {
		if n, ok := m.tabs[m.activeTab].(Notifier); ok {
			m.statusBar.Message = n.Notification()
		}
	}

	tabBarView := m.tabBar.View()

	var statusBarView string
	if m.cmdPalette.Visible() {
		statusBarView = m.cmdPalette.View()
	} else {
		statusBarView = m.statusBar.View()
	}

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
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m RootModel) contentHeight() int {
	h := m.height - 3
	if h < 1 {
		h = 1
	}
	return h
}
