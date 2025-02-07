// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/image"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

type mockImageClient struct {
	pulled     map[string]int
	idToName   map[string]string
	removed    map[string]int
	pullDelay  time.Duration
	pullReader io.ReadCloser
	lock       sync.Mutex
}

func newMockImageClient(idToName map[string]string, pullDelay time.Duration) *mockImageClient {
	return &mockImageClient{
		pulled:    make(map[string]int),
		removed:   make(map[string]int),
		idToName:  idToName,
		pullDelay: pullDelay,
	}
}

func (m *mockImageClient) ImagePull(ctx context.Context, refStr string, opts image.PullOptions) (io.ReadCloser, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("mockImageClient.ImagePull aborted: %w", ctx.Err())
	case <-time.After(m.pullDelay):
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	m.pulled[refStr]++
	return m.pullReader, nil
}

func (m *mockImageClient) ImageInspectWithRaw(ctx context.Context, id string) (types.ImageInspect, []byte, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	return types.ImageInspect{
		ID: m.idToName[id],
	}, []byte{}, nil
}

func (m *mockImageClient) ImageRemove(ctx context.Context, id string, opts image.RemoveOptions) ([]image.DeleteResponse, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.removed[id]++
	return []image.DeleteResponse{}, nil
}

type readErrorer struct {
	readErr    error
	closeError error
}

var _ io.ReadCloser = &readErrorer{}

func (r *readErrorer) Read(p []byte) (n int, err error) {
	return len(p), r.readErr
}

func (r *readErrorer) Close() error {
	return r.closeError
}

func TestDockerCoordinator_ConcurrentPulls(t *testing.T) {
	ci.Parallel(t)
	image := "foo"
	imageID := uuid.Generate()
	mapping := map[string]string{imageID: image}

	// Add a delay so we can get multiple queued up
	mock := newMockImageClient(mapping, 10*time.Millisecond)
	config := &dockerCoordinatorConfig{
		ctx:         context.Background(),
		logger:      testlog.HCLogger(t),
		cleanup:     true,
		client:      mock,
		removeDelay: 100 * time.Millisecond,
	}

	// Create a coordinator
	coordinator := newDockerCoordinator(config)

	id, _, _ := coordinator.PullImage(image, nil, uuid.Generate(), nil, 5*time.Minute, 2*time.Minute)
	for i := 0; i < 9; i++ {
		go func() {
			coordinator.PullImage(image, nil, uuid.Generate(), nil, 5*time.Minute, 2*time.Minute)
		}()
	}

	testutil.WaitForResult(func() (bool, error) {
		mock.lock.Lock()
		defer mock.lock.Unlock()
		p := mock.pulled[image]
		if p >= 10 {
			return false, fmt.Errorf("Wrong number of pulls: %d", p)
		}

		coordinator.imageLock.Lock()
		defer coordinator.imageLock.Unlock()
		// Check the reference count
		if references := coordinator.imageRefCount[id]; len(references) != 10 {
			return false, fmt.Errorf("Got reference count %d; want %d", len(references), 10)
		}

		// Ensure there is no pull future
		if len(coordinator.pullFutures) != 0 {
			return false, fmt.Errorf("Pull future exists after pull finished")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestDockerCoordinator_Pull_Remove(t *testing.T) {
	ci.Parallel(t)
	image := "foo"
	imageID := uuid.Generate()
	mapping := map[string]string{imageID: image}

	// Add a delay so we can get multiple queued up
	mock := newMockImageClient(mapping, 10*time.Millisecond)
	config := &dockerCoordinatorConfig{
		ctx:         context.Background(),
		logger:      testlog.HCLogger(t),
		cleanup:     true,
		client:      mock,
		removeDelay: 1 * time.Millisecond,
	}

	// Create a coordinator
	coordinator := newDockerCoordinator(config)

	id := ""
	callerIDs := make([]string, 10, 10)
	for i := 0; i < 10; i++ {
		callerIDs[i] = uuid.Generate()
		id, _, _ = coordinator.PullImage(image, nil, callerIDs[i], nil, 5*time.Minute, 2*time.Minute)
	}

	// Check the reference count
	if references := coordinator.imageRefCount[id]; len(references) != 10 {
		t.Fatalf("Got reference count %d; want %d", len(references), 10)
	}

	// Remove some
	for i := 0; i < 8; i++ {
		coordinator.RemoveImage(id, callerIDs[i])
	}

	// Check the reference count
	if references := coordinator.imageRefCount[id]; len(references) != 2 {
		t.Fatalf("Got reference count %d; want %d", len(references), 2)
	}

	// Remove all
	for i := 8; i < 10; i++ {
		coordinator.RemoveImage(id, callerIDs[i])
	}

	// Check the reference count
	if references := coordinator.imageRefCount[id]; len(references) != 0 {
		t.Fatalf("Got reference count %d; want %d", len(references), 0)
	}

	// Check that only one delete happened
	testutil.WaitForResult(func() (bool, error) {
		mock.lock.Lock()
		defer mock.lock.Unlock()
		removes := mock.removed[id]
		return removes == 1, fmt.Errorf("Wrong number of removes: %d", removes)
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Make sure there is no future still
	testutil.WaitForResult(func() (bool, error) {
		coordinator.imageLock.Lock()
		defer coordinator.imageLock.Unlock()
		_, ok := coordinator.deleteFuture[id]
		return !ok, fmt.Errorf("got delete future")
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

}

func TestDockerCoordinator_Remove_Cancel(t *testing.T) {
	ci.Parallel(t)
	image := "foo"
	imageID := uuid.Generate()
	mapping := map[string]string{imageID: image}

	mock := newMockImageClient(mapping, 1*time.Millisecond)
	config := &dockerCoordinatorConfig{
		ctx:         context.Background(),
		logger:      testlog.HCLogger(t),
		cleanup:     true,
		client:      mock,
		removeDelay: 100 * time.Millisecond,
	}

	// Create a coordinator
	coordinator := newDockerCoordinator(config)
	callerID := uuid.Generate()

	// Pull image
	id, _, _ := coordinator.PullImage(image, nil, callerID, nil, 5*time.Minute, 2*time.Minute)

	// Check the reference count
	if references := coordinator.imageRefCount[id]; len(references) != 1 {
		t.Fatalf("Got reference count %d; want %d", len(references), 1)
	}

	// Remove image
	coordinator.RemoveImage(id, callerID)

	// Check the reference count
	if references := coordinator.imageRefCount[id]; len(references) != 0 {
		t.Fatalf("Got reference count %d; want %d", len(references), 0)
	}

	// Pull image again within delay
	id, _, _ = coordinator.PullImage(image, nil, callerID, nil, 5*time.Minute, 2*time.Minute)

	// Check the reference count
	if references := coordinator.imageRefCount[id]; len(references) != 1 {
		t.Fatalf("Got reference count %d; want %d", len(references), 1)
	}

	// Check that only no delete happened
	if removes := mock.removed[id]; removes != 0 {
		t.Fatalf("Image deleted when it shouldn't have")
	}
}

func TestDockerCoordinator_No_Cleanup(t *testing.T) {
	ci.Parallel(t)
	image := "foo"
	imageID := uuid.Generate()
	mapping := map[string]string{imageID: image}

	mock := newMockImageClient(mapping, 1*time.Millisecond)
	config := &dockerCoordinatorConfig{
		ctx:         context.Background(),
		logger:      testlog.HCLogger(t),
		cleanup:     false,
		client:      mock,
		removeDelay: 1 * time.Millisecond,
	}

	// Create a coordinator
	coordinator := newDockerCoordinator(config)
	callerID := uuid.Generate()

	// Pull image
	id, _, _ := coordinator.PullImage(image, nil, callerID, nil, 5*time.Minute, 2*time.Minute)

	// Check the reference count
	if references := coordinator.imageRefCount[id]; len(references) != 0 {
		t.Fatalf("Got reference count %d; want %d", len(references), 0)
	}

	// Remove image
	coordinator.RemoveImage(id, callerID)

	// Check that only no delete happened
	if removes := mock.removed[id]; removes != 0 {
		t.Fatalf("Image deleted when it shouldn't have")
	}
}

func TestDockerCoordinator_Cleanup_HonorsCtx(t *testing.T) {
	ci.Parallel(t)
	image1ID := uuid.Generate()
	image2ID := uuid.Generate()

	mapping := map[string]string{image1ID: "foo", image2ID: "bar"}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mock := newMockImageClient(mapping, 1*time.Millisecond)
	config := &dockerCoordinatorConfig{
		ctx:         ctx,
		logger:      testlog.HCLogger(t),
		cleanup:     true,
		client:      mock,
		removeDelay: 1 * time.Millisecond,
	}

	// Create a coordinator
	coordinator := newDockerCoordinator(config)
	callerID := uuid.Generate()

	// Pull image
	id1, _, _ := coordinator.PullImage(image1ID, nil, callerID, nil, 5*time.Minute, 2*time.Minute)
	require.Len(t, coordinator.imageRefCount[id1], 1, "image reference count")

	id2, _, _ := coordinator.PullImage(image2ID, nil, callerID, nil, 5*time.Minute, 2*time.Minute)
	require.Len(t, coordinator.imageRefCount[id2], 1, "image reference count")

	// remove one image, cancel ctx, remove second, and assert only first image is cleanedup
	// Remove image
	coordinator.RemoveImage(id1, callerID)
	testutil.WaitForResult(func() (bool, error) {
		if _, ok := mock.removed[id1]; ok {
			return true, nil
		}
		return false, fmt.Errorf("expected image to delete found %v", mock.removed)
	}, func(err error) {
		require.NoError(t, err)
	})

	cancel()
	coordinator.RemoveImage(id2, callerID)

	// deletions occur in background, wait to ensure that
	// the image isn't deleted after a timeout
	time.Sleep(10 * time.Millisecond)

	// Check that only no delete happened
	require.Equal(t, map[string]int{id1: 1}, mock.removed, "removed images")
}

func TestDockerCoordinator_PullImage_ProgressError(t *testing.T) {
	// testing: "error reading image pull progress"

	ci.Parallel(t)

	timeout := time.Second // shut down the driver in 1s (should not happen)
	driverCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	mapping := map[string]string{uuid.Generate(): "foo"}
	mock := newMockImageClient(mapping, 1*time.Millisecond)
	config := &dockerCoordinatorConfig{
		ctx:         driverCtx,
		logger:      testlog.HCLogger(t),
		cleanup:     true,
		client:      mock,
		removeDelay: 1 * time.Millisecond,
	}
	coordinator := newDockerCoordinator(config)

	readErr := errors.New("a bad bad thing happened")
	mock.pullReader = &readErrorer{readErr: readErr}

	_, _, err := coordinator.PullImage("foo", nil, uuid.Generate(), nil, timeout, timeout)
	must.ErrorIs(t, err, readErr)
}

func TestDockerCoordinator_PullImage_Timeouts(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name          string
		driverTimeout time.Duration // used in driver context to simulate driver/agent shutdown
		pullTimeout   time.Duration // user provided `image_pull_timeout`
		pullDelay     time.Duration // mock delay - how long it "actually" takes to pull the image
		expectErr     string
	}{
		{
			name:          "pull completes",
			pullDelay:     10 * time.Millisecond,
			pullTimeout:   200 * time.Millisecond,
			driverTimeout: 400 * time.Millisecond,
			expectErr:     "",
		},
		{
			name:          "pull timeout",
			pullDelay:     400 * time.Millisecond,
			pullTimeout:   10 * time.Millisecond,
			driverTimeout: 200 * time.Millisecond,
			expectErr:     "mockImageClient.ImagePull aborted",
		},
		{
			name:          "driver shutdown",
			pullDelay:     400 * time.Millisecond,
			pullTimeout:   200 * time.Millisecond,
			driverTimeout: 10 * time.Millisecond,
			expectErr:     "wait aborted",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			driverCtx, cancel := context.WithTimeout(context.Background(), tc.driverTimeout)
			defer cancel()

			mapping := map[string]string{"foo:v1": "foo"}
			mock := newMockImageClient(mapping, tc.pullDelay)
			config := &dockerCoordinatorConfig{
				ctx:         driverCtx,
				logger:      testlog.HCLogger(t),
				cleanup:     true,
				client:      mock,
				removeDelay: 1 * time.Millisecond,
			}
			coordinator := newDockerCoordinator(config)
			progressTimeout := 10 * time.Millisecond // does not apply here

			id, _, err := coordinator.PullImage("foo:v1", nil, uuid.Generate(), nil,
				tc.pullTimeout, progressTimeout)

			if tc.expectErr == "" {
				must.NoError(t, err)
				must.Eq(t, "foo", id)
			} else {
				must.ErrorIs(t, err, context.DeadlineExceeded)
				must.ErrorContains(t, err, tc.expectErr)
			}
		})
	}
}
