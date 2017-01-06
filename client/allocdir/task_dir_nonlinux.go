// +build !linux

package allocdir

// currently a noop on non-Linux platforms
func (d *TaskDir) mountSpecialDirs() error {
	return nil
}

// currently a noop on non-Linux platforms
func (d *TaskDir) unmountSpecialDirs() error {
	return nil
}
