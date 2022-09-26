//go:build !release
// +build !release

package getter

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	clientconfig "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/stretchr/testify/require"
)

func TestDefaultGetter(t *testing.T) *Getter {
	getterConf, err := clientconfig.ArtifactConfigFromAgent(config.DefaultArtifactConfig())
	require.NoError(t, err)
	return NewGetter(hclog.NewNullLogger(), getterConf)
}
