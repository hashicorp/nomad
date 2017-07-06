package getter

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"sync"

	gg "github.com/hashicorp/go-getter"
	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	// getters is the map of getters suitable for Nomad. It is initialized once
	// and the lock is used to guard access to it.
	getters map[string]gg.Getter
	lock    sync.Mutex

	// supported is the set of download schemes supported by Nomad
	supported = []string{"http", "https", "s3", "hg", "git"}
)

const (
	// gitSSHPrefix is the prefix for dowwnloading via git using ssh
	gitSSHPrefix = "git@github.com:"
)

// EnvReplacer is an interface which can interpolate environment variables and
// is usually satisfied by env.TaskEnv.
type EnvReplacer interface {
	ReplaceEnv(string) string
}

// getClient returns a client that is suitable for Nomad downloading artifacts.
func getClient(src string, mode gg.ClientMode, dst string) *gg.Client {
	lock.Lock()
	defer lock.Unlock()

	// Return the pre-initialized client
	if getters == nil {
		getters = make(map[string]gg.Getter, len(supported))
		for _, getter := range supported {
			if impl, ok := gg.Getters[getter]; ok {
				getters[getter] = impl
			}
		}
	}

	return &gg.Client{
		Src:     src,
		Dst:     dst,
		Mode:    mode,
		Getters: getters,
	}
}

// getGetterUrl returns the go-getter URL to download the artifact.
func getGetterUrl(taskEnv EnvReplacer, artifact *structs.TaskArtifact) (string, error) {
	source := taskEnv.ReplaceEnv(artifact.GetterSource)

	// Handle an invalid URL when given a go-getter url such as
	// git@github.com:hashicorp/nomad.git
	gitSSH := false
	if strings.HasPrefix(source, gitSSHPrefix) {
		gitSSH = true
		source = source[len(gitSSHPrefix):]
	}

	u, err := url.Parse(source)
	if err != nil {
		return "", fmt.Errorf("failed to parse source URL %q: %v", artifact.GetterSource, err)
	}

	// Build the url
	q := u.Query()
	for k, v := range artifact.GetterOptions {
		q.Add(k, taskEnv.ReplaceEnv(v))
	}
	u.RawQuery = q.Encode()

	// Add the prefix back
	url := u.String()
	if gitSSH {
		url = fmt.Sprintf("%s%s", gitSSHPrefix, url)
	}

	return url, nil
}

// GetArtifact downloads an artifact into the specified task directory.
func GetArtifact(taskEnv EnvReplacer, artifact *structs.TaskArtifact, taskDir string) error {
	url, err := getGetterUrl(taskEnv, artifact)
	if err != nil {
		return newGetError(artifact.GetterSource, err, false)
	}

	// Download the artifact
	dest := filepath.Join(taskDir, artifact.RelativeDest)

	// Convert from string getter mode to go-getter const
	mode := gg.ClientModeAny
	switch artifact.GetterMode {
	case structs.GetterModeFile:
		mode = gg.ClientModeFile
	case structs.GetterModeDir:
		mode = gg.ClientModeDir
	}

	if err := getClient(url, mode, dest).Get(); err != nil {
		return newGetError(url, err, true)
	}

	return nil
}

// GetError wraps the underlying artifact fetching error with the URL. It
// implements the RecoverableError interface.
type GetError struct {
	URL         string
	Err         error
	recoverable bool
}

func newGetError(url string, err error, recoverable bool) *GetError {
	return &GetError{
		URL:         url,
		Err:         err,
		recoverable: recoverable,
	}
}

func (g *GetError) Error() string {
	return g.Err.Error()
}

func (g *GetError) IsRecoverable() bool {
	return g.recoverable
}
