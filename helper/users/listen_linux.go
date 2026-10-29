// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package users

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
	socketLabel, err := TaskSocketLabel()
	if err != nil || socketLabel == "" {
		// The client warns at start when the label cannot be found.
		return net.Listen("unix", path)
	}
	return listenLabeled(path, socketLabel)
}

type listenResult struct {
	ln  net.Listener
	err error
}

// listenLabeled creates the socket on a goroutine locked to its thread, because
// the socket creation label is a per-thread attribute. A thread whose label
// could not be reset stays locked to the goroutine and exits with it, so no
// other socket is created with the label.
func listenLabeled(path, socketLabel string) (net.Listener, error) {
	result := make(chan listenResult, 1)
	go func() {
		runtime.LockOSThread()
		ln, reset, err := listenWithSocketLabel(path, socketLabel)
		if reset {
			runtime.UnlockOSThread()
		}
		result <- listenResult{ln: ln, err: err}
	}()
	r := <-result
	return r.ln, r.err
}

// listenWithSocketLabel creates the socket with socketLabel on the calling
// thread, which the caller must have locked, and resets the thread's socket
// creation label. It reports whether the thread is left without the label.
func listenWithSocketLabel(path, socketLabel string) (net.Listener, bool, error) {
	if err := selinux.SetSocketLabel(socketLabel); err != nil {
		return nil, true, fmt.Errorf("unable to set SELinux label %q for socket: %w", socketLabel, err)
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		err = fmt.Errorf("unable to listen with SELinux label %q: %w", socketLabel, err)
	}
	return ln, selinux.SetSocketLabel("") == nil, err
}

var (
	taskSocketLabelLock sync.Mutex
	taskSocketLabel     string
)

// TaskSocketLabel returns the SELinux label task sockets are created with: the
// container process type from the host's container contexts, at level s0 when
// the policy has levels. It returns an empty label when SELinux is disabled.
func TaskSocketLabel() (string, error) {
	taskSocketLabelLock.Lock()
	defer taskSocketLabelLock.Unlock()

	if taskSocketLabel != "" || !selinux.GetEnabled() {
		return taskSocketLabel, nil
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
	taskSocketLabel = ctx.Get()
	return taskSocketLabel, nil
}
