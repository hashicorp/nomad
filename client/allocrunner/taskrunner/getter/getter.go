package getter

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"

	"github.com/hashicorp/go-cleanhttp"
	gg "github.com/hashicorp/go-getter"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/interfaces"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// gitSSHPrefix is the prefix for downloading via git using ssh
	gitSSHPrefix = "git@github.com:"
)

// Getter wraps go-getter calls in an artifact configuration.
type Getter struct {
	logger hclog.Logger

	// httpClient is a shared HTTP client for use across all http/https
	// Getter instantiations. The HTTP client is designed to be
	// thread-safe, and using a pooled transport will help reduce excessive
	// connections when clients are downloading lots of artifacts.
	httpClient *http.Client
	config     *config.ArtifactConfig
}

// NewGetter returns a new Getter instance. This function is called once per
// client and shared across alloc and task runners.
func NewGetter(logger hclog.Logger, config *config.ArtifactConfig) *Getter {
	return &Getter{
		logger: logger,
		httpClient: &http.Client{
			Transport: cleanhttp.DefaultPooledTransport(),
		},
		config: config,
	}
}

// GetArtifact downloads an artifact into the specified task directory.
func (g *Getter) GetArtifact(taskEnv interfaces.EnvReplacer, artifact *structs.TaskArtifact) (returnErr error) {
	// Recover from panics to avoid crashing the entire Nomad client due to
	// artifact download failures, such as bugs in go-getter.
	defer func() {
		if r := recover(); r != nil {
			g.logger.Error("panic while downloading artifact",
				"artifact", artifact.GetterSource,
				"error", r,
				"stack", string(debug.Stack()))
			returnErr = fmt.Errorf("getter panic: %v", r)
		}
	}()

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

	if err := g.getClient(ggURL, headers, mode, dest).Get(); err != nil {
		return newGetError(ggURL, err, true)
	}

	return nil
}

// getClient returns a client that is suitable for Nomad downloading artifacts.
func (g *Getter) getClient(src string, headers http.Header, mode gg.ClientMode, dst string) *gg.Client {
	return &gg.Client{
		Src:     src,
		Dst:     dst,
		Mode:    mode,
		Umask:   060000000,
		Getters: g.createGetters(headers),

		// This will prevent copying or writing files through symlinks.
		DisableSymlinks: true,

		// This will protect against decompression bombs.
		Decompressors: gg.LimitedDecompressors(g.config.DecompressionLimitFileCount, g.config.DecompressionLimitSize),
	}
}

func (g *Getter) createGetters(header http.Header) map[string]gg.Getter {
	httpGetter := &gg.HttpGetter{
		Netrc:  true,
		Client: g.httpClient,
		Header: header,

		// Do not support the custom X-Terraform-Get header and
		// associated logic.
		XTerraformGetDisabled: true,

		// Disable HEAD requests as they can produce corrupt files when
		// retrying a download of a resource that has changed.
		// hashicorp/go-getter#219
		DoNotCheckHeadFirst: true,

		// Read timeout for HTTP operations. Must be long enough to
		// accommodate large/slow downloads.
		ReadTimeout: g.config.HTTPReadTimeout,

		// Maximum download size. Must be large enough to accommodate
		// large downloads.
		MaxBytes: g.config.HTTPMaxBytes,
	}

	// Explicitly create fresh set of supported Getter for each Client, because
	// go-getter is not thread-safe. Use a shared HTTP client for http/https Getter,
	// with pooled transport which is thread-safe.
	//
	// If a getter type is not listed here, it is not supported (e.g. file).
	return map[string]gg.Getter{
		"git": &gg.GitGetter{
			Timeout: g.config.GitTimeout,
		},
		"hg": &gg.HgGetter{
			Timeout: g.config.HgTimeout,
		},
		"gcs": &gg.GCSGetter{
			Timeout: g.config.GCSTimeout,
		},
		"s3": &gg.S3Getter{
			Timeout: g.config.S3Timeout,
		},
		"http":  httpGetter,
		"https": httpGetter,
	}
}

// getGetterUrl returns the go-getter URL to download the artifact.
func getGetterUrl(taskEnv interfaces.EnvReplacer, artifact *structs.TaskArtifact) (string, error) {
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

func getHeaders(env interfaces.EnvReplacer, m map[string]string) http.Header {
	if len(m) == 0 {
		return nil
	}

	headers := make(http.Header, len(m))
	for k, v := range m {
		headers.Set(k, env.ReplaceEnv(v))
	}
	return headers
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
