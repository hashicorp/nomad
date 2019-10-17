package docker

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/stretchr/testify/require"
)

func fakeContainerList(t *testing.T) (nomadContainer, nonNomadContainer docker.APIContainers) {
	path := "./test-resources/docker/reconciler_containers_list.json"

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}

	var sampleContainerList []docker.APIContainers
	err = json.NewDecoder(f).Decode(&sampleContainerList)
	if err != nil {
		t.Fatalf("failed to decode container list: %v", err)
	}

	return sampleContainerList[0], sampleContainerList[1]
}

func Test_HasMount(t *testing.T) {
	nomadContainer, nonNomadContainer := fakeContainerList(t)

	require.True(t, hasMount(nomadContainer, "/alloc"))
	require.True(t, hasMount(nomadContainer, "/data"))
	require.True(t, hasMount(nomadContainer, "/secrets"))
	require.False(t, hasMount(nomadContainer, "/random"))

	require.False(t, hasMount(nonNomadContainer, "/alloc"))
	require.False(t, hasMount(nonNomadContainer, "/data"))
	require.False(t, hasMount(nonNomadContainer, "/secrets"))
	require.False(t, hasMount(nonNomadContainer, "/random"))
}

func Test_HasNomadName(t *testing.T) {
	nomadContainer, nonNomadContainer := fakeContainerList(t)

	require.True(t, hasNomadName(nomadContainer))
	require.False(t, hasNomadName(nonNomadContainer))
}

// TestDanglingContainerRemoval asserts containers without corresponding tasks
// are removed after the creation grace period.
func TestDanglingContainerRemoval(t *testing.T) {
	testutil.DockerCompatible(t)

	// start two containers: one tracked nomad container, and one unrelated container
	task, cfg, _ := dockerTask(t)
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	client, d, handle, cleanup := dockerSetup(t, task)
	defer cleanup()
	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	c, err := client.CreateContainer(docker.CreateContainerOptions{
		Name: "mytest-image-" + uuid.Generate(),
		Config: &docker.Config{
			Image: cfg.Image,
			Cmd:   append([]string{cfg.Command}, cfg.Args...),
		},
	})
	require.NoError(t, err)
	defer client.RemoveContainer(docker.RemoveContainerOptions{
		ID:    c.ID,
		Force: true,
	})

	err = client.StartContainer(c.ID, nil)
	require.NoError(t, err)

	dd := d.Impl().(*Driver)

	reconciler := newReconciler(dd)
	trackedContainers := map[string]bool{handle.containerID: true}

	tf := reconciler.trackedContainers()
	require.Contains(t, tf, handle.containerID)
	require.NotContains(t, tf, c.ID)

	// assert tracked containers should never be untracked
	untracked, err := reconciler.untrackedContainers(trackedContainers, time.Now())
	require.NoError(t, err)
	require.NotContains(t, untracked, handle.containerID)
	require.NotContains(t, untracked, c.ID)

	// assert we recognize nomad containers with appropriate cutoff
	untracked, err = reconciler.untrackedContainers(map[string]bool{}, time.Now())
	require.NoError(t, err)
	require.Contains(t, untracked, handle.containerID)
	require.NotContains(t, untracked, c.ID)

	// but ignore if creation happened before cutoff
	untracked, err = reconciler.untrackedContainers(map[string]bool{}, time.Now().Add(-1*time.Minute))
	require.NoError(t, err)
	require.NotContains(t, untracked, handle.containerID)
	require.NotContains(t, untracked, c.ID)

	// a full integration tests to assert that containers are removed
	prestineDriver := dockerDriverHarness(t, nil).Impl().(*Driver)
	prestineDriver.config.GC.DanglingContainers = ContainerGCConfig{
		Enabled:       true,
		period:        1 * time.Second,
		CreationGrace: 1 * time.Second,
	}
	nReconciler := newReconciler(prestineDriver)

	require.NoError(t, nReconciler.removeDanglingContainersIteration())

	_, err = client.InspectContainer(c.ID)
	require.NoError(t, err)

	_, err = client.InspectContainer(handle.containerID)
	require.Error(t, err)
	require.Contains(t, err.Error(), NoSuchContainerError)
}

// TestDanglingContainerRemoval_Stopped asserts stopped containers without
// corresponding tasks are not removed even if after creation grace period.
func TestDanglingContainerRemoval_Stopped(t *testing.T) {
	testutil.DockerCompatible(t)

	task, cfg, _ := dockerTask(t)
	task.Resources.NomadResources.Networks = nil
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	// Start two containers: one nomad container, and one stopped container
	// that acts like a nomad one
	client, d, handle, cleanup := dockerSetup(t, task)
	defer cleanup()
	require.NoError(t, d.WaitUntilStarted(task.ID, 5*time.Second))

	inspected, err := client.InspectContainer(handle.containerID)
	require.NoError(t, err)

	stoppedC, err := client.CreateContainer(docker.CreateContainerOptions{
		Name:       "mytest-image-" + uuid.Generate(),
		Config:     inspected.Config,
		HostConfig: inspected.HostConfig,
	})
	require.NoError(t, err)
	defer client.RemoveContainer(docker.RemoveContainerOptions{
		ID:    stoppedC.ID,
		Force: true,
	})

	err = client.StartContainer(stoppedC.ID, nil)
	require.NoError(t, err)

	err = client.StopContainer(stoppedC.ID, 60)
	require.NoError(t, err)

	dd := d.Impl().(*Driver)
	reconciler := newReconciler(dd)
	trackedContainers := map[string]bool{handle.containerID: true}

	// assert nomad container is tracked, and we ignore stopped one
	tf := reconciler.trackedContainers()
	require.Contains(t, tf, handle.containerID)
	require.NotContains(t, tf, stoppedC.ID)

	untracked, err := reconciler.untrackedContainers(trackedContainers, time.Now())
	require.NoError(t, err)
	require.NotContains(t, untracked, handle.containerID)
	require.NotContains(t, untracked, stoppedC.ID)

	// if we start container again, it'll be marked as untracked
	require.NoError(t, client.StartContainer(stoppedC.ID, nil))

	untracked, err = reconciler.untrackedContainers(trackedContainers, time.Now())
	require.NoError(t, err)
	require.NotContains(t, untracked, handle.containerID)
	require.Contains(t, untracked, stoppedC.ID)
}
