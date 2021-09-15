package structs

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestCSIVolumeClaim ensures that a volume claim workflows work as expected.
func TestCSIVolumeClaim(t *testing.T) {
	require := require.New(t)
	vol := NewCSIVolume("vol0", 0)
	vol.Schedulable = true
	vol.AccessMode = CSIVolumeAccessModeUnknown
	vol.AttachmentMode = CSIVolumeAttachmentModeUnknown
	vol.RequestedCapabilities = []*CSIVolumeCapability{
		{
			AccessMode:     CSIVolumeAccessModeMultiNodeSingleWriter,
			AttachmentMode: CSIVolumeAttachmentModeFilesystem,
		},
		{
			AccessMode:     CSIVolumeAccessModeMultiNodeReader,
			AttachmentMode: CSIVolumeAttachmentModeFilesystem,
		},
	}

	alloc1 := &Allocation{ID: "a1", Namespace: "n", JobID: "j"}
	alloc2 := &Allocation{ID: "a2", Namespace: "n", JobID: "j"}
	alloc3 := &Allocation{ID: "a3", Namespace: "n", JobID: "j3"}
	claim := &CSIVolumeClaim{
		AllocationID: alloc1.ID,
		NodeID:       "foo",
		State:        CSIVolumeClaimStateTaken,
	}

	// claim a read and ensure we are still schedulable
	claim.Mode = CSIVolumeClaimRead
	claim.AccessMode = CSIVolumeAccessModeMultiNodeReader
	claim.AttachmentMode = CSIVolumeAttachmentModeFilesystem
	require.NoError(vol.Claim(claim, alloc1))
	require.True(vol.ReadSchedulable())
	require.False(vol.WriteSchedulable())
	require.False(vol.WriteFreeClaims())
	require.Len(vol.ReadClaims, 1)
	require.Len(vol.WriteClaims, 0)
	require.Len(vol.PastClaims, 0)
	require.Equal(CSIVolumeAccessModeMultiNodeReader, vol.AccessMode)
	require.Equal(CSIVolumeAttachmentModeFilesystem, vol.AttachmentMode)
	require.Len(vol.RequestedCapabilities, 2)
	require.Equal(CSIVolumeAccessModeMultiNodeSingleWriter,
		vol.RequestedCapabilities[0].AccessMode)
	require.Equal(CSIVolumeAccessModeMultiNodeReader,
		vol.RequestedCapabilities[1].AccessMode)

	// claim a write and ensure we can't upgrade capabilities.
	claim.AccessMode = CSIVolumeAccessModeMultiNodeSingleWriter
	claim.Mode = CSIVolumeClaimWrite
	claim.AllocationID = alloc2.ID
	require.EqualError(vol.Claim(claim, alloc2), "unschedulable")
	require.True(vol.ReadSchedulable())
	require.False(vol.WriteSchedulable())
	require.False(vol.WriteFreeClaims())
	require.Len(vol.ReadClaims, 1)
	require.Len(vol.WriteClaims, 0)
	require.Len(vol.PastClaims, 0)
	require.Equal(CSIVolumeAccessModeMultiNodeReader, vol.AccessMode)
	require.Equal(CSIVolumeAttachmentModeFilesystem, vol.AttachmentMode)

	// release our last claim, including unpublish workflow
	claim.AllocationID = alloc1.ID
	claim.Mode = CSIVolumeClaimRead
	claim.State = CSIVolumeClaimStateReadyToFree
	vol.Claim(claim, nil)
	require.Len(vol.ReadClaims, 0)
	require.Len(vol.WriteClaims, 0)
	require.Equal(CSIVolumeAccessModeUnknown, vol.AccessMode)
	require.Equal(CSIVolumeAttachmentModeUnknown, vol.AttachmentMode)
	require.Len(vol.RequestedCapabilities, 2)
	require.Equal(CSIVolumeAccessModeMultiNodeSingleWriter,
		vol.RequestedCapabilities[0].AccessMode)
	require.Equal(CSIVolumeAccessModeMultiNodeReader,
		vol.RequestedCapabilities[1].AccessMode)

	// claim a write on the now-unclaimed volume and ensure we can upgrade
	// capabilities so long as they're in our RequestedCapabilities.
	claim.AccessMode = CSIVolumeAccessModeMultiNodeSingleWriter
	claim.Mode = CSIVolumeClaimWrite
	claim.State = CSIVolumeClaimStateTaken
	claim.AllocationID = alloc2.ID
	require.NoError(vol.Claim(claim, alloc2))
	require.Len(vol.ReadClaims, 0)
	require.Len(vol.WriteClaims, 1)
	require.Equal(CSIVolumeAccessModeMultiNodeSingleWriter, vol.AccessMode)
	require.Equal(CSIVolumeAttachmentModeFilesystem, vol.AttachmentMode)
	require.Len(vol.RequestedCapabilities, 2)
	require.Equal(CSIVolumeAccessModeMultiNodeSingleWriter,
		vol.RequestedCapabilities[0].AccessMode)
	require.Equal(CSIVolumeAccessModeMultiNodeReader,
		vol.RequestedCapabilities[1].AccessMode)

	// make the claim again to ensure its idempotent, and that the volume's
	// access mode is unchanged.
	require.NoError(vol.Claim(claim, alloc2))
	require.True(vol.ReadSchedulable())
	require.True(vol.WriteSchedulable())
	require.False(vol.WriteFreeClaims())
	require.Len(vol.ReadClaims, 0)
	require.Len(vol.WriteClaims, 1)
	require.Len(vol.PastClaims, 0)
	require.Equal(CSIVolumeAccessModeMultiNodeSingleWriter, vol.AccessMode)
	require.Equal(CSIVolumeAttachmentModeFilesystem, vol.AttachmentMode)

	// claim a read. ensure we are still schedulable and that we haven't
	// changed the access mode
	claim.AllocationID = alloc1.ID
	claim.Mode = CSIVolumeClaimRead
	claim.AccessMode = CSIVolumeAccessModeMultiNodeReader
	claim.AttachmentMode = CSIVolumeAttachmentModeFilesystem
	require.NoError(vol.Claim(claim, alloc1))
	require.True(vol.ReadSchedulable())
	require.True(vol.WriteSchedulable())
	require.False(vol.WriteFreeClaims())
	require.Len(vol.ReadClaims, 1)
	require.Len(vol.WriteClaims, 1)
	require.Len(vol.PastClaims, 0)
	require.Equal(CSIVolumeAccessModeMultiNodeSingleWriter, vol.AccessMode)
	require.Equal(CSIVolumeAttachmentModeFilesystem, vol.AttachmentMode)

	// ensure we can't change the attachment mode for a claimed volume
	claim.AttachmentMode = CSIVolumeAttachmentModeBlockDevice
	claim.AllocationID = alloc3.ID
	require.EqualError(vol.Claim(claim, alloc3),
		"cannot change attachment mode of claimed volume")
	claim.AttachmentMode = CSIVolumeAttachmentModeFilesystem

	// denormalize-on-read (simulating a volume we've gotten out of the state
	// store) and then ensure we cannot claim another write
	vol.WriteAllocs[alloc2.ID] = alloc2
	claim.Mode = CSIVolumeClaimWrite
	require.EqualError(vol.Claim(claim, alloc3), "volume max claim reached")

	// release the write claim but ensure it doesn't free up write claims
	// until after we've unpublished
	claim.AllocationID = alloc2.ID
	claim.State = CSIVolumeClaimStateUnpublishing
	vol.Claim(claim, nil)
	require.True(vol.ReadSchedulable())
	require.True(vol.WriteSchedulable())
	require.False(vol.WriteFreeClaims())
	require.Len(vol.ReadClaims, 1)
	require.Len(vol.WriteClaims, 1) // claim still exists until we're done
	require.Len(vol.PastClaims, 1)
	require.Equal(CSIVolumeAccessModeMultiNodeSingleWriter, vol.AccessMode)
	require.Equal(CSIVolumeAttachmentModeFilesystem, vol.AttachmentMode)

	// complete the unpublish workflow
	claim.State = CSIVolumeClaimStateReadyToFree
	vol.Claim(claim, nil)
	require.True(vol.ReadSchedulable())
	require.True(vol.WriteSchedulable())
	require.True(vol.WriteFreeClaims())
	require.Len(vol.ReadClaims, 1)
	require.Len(vol.WriteClaims, 0)
	require.Len(vol.WriteAllocs, 0)
	require.Len(vol.PastClaims, 0)
	require.Equal(CSIVolumeAccessModeMultiNodeSingleWriter, vol.AccessMode)
	require.Equal(CSIVolumeAttachmentModeFilesystem, vol.AttachmentMode)

	// release our last claim, including unpublish workflow
	claim.AllocationID = alloc1.ID
	claim.Mode = CSIVolumeClaimRead
	vol.Claim(claim, nil)
	require.Len(vol.ReadClaims, 0)
	require.Len(vol.WriteClaims, 0)
	require.Equal(CSIVolumeAccessModeUnknown, vol.AccessMode)
	require.Equal(CSIVolumeAttachmentModeUnknown, vol.AttachmentMode)
	require.Len(vol.RequestedCapabilities, 2)
	require.Equal(CSIVolumeAccessModeMultiNodeSingleWriter,
		vol.RequestedCapabilities[0].AccessMode)
	require.Equal(CSIVolumeAccessModeMultiNodeReader,
		vol.RequestedCapabilities[1].AccessMode)
}

// TestCSIVolumeClaim_CompatOldClaims ensures that volume created before
// v1.1.0 with claims that exist before v1.1.0 still work.
//
// COMPAT(1.3.0): safe to remove this test, but not the code, for 1.3.0
func TestCSIVolumeClaim_CompatOldClaims(t *testing.T) {
	require := require.New(t)
	vol := NewCSIVolume("vol0", 0)
	vol.Schedulable = true
	vol.AccessMode = CSIVolumeAccessModeMultiNodeSingleWriter
	vol.AttachmentMode = CSIVolumeAttachmentModeFilesystem

	alloc1 := &Allocation{ID: "a1", Namespace: "n", JobID: "j"}
	alloc2 := &Allocation{ID: "a2", Namespace: "n", JobID: "j"}
	alloc3 := &Allocation{ID: "a3", Namespace: "n", JobID: "j3"}
	claim := &CSIVolumeClaim{
		AllocationID: alloc1.ID,
		NodeID:       "foo",
		State:        CSIVolumeClaimStateTaken,
	}

	// claim a read and ensure we are still schedulable
	claim.Mode = CSIVolumeClaimRead
	require.NoError(vol.Claim(claim, alloc1))
	require.True(vol.ReadSchedulable())
	require.True(vol.WriteSchedulable())
	require.True(vol.WriteFreeClaims())
	require.Len(vol.ReadClaims, 1)
	require.Len(vol.WriteClaims, 0)
	require.Len(vol.PastClaims, 0)
	require.Equal(CSIVolumeAccessModeMultiNodeSingleWriter, vol.AccessMode)
	require.Equal(CSIVolumeAttachmentModeFilesystem, vol.AttachmentMode)
	require.Len(vol.RequestedCapabilities, 1)
	require.Equal(CSIVolumeAccessModeMultiNodeSingleWriter,
		vol.RequestedCapabilities[0].AccessMode)
	require.Equal(CSIVolumeAttachmentModeFilesystem,
		vol.RequestedCapabilities[0].AttachmentMode)

	// claim a write and ensure we no longer have free write claims
	claim.Mode = CSIVolumeClaimWrite
	claim.AllocationID = alloc2.ID
	require.NoError(vol.Claim(claim, alloc2))
	require.True(vol.ReadSchedulable())
	require.True(vol.WriteSchedulable())
	require.False(vol.WriteFreeClaims())
	require.Len(vol.ReadClaims, 1)
	require.Len(vol.WriteClaims, 1)
	require.Len(vol.PastClaims, 0)
	require.Equal(CSIVolumeAccessModeMultiNodeSingleWriter, vol.AccessMode)
	require.Equal(CSIVolumeAttachmentModeFilesystem, vol.AttachmentMode)

	// denormalize-on-read (simulating a volume we've gotten out of the state
	// store) and then ensure we cannot claim another write
	vol.WriteAllocs[alloc2.ID] = alloc2
	claim.AllocationID = alloc3.ID
	require.EqualError(vol.Claim(claim, alloc3), "volume max claim reached")

	// release the write claim but ensure it doesn't free up write claims
	// until after we've unpublished
	claim.AllocationID = alloc2.ID
	claim.State = CSIVolumeClaimStateUnpublishing
	vol.Claim(claim, nil)
	require.True(vol.ReadSchedulable())
	require.True(vol.WriteSchedulable())
	require.False(vol.WriteFreeClaims())
	require.Len(vol.ReadClaims, 1)
	require.Len(vol.WriteClaims, 1) // claim still exists until we're done
	require.Len(vol.PastClaims, 1)
	require.Equal(CSIVolumeAccessModeMultiNodeSingleWriter, vol.AccessMode)
	require.Equal(CSIVolumeAttachmentModeFilesystem, vol.AttachmentMode)

	// complete the unpublish workflow
	claim.State = CSIVolumeClaimStateReadyToFree
	vol.Claim(claim, nil)
	require.True(vol.ReadSchedulable())
	require.True(vol.WriteSchedulable())
	require.True(vol.WriteFreeClaims())
	require.Len(vol.ReadClaims, 1)
	require.Len(vol.WriteClaims, 0)
	require.Len(vol.WriteAllocs, 0)
	require.Len(vol.PastClaims, 0)
	require.Equal(CSIVolumeAccessModeMultiNodeSingleWriter, vol.AccessMode)
	require.Equal(CSIVolumeAttachmentModeFilesystem, vol.AttachmentMode)

	// release our last claim, including unpublish workflow
	claim.AllocationID = alloc1.ID
	claim.Mode = CSIVolumeClaimRead
	vol.Claim(claim, nil)
	require.Len(vol.ReadClaims, 0)
	require.Len(vol.WriteClaims, 0)
	require.Equal(CSIVolumeAccessModeUnknown, vol.AccessMode)
	require.Equal(CSIVolumeAttachmentModeUnknown, vol.AttachmentMode)
	require.Equal(CSIVolumeAccessModeMultiNodeSingleWriter,
		vol.RequestedCapabilities[0].AccessMode)
	require.Equal(CSIVolumeAttachmentModeFilesystem,
		vol.RequestedCapabilities[0].AttachmentMode)
}

// TestCSIVolumeClaim_CompatNewClaimsOK ensures that a volume created
// before v1.1.0 is compatible with new claims.
//
// COMPAT(1.3.0): safe to remove this test, but not the code, for 1.3.0
func TestCSIVolumeClaim_CompatNewClaimsOK(t *testing.T) {
	require := require.New(t)
	vol := NewCSIVolume("vol0", 0)
	vol.Schedulable = true
	vol.AccessMode = CSIVolumeAccessModeMultiNodeSingleWriter
	vol.AttachmentMode = CSIVolumeAttachmentModeFilesystem

	alloc1 := &Allocation{ID: "a1", Namespace: "n", JobID: "j"}
	alloc2 := &Allocation{ID: "a2", Namespace: "n", JobID: "j"}
	alloc3 := &Allocation{ID: "a3", Namespace: "n", JobID: "j3"}
	claim := &CSIVolumeClaim{
		AllocationID: alloc1.ID,
		NodeID:       "foo",
		State:        CSIVolumeClaimStateTaken,
	}

	// claim a read and ensure we are still schedulable
	claim.Mode = CSIVolumeClaimRead
	claim.AccessMode = CSIVolumeAccessModeMultiNodeReader
	claim.AttachmentMode = CSIVolumeAttachmentModeFilesystem
	require.NoError(vol.Claim(claim, alloc1))
	require.True(vol.ReadSchedulable())
	require.True(vol.WriteSchedulable())
	require.True(vol.WriteFreeClaims())
	require.Len(vol.ReadClaims, 1)
	require.Len(vol.WriteClaims, 0)
	require.Len(vol.PastClaims, 0)
	require.Equal(CSIVolumeAccessModeMultiNodeSingleWriter, vol.AccessMode)
	require.Equal(CSIVolumeAttachmentModeFilesystem, vol.AttachmentMode)
	require.Len(vol.RequestedCapabilities, 1)
	require.Equal(CSIVolumeAccessModeMultiNodeSingleWriter,
		vol.RequestedCapabilities[0].AccessMode)
	require.Equal(CSIVolumeAttachmentModeFilesystem,
		vol.RequestedCapabilities[0].AttachmentMode)

	// claim a write and ensure we no longer have free write claims
	claim.Mode = CSIVolumeClaimWrite
	claim.AllocationID = alloc2.ID
	require.NoError(vol.Claim(claim, alloc2))
	require.True(vol.ReadSchedulable())
	require.True(vol.WriteSchedulable())
	require.False(vol.WriteFreeClaims())
	require.Len(vol.ReadClaims, 1)
	require.Len(vol.WriteClaims, 1)
	require.Len(vol.PastClaims, 0)
	require.Equal(CSIVolumeAccessModeMultiNodeSingleWriter, vol.AccessMode)
	require.Equal(CSIVolumeAttachmentModeFilesystem, vol.AttachmentMode)

	// ensure we can't change the attachment mode for a claimed volume
	claim.AttachmentMode = CSIVolumeAttachmentModeBlockDevice
	require.EqualError(vol.Claim(claim, alloc2),
		"cannot change attachment mode of claimed volume")
	claim.AttachmentMode = CSIVolumeAttachmentModeFilesystem

	// denormalize-on-read (simulating a volume we've gotten out of the state
	// store) and then ensure we cannot claim another write
	vol.WriteAllocs[alloc2.ID] = alloc2
	claim.AllocationID = alloc3.ID
	require.EqualError(vol.Claim(claim, alloc3), "volume max claim reached")

	// release the write claim but ensure it doesn't free up write claims
	// until after we've unpublished
	claim.AllocationID = alloc2.ID
	claim.State = CSIVolumeClaimStateUnpublishing
	vol.Claim(claim, nil)
	require.True(vol.ReadSchedulable())
	require.True(vol.WriteSchedulable())
	require.False(vol.WriteFreeClaims())
	require.Len(vol.ReadClaims, 1)
	require.Len(vol.WriteClaims, 1) // claim still exists until we're done
	require.Len(vol.PastClaims, 1)
	require.Equal(CSIVolumeAccessModeMultiNodeSingleWriter, vol.AccessMode)
	require.Equal(CSIVolumeAttachmentModeFilesystem, vol.AttachmentMode)

	// complete the unpublish workflow
	claim.State = CSIVolumeClaimStateReadyToFree
	vol.Claim(claim, nil)
	require.True(vol.ReadSchedulable())
	require.True(vol.WriteSchedulable())
	require.True(vol.WriteFreeClaims())
	require.Len(vol.ReadClaims, 1)
	require.Len(vol.WriteClaims, 0)
	require.Len(vol.WriteAllocs, 0)
	require.Len(vol.PastClaims, 0)
	require.Equal(CSIVolumeAccessModeMultiNodeSingleWriter, vol.AccessMode)
	require.Equal(CSIVolumeAttachmentModeFilesystem, vol.AttachmentMode)

	// release our last claim, including unpublish workflow
	claim.AllocationID = alloc1.ID
	claim.Mode = CSIVolumeClaimRead
	vol.Claim(claim, nil)
	require.Len(vol.ReadClaims, 0)
	require.Len(vol.WriteClaims, 0)
	require.Equal(CSIVolumeAccessModeUnknown, vol.AccessMode)
	require.Equal(CSIVolumeAttachmentModeUnknown, vol.AttachmentMode)
	require.Equal(CSIVolumeAccessModeMultiNodeSingleWriter,
		vol.RequestedCapabilities[0].AccessMode)
	require.Equal(CSIVolumeAttachmentModeFilesystem,
		vol.RequestedCapabilities[0].AttachmentMode)
}

// TestCSIVolumeClaim_CompatNewClaimsNoUpgrade ensures that a volume created
// before v1.1.0 is compatible with new claims, but prevents unexpected
// capability upgrades.
//
// COMPAT(1.3.0): safe to remove this test, but not the code, for 1.3.0
func TestCSIVolumeClaim_CompatNewClaimsNoUpgrade(t *testing.T) {
	require := require.New(t)
	vol := NewCSIVolume("vol0", 0)
	vol.Schedulable = true
	vol.AccessMode = CSIVolumeAccessModeMultiNodeReader
	vol.AttachmentMode = CSIVolumeAttachmentModeFilesystem

	alloc1 := &Allocation{ID: "a1", Namespace: "n", JobID: "j"}
	alloc2 := &Allocation{ID: "a2", Namespace: "n", JobID: "j"}
	claim := &CSIVolumeClaim{
		AllocationID: alloc1.ID,
		NodeID:       "foo",
		State:        CSIVolumeClaimStateTaken,
	}

	// claim a read and ensure we are still schedulable
	claim.Mode = CSIVolumeClaimRead
	claim.AccessMode = CSIVolumeAccessModeMultiNodeReader
	claim.AttachmentMode = CSIVolumeAttachmentModeFilesystem
	require.NoError(vol.Claim(claim, alloc1))
	require.True(vol.ReadSchedulable())
	require.False(vol.WriteSchedulable())
	require.False(vol.WriteFreeClaims())
	require.Len(vol.ReadClaims, 1)
	require.Len(vol.WriteClaims, 0)
	require.Len(vol.PastClaims, 0)
	require.Equal(CSIVolumeAccessModeMultiNodeReader, vol.AccessMode)
	require.Equal(CSIVolumeAttachmentModeFilesystem, vol.AttachmentMode)
	require.Len(vol.RequestedCapabilities, 1)
	require.Equal(CSIVolumeAccessModeMultiNodeReader,
		vol.RequestedCapabilities[0].AccessMode)
	require.Equal(CSIVolumeAttachmentModeFilesystem,
		vol.RequestedCapabilities[0].AttachmentMode)

	// claim a write and ensure we can't upgrade capabilities.
	claim.AccessMode = CSIVolumeAccessModeMultiNodeSingleWriter
	claim.Mode = CSIVolumeClaimWrite
	claim.AllocationID = alloc2.ID
	require.EqualError(vol.Claim(claim, alloc2), "unschedulable")
	require.True(vol.ReadSchedulable())
	require.False(vol.WriteSchedulable())
	require.False(vol.WriteFreeClaims())
	require.Len(vol.ReadClaims, 1)
	require.Len(vol.WriteClaims, 0)
	require.Len(vol.PastClaims, 0)
	require.Equal(CSIVolumeAccessModeMultiNodeReader, vol.AccessMode)
	require.Equal(CSIVolumeAttachmentModeFilesystem, vol.AttachmentMode)
	require.Len(vol.RequestedCapabilities, 1)
	require.Equal(CSIVolumeAccessModeMultiNodeReader,
		vol.RequestedCapabilities[0].AccessMode)
	require.Equal(CSIVolumeAttachmentModeFilesystem,
		vol.RequestedCapabilities[0].AttachmentMode)

	// release our last claim, including unpublish workflow
	claim.AllocationID = alloc1.ID
	claim.Mode = CSIVolumeClaimRead
	claim.State = CSIVolumeClaimStateReadyToFree
	vol.Claim(claim, nil)
	require.Len(vol.ReadClaims, 0)
	require.Len(vol.WriteClaims, 0)
	require.Equal(CSIVolumeAccessModeUnknown, vol.AccessMode)
	require.Equal(CSIVolumeAttachmentModeUnknown, vol.AttachmentMode)
	require.Equal(CSIVolumeAccessModeMultiNodeReader,
		vol.RequestedCapabilities[0].AccessMode)
	require.Equal(CSIVolumeAttachmentModeFilesystem,
		vol.RequestedCapabilities[0].AttachmentMode)

	// claim a write on the now-unclaimed volume and ensure we still can't
	// upgrade capabilities.
	claim.AccessMode = CSIVolumeAccessModeMultiNodeSingleWriter
	claim.Mode = CSIVolumeClaimWrite
	claim.State = CSIVolumeClaimStateTaken
	claim.AllocationID = alloc2.ID
	require.EqualError(vol.Claim(claim, alloc2), "unschedulable")
	require.Len(vol.ReadClaims, 0)
	require.Len(vol.WriteClaims, 0)
	require.Equal(CSIVolumeAccessModeUnknown, vol.AccessMode)
	require.Equal(CSIVolumeAttachmentModeUnknown, vol.AttachmentMode)
	require.Equal(CSIVolumeAccessModeMultiNodeReader,
		vol.RequestedCapabilities[0].AccessMode)
	require.Equal(CSIVolumeAttachmentModeFilesystem,
		vol.RequestedCapabilities[0].AttachmentMode)
}

func TestVolume_Copy(t *testing.T) {

	a1 := MockAlloc()
	a2 := MockAlloc()
	a3 := MockAlloc()
	c1 := &CSIVolumeClaim{
		AllocationID:   a1.ID,
		NodeID:         a1.NodeID,
		ExternalNodeID: "c1",
		Mode:           CSIVolumeClaimRead,
		State:          CSIVolumeClaimStateTaken,
	}
	c2 := &CSIVolumeClaim{
		AllocationID:   a2.ID,
		NodeID:         a2.NodeID,
		ExternalNodeID: "c2",
		Mode:           CSIVolumeClaimRead,
		State:          CSIVolumeClaimStateNodeDetached,
	}
	c3 := &CSIVolumeClaim{
		AllocationID:   a3.ID,
		NodeID:         a3.NodeID,
		ExternalNodeID: "c3",
		Mode:           CSIVolumeClaimWrite,
		State:          CSIVolumeClaimStateTaken,
	}

	v1 := &CSIVolume{
		ID:             "vol1",
		Name:           "vol1",
		ExternalID:     "vol-abcdef",
		Namespace:      "default",
		Topologies:     []*CSITopology{{Segments: map[string]string{"AZ1": "123"}}},
		AccessMode:     CSIVolumeAccessModeSingleNodeWriter,
		AttachmentMode: CSIVolumeAttachmentModeBlockDevice,
		MountOptions:   &CSIMountOptions{FSType: "ext4", MountFlags: []string{"ro", "noatime"}},
		Secrets:        CSISecrets{"mysecret": "myvalue"},
		Parameters:     map[string]string{"param1": "val1"},
		Context:        map[string]string{"ctx1": "val1"},

		ReadAllocs:  map[string]*Allocation{a1.ID: a1, a2.ID: nil},
		WriteAllocs: map[string]*Allocation{a3.ID: a3},

		ReadClaims:  map[string]*CSIVolumeClaim{a1.ID: c1, a2.ID: c2},
		WriteClaims: map[string]*CSIVolumeClaim{a3.ID: c3},
		PastClaims:  map[string]*CSIVolumeClaim{},

		Schedulable:         true,
		PluginID:            "moosefs",
		Provider:            "n/a",
		ProviderVersion:     "1.0",
		ControllerRequired:  true,
		ControllersHealthy:  2,
		ControllersExpected: 2,
		NodesHealthy:        4,
		NodesExpected:       5,
		ResourceExhausted:   time.Now(),
	}

	v2 := v1.Copy()
	if !reflect.DeepEqual(v1, v2) {
		t.Fatalf("Copy() returned an unequal Volume; got %#v; want %#v", v1, v2)
	}

	v1.ReadClaims[a1.ID].State = CSIVolumeClaimStateReadyToFree
	v1.ReadAllocs[a2.ID] = a2
	v1.WriteAllocs[a3.ID].ClientStatus = AllocClientStatusComplete
	v1.MountOptions.FSType = "zfs"

	if v2.ReadClaims[a1.ID].State == CSIVolumeClaimStateReadyToFree {
		t.Fatalf("Volume.Copy() failed; changes to original ReadClaims seen in copy")
	}
	if v2.ReadAllocs[a2.ID] != nil {
		t.Fatalf("Volume.Copy() failed; changes to original ReadAllocs seen in copy")
	}
	if v2.WriteAllocs[a3.ID].ClientStatus == AllocClientStatusComplete {
		t.Fatalf("Volume.Copy() failed; changes to original WriteAllocs seen in copy")
	}
	if v2.MountOptions.FSType == "zfs" {
		t.Fatalf("Volume.Copy() failed; changes to original MountOptions seen in copy")
	}

}

func TestCSIPluginJobs(t *testing.T) {
	plug := NewCSIPlugin("foo", 1000)
	controller := &Job{
		ID:   "job",
		Type: "service",
		TaskGroups: []*TaskGroup{{
			Name:  "foo",
			Count: 11,
			Tasks: []*Task{{
				CSIPluginConfig: &TaskCSIPluginConfig{
					ID:   "foo",
					Type: CSIPluginTypeController,
				},
			}},
		}},
	}

	summary := &JobSummary{}

	plug.AddJob(controller, summary)
	require.Equal(t, 11, plug.ControllersExpected)

	// New job id & make it a system node plugin job
	node := controller.Copy()
	node.ID = "bar"
	node.Type = "system"
	node.TaskGroups[0].Tasks[0].CSIPluginConfig.Type = CSIPluginTypeNode

	summary = &JobSummary{
		Summary: map[string]TaskGroupSummary{
			"foo": {
				Queued:   1,
				Running:  1,
				Starting: 1,
			},
		},
	}

	plug.AddJob(node, summary)
	require.Equal(t, 3, plug.NodesExpected)

	plug.DeleteJob(node, summary)
	require.Equal(t, 0, plug.NodesExpected)
	require.Empty(t, plug.NodeJobs[""])

	plug.DeleteJob(controller, nil)
	require.Equal(t, 0, plug.ControllersExpected)
	require.Empty(t, plug.ControllerJobs[""])
}

func TestCSIPluginCleanup(t *testing.T) {
	plug := NewCSIPlugin("foo", 1000)
	plug.AddPlugin("n0", &CSIInfo{
		PluginID:                 "foo",
		AllocID:                  "a0",
		Healthy:                  true,
		Provider:                 "foo-provider",
		RequiresControllerPlugin: true,
		RequiresTopologies:       false,
		ControllerInfo:           &CSIControllerInfo{},
	})

	plug.AddPlugin("n0", &CSIInfo{
		PluginID:                 "foo",
		AllocID:                  "a0",
		Healthy:                  true,
		Provider:                 "foo-provider",
		RequiresControllerPlugin: true,
		RequiresTopologies:       false,
		NodeInfo:                 &CSINodeInfo{},
	})

	require.Equal(t, 1, plug.ControllersHealthy)
	require.Equal(t, 1, plug.NodesHealthy)

	err := plug.DeleteNode("n0")
	require.NoError(t, err)

	require.Equal(t, 0, plug.ControllersHealthy)
	require.Equal(t, 0, plug.NodesHealthy)

	require.Equal(t, 0, len(plug.Controllers))
	require.Equal(t, 0, len(plug.Nodes))
}

func TestDeleteNodeForType_Controller(t *testing.T) {
	info := &CSIInfo{
		PluginID:                 "foo",
		AllocID:                  "a0",
		Healthy:                  true,
		Provider:                 "foo-provider",
		RequiresControllerPlugin: true,
		RequiresTopologies:       false,
		ControllerInfo:           &CSIControllerInfo{},
	}

	plug := NewCSIPlugin("foo", 1000)

	plug.Controllers["n0"] = info
	plug.ControllersHealthy = 1

	err := plug.DeleteNodeForType("n0", CSIPluginTypeController)
	require.NoError(t, err)

	require.Equal(t, 0, plug.ControllersHealthy)
	require.Equal(t, 0, len(plug.Controllers))
}

func TestDeleteNodeForType_NilController(t *testing.T) {
	plug := NewCSIPlugin("foo", 1000)

	plug.Controllers["n0"] = nil
	plug.ControllersHealthy = 1

	err := plug.DeleteNodeForType("n0", CSIPluginTypeController)
	require.Error(t, err)
	require.Equal(t, 1, len(plug.Controllers))

	_, ok := plug.Controllers["foo"]
	require.False(t, ok)
}

func TestDeleteNodeForType_Node(t *testing.T) {
	info := &CSIInfo{
		PluginID:                 "foo",
		AllocID:                  "a0",
		Healthy:                  true,
		Provider:                 "foo-provider",
		RequiresControllerPlugin: true,
		RequiresTopologies:       false,
		NodeInfo:                 &CSINodeInfo{},
	}

	plug := NewCSIPlugin("foo", 1000)

	plug.Nodes["n0"] = info
	plug.NodesHealthy = 1

	err := plug.DeleteNodeForType("n0", CSIPluginTypeNode)
	require.NoError(t, err)

	require.Equal(t, 0, plug.NodesHealthy)
	require.Equal(t, 0, len(plug.Nodes))
}

func TestDeleteNodeForType_NilNode(t *testing.T) {
	plug := NewCSIPlugin("foo", 1000)

	plug.Nodes["n0"] = nil
	plug.NodesHealthy = 1

	err := plug.DeleteNodeForType("n0", CSIPluginTypeNode)
	require.Error(t, err)
	require.Equal(t, 1, len(plug.Nodes))

	_, ok := plug.Nodes["foo"]
	require.False(t, ok)
}

func TestDeleteNodeForType_Monolith(t *testing.T) {
	controllerInfo := &CSIInfo{
		PluginID:                 "foo",
		AllocID:                  "a0",
		Healthy:                  true,
		Provider:                 "foo-provider",
		RequiresControllerPlugin: true,
		RequiresTopologies:       false,
		ControllerInfo:           &CSIControllerInfo{},
	}

	nodeInfo := &CSIInfo{
		PluginID:                 "foo",
		AllocID:                  "a0",
		Healthy:                  true,
		Provider:                 "foo-provider",
		RequiresControllerPlugin: true,
		RequiresTopologies:       false,
		NodeInfo:                 &CSINodeInfo{},
	}

	plug := NewCSIPlugin("foo", 1000)

	plug.Controllers["n0"] = controllerInfo
	plug.ControllersHealthy = 1

	plug.Nodes["n0"] = nodeInfo
	plug.NodesHealthy = 1

	err := plug.DeleteNodeForType("n0", CSIPluginTypeMonolith)
	require.NoError(t, err)

	require.Equal(t, 0, len(plug.Controllers))
	require.Equal(t, 0, len(plug.Nodes))

	_, ok := plug.Nodes["foo"]
	require.False(t, ok)

	_, ok = plug.Controllers["foo"]
	require.False(t, ok)
}

func TestDeleteNodeForType_Monolith_NilController(t *testing.T) {
	plug := NewCSIPlugin("foo", 1000)

	plug.Controllers["n0"] = nil
	plug.ControllersHealthy = 1

	nodeInfo := &CSIInfo{
		PluginID:                 "foo",
		AllocID:                  "a0",
		Healthy:                  true,
		Provider:                 "foo-provider",
		RequiresControllerPlugin: true,
		RequiresTopologies:       false,
		NodeInfo:                 &CSINodeInfo{},
	}

	plug.Nodes["n0"] = nodeInfo
	plug.NodesHealthy = 1

	err := plug.DeleteNodeForType("n0", CSIPluginTypeMonolith)
	require.Error(t, err)

	require.Equal(t, 1, len(plug.Controllers))
	require.Equal(t, 0, len(plug.Nodes))

	_, ok := plug.Nodes["foo"]
	require.False(t, ok)

	_, ok = plug.Controllers["foo"]
	require.False(t, ok)
}

func TestDeleteNodeForType_Monolith_NilNode(t *testing.T) {
	plug := NewCSIPlugin("foo", 1000)

	plug.Nodes["n0"] = nil
	plug.NodesHealthy = 1

	controllerInfo := &CSIInfo{
		PluginID:                 "foo",
		AllocID:                  "a0",
		Healthy:                  true,
		Provider:                 "foo-provider",
		RequiresControllerPlugin: true,
		RequiresTopologies:       false,
		ControllerInfo:           &CSIControllerInfo{},
	}

	plug.Controllers["n0"] = controllerInfo
	plug.ControllersHealthy = 1

	err := plug.DeleteNodeForType("n0", CSIPluginTypeMonolith)
	require.Error(t, err)

	require.Equal(t, 0, len(plug.Controllers))
	require.Equal(t, 1, len(plug.Nodes))

	_, ok := plug.Nodes["foo"]
	require.False(t, ok)

	_, ok = plug.Controllers["foo"]
	require.False(t, ok)
}
