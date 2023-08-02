//go:build !linux

package cgroupslib

// LinuxResourcesPath does nothing on non-Linux systems
func LinuxResourcesPath(string, string) string {
	return ""
}
