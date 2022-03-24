//go:build linux

package docker

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
)

func TestCoordinate_NomadCgroup(t *testing.T) {
	ci.Parallel(t)

	result := (coordinate{
		containerID: "c6d05b36f4f56619ca59fbce921115e87dda1661860a4670e3e35ecfa3571ba1",
		allocID:     "27ee5321-28d6-22d7-9426-4e1888da8e7d",
		task:        "redis",
		path:        "/nomad.scope/27ee5321-28d6-22d7-9426-4e1888da8e7d.redis.scope",
	}).NomadCgroup()
	exp := "/sys/fs/cgroup/nomad.scope/27ee5321-28d6-22d7-9426-4e1888da8e7d.redis.scope"
	require.Equal(t, exp, result)
}

func TestCoordinate_DockerCgroup(t *testing.T) {
	ci.Parallel(t)

	result := (coordinate{
		containerID: "c6d05b36f4f56619ca59fbce921115e87dda1661860a4670e3e35ecfa3571ba1",
		allocID:     "27ee5321-28d6-22d7-9426-4e1888da8e7d",
		task:        "redis",
		path:        "/nomad.scope/27ee5321-28d6-22d7-9426-4e1888da8e7d.redis.scope",
	}).DockerCgroup()
	exp := "/sys/fs/cgroup/nomad.scope/docker-c6d05b36f4f56619ca59fbce921115e87dda1661860a4670e3e35ecfa3571ba1.scope"
	require.Equal(t, exp, result)
}
