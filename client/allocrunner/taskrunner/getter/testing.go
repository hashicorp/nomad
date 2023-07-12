//go:build !release

package getter

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	clientconfig "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/shoenig/test/must"
)

// TestDefaultGetter creates a Getter suitable for unit test cases.
func TestDefaultGetter(t *testing.T) *Getter {
	defaultConfig := config.DefaultArtifactConfig()
	defaultConfig.DecompressionSizeLimit = pointer.Of("1MB")
	defaultConfig.DecompressionFileCountLimit = pointer.Of(10)
	getterConf, err := clientconfig.ArtifactConfigFromAgent(defaultConfig)
	must.NoError(t, err)
	return NewGetter(hclog.NewNullLogger(), getterConf)
}
