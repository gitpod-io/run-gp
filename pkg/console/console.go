// Copyright (c) 2022 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package console

import (
	"fmt"
	"io"

	"github.com/sirupsen/logrus"
)

var Default Log = ConsoleLog{}

func Init(l Log) {
	Default = l
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
	Discard()
}

type Phase interface {
	Success()
	Failure(reason string)
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

func (c ConsoleLog) Debugf(format string, args ...interface{}) {
	logrus.Debugf(format, args...)
}

func (c ConsoleLog) Infof(format string, args ...interface{}) {
	logrus.Infof(format, args...)
}

func (c ConsoleLog) Warnf(format string, args ...interface{}) {
	logrus.Warnf(format, args...)
}

// Log implements Log
func (c ConsoleLog) Writer() Logs {
	return noopWriteCloser{c.w}
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

func (noopWriteCloser) Discard() {}
