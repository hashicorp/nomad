package spawn

import "syscall"

const STILL_ACTIVE = 259

func (s *Spawner) Alive() bool {
	const da = syscall.STANDARD_RIGHTS_READ | syscall.PROCESS_QUERY_INFORMATION | syscall.SYNCHRONIZE
	h, e := syscall.OpenProcess(da, false, uint32(s.SpawnPid))
	if e != nil {
		return false
	}

	var ec uint32
	e = syscall.GetExitCodeProcess(h, &ec)
	if e != nil {
		return false
	}

	return ec == STILL_ACTIVE
}
