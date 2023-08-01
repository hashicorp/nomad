//go:build darwin
// +build darwin

package host

func mountedPaths() []string {
	return []string{"/"}
}
