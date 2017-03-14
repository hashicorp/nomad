// +build !nomad_test

package discover

func isNomad(path, nomadExe string) bool {
	return true
}
