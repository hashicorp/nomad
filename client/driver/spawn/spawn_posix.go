// +build !windows

package spawn

import "syscall"

func (s *Spawner) Alive() bool {
	if s.spawn == nil {
		return false
	}

	err := s.spawn.Signal(syscall.Signal(0))
	return err == nil
}
