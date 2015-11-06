// +build !windows

package spawn

import (
	"os"
	"syscall"
)

func (s *Spawner) Alive() bool {
	if s.spawn == nil {
		var err error
		if s.spawn, err = os.FindProcess(s.SpawnPid); err != nil {
			return false
		}
	}

	err := s.spawn.Signal(syscall.Signal(0))
	return err == nil
}
