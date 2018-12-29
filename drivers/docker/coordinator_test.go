package docker

import (
	"fmt"
	"sync"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/testutil"
)

type mockImageClient struct {
	pulled    map[string]int
	idToName  map[string]string
	removed   map[string]int
	pullDelay time.Duration
	lock      sync.Mutex
}

func newMockImageClient(idToName map[string]string, pullDelay time.Duration) *mockImageClient {
	return &mockImageClient{
		pulled:    make(map[string]int),
		removed:   make(map[string]int),
		idToName:  idToName,
		pullDelay: pullDelay,
	}
}

func (m *mockImageClient) PullImage(opts docker.PullImageOptions, auth docker.AuthConfiguration) error {
	time.Sleep(m.pullDelay)
	m.lock.Lock()
	defer m.lock.Unlock()
	m.pulled[opts.Repository]++
	return nil
}

func (m *mockImageClient) InspectImage(id string) (*docker.Image, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	return &docker.Image{
		ID: m.idToName[id],
	}, nil
}

func (m *mockImageClient) RemoveImage(id string) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.removed[id]++
	return nil
}

func TestDockerCoordinator_ConcurrentPulls(t *testing.T) {
	t.Parallel()
	image := "foo"
	imageID := uuid.Generate()
	mapping := map[string]string{imageID: image}

	// Add a delay so we can get multiple queued up
	mock := newMockImageClient(mapping, 10*time.Millisecond)
	config := &dockerCoordinatorConfig{
		logger:      testlog.HCLogger(t),
		cleanup:     true,
		client:      mock,
		removeDelay: 100 * time.Millisecond,
	}

	// Create a coordinator
	coordinator := newDockerCoordinator(config)

	id, _ := coordinator.PullImage(image, nil, uuid.Generate(), nil)
	for i := 0; i < 9; i++ {
		go func() {
			coordinator.PullImage(image, nil, uuid.Generate(), nil)
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
	t.Parallel()
	image := "foo"
	imageID := uuid.Generate()
	mapping := map[string]string{imageID: image}

	// Add a delay so we can get multiple queued up
	mock := newMockImageClient(mapping, 10*time.Millisecond)
	config := &dockerCoordinatorConfig{
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
		id, _ = coordinator.PullImage(image, nil, callerIDs[i], nil)
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
	coordinator.imageLock.Lock()
	if _, ok := coordinator.deleteFuture[id]; ok {
		t.Fatal("Got delete future")
	}
	coordinator.imageLock.Unlock()
}

func TestDockerCoordinator_Remove_Cancel(t *testing.T) {
	t.Parallel()
	image := "foo"
	imageID := uuid.Generate()
	mapping := map[string]string{imageID: image}

	mock := newMockImageClient(mapping, 1*time.Millisecond)
	config := &dockerCoordinatorConfig{
		logger:      testlog.HCLogger(t),
		cleanup:     true,
		client:      mock,
		removeDelay: 100 * time.Millisecond,
	}

	// Create a coordinator
	coordinator := newDockerCoordinator(config)
	callerID := uuid.Generate()

	// Pull image
	id, _ := coordinator.PullImage(image, nil, callerID, nil)

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
	id, _ = coordinator.PullImage(image, nil, callerID, nil)

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
	t.Parallel()
	image := "foo"
	imageID := uuid.Generate()
	mapping := map[string]string{imageID: image}

	mock := newMockImageClient(mapping, 1*time.Millisecond)
	config := &dockerCoordinatorConfig{
		logger:      testlog.HCLogger(t),
		cleanup:     false,
		client:      mock,
		removeDelay: 1 * time.Millisecond,
	}

	// Create a coordinator
	coordinator := newDockerCoordinator(config)
	callerID := uuid.Generate()

	// Pull image
	id, _ := coordinator.PullImage(image, nil, callerID, nil)

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
