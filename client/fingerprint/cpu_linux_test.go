package fingerprint

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/nomad/client/config"
	"github.com/stretchr/testify/require"
)

func Test_deriveCpuset(t *testing.T) {

	c, e := deriveCpuset(&FingerprintRequest{Config: &config.Config{CgroupParent: "/"}})
	require.NoError(t, e)
	spew.Dump(c)
}
