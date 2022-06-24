// Copyright (c) 2022 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package console

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	supervisor "github.com/gitpod-io/gitpod/supervisor/api"
	"github.com/segmentio/textio"
	"google.golang.org/grpc"
)

type WorkspaceAccessInfo struct {
	WorkspaceFolder string
	HTTPPort        int
	SSHPort         int
	SupervisorPort  int
}

type ObserveOpts struct {
	OnFail       func()
	ObserveTasks bool
	OnTasksDone  func()
}

func Observe(ctx context.Context, log Log, access WorkspaceAccessInfo, opts ObserveOpts) Logs {
	rr, rw := io.Pipe()

	var (
		steady string
		phase  = "starting"
	)
	p := log.StartPhase("["+phase+"]", "workspace image")

	go func() {
		extensions := make(map[string]struct{})

		var workspaceURL string
		scanner := bufio.NewScanner(rr)
		for scanner.Scan() {
			var (
				resetPhase = true
				failure    string
			)
			line := scanner.Text()

			switch {
			case strings.Contains(line, "Error response from daemon:"):
				resetPhase = true
				failure = line
			case strings.Contains(line, "Web UI available"):
				prefix := "folder"
				if strings.HasSuffix(access.WorkspaceFolder, ".code-workspace") {
					prefix = "workspace"
				}
				workspaceURL = fmt.Sprintf("http://localhost:%d/?%s=%s", access.HTTPPort, prefix, access.WorkspaceFolder)

				phase = "running"
				steady = fmt.Sprintf("workspace at %s", workspaceURL)
				log.SetWorkspaceAccess(WorkspaceAccess{
					URL:     workspaceURL,
					SSHPort: access.SSHPort,
				})
			case strings.Contains(line, "Installing extensions"):
				phase = "installing extensions"
				steady = "running " + steady
			case strings.Contains(line, "IDE was stopped"):
				phase = "restarting"
				steady = "the workspace"
				failure = "IDE was stopped"
			case strings.Contains(line, "Installing extension:"):
				segs := strings.Split(line, "Installing extension:")
				extensions[segs[1]] = struct{}{}
				resetPhase = false
			case strings.Contains(line, "Downloaded extension"):
				for k := range extensions {
					if strings.Contains(line, k) {
						delete(extensions, k)
					}
				}
				if len(extensions) == 0 {
					phase = "ready"
				} else {
					resetPhase = false
				}
			case opts.ObserveTasks && strings.Contains(line, "task terminal has been started"):
				if phase != "running" && steady != "tasks" {
					phase = "running"
					steady = "tasks"
					resetPhase = true
				}

				if access.SupervisorPort > 0 {
					observeSupervisorTasks(ctx, log, access.SupervisorPort)
					if f := opts.OnTasksDone; f != nil {
						log.Debugf("all tasks are done")
						f()
					}
					return
				}
			default:
				resetPhase = false
			}

			if !resetPhase {
				continue
			}
			if failure != "" {
				if opts.OnFail != nil {
					opts.OnFail()
				}

				p.Failure(failure)
			} else {
				p.Success()
			}
			failure = ""
			p = log.StartPhase(phase, steady)
		}
	}()

	logs := log.Writer()
	return noopWriteCloser{io.MultiWriter(rw, logs)}
}

func observeSupervisorTasks(ctx context.Context, log Log, port int) {
	var (
		taskFailure []string
		phase       = log.StartPhase("running", "tasks")
		mu          sync.Mutex
	)
	defer func() {
		if len(taskFailure) == 0 {
			phase.Success()
		} else {
			phase.Failure(strings.Join(taskFailure, ". "))
		}
	}()

	conn, err := grpc.DialContext(ctx, fmt.Sprintf("localhost:%d", port), grpc.WithInsecure())
	if err != nil {
		taskFailure = []string{fmt.Sprintf("cannot connect to tasks: %v", err)}
		return
	}
	defer conn.Close()

	status := supervisor.NewStatusServiceClient(conn)
	tasks, err := status.TasksStatus(ctx, &supervisor.TasksStatusRequest{Observe: true})
	if err != nil {
		taskFailure = []string{fmt.Sprintf("cannot connect to tasks: %v", err)}
		return
	}

	observer := make(map[string]context.CancelFunc)

	tout := log.Writer()
	defer tout.Close()

	for {
		update, err := tasks.Recv()
		if err != nil {
			return
		}

		if len(update.Tasks) == 0 {
			return
		}

		var closedTaskCount int
		for _, task := range update.Tasks {
			switch task.State {
			case supervisor.TaskState_running:
				if _, ok := observer[task.Id]; ok {
					continue
				}
				tctx, cancel := context.WithCancel(ctx)
				go func(task *supervisor.TaskStatus) {
					defer log.Debugf("task %v is done", task.Id)

					success, err := observeTask(tctx, log, tout, conn, task.Presentation.Name, task.Terminal)
					if err != nil {
						log.Warnf(err.Error())
						return
					}

					mu.Lock()
					defer mu.Unlock()
					if !success {
						taskFailure = append(taskFailure, fmt.Sprintf("task %s failed", task.Presentation.Name))
					}
				}(task)
				observer[task.Id] = cancel
			case supervisor.TaskState_closed:
				cancel, ok := observer[task.Id]
				if !ok {
					continue
				}
				cancel()
				delete(observer, task.Id)
				log.Debugf("task %v is closed", task.Id)
				closedTaskCount++

				if closedTaskCount >= len(update.Tasks) {
					return
				}
			}
		}
	}
}

func observeTask(ctx context.Context, log Log, out io.Writer, con *grpc.ClientConn, name, terminalID string) (success bool, err error) {
	if name != "" {
		out = textio.NewPrefixWriter(out, fmt.Sprintf("[%s] ", name))
	}

	term := supervisor.NewTerminalServiceClient(con)
	listen, err := term.Listen(ctx, &supervisor.ListenTerminalRequest{
		Alias: terminalID,
	})
	if err != nil {
		return false, fmt.Errorf("cannot listen to task %s", name)
	}

	for {
		resp, err := listen.Recv()
		if err == io.EOF {
			return success, nil
		}
		if err != nil {
			return false, err
		}
		switch msg := resp.Output.(type) {
		case *supervisor.ListenTerminalResponse_Data:
			out.Write(msg.Data)
		case *supervisor.ListenTerminalResponse_ExitCode:
			log.Debugf("task %s exited with status %d", name, msg.ExitCode)
			return msg.ExitCode == 0, nil
		}
	}
}
