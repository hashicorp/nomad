// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package winsvc

import (
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
)

// Commands that are currently supported
const SERVICE_ACCEPTED_COMMANDS = svc.AcceptStop | svc.AcceptShutdown

type serviceWindows struct {
	evtLog Eventlog
}

func init() {
	isSvc, err := svc.IsWindowsService()
	if err != nil {
		panic(err)
	}
	// This should only run when running
	// as a service
	if !isSvc {
		return
	}

	go executeWindowsService()
}

// Execute implements the Windows service Handler type. It will be
// called at the start of the service, and the service will exit
// once Execute completes.
func (srv serviceWindows) Execute(args []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	const accCommands = svc.AcceptStop | svc.AcceptShutdown
	s <- svc.Status{State: svc.Running, Accepts: SERVICE_ACCEPTED_COMMANDS}
	srv.evtLog.Info(uint32(EventServiceStarting), "service starting")
LOOP:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				s <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				srv.evtLog.Info(uint32(EventLogMessage), "service stop requested")
				s <- svc.Status{State: svc.StopPending}
				chanGraceExit <- 1
			}
		case e := <-chanEvents:
			switch e.Level() {
			case EVENTLOG_LEVEL_INFO:
				srv.evtLog.Info(uint32(e.Kind()), e.Message())
			case EVENTLOG_LEVEL_WARN:
				srv.evtLog.Warning(uint32(e.Kind()), e.Message())
			case EVENTLOG_LEVEL_ERROR:
				srv.evtLog.Error(uint32(e.Kind()), e.Message())
			}

			if e.Kind() == EventServiceStopped {
				break LOOP
			}
		}
	}

	return false, 0
}

func executeWindowsService() {
	var evtLog Eventlog
	evtLog, err := eventlog.Open(WINDOWS_SERVICE_NAME)
	if err != nil {
		// Eventlog will only be available if the
		// service was properly registered. If the
		// service was manually setup, it will likely
		// not have been registered with the eventlog
		// so it will not be available. In that case
		// just stub out the eventlog.
		evtLog = &nullEventlog{}
	}
	defer evtLog.Close()

	svc.Run(WINDOWS_SERVICE_NAME, serviceWindows{evtLog: evtLog})
}
