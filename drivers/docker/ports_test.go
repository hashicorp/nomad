package docker

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/helper/testlog"
)

func TestPublishedPorts_add(t *testing.T) {
	p := newPublishedPorts(testlog.HCLogger(t))
	p.add("label", "10.0.0.1", 1234, 80)
	p.add("label", "10.0.0.1", 5678, 80)
	for _, bindings := range p.publishedPorts {
		require.Len(t, bindings, 2)
	}
	require.Len(t, p.exposedPorts, 2)
}
