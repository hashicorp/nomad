package driver

import (
	"fmt"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

type mockImageClient struct {
	pulled    map[string]int
	idToName  map[string]string
	removed   map[string]int
	pullDelay time.Duration
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
	m.pulled[opts.Repository]++
	return nil
}

func (m *mockImageClient) InspectImage(id string) (*docker.Image, error) {
	return &docker.Image{
		ID: m.idToName[id],
	}, nil
}

func (m *mockImageClient) RemoveImage(id string) error {
	m.removed[id]++
	return nil
}

func TestDockerCoordinator_ConcurrentPulls(t *testing.T) {
	image := "foo"
	imageID := structs.GenerateUUID()
	mapping := map[string]string{imageID: image}

	// Add a delay so we can get multiple queued up
	mock := newMockImageClient(mapping, 10*time.Millisecond)
	config := &dockerCoordinatorConfig{
		logger:      testLogger(),
		cleanup:     true,
		client:      mock,
		removeDelay: 100 * time.Millisecond,
	}

	// Create a coordinator
	coordinator := NewDockerCoordinator(config)

	id := ""
	for i := 0; i < 10; i++ {
		id, _ = coordinator.PullImage(image, nil)
	}

	if p := mock.pulled[image]; p != 1 {
		t.Fatalf("Got multiple pulls %d", p)
	}

	// Check the reference count
	if r := coordinator.imageRefCount[id]; r != 10 {
		t.Fatalf("Got reference count %d; want %d", r, 10)
	}
}

func TestDockerCoordinator_Pull_Remove(t *testing.T) {
	image := "foo"
	imageID := structs.GenerateUUID()
	mapping := map[string]string{imageID: image}

	// Add a delay so we can get multiple queued up
	mock := newMockImageClient(mapping, 10*time.Millisecond)
	config := &dockerCoordinatorConfig{
		logger:      testLogger(),
		cleanup:     true,
		client:      mock,
		removeDelay: 1 * time.Millisecond,
	}

	// Create a coordinator
	coordinator := NewDockerCoordinator(config)

	id := ""
	for i := 0; i < 10; i++ {
		id, _ = coordinator.PullImage(image, nil)
	}

	// Check the reference count
	if r := coordinator.imageRefCount[id]; r != 10 {
		t.Fatalf("Got reference count %d; want %d", r, 10)
	}

	// Remove some
	for i := 0; i < 8; i++ {
		coordinator.RemoveImage(id)
	}

	// Check the reference count
	if r := coordinator.imageRefCount[id]; r != 2 {
		t.Fatalf("Got reference count %d; want %d", r, 2)
	}

	// Remove all
	for i := 0; i < 2; i++ {
		coordinator.RemoveImage(id)
	}

	// Check the reference count
	if r := coordinator.imageRefCount[id]; r != 0 {
		t.Fatalf("Got reference count %d; want %d", r, 0)
	}

	// Check that only one delete happened
	testutil.WaitForResult(func() (bool, error) {
		removes := mock.removed[id]
		return removes == 1, fmt.Errorf("Wrong number of removes: %d", removes)
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Make sure there is no future still
	if _, ok := coordinator.deleteFuture[id]; ok {
		t.Fatal("Got delete future")
	}
}

func TestDockerCoordinator_Remove_Cancel(t *testing.T) {
	image := "foo"
	imageID := structs.GenerateUUID()
	mapping := map[string]string{imageID: image}

	mock := newMockImageClient(mapping, 1*time.Millisecond)
	config := &dockerCoordinatorConfig{
		logger:      testLogger(),
		cleanup:     true,
		client:      mock,
		removeDelay: 1 * time.Millisecond,
	}

	// Create a coordinator
	coordinator := NewDockerCoordinator(config)

	// Pull image
	id, _ := coordinator.PullImage(image, nil)

	// Check the reference count
	if r := coordinator.imageRefCount[id]; r != 1 {
		t.Fatalf("Got reference count %d; want %d", r, 10)
	}

	// Remove image
	coordinator.RemoveImage(id)

	// Check the reference count
	if r := coordinator.imageRefCount[id]; r != 0 {
		t.Fatalf("Got reference count %d; want %d", r, 0)
	}

	// Pull image again within delay
	id, _ = coordinator.PullImage(image, nil)

	// Check the reference count
	if r := coordinator.imageRefCount[id]; r != 1 {
		t.Fatalf("Got reference count %d; want %d", r, 0)
	}

	// Check that only no delete happened
	if removes := mock.removed[id]; removes != 0 {
		t.Fatalf("Image deleted when it shouldn't have")
	}
}

func TestDockerCoordinator_No_Cleanup(t *testing.T) {
	image := "foo"
	imageID := structs.GenerateUUID()
	mapping := map[string]string{imageID: image}

	mock := newMockImageClient(mapping, 1*time.Millisecond)
	config := &dockerCoordinatorConfig{
		logger:      testLogger(),
		cleanup:     false,
		client:      mock,
		removeDelay: 1 * time.Millisecond,
	}

	// Create a coordinator
	coordinator := NewDockerCoordinator(config)

	// Pull image
	id, _ := coordinator.PullImage(image, nil)

	// Check the reference count
	if r := coordinator.imageRefCount[id]; r != 0 {
		t.Fatalf("Got reference count %d; want %d", r, 10)
	}

	// Remove image
	coordinator.RemoveImage(id)

	// Check that only no delete happened
	if removes := mock.removed[id]; removes != 0 {
		t.Fatalf("Image deleted when it shouldn't have")
	}
}
