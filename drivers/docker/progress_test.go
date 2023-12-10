// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
)

func Test_DockerImageProgressManager(t *testing.T) {
	ci.Parallel(t)

	pm := &imageProgressManager{
		imageProgress: &imageProgress{
			timestamp: time.Now(),
			layers:    make(map[string]*layerProgress),
		},
	}

	_, err := pm.Write([]byte(`{"status":"Pulling from library/golang","id":"1.9.5"}
{"status":"Pulling fs layer","progressDetail":{},"id":"c73ab1c6897b"}
{"status":"Pulling fs layer","progressDetail":{},"id":"1ab373b3deae"}
`))
	require.NoError(t, err)
	require.Equal(t, 2, len(pm.imageProgress.layers), "number of layers should be 2")

	cur := pm.imageProgress.currentBytes()
	require.Zero(t, cur)
	tot := pm.imageProgress.totalBytes()
	require.Zero(t, tot)

	_, err = pm.Write([]byte(`{"status":"Pulling fs layer","progress`))
	require.NoError(t, err)
	require.Equal(t, 2, len(pm.imageProgress.layers), "number of layers should be 2")

	_, err = pm.Write([]byte(`Detail":{},"id":"b542772b4177"}` + "\n"))
	require.NoError(t, err)
	require.Equal(t, 3, len(pm.imageProgress.layers), "number of layers should be 3")

	_, err = pm.Write([]byte(`{"status":"Downloading","progressDetail":{"current":45800,"total":4335495},"progress":"[\u003e                                                  ]   45.8kB/4.335MB","id":"b542772b4177"}
{"status":"Downloading","progressDetail":{"current":113576,"total":11108010},"progress":"[\u003e                                                  ]  113.6kB/11.11MB","id":"1ab373b3deae"}
{"status":"Downloading","progressDetail":{"current":694257,"total":4335495},"progress":"[========\u003e                                          ]  694.3kB/4.335MB","id":"b542772b4177"}` + "\n"))
	require.NoError(t, err)
	require.Equal(t, 3, len(pm.imageProgress.layers), "number of layers should be 3")
	require.Equal(t, int64(807833), pm.imageProgress.currentBytes())
	require.Equal(t, int64(15443505), pm.imageProgress.totalBytes())

	_, err = pm.Write([]byte(`{"status":"Download complete","progressDetail":{},"id":"b542772b4177"}` + "\n"))
	require.NoError(t, err)
	require.Equal(t, 3, len(pm.imageProgress.layers), "number of layers should be 3")
	require.Equal(t, int64(4449071), pm.imageProgress.currentBytes())
	require.Equal(t, int64(15443505), pm.imageProgress.totalBytes())
}
