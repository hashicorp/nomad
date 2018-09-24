// +build darwin dragonfly freebsd netbsd openbsd solaris windows

package executor

// resourceContainerContext is a platform-specific struct for managing a
// resource container.
type resourceContainerContext struct {
}

func (rc *resourceContainerContext) executorCleanup() error {
	return nil
}
