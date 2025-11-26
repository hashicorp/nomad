// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/client/config"
)

// RetryWrapper is a fingerprinter wrapper that adds retry logic to an existing
// fingerprinter. This is currently supported for environment fingerprinters
// only and is controller via the client fingerprinter config.
type RetryWrapper struct {

	// fingerprinter is the underlying fingerprinter being wrapped with retry
	// logic.
	fingerprinter Fingerprint

	// name is the name of the fingerprinter being wrapped and is used to pull
	// any configuration for it.
	name string

	logger hclog.Logger

	// StaticFingerprinter is embedded to indicate that this fingerprinter does
	// not support periodic execution.
	StaticFingerprinter
}

// NewRetryWrapper wraps the passed fingerprinter with retry logic. The returned
// fingerprinter will consult the client configuration for any retry settings.
//
// It staisifes the Fingerprinter interface and is a static fingerprinter, so
// does not support periodic execution.
func NewRetryWrapper(fingerprinter Fingerprint, logger hclog.Logger, name string) Fingerprint {
	return &RetryWrapper{
		fingerprinter: fingerprinter,
		logger:        logger,
		name:          name,
	}
}

// Fingerprint executes the underlying fingerprinter with retry logic based
// on the client configuration and implements the Fingerprinter interface.
//
// If the fingerprinter fails after all retry attempts, the error from the last
// attempt is returned, unless the configuration indicates that failures should
// be skipped for this fingerprinter and the error is of the type that indicates
// an initial probe failure.
func (rw *RetryWrapper) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {

	cfg := req.Config.Fingerprinters[rw.name]

	var (
		attempts int
		err      error
	)

	// Ensure we default to a 2 second retry interval if not configured. Doing
	// this here means we do not have to do this each loop iteration at the
	// cost of the config potentially being empty and the loop exiting after the
	// first attempt.
	retryInterval := 2 * time.Second

	if cfg != nil && cfg.RetryInterval > 0 {
		retryInterval = cfg.RetryInterval
	}

	for {
		err = rw.fingerprinter.Fingerprint(req, resp)
		if err == nil {
			return nil
		}

		// If the fingerprinter does not have a config, no retry behaviour is
		// defined, so exit the loop.
		if cfg == nil {
			break
		}

		var shouldRetry bool

		switch cfg.RetryAttempts {
		case -1:
			shouldRetry = true
		case 0:
		default:
			if attempts < cfg.RetryAttempts {
				shouldRetry = true
			}
		}

		if !shouldRetry {
			break
		}

		rw.logger.Warn("fingerprinting failed, retrying",
			"current_attempts", attempts,
			"retry_attempts", cfg.RetryAttempts,
			"retry_interval", retryInterval,
			"error", err,
		)

		attempts++
		time.Sleep(retryInterval)
	}

	if shouldSkipEnvFingerprinter(cfg, err) {
		rw.logger.Debug("error performing initial probe, skipping")
		return nil
	}

	rw.logger.Error("fingerprinting failed after all attempts", "error", err)
	return err
}

// errEnvProbeQueryFailed is used to indicate that the initial probe to
// determine if the environment fingerprinter is applicable has failed.
var errEnvProbeQueryFailed = errors.New("fingerprint initial probe failed")

// wrapProbeError wraps the passed error with errEnvProbeQueryFailed to indicate
// that the initial probe has failed.
func wrapProbeError(err error) error {
	return fmt.Errorf("%w: %w", errEnvProbeQueryFailed, err)
}

// shouldSkipEnvFingerprinter determines if an environment fingerprinter should
// be skipped based on the passed configuration and error from the
// fingerprinter. Skipped indicates the client is not running in the environment
// the fingerprinter is designed for.
func shouldSkipEnvFingerprinter(cfg *config.Fingerprint, err error) bool {

	if err == nil {
		return false
	}

	if cfg != nil && cfg.ExitOnFailure != nil {
		return !*cfg.ExitOnFailure
	}

	return errors.Is(err, errEnvProbeQueryFailed)
}
