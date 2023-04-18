//go:build dragonfly || linux || solaris
// +build dragonfly linux solaris

package agent

import (
	"os"
	"syscall"
	"time"
)

func (l *logFile) createTime(stat os.FileInfo) time.Time {
	stat_t := stat.Sys().(*syscall.Stat_t)
	createTime := stat_t.Ctim
	// Sec and Nsec are int32 in 32-bit architectures.
	return time.Unix(int64(createTime.Sec), int64(createTime.Nsec)) //nolint:unconvert
}
