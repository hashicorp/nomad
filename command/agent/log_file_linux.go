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
	return time.Unix(createTime.Sec, createTime.Nsec)
}
