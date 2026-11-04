// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package clientsockets

import (
	"errors"
	"fmt"
	"net"
	"runtime"
	"sync"

	"github.com/opencontainers/selinux/go-selinux"
	"github.com/opencontainers/selinux/go-selinux/label"
)

// ListenTaskSocket creates a unix socket that tasks connect to. Under SELinux a
// unix socket carries the label of the process that created it, and a confined
// container may not connect to a socket of the Nomad agent's domain. With
// selinuxLabel set, the socket is therefore created with the label from
// TaskSocketLabel, which container policy allows containers of any category to
// connect to. Without SELinux or container policy it is a plain socket.
func ListenTaskSocket(path string, selinuxLabel bool) (net.Listener, error) {
	if !selinuxLabel {
		return net.Listen("unix", path)
	}
	l, err := labeledSocketListener()
	if err != nil || l == nil {
		// The client warns at start when StartTaskSocketLabeler fails.
		return net.Listen("unix", path)
	}
	return l.listen(path)
}

// StartTaskSocketLabeler starts the thread that creates labeled task sockets.
// The client calls it at start, so a label that cannot be found or set is
// reported then instead of when the first task socket falls back to a plain
// socket. It returns nil when SELinux is disabled.
func StartTaskSocketLabeler() error {
	_, err := labeledSocketListener()
	return err
}

// taskSocketLabel looks up the label once. The label comes from the host's
// SELinux policy, not from the agent's configuration, so it does not change
// when the configuration is reloaded.
var taskSocketLabel = sync.OnceValues(findTaskSocketLabel)

// TaskSocketLabel returns the SELinux label task sockets are created with: the
// container process type from the host's container contexts, at level s0 when
// the policy has levels. It returns an empty label when SELinux is disabled.
func TaskSocketLabel() (string, error) {
	return taskSocketLabel()
}

func findTaskSocketLabel() (string, error) {
	if !selinux.GetEnabled() {
		return "", nil
	}

	processLabel, _, err := label.InitLabels(nil)
	if err != nil {
		return "", fmt.Errorf("unable to find the container process label: %w", err)
	}
	if processLabel == "" {
		return "", errors.New("no container process label in the SELinux policy")
	}
	selinux.ReleaseLabel(processLabel)

	ctx, err := selinux.NewContext(processLabel)
	if err != nil {
		return "", fmt.Errorf("unable to parse the container process label %q: %w", processLabel, err)
	}
	if ctx["level"] != "" {
		ctx["level"] = "s0"
	}
	return ctx.Get(), nil
}

// labeledSocketListener starts the socketLabeler once, from
// StartTaskSocketLabeler at client start or else from the first labeled socket.
// It returns nil without an error when SELinux is disabled.
var labeledSocketListener = sync.OnceValues(startSocketLabeler)

// socketLabeler creates sockets with the task socket label. The socket creation
// label is a per-thread attribute, so all labeled sockets are created by one
// goroutine that stays locked to its thread for the life of the agent, with
// the label set once. No other goroutine runs on that thread, so no other
// socket is created with the label.
type socketLabeler struct {
	requests chan listenRequest
}

type listenRequest struct {
	path   string
	result chan<- listenResult
}

type listenResult struct {
	ln  net.Listener
	err error
}

// startSocketLabeler starts the labeler goroutine and waits until it has set
// the label on its thread, so that a label the kernel rejects is returned here.
func startSocketLabeler() (*socketLabeler, error) {
	socketLabel, err := TaskSocketLabel()
	if err != nil || socketLabel == "" {
		return nil, err
	}

	l := &socketLabeler{requests: make(chan listenRequest)}
	started := make(chan error, 1)
	go l.run(socketLabel, started)
	if err := <-started; err != nil {
		return nil, err
	}
	return l, nil
}

// run locks the goroutine to its thread and never unlocks it. If the label
// cannot be set, the goroutine exits while still locked, so the runtime
// discards the thread instead of reusing it.
func (l *socketLabeler) run(socketLabel string, started chan<- error) {
	runtime.LockOSThread()
	if err := selinux.SetSocketLabel(socketLabel); err != nil {
		started <- fmt.Errorf("unable to set SELinux label %q for sockets: %w", socketLabel, err)
		return
	}
	started <- nil

	for req := range l.requests {
		ln, err := net.Listen("unix", req.path)
		if err != nil {
			err = fmt.Errorf("unable to listen with SELinux label %q: %w", socketLabel, err)
		}
		req.result <- listenResult{ln: ln, err: err}
	}
}

// listen asks the labeler goroutine to create the socket at path and waits for
// the result. It is safe for concurrent use.
func (l *socketLabeler) listen(path string) (net.Listener, error) {
	result := make(chan listenResult, 1)
	l.requests <- listenRequest{path: path, result: result}
	r := <-result
	return r.ln, r.err
}
