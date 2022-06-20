// Copyright (c) 2022 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package console

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/muesli/reflow/indent"
	"github.com/pterm/pterm"
	"github.com/sirupsen/logrus"
)

func NewBubbleTeaUI(verbose bool) (log *BubbleTeaUI, done <-chan struct{}, err error) {
	var opts []tea.ProgramOption
	isterm := isatty.IsTerminal(os.Stdout.Fd())
	if verbose {
		logrus.SetLevel(logrus.DebugLevel)
		opts = []tea.ProgramOption{tea.WithoutRenderer()}
	} else if isterm {
		// If we're in TUI mode, discard log output
		logrus.SetOutput(ioutil.Discard)
	}
	if !isterm {
		// If we're in daemon mode don't render the TUI
		opts = []tea.ProgramOption{tea.WithoutRenderer()}
	}

	if verbose {

		logrus.SetOutput(os.Stdout)
	}

	m := newUIModel()
	p := tea.NewProgram(m, opts...)
	go func() {
		p.Start()
		close(m.done)
	}()

	return &BubbleTeaUI{prog: p}, m.done, err
}

type BubbleTeaUI struct {
	prog    *tea.Program
	verbose bool
}

func (ui *BubbleTeaUI) Quit() {
	ui.prog.Send(tea.Quit())
	time.Sleep(100 * time.Millisecond)
}

// Debugf implements Log
func (*BubbleTeaUI) Debugf(format string, args ...interface{}) {
	logrus.Debugf(format, args...)
}

// Infof implements Log
func (*BubbleTeaUI) Infof(format string, args ...interface{}) {
	logrus.Infof(format, args...)
}

// Warnf implements Log
func (ui *BubbleTeaUI) Warnf(format string, args ...interface{}) {
	logrus.Warnf(format, args...)
	ui.prog.Send(msgWarning(fmt.Sprintf(format, args...)))
}

// StartPhase implements Log
func (ui *BubbleTeaUI) StartPhase(name string, description string) Phase {
	desc := name + " " + description
	ui.prog.Send(msgPhaseStart(desc))
	return &bubblePhase{
		parent: ui,
		start:  time.Now(),
		desc:   desc,
	}
}

// Writer implements Log
func (ui *BubbleTeaUI) Writer() Logs {
	rr, rw := io.Pipe()

	go func() {
		r := bufio.NewScanner(rr)
		for r.Scan() {
			l := r.Text()
			ui.prog.Send(msgLogLine(l))
		}
	}()

	return &bubbleLogs{WriteCloser: rw, parent: ui}
}

type bubbleLogs struct {
	io.WriteCloser
	parent *BubbleTeaUI
}

func (b *bubbleLogs) Discard() {
	b.parent.prog.Send(msgDiscardLogs{})
}

type bubblePhase struct {
	parent *BubbleTeaUI
	start  time.Time
	desc   string
}

// Failure implements Phase
func (p *bubblePhase) Failure(reason string) {
	p.parent.prog.Send(msgPhaseDone{
		Duration: time.Since(p.start),
		Desc:     p.desc,
		Failure:  reason,
	})
}

// Success implements Phase
func (p *bubblePhase) Success() {
	p.parent.prog.Send(msgPhaseDone{
		Duration: time.Since(p.start),
		Desc:     p.desc,
	})
}

type msgPhaseDone uiPhase
type msgPhaseStart string
type msgLogLine string
type msgDiscardLogs struct{}
type msgWarning string

var _ Log = &BubbleTeaUI{}

type uiModel struct {
	spinner      spinner.Model
	phases       []uiPhase
	currentPhase string

	warnings []string

	done chan struct{}
	logs []string
}

type uiPhase struct {
	Duration time.Duration
	Desc     string
	Failure  string
}

func newUIModel() uiModel {
	sp := spinner.New()
	sp.Spinner = spinner.Points
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff8a00"))
	return uiModel{
		spinner: sp,
		done:    make(chan struct{}),
	}
}

func (m uiModel) Init() tea.Cmd {
	return spinner.Tick
}

func (m uiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case msgPhaseStart:
		m.currentPhase = string(msg)
		logrus.Infof("%s starting", msg)
	case msgPhaseDone:
		m.currentPhase = ""
		p := uiPhase(msg)
		m.phases = append(m.phases, p)
		if p.Failure == "" {
			logrus.WithField("duration", p.Duration).Infof("%s done", p.Desc)
		} else {
			logrus.WithField("duration", p.Duration).WithField("failure", p.Failure).Errorf("%s done", p.Desc)
		}
	case msgLogLine:
		m.logs = append(m.logs, string(msg))
		if len(m.logs) >= 10 {
			m.logs = m.logs[1:]
		}
		logrus.Info(msg)
	case msgDiscardLogs:
		m.logs = nil
	case msgWarning:
		m.warnings = append(m.warnings, string(msg))
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

var banner = `    
   _______  ______     ____ _____ 
  / ___/ / / / __ \   / __ ` + "`" + `/ __ \
 / /  / /_/ / / / /  / /_/ / /_/ /
/_/   \__,_/_/ /_/   \__, / .___/ 
                     /____/_/      
`

var (
	stylePhaseDone     = lipgloss.NewStyle().Background(lipgloss.Color("#16825d")).Render
	stylePhaseFailed   = lipgloss.NewStyle().Background(lipgloss.Color("#f51f1f")).Render
	stylePhaseDuration = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true).Render
	styleHelp          = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render
	styleWarning       = lipgloss.NewStyle().Background(lipgloss.Color("#ffbe5c")).Bold(true).Render
)

func (m uiModel) View() string {
	var s string

	lines := strings.Split(banner, "\n")
	start, _ := pterm.NewRGBFromHEX("#ff8a00")
	end, _ := pterm.NewRGBFromHEX("#ffbe5c")
	for _, line := range lines {
		for i := range line {
			s += start.Fade(0, float32(len(line)), float32(i), end).Sprint(line[i : i+1])
		}
		s += "\n"
	}

	if len(m.warnings) > 0 {
		for _, w := range m.warnings {
			s += styleWarning(" WARNING ") + " " + w + "\n"
		}
		s += "\n"
	}

	for _, p := range m.phases {
		if p.Failure == "" {
			s += stylePhaseDone(" SUCCESS ")
		} else {
			s += stylePhaseFailed(" FAILURE ")
		}
		s += p.Desc + stylePhaseDuration(fmt.Sprintf(" (%3.3fs) ", p.Duration.Seconds()))
		if p.Failure != "" {
			s += "\n          " + p.Failure
		}
		s += "\n"
	}

	if m.currentPhase != "" {
		s += "      " + m.spinner.View() + " " + m.currentPhase + "\n\n"
	}

	for _, res := range m.logs {
		s += res + "\n"
	}
	s += "\n"

	return indent.String(s, 1)
}
