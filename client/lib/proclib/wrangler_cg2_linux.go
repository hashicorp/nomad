//go:build linux

package proclib

type LinuxWranglerCG2 struct {
}

func New() *LinuxWranglerCG2 {
	return &LinuxWranglerCG2{}
}

func (w *LinuxWranglerCG2) Kill() error {
	return nil
}

func (w *LinuxWranglerCG2) Cleanup() error {
	return nil
}

func (w *LinuxWranglerCG2) SetAttributes(map[string]string) {
	//
}
