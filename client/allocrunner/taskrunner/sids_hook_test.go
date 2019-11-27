package taskrunner

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

var _ interfaces.TaskPrestartHook = (*sidsHook)(nil)

func tmpDir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "sids-")
	require.NoError(t, err)
	return dir
}

func cleanupDir(t *testing.T, dir string) {
	err := os.RemoveAll(dir)
	require.NoError(t, err)
}

func TestSIDSHook_recoverToken(t *testing.T) {
	t.Parallel()

	r := require.New(t)
	secrets := tmpDir(t)
	defer cleanupDir(t, secrets)

	h := newSIDSHook(sidsHookConfig{
		task:   &structs.Task{Name: "task1"},
		logger: testlog.HCLogger(t),
	})

	expected := "12345678-1234-1234-1234-1234567890"
	err := h.writeToken(secrets, expected)
	r.NoError(err)

	token, err := h.recoverToken(secrets)
	r.NoError(err)
	r.Equal(expected, token)
}

func TestSIDSHook_recoverToken_empty(t *testing.T) {
	t.Parallel()

	r := require.New(t)
	secrets := tmpDir(t)
	defer cleanupDir(t, secrets)

	h := newSIDSHook(sidsHookConfig{
		task:   &structs.Task{Name: "task1"},
		logger: testlog.HCLogger(t),
	})

	token, err := h.recoverToken(secrets)
	r.NoError(err)
	r.Empty(token)
}

func TestSIDSHook_deriveSIToken(t *testing.T) {
	t.Parallel()

	r := require.New(t)
	secrets := tmpDir(t)
	defer cleanupDir(t, secrets)

	h := newSIDSHook(sidsHookConfig{
		alloc:      &structs.Allocation{ID: "a1"},
		task:       &structs.Task{Name: "task1"},
		logger:     testlog.HCLogger(t),
		sidsClient: consul.NewMockServiceIdentitiesClient(),
	})

	ctx := context.Background()
	token, err := h.deriveSIToken(ctx)
	r.NoError(err)
	r.True(helper.IsUUID(token))
}

func TestSIDSHook_computeBackoff(t *testing.T) {
	t.Parallel()

	try := func(i int, exp time.Duration) {
		result := computeBackoff(i)
		require.Equal(t, exp, result)
	}

	try(0, time.Duration(0))
	try(1, 20*time.Second)
	try(2, 80*time.Second)
	try(3, 320*time.Second)
	try(4, sidsBackoffLimit)
}

func TestSIDSHook_backoff(t *testing.T) {
	t.Parallel()
	r := require.New(t)

	ctx := context.Background()
	stop := !backoff(ctx, 0)
	r.False(stop)
}

func TestSIDSHook_backoffKilled(t *testing.T) {
	t.Parallel()
	r := require.New(t)

	ctx, cancel := context.WithTimeout(context.Background(), 1)
	defer cancel()

	stop := !backoff(ctx, 1000)
	r.True(stop)
}
