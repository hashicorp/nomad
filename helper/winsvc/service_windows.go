//+build windows

package winsvc

import (
	wsvc "golang.org/x/sys/windows/svc"
)

type serviceWindows struct{}

func init() {
	interactive, err := wsvc.IsAnInteractiveSession()
	if err != nil {
		panic(err)
	}
	// Cannot run as a service when running interactively
	if interactive {
		return
	}
	go func() {
		_ = wsvc.Run("", serviceWindows{})
	}()
}

func (serviceWindows) Execute(args []string, r <-chan wsvc.ChangeRequest, s chan<- wsvc.Status) (svcSpecificEC bool, exitCode uint32) {
	const accCommands = wsvc.AcceptStop | wsvc.AcceptShutdown
	s <- wsvc.Status{State: wsvc.Running, Accepts: accCommands}
	for {
		c := <-r
		switch c.Cmd {
		case wsvc.Interrogate:
			s <- c.CurrentStatus
		case wsvc.Stop, wsvc.Shutdown:
			s <- wsvc.Status{State: wsvc.StopPending}
			chanGraceExit <- 1
			return false, 0
		}
	}

	return false, 0
}
