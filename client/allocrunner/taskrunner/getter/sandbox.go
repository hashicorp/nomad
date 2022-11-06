package getter

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/interfaces"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Sandbox is used for launching "getter" sub-process helpers for downloading
// artifacts. A Nomad client creates one of these and the task runner will call
// Get per artifact. Think "one process per browser tab" security model.
type Sandbox interface {
	Get(interfaces.EnvReplacer, *structs.TaskArtifact) error
}

// New creates a Sandbox with the given ArtifactConfig.
func New(ac *config.ArtifactConfig, logger hclog.Logger) Sandbox {
	return &sandbox{
		logger: logger.Named("artifact"),
		ac:     ac,
	}
}

type sandbox struct {
	logger hclog.Logger
	ac     *config.ArtifactConfig
}

func (s *sandbox) Get(env interfaces.EnvReplacer, artifact *structs.TaskArtifact) error {
	s.logger.Debug("get", "source", artifact.GetterSource, "destination", artifact.RelativeDest)

	source, err := getURL(env, artifact)
	if err != nil {
		return err
	}

	destination, err := getDestination(env, artifact)
	if err != nil {
		return err
	}

	mode := getMode(artifact)
	headers := getHeaders(env, artifact)
	dir := getTaskDir(env)

	params := &parameters{
		HTTPReadTimeout: s.ac.HTTPReadTimeout,
		HTTPMaxBytes:    s.ac.HTTPMaxBytes,
		GCSTimeout:      s.ac.GCSTimeout,
		GitTimeout:      s.ac.GitTimeout,
		HgTimeout:       s.ac.HgTimeout,
		S3Timeout:       s.ac.S3Timeout,
		Mode:            mode,
		Source:          source,
		Destination:     destination,
		Headers:         headers,
		TaskDir:         dir,
	}

	if err = runCmd(params, s.logger); err != nil {
		return err
	}
	return nil
}
