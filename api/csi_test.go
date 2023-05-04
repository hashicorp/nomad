package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

// TestCSIVolumes_CRUD fails because of a combination of removing the job to plugin creation
// pathway and checking for plugin existence (but not yet health) at registration time.
// There are two possible solutions:
// 1. Expose the test server RPC server and force a Node.Update to fingerprint a plugin
// 2. Build and deploy a dummy CSI plugin via a job, and have it really fingerprint
func TestCSIVolumes_CRUD(t *testing.T) {
	testutil.Parallel(t)

	c, s, root := makeACLClient(t, nil, nil)
	defer s.Stop()
	v := c.CSIVolumes()

	// Successful empty result
	vols, qm, err := v.List(nil)
	must.NoError(t, err)
	// must.Positive(t, qm.LastIndex) TODO(tgross), this was always broken?
	_ = qm
	must.SliceEmpty(t, vols)

	_ = root
	// FIXME we're bailing out here until one of the fixes is available
	/*

		// Authorized QueryOpts. Use the root token to just bypass ACL details
		opts := &QueryOptions{
			Region:    "global",
			Namespace: "default",
			AuthToken: root.SecretID,
		}

		wpts := &WriteOptions{
			Region:    "global",
			Namespace: "default",
			AuthToken: root.SecretID,
		}

		// Create node plugins
		nodes, _, err := c.Nodes().List(nil)
		require.NoError(t, err)
		require.Equal(t, 1, len(nodes))

		nodeStub := nodes[0]
		node, _, err := c.Nodes().Info(nodeStub.ID, nil)
		require.NoError(t, err)
		node.CSINodePlugins = map[string]*CSIInfo{
			"foo": {
				PluginID:                 "foo",
				Healthy:                  true,
				RequiresControllerPlugin: false,
				RequiresTopologies:       false,
				NodeInfo: &CSINodeInfo{
					ID:         nodeStub.ID,
					MaxVolumes: 200,
				},
			},
		}

		// Register a volume
		// This id is here as a string to avoid importing helper, which causes the lint
		// rule that checks that the api package is isolated to fail
		id := "DEADBEEF-31B5-8F78-7986-DD404FDA0CD1"
		_, err = v.Register(&CSIVolume{
			ID:             id,
			Namespace:      "default",
			PluginID:       "foo",
			AccessMode:     CSIVolumeAccessModeMultiNodeSingleWriter,
			AttachmentMode: CSIVolumeAttachmentModeFilesystem,
			Topologies:     []*CSITopology{{Segments: map[string]string{"foo": "bar"}}},
		}, wpts)
		require.NoError(t, err)

		// Successful result with volumes
		vols, qm, err = v.List(opts)
		require.NoError(t, err)
		require.NotEqual(t, 0, qm.LastIndex)
		require.Equal(t, 1, len(vols))

		// Successful info query
		vol, qm, err := v.Info(id, opts)
		require.NoError(t, err)
		require.Equal(t, "bar", vol.Topologies[0].Segments["foo"])

		// Deregister the volume
		err = v.Deregister(id, true, wpts)
		require.NoError(t, err)

		// Successful empty result
		vols, qm, err = v.List(nil)
		require.NoError(t, err)
		require.NotEqual(t, 0, qm.LastIndex)
		require.Equal(t, 0, len(vols))

		// Failed info query
		vol, qm, err = v.Info(id, opts)
		require.Error(t, err, "missing")

	*/
}
