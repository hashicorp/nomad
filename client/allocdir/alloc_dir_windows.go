package allocdir

import "errors"

func (d *AllocDir) linkOrCopy(src, dst string) error {
	return fileCopy(src, dst)
}

// The windows version does nothing currently.
func (d *AllocDir) mountSharedDir(taskDir string) error {
	return errors.New("Mount on Windows not supported.")
}
