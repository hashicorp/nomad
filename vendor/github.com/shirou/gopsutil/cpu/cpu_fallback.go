// +build !darwin,!linux,!freebsd,!openbsd,!solaris,!windows

package cpu

import (
	"github.com/shirou/gopsutil/internal/common"
)

func Times(percpu bool) ([]TimesStat, error) {
	return []TimesStat{}, common.ErrNotImplementedError
}

func Info() ([]InfoStat, error) {
	return []InfoStat{}, common.ErrNotImplementedError
}
