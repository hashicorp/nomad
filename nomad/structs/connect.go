package structs

const (
	// envoyImageFormat is the default format string used for official envoy Docker
	// images with the tag being the semver of the version of envoy. Nomad fakes
	// interpolation of ${NOMAD_envoy_version} by replacing it with the version
	// string for envoy that Consul reports as preferred.
	//
	// Folks wanting to build and use custom images while still having Nomad refer
	// to specific versions as preferred by Consul would set meta.connect.sidecar_image
	// to something like: "custom/envoy:${NOMAD_envoy_version}".
	EnvoyImageFormat = "envoyproxy/envoy:v" + EnvoyVersionVar

	// envoyVersionVar will be replaced with the Envoy version string when
	// used in the meta.connect.sidecar_image variable.
	EnvoyVersionVar = "${NOMAD_envoy_version}"
)
