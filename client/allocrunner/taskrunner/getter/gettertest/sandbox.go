package gettertest

import (
	"testing"

	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/getter"
	cconfig "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/testlog"
	sconfig "github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/shoenig/test/must"
)

// NewSandbox creates a real artifact downloader configured via the default
// artifact config.
func NewSandbox(t *testing.T) getter.Sandbox {
	ac, err := cconfig.ArtifactConfigFromAgent(sconfig.DefaultArtifactConfig())
	must.NoError(t, err)
	return getter.New(ac, testlog.HCLogger(t))
}
