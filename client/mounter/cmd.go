// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mounter

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/hashicorp/go-hclog"
)

const defaultUDSPath = "/var/run/nomad-mounter.sock"
const defaultUser = "nomad"

// This init() must be initialized last in package, so to avoid conflicts, don't
// include any other init() in this package. See eeaa95d commit message for more
// details.
func init() {
	if len(os.Args) > 1 && os.Args[1] == "mounter" {

		var udsPath, user string
		flag.StringVar(&udsPath, "socket", defaultUDSPath, "path to unix domain socket listener")
		flag.StringVar(&user, "user", defaultUser, "nomad agent's user")
		flag.Parse()

		logger := hclog.New(&hclog.LoggerOptions{
			Level:      hclog.Trace,
			JSONFormat: true,
			Name:       "mounter",
		})

		srv, err := NewMounterServer(logger, udsPath, user)
		if err != nil {
			logger.Error("could not start mounter", "error", err)
			os.Exit(1)
		}

		logger.Info("starting mounter", "socket", udsPath, "user", user)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			signalCh := make(chan os.Signal, 1)
			signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGPIPE)
			<-signalCh
			cancel()
			logger.Info("shutting down")
		}()

		err = srv.Run(ctx) // blocks until exit
		if err != nil && err != ctx.Err() {
			logger.Error("mounter got fatal error", "error", err)
			os.Exit(1)
		}

		os.Exit(0)
	}
}
