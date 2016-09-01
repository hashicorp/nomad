package allocdir

import "io/ioutil"

// TestCreateSecretDirFn is used to create a secret dir suitable for testing
func TestCreateSecretDirFn(_, _ string) (string, error) {
	return ioutil.TempDir("", "")
}
