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

	ver, vErr := version.NewVersion(v)
	if vErr != nil {
		return "", false
	}
	if strings.Contains(ver.Metadata(), "ent") {
		return "ent", true
	}
	return "oss", true
}

func Namespaces(info Self) (string, bool) {
	return feature("Namespaces", info)
}

// Feature returns whether the indicated feature is enabled by Consul and the
// associated License.
// possible values as of v1.9.5+ent:
//   Automated Backups, Automated Upgrades, Enhanced Read Scalability,
//   Network Segments, Redundancy Zone, Advanced Network Federation,
//   Namespaces, SSO, Audit Logging
func feature(name string, info Self) (string, bool) {
	lic, licOK := info["Stats"]["license"].(map[string]interface{})
	if !licOK {
		return "", false
	}

	features, exists := lic["features"].(string)
	if !exists {
		return "", false
	}

	if !strings.Contains(features, name) {
		return "", false
	}

	return "true", true
}
