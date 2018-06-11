// +build ent

package version

func init() {
	// Metadata specifies the type of binary other than the default open-source
	// version, such as "ent", "pro", etc.
	VersionMetadata = "ent"
}
