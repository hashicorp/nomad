// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

// Package clientsockets creates the unix sockets that tasks connect to
package clientsockets

import (
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/users"
	"github.com/hashicorp/nomad/plugins/drivers/fsisolation"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/opencontainers/selinux/go-selinux/label"
)

// labeler is the goroutine that creates labeled sockets, or nil when confined
// tasks get plain sockets. ListenTaskSocket and Fingerprint read it; only
// Configure writes it.
var labeler atomic.Pointer[socketLabeler]

// configureMu serializes Configure: the client package starts many clients in
// parallel tests and Reload may overlap with start.
var configureMu sync.Mutex

// Confined reports whether tasks with this filesystem isolation run as
// confined containers, whose sockets need the container SELinux type when
// labeling is on. Only image isolation (Docker, podman, containerd) does; exec,
// raw_exec, java and qemu run tasks in the agent's own domain, which can
// already connect to the agent's sockets.
func Confined(mode fsisolation.Mode) bool {
	return mode == fsisolation.Image
}

// Configure selects how sockets for confined tasks are created in later calls
// to ListenTaskSocket or SocketFileFor. With label=false, or when SELinux is
// disabled, confined tasks get plain sockets. With label=true the container
// process label is looked up from the host policy and a goroutine locked to its
// thread is started with that label set, so every socket it creates carries the
// label. On error we fall back to unconfined sockets.
//
// The client calls this once at start, before fingerprinting, and again from
// Reload. On reload sockets already created keep the label they were created
// with.
func Configure(label bool) error {
	configureMu.Lock()
	defer configureMu.Unlock()

	if !label {
		setLabeler(nil)
		return nil
	}
	socketLabel, err := findTaskSocketLabel()
	if err != nil || socketLabel == "" {
		setLabeler(nil)
		return err
	}
	if l := labeler.Load(); l != nil && l.label == socketLabel {
		return nil
	}
	l, err := startSocketLabeler(socketLabel)
	if err != nil {
		setLabeler(nil)
		return err
	}
	setLabeler(l)
	return nil
}

// setLabeler makes l the labeler and stops the one it replaces, whether by a
// new labeler or by nil, so no locked thread is left behind. A labeler never
// changes its label, so its thread is not reused for anything else.
func setLabeler(l *socketLabeler) {
	if old := labeler.Swap(l); old != nil {
		close(old.done)
	}
}

// Fingerprint reports the label sockets of confined tasks are created with and
// whether labeling is active, for the selinux fingerprinter.
func Fingerprint() (string, bool) {
	if l := labeler.Load(); l != nil {
		return l.label, true
	}
	return "", false
}

// ListenTaskSocket creates the unix socket at path. If confined is true (see
// Confined) and Configure enabled labeling, the labeler creates it. Otherwise
// it is a plain net.Listen.
func ListenTaskSocket(path string, confined bool) (net.Listener, error) {
	if l := labeler.Load(); confined && l != nil {
		return l.listen(path)
	}
	return net.Listen("unix", path)
}

// SocketFileFor creates the unix socket at path with ListenTaskSocket, which
// also defines confined, and makes it usable only by username. Where that is
// not possible, as on Windows or with an agent that is not root, it makes the
// socket usable by all users instead, so the task can still connect. Non-fatal
// errors are logged.
//
// See users.WriteFileFor if writing a regular file.
func SocketFileFor(logger hclog.Logger, path, username string, confined bool) (net.Listener, error) {
	if err := os.RemoveAll(path); err != nil {
		logger.Warn("error removing socket", "path", path, "error", err)
	}

	udsln, err := ListenTaskSocket(path, confined)
	if err != nil {
		return nil, err
	}

	if username != "" {
		// Try to set perms on socket file to least privileges.
		if err := setSocketOwner(path, username); err == nil {
			// Success! Exit early
			return udsln, nil
		}

		// This error is expected to always occur in some environments (Windows,
		// non-root agents), so don't log above Trace.
		logger.Trace("failed to set user on socket", "path", path, "user", username, "error", err)
	}

	// Opportunistic least privileges failed above, so make sure anyone can use
	// the socket.
	if err := os.Chmod(path, 0o666); err != nil {
		logger.Warn("error setting socket permissions", "path", path, "error", err)
	}

	return udsln, nil
}

func setSocketOwner(path, username string) error {
	uid, _, _, err := users.LookupUnix(username)
	if err != nil {
		return err
	}

	if err := os.Chown(path, uid, -1); err != nil {
		return err
	}

	if err := os.Chmod(path, 0o600); err != nil {
		// Awkward situation that is hopefully impossible to reach where we could
		// chown the socket but not change its mode.
		return err
	}

	return nil
}

// findTaskSocketLabel returns the label sockets of confined tasks are created
// with: the container process type from the host's container contexts, at level
// s0 when the policy has levels. It returns an empty label when SELinux is
// disabled, which it always is on platforms other than Linux. go-selinux reads
// the container contexts once per process, so a changed container type, as from
// a container-selinux update, takes effect only after an agent restart.
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

// socketLabeler creates sockets with its label. Under SELinux a unix socket
// carries the label of the process that created it, and a confined container
// may not connect to a socket of the agent's domain. The socket creation label
// is a per-thread attribute, so all sockets of a labeler are created by one
// goroutine that stays locked to its thread with the label set once. No other
// goroutine runs on that thread, so no other socket is created with the label.
type socketLabeler struct {
	label    string
	requests chan request

	// done is closed when Configure replaces the labeler. requests is never
	// closed, as ListenTaskSocket may still send on it after the swap.
	done chan struct{}
}

// errStopped is returned for a socket requested from a labeler that Configure
// replaced while the request was on its way.
var errStopped = errors.New("socket labeler stopped")

type request struct {
	path   string
	result chan<- result
}

type result struct {
	ln  net.Listener
	err error
}

// startSocketLabeler starts the labeler goroutine and waits until it has set
// the label on its thread, so that a label the kernel rejects is returned here.
func startSocketLabeler(socketLabel string) (*socketLabeler, error) {
	l := &socketLabeler{
		label:    socketLabel,
		requests: make(chan request),
		done:     make(chan struct{}),
	}
	started := make(chan error, 1)
	go l.run(started)
	if err := <-started; err != nil {
		return nil, err
	}
	return l, nil
}

// run locks the goroutine to its thread and never unlocks it. When the label
// cannot be set or the labeler is stopped, the goroutine exits while still
// locked, so the runtime discards the thread instead of reusing it.
func (l *socketLabeler) run(started chan<- error) {
	runtime.LockOSThread()
	if err := selinux.SetSocketLabel(l.label); err != nil {
		started <- fmt.Errorf("unable to set SELinux label %q for sockets: %w", l.label, err)
		return
	}
	started <- nil

	for {
		select {
		case req := <-l.requests:
			ln, err := net.Listen("unix", req.path)
			if err != nil {
				err = fmt.Errorf("unable to listen with SELinux label %q: %w", l.label, err)
			}
			req.result <- result{ln: ln, err: err}
		case <-l.done:
			return
		}
	}
}

// listen asks the labeler goroutine to create the socket at path and waits for
// the result. It is safe for concurrent use. If Configure stopped the labeler
// before it took the request, listen returns an error instead of blocking.
func (l *socketLabeler) listen(path string) (net.Listener, error) {
	res := make(chan result, 1)
	select {
	case l.requests <- request{path: path, result: res}:
	case <-l.done:
		return nil, errStopped
	}
	r := <-res
	return r.ln, r.err
}
