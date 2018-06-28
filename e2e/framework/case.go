package framework

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestSuite struct {
	Component string

	CanRunLocal bool
	Cases       []TestCase
	Constraints Constraints
	Parallel    bool
	Slow        bool
}

type Constraints struct {
	CloudProvider string
	OS            string
	Arch          string
	Environment   string
	Tags          []string
}

func (c Constraints) matches(env Environment) error {
	if len(c.CloudProvider) != 0 && c.CloudProvider != env.Provider {
		return fmt.Errorf("provider constraint does not match environment")
	}

	if len(c.OS) != 0 && c.OS != env.OS {
		return fmt.Errorf("os constraint does not match environment")
	}

	if len(c.Arch) != 0 && c.Arch != env.Arch {
		return fmt.Errorf("arch constraint does not match environment")
	}

	if len(c.Environment) != 0 && c.Environment != env.Name {
		return fmt.Errorf("environment constraint does not match environment name")
	}

	for _, t := range c.Tags {
		if _, ok := env.Tags[t]; !ok {
			return fmt.Errorf("tags constraint failed, tag '%s' is not included in environment")
		}
	}
	return nil
}

type TC struct {
	*assert.Assertions
	require *require.Assertions
	t       *testing.T

	cluster *ClusterInfo
	prefix  string
	name    string
}

func (tc *TC) Nomad() *api.Client {
	return tc.cluster.NomadClient
}

func (tc *TC) Prefix() string {
	return fmt.Sprintf("%s-", tc.cluster.ID)
}

func (tc *TC) Name() string {
	return tc.cluster.Name
}

func (tc *TC) T() *testing.T {
	return tc.t
}

func (tc *TC) SetT(t *testing.T) {
	tc.t = t
	tc.Assertions = assert.New(t)
	tc.require = require.New(t)
}

func (tc *TC) setClusterInfo(info *ClusterInfo) {
	tc.cluster = info
}
