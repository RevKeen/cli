package ui

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type spinnerModel struct {
	spinner spinner.Model
	message string
	done    bool
	err     error
	result  interface{}
	work    func() (interface{}, error)
}

type workDoneMsg struct {
	result interface{}
	err    error
}

func (m spinnerModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		result, err := m.work()
		return workDoneMsg{result: result, err: err}
	})
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case workDoneMsg:
		m.done = true
		m.result = msg.result
		m.err = msg.err
		return m, tea.Quit
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.err = fmt.Errorf("cancelled")
			return m, tea.Quit
		}
	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m spinnerModel) View() string {
	if m.done {
		return ""
	}
	return fmt.Sprintf("%s %s", m.spinner.View(), MutedStyle.Render(m.message))
}

// WithSpinner runs work while showing a spinner on stderr when interactive.
// When not interactive, it runs work silently.
func WithSpinner(interactive bool, message string, work func() error) error {
	_, err := WithSpinnerResult(interactive, message, func() (interface{}, error) {
		return nil, work()
	})
	return err
}

// WithSpinnerResult runs work and returns its result, with an optional spinner.
func WithSpinnerResult(interactive bool, message string, work func() (interface{}, error)) (interface{}, error) {
	if !interactive {
		return work()
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ColorBrand)

	m := spinnerModel{
		spinner: s,
		message: message,
		work:    work,
	}

	p := tea.NewProgram(m, tea.WithOutput(os.Stderr), tea.WithInput(os.Stdin))
	final, err := p.Run()
	if err != nil {
		// Fallback if terminal program fails
		time.Sleep(10 * time.Millisecond)
		return work()
	}
	sm, ok := final.(spinnerModel)
	if !ok {
		return work()
	}
	return sm.result, sm.err
}

// PrintChecklist renders doctor-style pass/warn/fail rows.
func PrintChecklist(w io.Writer, title string, ready bool, rows []ChecklistRow) {
	state := SuccessStyle.Render("READY")
	if !ready {
		state = DangerStyle.Render("NOT READY")
	}
	_, _ = fmt.Fprintf(w, "%s: %s\n\n", TitleStyle.Render(title), state)
	for _, row := range rows {
		marker := MutedStyle.Render("•")
		switch row.Status {
		case "pass":
			marker = SuccessStyle.Render("✓")
		case "warn":
			marker = WarnStyle.Render("!")
		case "fail":
			marker = DangerStyle.Render("✗")
		}
		_, _ = fmt.Fprintf(w, "  %s %-22s %s\n", marker, row.ID, row.Message)
		if row.NextAction != "" {
			_, _ = fmt.Fprintf(w, "      %s %s\n", MutedStyle.Render("→"), row.NextAction)
		}
	}
}

// ChecklistRow is a single doctor/checklist line.
type ChecklistRow struct {
	ID         string
	Status     string
	Message    string
	NextAction string
}
