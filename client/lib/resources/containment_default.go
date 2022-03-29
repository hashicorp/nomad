//go:build !linux

package resources

type containment struct {
	// non-linux executors currently do not create resources to be cleaned up
}

func (c *containment) Cleanup() error {
	return nil
}
