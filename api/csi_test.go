package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCSIVolumes_CRUD(t *testing.T) {
	t.Parallel()
	c, s, root := makeACLClient(t, nil, nil)
	defer s.Stop()
	v := c.CSIVolumes()

	// Successful empty result
	vols, qm, err := v.List(nil)
	require.NoError(t, err)
	require.NotEqual(t, 0, qm.LastIndex)
	require.Equal(t, 0, len(vols))

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

	// Register a plugin job
	j := c.Jobs()
	job := testJob()
	job.Namespace = stringToPtr("default")
	job.TaskGroups[0].Tasks[0].CSIPluginConfig = &TaskCSIPluginConfig{
		ID:       "foo",
		Type:     "monolith",
		MountDir: "/not-empty",
	}
	_, _, err = j.Register(job, wpts)
	require.NoError(t, err)

	// Register a volume
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
	err = v.Deregister(id, wpts)
	require.NoError(t, err)

	// Successful empty result
	vols, qm, err = v.List(nil)
	require.NoError(t, err)
	require.NotEqual(t, 0, qm.LastIndex)
	require.Equal(t, 0, len(vols))

	// Failed info query
	vol, qm, err = v.Info(id, opts)
	require.Error(t, err, "missing")
}

/* This is obsolete, but plugin creation via fingerprinting is an e2e test
func TestCSIPlugins_viaJob(t *testing.T) {
	t.Parallel()
	c, s, root := makeACLClient(t, nil, nil)
	defer s.Stop()
	p := c.CSIPlugins()

	// Successful empty result
	plugs, qm, err := p.List(nil)
	require.NoError(t, err)
	require.NotEqual(t, 0, qm.LastIndex)
	require.Equal(t, 0, len(plugs))

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

	// Register a plugin job
	j := c.Jobs()
	job := testJob()
	job.Namespace = stringToPtr("default")
	job.TaskGroups[0].Tasks[0].CSIPluginConfig = &TaskCSIPluginConfig{
		ID:       "foo",
		Type:     "monolith",
		MountDir: "/not-empty",
	}
	_, _, err = j.Register(job, wpts)
	require.NoError(t, err)

	// Successful result with the plugin
	plugs, qm, err = p.List(opts)
	require.NoError(t, err)
	require.NotEqual(t, 0, qm.LastIndex)
	require.Equal(t, 1, len(plugs))

	// Successful info query
	plug, qm, err := p.Info("foo", opts)
	require.NoError(t, err)
	require.NotNil(t, plug.Jobs[*job.Namespace][*job.ID])
	require.Equal(t, *job.ID, *plug.Jobs[*job.Namespace][*job.ID].ID)
}
*/
