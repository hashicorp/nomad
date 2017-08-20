package testutil

var (
	// NomadTest marks whether the build has "nomad_test" build flag
	NomadTest = false
)

func IsNomadTest() bool {
	return NomadTest
}
