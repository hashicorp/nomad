//go:build linux

package proclib

// LinuxWranglerCG1 is an implementation of ProcessWrangler that leverages
// cgroups v1 on older Linux systems.
//
// e.g. Ubuntu 20.04 / RHEL 8 and previous versions.
type LinuxWranglerCG1 struct {
	parentCgroup string
}

func newCG1(c *Configs) ProcessWrangler {
	return func(task Task) ProcessWrangler {
		nlog.Info("newCG1()", "task", task)
		return &LinuxWranglerCG1{
			parentCgroup: c.ParentCgroup,
		}
	}
}

func (w *LinuxWranglerCG1) Kill() error {
	return nil
}

func (w *LinuxWranglerCG1) Cleanup() error {
	return nil
}
