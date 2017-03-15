// +build !darwin,!linux,!freebsd,!openbsd,!solaris,!windows

package mem

import "github.com/shirou/gopsutil/internal/common"

func VirtualMemory() (*VirtualMemoryStat, error) {
	return nil, common.ErrNotImplementedError
}

func SwapMemory() (*SwapMemoryStat, error) {
	return nil, common.ErrNotImplementedError
}
