package nomad

import (
	"time"

	"github.com/hashicorp/go-hclog"
	"golang.org/x/exp/slices"
)

// LicenseConfig allows for tunable licensing config
// primarily used for enterprise testing
type LicenseConfig struct {
	// BuildDate is the time of the git commit used to build the program.
	BuildDate time.Time

	// LicenseEnvBytes is the license bytes to use for the server's license
	LicenseEnvBytes string

	// LicensePath is the path to use for the server's license
	LicensePath string

	// AdditionalPubKeys is a set of public keys to
	AdditionalPubKeys []string

	Logger hclog.InterceptLogger
}

func (c *LicenseConfig) Copy() *LicenseConfig {
	if c == nil {
		return nil
	}

	nc := *c
	nc.AdditionalPubKeys = slices.Clone(c.AdditionalPubKeys)
	return &nc
}
