// These functions are coming from consul/path.go
package helper

import (
	"os"
	"path/filepath"
)

// EnsurePath is used to make sure a path exists
func EnsurePath(path string, dir bool) error {
	if !dir {
		path = filepath.Dir(path)
	}
	return os.MkdirAll(path, 0755)
}
