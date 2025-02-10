// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package agent

import (
	"io"
	"net"
	"os"
	"time"
)

const sdNotifySocketEnvVar = "NOTIFY_SOCKET"

// openNotify opens the systemd notify socket only if the expected env var has
// been set, because the systemd unit file is Type=notify or Type=notify-reload
// (systemd 253+). It then unsets the env var in the agent process so that child
// processes can't accidentally inherit it. This function returns (nil, nil) if
// the env var isn't set.
func openNotify() (io.WriteCloser, error) {
	socketPath := os.Getenv(sdNotifySocketEnvVar)
	if socketPath == "" {
		return nil, nil
	}

	defer os.Unsetenv(sdNotifySocketEnvVar)
	conn, err := net.DialTimeout("unixgram", socketPath, time.Second)
	return conn, err
}

// sdNotify sends the message on the systemd notify socket, and gracefully
// handles a nil socket
func sdNotify(w io.Writer, msg string) {
	if w == nil || msg == "" {
		return
	}
	w.Write([]byte(msg))
}
