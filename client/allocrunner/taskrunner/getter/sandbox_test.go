package getter

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/users"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func setupDir(t *testing.T) (string, string) {
	uid, gid := users.NobodyIDs()

	allocDir := t.TempDir()
	taskDir := filepath.Join(allocDir, "local")
	topDir := filepath.Dir(allocDir)

	must.NoError(t, os.Chown(topDir, int(uid), int(gid)))
	must.NoError(t, os.Chmod(topDir, 0o755))

	must.NoError(t, os.Chown(allocDir, int(uid), int(gid)))
	must.NoError(t, os.Chmod(allocDir, 0o755))

	must.NoError(t, os.Mkdir(taskDir, 0o755))
	must.NoError(t, os.Chown(taskDir, int(uid), int(gid)))
	must.NoError(t, os.Chmod(taskDir, 0o755))
	return allocDir, taskDir
}

func artifactConfig(timeout time.Duration) *config.ArtifactConfig {
	return &config.ArtifactConfig{
		HTTPReadTimeout: timeout,
		HTTPMaxBytes:    1e6,
		GCSTimeout:      timeout,
		GitTimeout:      timeout,
		HgTimeout:       timeout,
		S3Timeout:       timeout,
	}
}

// comprehensive scenarios tested in e2e/artifact

func TestSandbox_Get_http(t *testing.T) {
	testutil.RequireRoot(t)
	logger := testlog.HCLogger(t)

	ac := artifactConfig(10 * time.Second)
	sbox := New(ac, logger)

	_, taskDir := setupDir(t)
	env := noopTaskEnv(taskDir)

	artifact := &structs.TaskArtifact{
		GetterSource: "https://raw.githubusercontent.com/hashicorp/go-set/main/go.mod",
		RelativeDest: "local/downloads",
	}

	err := sbox.Get(env, artifact)
	must.NoError(t, err)

	b, err := os.ReadFile(filepath.Join(taskDir, "local", "downloads", "go.mod"))
	must.NoError(t, err)
	must.StrContains(t, string(b), "module github.com/hashicorp/go-set")
}
