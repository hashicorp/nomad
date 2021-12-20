//go:build darwin || freebsd || netbsd || openbsd
// +build darwin freebsd netbsd openbsd

package agent

import (
	"os"
	"syscall"
	"time"
)

func (l *logFile) createTime(stat os.FileInfo) time.Time {
	stat_t := stat.Sys().(*syscall.Stat_t)
	createTime := stat_t.Ctimespec
	return time.Unix(createTime.Sec, createTime.Nsec)
}
