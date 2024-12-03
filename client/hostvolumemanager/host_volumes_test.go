// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hostvolumemanager

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/shoenig/test/must"
)

// db TODO(1.10.0): improve hostvolumemanager tests.

func TestNewHostVolumeManager_restoreState(t *testing.T) {
	log := testlog.HCLogger(t)
	vol := &HostVolumeState{
		ID: "test-vol-id",
		CreateReq: &cstructs.ClientHostVolumeCreateRequest{
			ID:       "test-vol-id",
			PluginID: "mkdir",
		},
	}

	t.Run("happy", func(t *testing.T) {
		// put our volume in state
		state := newMockState()
		must.NoError(t, state.PutDynamicHostVolume(vol))

		// new volume manager should load it from state and run Create,
		// resulting in a volume directory in this mountDir.
		mountDir := t.TempDir()

		_, err := NewHostVolumeManager(log, state, time.Second, "/wherever", mountDir)
		must.NoError(t, err)

		volPath := filepath.Join(mountDir, vol.ID)
		must.DirExists(t, volPath)
	})

	t.Run("get error", func(t *testing.T) {
		state := newMockState()
		state.getErr = errors.New("get error")
		_, err := NewHostVolumeManager(log, state, time.Second, "/wherever", "/wherever")
		// error loading state should break the world
		must.ErrorIs(t, err, state.getErr)
	})

	t.Run("put error", func(t *testing.T) {
		state := newMockState()
		// add the volume to try to restore
		must.NoError(t, state.PutDynamicHostVolume(vol))

		// make sure mock state actually errors, to ensure we're ignoring it
		state.putErr = errors.New("put error")
		must.Error(t, state.PutDynamicHostVolume(vol))

		// restore process also attempts a state Put during vol Create
		_, err := NewHostVolumeManager(log, state, time.Second, "/wherever", t.TempDir())
		// but it should not cause the whole agent to fail to start
		must.NoError(t, err)
	})
}

var _ HostVolumeStateManager = &mockState{}

func newMockState() *mockState {
	return &mockState{
		vols: make(map[string]*HostVolumeState),
	}
}

type mockState struct {
	mut  sync.Mutex
	vols map[string]*HostVolumeState

	putErr, getErr, deleteErr error
}

func (s *mockState) PutDynamicHostVolume(vol *HostVolumeState) error {
	if s.putErr != nil {
		return s.putErr
	}
	s.mut.Lock()
	defer s.mut.Unlock()
	s.vols[vol.ID] = vol
	return nil
}

func (s *mockState) GetDynamicHostVolumes() ([]*HostVolumeState, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	s.mut.Lock()
	defer s.mut.Unlock()
	var vols []*HostVolumeState
	for _, v := range s.vols {
		vols = append(vols, v)
	}
	return vols, nil
}

func (s *mockState) DeleteDynamicHostVolume(id string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.mut.Lock()
	defer s.mut.Unlock()
	delete(s.vols, id)
	return nil
}
