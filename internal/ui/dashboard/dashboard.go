package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/revkeen/cli/internal/config"
	"github.com/revkeen/cli/internal/ui"
	"github.com/spf13/cobra"
)

type navItem int

const (
	navHome navItem = iota
	navCustomers
	navInvoices
	navSubscriptions
	navCart
	navDoctor
	navHelp
)

var navLabels = []string{
	"Home",
	"Customers",
	"Invoices",
	"Subscriptions",
	"Cart",
	"Doctor",
	"Help",
}

type model struct {
	cmd        *cobra.Command
	width      int
	height     int
	active     navItem
	status     string
	content    string
	loading    bool
	list       list.Model
	viewport   viewport.Model
	ready      bool
	errMessage string
}

type item struct {
	title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type loadedMsg struct {
	content string
	items   []list.Item
	err     error
}

// Run starts the interactive dashboard.
func Run(cmd *cobra.Command) error {
	ui.ApplyNoColor(cmd)
	m := model{
		cmd:     cmd,
		active:  navHome,
		status:  "Ready",
		content: "Loading…",
		list:    list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0),
	}
	m.list.SetShowStatusBar(false)
	m.list.SetFilteringEnabled(true)
	m.list.Title = "Results"

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m model) Init() tea.Cmd {
	return m.fetchCmd()
}

func (m model) fetchCmd() tea.Cmd {
	active := m.active
	cmd := m.cmd
	return func() tea.Msg {
		switch active {
		case navHome:
			return loadedMsg{content: homeContent()}
		case navHelp:
			return loadedMsg{content: helpContent()}
		case navCustomers:
			return fetchList(cmd, "/customers", "Customers")
		case navInvoices:
			return fetchList(cmd, "/invoices", "Invoices")
		case navSubscriptions:
			return fetchList(cmd, "/subscriptions", "Subscriptions")
		case navCart:
			return fetchJSON(cmd, "/storefront/status", "Cart / storefront status")
		case navDoctor:
			return loadedMsg{content: doctorHint()}
		default:
			return loadedMsg{content: homeContent()}
		}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.layout()
		return m, nil
	case loadedMsg:
		m.loading = false
		if msg.err != nil {
			m.errMessage = msg.err.Error()
			m.content = ui.DangerStyle.Render("Error: ") + msg.err.Error()
			m.list.SetItems(nil)
			m.viewport.SetContent(m.content)
			return m, nil
		}
		m.errMessage = ""
		m.content = msg.content
		if msg.items != nil {
			m.list.SetItems(msg.items)
		} else {
			m.list.SetItems(nil)
		}
		m.viewport.SetContent(m.content)
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?", "h":
			if m.active != navHelp {
				m.active = navHelp
				m.loading = true
				return m, m.fetchCmd()
			}
		case "tab", "right", "l":
			m.active = (m.active + 1) % navItem(len(navLabels))
			m.loading = true
			return m, m.fetchCmd()
		case "shift+tab", "left":
			m.active = (m.active - 1 + navItem(len(navLabels))) % navItem(len(navLabels))
			m.loading = true
			return m, m.fetchCmd()
		case "1", "2", "3", "4", "5", "6", "7":
			idx := int(msg.String()[0] - '1')
			if idx >= 0 && idx < len(navLabels) {
				m.active = navItem(idx)
				m.loading = true
				return m, m.fetchCmd()
			}
		case "r":
			m.loading = true
			return m, m.fetchCmd()
		}
	}

	var cmds []tea.Cmd
	if m.active == navCustomers || m.active == navInvoices || m.active == navSubscriptions {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m *model) layout() {
	navWidth := 22
	mainWidth := m.width - navWidth - 4
	if mainWidth < 20 {
		mainWidth = 20
	}
	mainHeight := m.height - 4
	if mainHeight < 5 {
		mainHeight = 5
	}
	m.list.SetSize(mainWidth, mainHeight)
	m.viewport = viewport.New(mainWidth, mainHeight)
	m.viewport.SetContent(m.content)
}

func (m model) View() string {
	if !m.ready {
		return "Loading dashboard…"
	}

	header := ui.HeaderStyle.Width(m.width).Render(" RevKeen  ·  Terminal Dashboard  ·  q quit  ·  tab navigate  ·  r refresh  ·  ? help ")

	var navLines []string
	for i, label := range navLabels {
		prefix := fmt.Sprintf("%d ", i+1)
		line := prefix + label
		if navItem(i) == m.active {
			navLines = append(navLines, ui.NavActiveStyle.Render("▸ "+line))
		} else {
			navLines = append(navLines, ui.NavIdleStyle.Render("  "+line))
		}
	}
	nav := ui.BorderStyle.Width(20).Render(strings.Join(navLines, "\n"))

	var main string
	if m.loading {
		main = ui.MutedStyle.Render("Loading…")
	} else if m.active == navCustomers || m.active == navInvoices || m.active == navSubscriptions {
		main = m.list.View()
	} else {
		main = m.viewport.View()
	}
	main = ui.BorderStyle.Width(m.width - 26).Render(main)

	body := lipgloss.JoinHorizontal(lipgloss.Top, nav, "  ", main)
	status := ui.MutedStyle.Width(m.width).Render(" " + m.status + "  ·  " + config.OriginBaseURL(config.Load()))
	return lipgloss.JoinVertical(lipgloss.Left, header, body, status)
}

func homeContent() string {
	cfg := config.Load()
	auth := "not authenticated"
	switch {
	case config.ResolveAPIKey(cfg) != "":
		auth = "API key (" + config.MaskAPIKey(config.ResolveAPIKey(cfg)) + ")"
	case cfg.Auth.Mode == "oauth" && cfg.Auth.OAuth.AccessToken != "":
		auth = "OAuth"
	}
	return ui.TitleStyle.Render("Welcome to RevKeen") + "\n\n" +
		fmt.Sprintf("Auth:        %s\n", auth) +
		fmt.Sprintf("Environment: %s\n", config.OriginBaseURL(cfg)) +
		fmt.Sprintf("API prefix:  %s\n\n", config.ResolveBaseURL(cfg)) +
		ui.SubtitleStyle.Render("Quick actions") + "\n" +
		"  2  Browse customers\n" +
		"  3  Browse invoices\n" +
		"  5  Cart / storefront status\n" +
		"  6  Doctor guidance\n\n" +
		ui.MutedStyle.Render("Shell escape: revkeen customers list --json")
}

func helpContent() string {
	return ui.TitleStyle.Render("Keybindings") + "\n\n" +
		"  tab / l     Next section\n" +
		"  shift+tab   Previous section\n" +
		"  1–7         Jump to section\n" +
		"  /           Filter list (on list views)\n" +
		"  r           Refresh\n" +
		"  q           Quit\n\n" +
		"Docs: https://docs.revkeen.com/docs/cli\n"
}

func doctorHint() string {
	return ui.TitleStyle.Render("Doctor") + "\n\n" +
		"Run a full Cart health check from the shell:\n\n" +
		ui.SuccessStyle.Render("  revkeen doctor") + "\n\n" +
		"JSON for agents:\n\n" +
		"  revkeen doctor --json --agent\n"
}

func fetchList(cmd *cobra.Command, path, title string) loadedMsg {
	body, err := apiGET(cmd, path)
	if err != nil {
		return loadedMsg{err: err}
	}
	var envelope struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return loadedMsg{content: string(body)}
	}
	items := make([]list.Item, 0, len(envelope.Data))
	for _, row := range envelope.Data {
		id := fmt.Sprintf("%v", first(row, "id", "object"))
		desc := fmt.Sprintf("%v", first(row, "email", "name", "status", "object"))
		items = append(items, item{title: id, desc: desc})
	}
	if len(items) == 0 {
		return loadedMsg{content: ui.MutedStyle.Render("No " + strings.ToLower(title) + " found."), items: items}
	}
	return loadedMsg{
		content: title,
		items:   items,
	}
}

func fetchJSON(cmd *cobra.Command, path, title string) loadedMsg {
	body, err := apiGET(cmd, path)
	if err != nil {
		return loadedMsg{err: err}
	}
	var pretty interface{}
	if err := json.Unmarshal(body, &pretty); err != nil {
		return loadedMsg{content: ui.TitleStyle.Render(title) + "\n\n" + string(body)}
	}
	formatted, _ := json.MarshalIndent(pretty, "", "  ")
	return loadedMsg{content: ui.TitleStyle.Render(title) + "\n\n" + string(formatted)}
}

func apiGET(cmd *cobra.Command, path string) ([]byte, error) {
	cfg := config.Load()
	apiKey := config.ResolveAPIKeyFromCmd(cmd, cfg)
	if apiKey == "" && cfg.Auth.OAuth.AccessToken == "" {
		return nil, fmt.Errorf("not authenticated — run revkeen auth login")
	}
	url := config.ResolveBaseURL(cfg) + path
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("RevKeen-Version", "2026-05-01")
	if apiKey != "" {
		req.Header.Set("x-api-key", apiKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+cfg.Auth.OAuth.AccessToken)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return body, nil
}

func first(m map[string]interface{}, keys ...string) interface{} {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil && fmt.Sprintf("%v", v) != "" {
			return v
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
