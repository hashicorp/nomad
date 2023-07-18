//go:build linux

package proclib

// LinuxWranglerCG2 is an implementation of ProcessWrangler that leverages
// cgroups v2 on modern Linux systems.
//
// e.g. Ubuntu 22.04 / RHEL 9 and later versions.
type LinuxWranglerCG2 struct {
	parentCgroup string
}

func newCG2(c *Configs) cg2 {
	return func(task Task) ProcessWrangler {
		nlog.Info("newCG2()", "task", task)
		return &LinuxWranglerCG2{
			parentCgroup: c.ParentCgroup,
		}
	}
}

func (w *LinuxWranglerCG2) Kill() error {
	return nil
}

func (w *LinuxWranglerCG2) Cleanup() error {
	return nil
}
