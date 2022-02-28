//go:build !linux

package docker

type CpusetFixer interface {
	Start()
}

func newCpusetFixer(*Driver) CpusetFixer {
	return new(noop)
}

type noop struct {
	// empty
}

func (*noop) Start() {
	// empty
}
