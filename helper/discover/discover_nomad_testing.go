// +build nomad_test

package discover

import "path/filepath"

// When running tests we do not want to return the test binary that is running
// so we filter it and force discovery via the other mechanisms. Any test that
// fork/execs Nomad should be run with `-tags nomad_test`.
func isNomad(path, nomadExe string) bool {
	if filepath.Base(path) == nomadExe {
		return true
	}

	return false
}
