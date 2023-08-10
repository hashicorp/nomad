// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"strings"

	"github.com/hashicorp/go-version"
)

// Self represents the response body from Consul /v1/agent/self API endpoint.
// Care must always be taken to do type checks when casting, as structure could
// potentially change over time.
type Self = map[string]map[string]interface{}

func SKU(info Self) (string, bool) {
	v, ok := info["Config"]["Version"].(string)
	if !ok {
		return "", ok
	}

	ver, vErr := version.NewVersion(strings.TrimSpace(v))
	if vErr != nil {
		return "", false
	}
	if strings.Contains(ver.Metadata(), "ent") {
		return "ent", true
	}
	return "oss", true
}

// Namespaces returns true if the "Namespaces" feature is enabled in Consul, and
// false otherwise. Consul OSS will always return false, and Consul ENT will return
// false if the license file does not contain the necessary feature.
func Namespaces(info Self) bool {
	return feature("Namespaces", info)
}

// feature returns whether the indicated feature is enabled by Consul and the
// associated License.
// possible values as of v1.9.5+ent:
//
//	Automated Backups, Automated Upgrades, Enhanced Read Scalability,
//	Network Segments, Redundancy Zone, Advanced Network Federation,
//	Namespaces, SSO, Audit Logging
func feature(name string, info Self) bool {
	lic, licOK := info["Stats"]["license"].(map[string]interface{})
	if !licOK {
		return false
	}

	features, exists := lic["features"].(string)
	if !exists {
		return false
	}

	return strings.Contains(features, name)
}
