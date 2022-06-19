// Copyright (c) 2022 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package console

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/pterm/pterm"
	"github.com/sirupsen/logrus"
)

var Default Log = PTermLog{}

func Init(verbose bool) {
	Default = PTermLog{Verbose: verbose}
}

type Log interface {
	// Writer starts a new log printing session which ends once the writer is closed
	Writer() Logs

	StartPhase(name, description string) Phase
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
}

type Logs interface {
	io.WriteCloser
	Show()
}

type Phase interface {
	Success()
	Failure(reason string)
}

type VerboseLogs struct {
	Logs
}

func (w *VerboseLogs) Write(p []byte) (int, error) {
	os.Stdout.Write(p)
	return w.Logs.Write(p)
}

type PTermLog struct {
	Verbose bool
}

func (ptl PTermLog) Writer() Logs {
	f, err := ioutil.TempFile("", "rungp-*.log")
	if err != nil {
		return noopWriteCloser{&areaWriter{Area: &pterm.DefaultArea}}
	}

	res := &filebackedLogs{f}
	if ptl.Verbose {
		return &VerboseLogs{res}
	}
	return res
}

func (ptl PTermLog) Debugf(format string, args ...interface{}) {
	if ptl.Verbose {
		logrus.SetLevel(logrus.DebugLevel)
	}
	logrus.Debugf(format, args...)
}

func (PTermLog) Infof(format string, args ...interface{}) {
	logrus.Infof(format, args...)
}

func (ptl PTermLog) Warnf(format string, args ...interface{}) {
	logrus.Warnf(format, args...)
}

type filebackedLogs struct {
	*os.File
}

func (fb *filebackedLogs) Show() {
	_, _ = fb.Seek(0, 0)
	ctnt, _ := io.ReadAll(fb)
	pterm.DefaultArea.Update(string(ctnt))
}

func (PTermLog) StartPhase(name, description string) Phase {
	s, _ := pterm.DefaultSpinner.WithRemoveWhenDone(false).WithShowTimer(true).Start(name + " " + description)
	return ptermPhase{Spinner: s}
}

type ptermPhase struct {
	Spinner *pterm.SpinnerPrinter
}

func (p ptermPhase) Success() {
	p.Spinner.Success()
}

func (p ptermPhase) Failure(reason string) {
	p.Spinner.Fail(reason)
}

type areaWriter struct {
	buf  string
	Area *pterm.AreaPrinter
}

func (a *areaWriter) Write(buf []byte) (n int, err error) {
	a.buf += string(buf)
	a.Area.Update(a.buf)
	return len(buf), nil
}

type ConsoleLog struct {
	w io.Writer
}

func NewConsoleLog(w io.Writer) ConsoleLog {
	return ConsoleLog{
		w: w,
	}
}

var _ Log = ConsoleLog{}

// FixedMessage implements Log
func (c ConsoleLog) Debugf(format string, args ...interface{}) {
	fmt.Fprintf(c.w, format, args...)
}

// FixedMessage implements Log
func (c ConsoleLog) Infof(format string, args ...interface{}) {
	fmt.Fprintf(c.w, format, args...)
}

// Log implements Log
func (c ConsoleLog) Writer() Logs {
	return noopWriteCloser{c.w}
}

func (c ConsoleLog) Warnf(format string, args ...interface{}) {
	c.Infof("[WARN] "+format, args...)
}

// StartPhase implements Log
func (c ConsoleLog) StartPhase(name, description string) Phase {
	fmt.Fprintf(c.w, "[%s] %s\n", name, description)
	return consolePhase{
		w: c.w,
		n: name,
	}
}

type consolePhase struct {
	w io.Writer
	n string
}

func (c consolePhase) Success() {
	fmt.Fprintf(c.w, "[%s] DONE\n", c.n)
}

func (c consolePhase) Failure(reason string) {
	fmt.Fprintf(c.w, "[%s] FAILED! %s\n", c.n, reason)
}

type noopWriteCloser struct{ io.Writer }

func (noopWriteCloser) Close() error {
	return nil
}

func (noopWriteCloser) Show() {}
