package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCSIVolumes_CRUD(t *testing.T) {
	t.Parallel()
	c, s, root := makeACLClient(t, nil, nil)
	defer s.Stop()
	v := c.CSIVolumes()

	// Successful empty result
	vols, qm, err := v.List(nil)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, qm.LastIndex)
	assert.Equal(t, 0, len(vols))

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

	// Register a volume
	v.Register(&CSIVolume{
		ID:             "DEADBEEF-63C7-407F-AE82-C99FBEF78FEB",
		Driver:         "minnie",
		Namespace:      "default",
		AccessMode:     CSIVolumeAccessModeMultiNodeSingleWriter,
		AttachmentMode: CSIVolumeAttachmentModeFilesystem,
		Topologies:     []*CSITopology{{Segments: map[string]string{"foo": "bar"}}},
	}, wpts)

	// Successful result with volumes
	vols, qm, err = v.List(opts)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, qm.LastIndex)
	assert.Equal(t, 1, len(vols))

	// Successful info query
	vol, qm, err := v.Info("DEADBEEF-63C7-407F-AE82-C99FBEF78FEB", opts)
	assert.NoError(t, err)
	assert.Equal(t, "minnie", vol.Driver)
	assert.Equal(t, "bar", vol.Topologies[0].Segments["foo"])

	// Deregister the volume
	err = v.Deregister("DEADBEEF-63C7-407F-AE82-C99FBEF78FEB", wpts)
	assert.NoError(t, err)

	// Successful empty result
	vols, qm, err = v.List(nil)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, qm.LastIndex)
	assert.Equal(t, 0, len(vols))

	// Failed info query
	vol, qm, err = v.Info("DEADBEEF-63C7-407F-AE82-C99FBEF78FEB", opts)
	assert.Error(t, err, "missing")
}
