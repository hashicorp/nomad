// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package getter

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/interfaces"
	"github.com/hashicorp/nomad/nomad/structs"
)

// New creates a Sandbox with the given ArtifactConfig.
func New(ac *config.ArtifactConfig, logger hclog.Logger) *Sandbox {
	return &Sandbox{
		logger: logger.Named("artifact"),
		ac:     ac,
	}
}

// A Sandbox is used to download artifacts.
type Sandbox struct {
	logger hclog.Logger
	ac     *config.ArtifactConfig
}

func (s *Sandbox) Get(env interfaces.EnvReplacer, artifact *structs.TaskArtifact) error {
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
	allocDir, taskDir := getWritableDirs(env)

	params := &parameters{
		// downloader configuration
		HTTPReadTimeout:             s.ac.HTTPReadTimeout,
		HTTPMaxBytes:                s.ac.HTTPMaxBytes,
		GCSTimeout:                  s.ac.GCSTimeout,
		GitTimeout:                  s.ac.GitTimeout,
		HgTimeout:                   s.ac.HgTimeout,
		S3Timeout:                   s.ac.S3Timeout,
		DecompressionLimitFileCount: s.ac.DecompressionLimitFileCount,
		DecompressionLimitSize:      s.ac.DecompressionLimitSize,
		DisableFilesystemIsolation:  s.ac.DisableFilesystemIsolation,
		SetEnvironmentVariables:     s.ac.SetEnvironmentVariables,

		// artifact configuration
		Mode:        mode,
		Source:      source,
		Destination: destination,
		Headers:     headers,

		// task filesystem
		AllocDir: allocDir,
		TaskDir:  taskDir,
	}

	if err = s.runCmd(params); err != nil {
		return err
	}
	return nil
}
