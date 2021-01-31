package getter

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
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
	supported = []string{"http", "https", "s3", "hg", "git", "gcs"}
)

const (
	// gitSSHPrefix is the prefix for downloading via git using ssh
	gitSSHPrefix = "git@github.com:"
)

// EnvReplacer is an interface which can interpolate environment variables and
// is usually satisfied by taskenv.TaskEnv.
type EnvReplacer interface {
	ReplaceEnv(string) string
	ClientPath(string, bool) (string, bool)
}

func makeGetters(headers http.Header) map[string]gg.Getter {
	getters := make(map[string]gg.Getter, len(supported))
	for _, getter := range supported {
		switch {
		case getter == "http" && len(headers) > 0:
			fallthrough
		case getter == "https" && len(headers) > 0:
			getters[getter] = &gg.HttpGetter{
				Netrc:  true,
				Header: headers,
			}
		default:
			if defaultGetter, ok := gg.Getters[getter]; ok {
				getters[getter] = defaultGetter
			}
		}
	}
	return getters
}

// getClient returns a client that is suitable for Nomad downloading artifacts.
func getClient(src string, headers http.Header, mode gg.ClientMode, dst string) *gg.Client {
	client := &gg.Client{
		Src:   src,
		Dst:   dst,
		Mode:  mode,
		Umask: 060000000,
	}

	switch len(headers) {
	case 0:
		// When no headers are present use the memoized getters, creating them
		// on demand if they do not exist yet.
		lock.Lock()
		if getters == nil {
			getters = makeGetters(nil)
		}
		lock.Unlock()
		client.Getters = getters
	default:
		// When there are headers present, we must create fresh gg.HttpGetter
		// objects, because that is where gg stores the headers to use in its
		// artifact HTTP GET requests.
		client.Getters = makeGetters(headers)
	}

	return client
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
	ggURL := u.String()
	if gitSSH {
		ggURL = fmt.Sprintf("%s%s", gitSSHPrefix, ggURL)
	}

	return ggURL, nil
}

func getHeaders(env EnvReplacer, m map[string]string) http.Header {
	if len(m) == 0 {
		return nil
	}

	headers := make(http.Header, len(m))
	for k, v := range m {
		headers.Set(k, env.ReplaceEnv(v))
	}
	return headers
}

// GetArtifact downloads an artifact into the specified task directory.
func GetArtifact(taskEnv EnvReplacer, artifact *structs.TaskArtifact) error {
	ggURL, err := getGetterUrl(taskEnv, artifact)
	if err != nil {
		return newGetError(artifact.GetterSource, err, false)
	}

	dest, escapes := taskEnv.ClientPath(artifact.RelativeDest, true)
	// Verify the destination is still in the task sandbox after interpolation
	if escapes {
		return newGetError(artifact.RelativeDest,
			errors.New("artifact destination path escapes the alloc directory"),
			false)
	}

	// Convert from string getter mode to go-getter const
	mode := gg.ClientModeAny
	switch artifact.GetterMode {
	case structs.GetterModeFile:
		mode = gg.ClientModeFile
	case structs.GetterModeDir:
		mode = gg.ClientModeDir
	}

	headers := getHeaders(taskEnv, artifact.GetterHeaders)
	if err := getClient(ggURL, headers, mode, dest).Get(); err != nil {
		return newGetError(ggURL, err, true)
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
