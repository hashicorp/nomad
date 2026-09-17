package filesystem

import (
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-set/v3"
)

const (
	// idUnsupported is what the uid/gid will be set to on platforms (eg
	// Windows) that don't support integer ownership identifiers.
	idUnsupported = -1

	// fileMode777 is a constant that represents the file mode rwxrwxrwx
	fileMode777 = os.FileMode(0o777)

	// fileMode710 is a constant that represents the file mode rwx--x---
	fileMode710 = os.FileMode(0o710)

	// fileMode755 is a constant that represents the file mode rwxr-xr-x
	fileMode755 = os.FileMode(0o755)

	// fileMode666 is a constant that represents the file mode rw-rw-rw-
	fileMode666 = os.FileMode(0o666)
)

var (
	// SnapshotErrorTime is the sentinel time that will be used on the
	// error file written by Snapshot when it encounters as error.
	SnapshotErrorTime = time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)

	// The name of the directory that is shared across tasks in a task group.
	SharedAllocName = "alloc"

	// Name of the directory where logs of Tasks are written
	LogDirName = "logs"

	// SharedDataDir is one of the shared allocation directories. It is
	// included in snapshots.
	SharedDataDir = "data"

	// TmpDirName is the name of the temporary directory in each alloc and
	// task.
	TmpDirName = "tmp"

	// The set of directories that exist inside each shared alloc directory.
	SharedAllocDirs = []string{LogDirName, TmpDirName, SharedDataDir}

	// The name of the directory that exists inside each task directory
	// regardless of driver.
	TaskLocal = "local"

	// TaskSecrets is the name of the secret directory inside each task
	// directory
	TaskSecrets = "secrets"

	// TaskPrivate is the name of the private directory inside each task
	// directory
	TaskPrivate = "private"

	// TaskDirs is the set of directories created in each tasks directory.
	TaskDirs = map[string]os.FileMode{TmpDirName: os.ModeSticky | fileMode777}

	// AllocGRPCSocket is the path relative to the task dir root for the
	// unix socket connected to Consul's gRPC endpoint.
	AllocGRPCSocket = filepath.Join(SharedAllocName, TmpDirName, "consul_grpc.sock")

	// AllocHTTPSocket is the path relative to the task dir root for the unix
	// socket connected to Consul's HTTP endpoint.
	AllocHTTPSocket = filepath.Join(SharedAllocName, TmpDirName, "consul_http.sock")
)

type TaskDir struct {
	// AllocDir is the path to the alloc directory on the host.
	// (not to be conflated with client.alloc_dir)
	//
	// <alloc_dir>
	AllocDir string

	// Dir is the path to Task directory on the host.
	//
	// <task_dir>
	Dir string

	// MountsAllocDir is the path to the alloc directory on the host that has
	// been bind mounted under <client.mounts_dir>
	//
	// <client.mounts_dir>/<allocid-task>/alloc -> <alloc_dir>
	MountsAllocDir string

	// MountsTaskDir is the path to the task directory on the host that has been
	// bind mounted under <client.mounts_dir>
	//
	// <client.mounts_dir>/<allocid-task>/task -> <task_dir>
	MountsTaskDir string

	// MountsSecretsDir is the path to the secrets directory on the host that
	// has been bind mounted under <client.mounts_dir>
	//
	// <client.mounts_dir>/<allocid-task>/task/secrets -> <secrets_dir>
	MountsSecretsDir string

	// SharedAllocDir is the path to shared alloc directory on the host
	//
	// <alloc_dir>/alloc/
	SharedAllocDir string

	// SharedTaskDir is the path to the shared alloc directory linked into
	// the task directory on the host.
	//
	// <task_dir>/alloc/
	SharedTaskDir string

	// LocalDir is the path to the task's local directory on the host
	//
	// <task_dir>/local/
	LocalDir string

	// LogDir is the path to the task's log directory on the host
	//
	// <alloc_dir>/alloc/logs/
	LogDir string

	// SecretsDir is the path to secrets/ directory on the host
	//
	// <task_dir>/secrets/
	SecretsDir string

	// secretsInMB is the configured size of the secrets directory
	secretsInMB int

	// PrivateDir is the path to private/ directory on the host
	//
	// <task_dir>/private/
	PrivateDir string

	// skip embedding these paths in chroots. Used for avoiding embedding
	// client.alloc_dir and client.mounts_dir recursively.
	skip *set.Set[string]

	// logger for this task
	logger hclog.Logger
}
