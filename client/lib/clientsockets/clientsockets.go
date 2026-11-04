// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

// Package clientsockets creates the unix sockets that tasks connect to: the
// Task API socket and the Consul HTTP and gRPC sockets. With SELinux, the
// sockets can carry the container process type so that confined containers
// may connect to them.
package clientsockets

import (
	"net"
	"os"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/users"
)

// SocketFileFor creates a unix domain socket file on the specified path and,
// if possible, makes it usable by only the specified user. Failing that it
// will leave the socket open to all users. Non-fatal errors are logged. With
// selinuxLabel set, confined containers can connect to it (see ListenTaskSocket).
//
// See users.WriteFileFor if writing a regular file.
func SocketFileFor(logger hclog.Logger, path, username string, selinuxLabel bool) (net.Listener, error) {
	if err := os.RemoveAll(path); err != nil {
		logger.Warn("error removing socket", "path", path, "error", err)
	}

	udsln, err := ListenTaskSocket(path, selinuxLabel)
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
