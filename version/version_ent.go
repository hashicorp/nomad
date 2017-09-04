// +build ent

package version

func init() {
	// The main version number that is being run at the moment.
	Version = "0.7.0"

	// A pre-release marker for the version. If this is "" (empty string)
	// then it means that it is a final release. Otherwise, this is a pre-release
	// such as "dev" (in development), "beta", "rc1", etc.
	VersionPrerelease = "dev"

	// Metadata specifies the type of binary other than the default open-source
	// version, such as "ent", "pro", etc.
	VersionMetadata = "ent"
}
