// Package envoy provides a high level view of the variables that go into
// selecting an envoy version.
package envoy

import (
	"fmt"
)

const (
	// SidecarMetaParam is the parameter name used to configure the connect sidecar
	// at the client level. Setting this option in client configuration takes the
	// lowest precedence.
	//
	// If this meta option is not set in client configuration, it defaults to
	// ImageFormat, so that Nomad will defer envoy version selection to Consul.
	SidecarMetaParam = "connect.sidecar_image"

	// SidecarConfigVar is used as the default config.image value for connect
	// sidecar proxies, when they are injected in the job connect mutator.
	SidecarConfigVar = "${meta." + SidecarMetaParam + "}"

	// GatewayMetaParam is the parameter name used to configure the connect gateway
	// at the client level. Setting this option in client configuration takes the
	// lowest precedence.
	//
	// If this meta option is not set in client configuration, it defaults to
	// ImageFormat, so that Nomad will defer envoy version selection to Consul.
	GatewayMetaParam = "connect.gateway_image"

	// GatewayConfigVar is used as the default config.image value for connect
	// gateway proxies, when they are injected in the job connect mutator.
	GatewayConfigVar = "${meta." + GatewayMetaParam + "}"

	// ImageFormat is the default format string used for official envoy Docker
	// images with the tag being the semver of the version of envoy. Nomad fakes
	// interpolation of ${NOMAD_envoy_version} by replacing it with the version
	// string for envoy that Consul reports as preferred.
	//
	// Folks wanting to build and use custom images while still having Nomad refer
	// to specific versions as preferred by Consul would set meta.connect.sidecar_image
	// to something like: "custom/envoy:${NOMAD_envoy_version}".
	ImageFormat = "envoyproxy/envoy:v" + VersionVar

	// VersionVar will be replaced with the Envoy version string when
	// used in the meta.connect.sidecar_image variable.
	VersionVar = "${NOMAD_envoy_version}"

	// FallbackImage is the image set in the node meta by default
	// to be used by Consul Connect sidecar tasks. As of Nomad 1.0, this value
	// is only used as a fallback when the version of Consul does not yet support
	// dynamic envoy versions.
	FallbackImage = "envoyproxy/envoy:v1.11.2@sha256:a7769160c9c1a55bb8d07a3b71ce5d64f72b1f665f10d81aa1581bc3cf850d09"
)

// PortLabel creates a consistent port label using the inputs of a prefix,
// service name, and optional suffix. The prefix should be the Kind part of
// TaskKind the envoy is being configured for.
func PortLabel(prefix, service, suffix string) string {
	if suffix == "" {
		return fmt.Sprintf("%s-%s", prefix, service)
	}
	return fmt.Sprintf("%s-%s-%s", prefix, service, suffix)
}
