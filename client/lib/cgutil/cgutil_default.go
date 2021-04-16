// +build !linux

package cgutil

const (
	DefaultCgroupParent = ""
)

func InitCpusetParent(string) error { return nil }
