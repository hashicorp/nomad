package docker

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/uuid"
	tu "github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

var sampleContainerList []docker.APIContainers
var sampleNomadContainerListItem docker.APIContainers
var sampleNonNomadContainerListItem docker.APIContainers

func init() {
	path := "./test-resources/docker/reconciler_containers_list.json"

	f, err := os.Open(path)
	if err != nil {
		return
	}

	err = json.NewDecoder(f).Decode(&sampleContainerList)
	if err != nil {
		return
	}

	sampleNomadContainerListItem = sampleContainerList[0]
	sampleNonNomadContainerListItem = sampleContainerList[1]
}

func Test_HasMount(t *testing.T) {
	require.True(t, hasMount(sampleNomadContainerListItem, "/alloc"))
	require.True(t, hasMount(sampleNomadContainerListItem, "/data"))
	require.True(t, hasMount(sampleNomadContainerListItem, "/secrets"))
	require.False(t, hasMount(sampleNomadContainerListItem, "/random"))

	require.False(t, hasMount(sampleNonNomadContainerListItem, "/alloc"))
	require.False(t, hasMount(sampleNonNomadContainerListItem, "/data"))
	require.False(t, hasMount(sampleNonNomadContainerListItem, "/secrets"))
	require.False(t, hasMount(sampleNonNomadContainerListItem, "/random"))
}

func Test_HasNomadName(t *testing.T) {
	require.True(t, hasNomadName(sampleNomadContainerListItem))
	require.False(t, hasNomadName(sampleNonNomadContainerListItem))
}

func TestHasEnv(t *testing.T) {
	envvars := []string{
		"NOMAD_ALLOC_DIR=/alloc",
		"NOMAD_ALLOC_ID=72bfa388-024e-a903-45b8-2bc28b74ed69",
		"NOMAD_ALLOC_INDEX=0",
		"NOMAD_ALLOC_NAME=example.cache[0]",
		"NOMAD_CPU_LIMIT=500",
		"NOMAD_DC=dc1",
		"NOMAD_GROUP_NAME=cache",
		"NOMAD_JOB_NAME=example",
		"NOMAD_MEMORY_LIMIT=256",
		"NOMAD_NAMESPACE=default",
		"NOMAD_REGION=global",
		"NOMAD_SECRETS_DIR=/secrets",
		"NOMAD_TASK_DIR=/local",
		"NOMAD_TASK_NAME=redis",
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"GOSU_VERSION=1.10",
		"REDIS_VERSION=3.2.12",
		"REDIS_DOWNLOAD_URL=http://download.redis.io/releases/redis-3.2.12.tar.gz",
		"REDIS_DOWNLOAD_SHA=98c4254ae1be4e452aa7884245471501c9aa657993e0318d88f048093e7f88fd",
	}

	require.True(t, hasEnvVar(envvars, "NOMAD_ALLOC_ID"))
	require.True(t, hasEnvVar(envvars, "NOMAD_ALLOC_DIR"))
	require.True(t, hasEnvVar(envvars, "GOSU_VERSION"))

	require.False(t, hasEnvVar(envvars, "NOMAD_ALLOC_"))
	require.False(t, hasEnvVar(envvars, "OTHER_VARIABLE"))
}

func TestDanglingContainerRemoval(t *testing.T) {
	if !tu.IsCI() {
		t.Parallel()
	}
	testutil.DockerCompatible(t)

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
	trackedContainers := map[string]bool{handle.containerID: true}

	{
		tf := dd.trackedContainers()
		require.Contains(t, tf, handle.containerID)
		require.NotContains(t, tf, c.ID)
	}

	untracked, err := dd.untrackedContainers(trackedContainers, 1*time.Minute)
	require.NoError(t, err)
	require.NotContains(t, untracked, handle.containerID)
	require.NotContains(t, untracked, c.ID)

	untracked, err = dd.untrackedContainers(map[string]bool{}, 0)
	require.NoError(t, err)
	require.Contains(t, untracked, handle.containerID)
	require.NotContains(t, untracked, c.ID)

	// Actually try to kill hosts
	prestineDriver := dockerDriverHarness(t, nil).Impl().(*Driver)
	prestineDriver.config.GC.DanglingContainers = ContainerGCConfig{
		Enabled:         true,
		period:          1 * time.Second,
		creationTimeout: 1 * time.Second,
	}
	require.NoError(t, prestineDriver.removeDanglingContainersIteration())

	_, err = client.InspectContainer(c.ID)
	require.NoError(t, err)

	_, err = client.InspectContainer(handle.containerID)
	require.Error(t, err)
	require.Contains(t, err.Error(), NoSuchContainerError)
}
