package getter

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
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
		Umask:   060000000,
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
func GetArtifact(taskEnv EnvReplacer, artifact *structs.TaskArtifact, taskDir string, cacheDir string) error {
	url, err := getGetterUrl(taskEnv, artifact)
	if err != nil {
		return newGetError(artifact.GetterSource, err, false)
	}

	// Convert from string getter mode to go-getter const
	mode := gg.ClientModeAny
	switch artifact.GetterMode {
	case structs.GetterModeFile:
		mode = gg.ClientModeFile
	case structs.GetterModeDir:
		mode = gg.ClientModeDir
	}

	dest := filepath.Join(taskDir, artifact.RelativeDest)
	if len(cacheDir) > 0 {
		// a cache directory has been provided, so check it for the resource
		// compute the cache key (sha1)
		hash := sha1.New()
		hash.Write([]byte(url))
		cacheKey := hex.EncodeToString(hash.Sum(nil))
		cacheFolder := filepath.Join(cacheDir, cacheKey)

		if _, err := os.Stat(cacheFolder); os.IsNotExist(err) {
			// file has not been cached yet, so download it
			if err := getClient(url, mode, cacheFolder).Get(); err != nil {
				return newGetError(url, err, true)
			}
		}

		// copy the cached asset over to the taskDir
		if fi, err := os.Stat(cacheFolder); err != nil {
			return newGetError(url, err, true)
		} else if fi.IsDir() {
			// we got a folder to replicate
			err = copyDir(cacheFolder, dest)
			if err != nil {
				return newGetError(url, err, true)
			}
		} else {
			// we got a single file to copy
			err = copyFile(cacheFolder, dest)
			if err != nil {
				return newGetError(url, err, true)
			}
		}
	} else {
		if err := getClient(url, mode, dest).Get(); err != nil {
			return newGetError(url, err, true)
		}
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

// copy a single file from src to dst
func copyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	srcF, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcF.Close()

	dstF, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstF.Close()

	_, err = io.Copy(dstF, srcF)
	if err != nil {
		return err
	}

	// Chmod it
	return os.Chmod(dst, srcInfo.Mode())
}

// copy an entire folder (including sub-folders) from src to dst
func copyDir(src, dst string) error {
	src, err := filepath.EvalSymlinks(src)
	if err != nil {
		return err
	}

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == src {
			return nil
		}

		// The "path" has the src prefixed to it. We need to join our
		// destination with the path without the src on it.
		dstPath := filepath.Join(dst, path[len(src):])

		// If we have a directory, make that subdirectory, then continue
		// the walk.
		if info.IsDir() {
			if path == filepath.Join(src, dst) {
				// dst is in src; don't walk it.
				return nil
			}

			if err := os.MkdirAll(dstPath, 0755); err != nil {
				return err
			}

			return nil
		}

		// If we have a file, copy the contents.
		if err := copyFile(path, dstPath); err != nil {
			return err
		}

		return nil
	}

	return filepath.Walk(src, walkFn)
}
