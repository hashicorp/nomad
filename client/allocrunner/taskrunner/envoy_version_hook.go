// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-version"
	ifs "github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper/envoy"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// envoyVersionHookName is the name of this hook and appears in logs.
	envoyVersionHookName = "envoy_version"
)

type envoyVersionHookConfig struct {
	alloc         *structs.Allocation
	proxiesClient consul.SupportedProxiesAPI
	logger        hclog.Logger
}

func newEnvoyVersionHookConfig(alloc *structs.Allocation, proxiesClient consul.SupportedProxiesAPI, logger hclog.Logger) *envoyVersionHookConfig {
	return &envoyVersionHookConfig{
		alloc:         alloc,
		logger:        logger,
		proxiesClient: proxiesClient,
	}
}

// envoyVersionHook is used to determine and set the Docker image used for Consul
// Connect sidecar proxy tasks. It will query Consul for a set of preferred Envoy
// versions if the task image is unset or references ${NOMAD_envoy_version}. If
// Consul does not report its supported envoy versions then the hood will fail.
type envoyVersionHook struct {
	// alloc is the allocation with the envoy task being rewritten.
	alloc *structs.Allocation

	// proxiesClient is the subset of the Consul API for getting information
	// from Consul about the versions of Envoy it supports.
	proxiesClient consul.SupportedProxiesAPI

	// logger is used to log things.
	logger hclog.Logger
}

func newEnvoyVersionHook(c *envoyVersionHookConfig) *envoyVersionHook {
	return &envoyVersionHook{
		alloc:         c.alloc,
		proxiesClient: c.proxiesClient,
		logger:        c.logger.Named(envoyVersionHookName),
	}
}

func (_ *envoyVersionHook) Name() string {
	return envoyVersionHookName
}

func (h *envoyVersionHook) Prestart(_ context.Context, request *ifs.TaskPrestartRequest, _ *ifs.TaskPrestartResponse) error {
	// First interpolation of the task image. Typically this turns the default
	// ${meta.connect.sidecar_task} into docker.io/envoyproxy/envoy:v${NOMAD_envoy_version}
	// but could be a no-op or some other value if so configured.
	h.interpolateImage(request.Task, request.TaskEnv)

	// Detect whether this hook needs to run and return early if not. Only run if:
	// - task uses docker driver
	// - task is a connect sidecar or gateway
	// - task image needs ${NOMAD_envoy_version} resolved
	if h.skip(request) {
		return nil
	}

	// We either need to acquire Consul's preferred Envoy version or fallback
	// to the legacy default. Query Consul and use the (possibly empty) result.
	proxies, err := h.proxiesClient.Proxies()
	if err != nil {
		return fmt.Errorf("error retrieving supported Envoy versions from Consul: %w", err)
	}

	// Second [pseudo] interpolation of task image. This determines the concrete
	// Envoy image identifier by applying version string substitution of
	// ${NOMAD_envoy_version} acquired from Consul.
	image, err := h.tweakImage(h.taskImage(request.Task.Config), proxies)
	if err != nil {
		return fmt.Errorf("error interpreting desired Envoy version from Consul: %w", err)
	}

	// Set the resulting image.
	h.logger.Trace("setting task envoy image", "image", image)
	request.Task.Config["image"] = image
	return nil
}

// interpolateImage applies the first pass of interpolation on the task's
// config.image value. This is where ${meta.connect.sidecar_image} or
// ${meta.connect.gateway_image} becomes something that might include the
// ${NOMAD_envoy_version} pseudo variable for further resolution.
func (_ *envoyVersionHook) interpolateImage(task *structs.Task, env *taskenv.TaskEnv) {
	value, exists := task.Config["image"]
	if !exists {
		return
	}

	image, ok := value.(string)
	if !ok {
		return
	}

	task.Config["image"] = env.ReplaceEnv(image)
}

// skip will return true if the request does not contain a task that should have
// its envoy proxy version resolved automatically.
func (h *envoyVersionHook) skip(request *ifs.TaskPrestartRequest) bool {
	switch {
	case !request.Task.UsesConnectSidecar():
		return true
	case !h.needsVersion(request.Task.Config):
		return true
	}
	return false
}

// getConfiguredImage extracts the configured config.image value from the request.
// If the image is empty or not a string, Nomad will fallback to the normal
// official Envoy image as if the setting was not configured. This is also what
// Nomad would do if the sidecar_task was not set in the first place.
func (h *envoyVersionHook) taskImage(config map[string]interface{}) string {
	value, exists := config["image"]
	if !exists {
		return envoy.ImageFormat
	}

	image, ok := value.(string)
	if !ok {
		return envoy.ImageFormat
	}

	return image
}

// needsVersion returns true if the docker.config.image is making use of the
// ${NOMAD_envoy_version} faux environment variable, or
// Nomad does not need to query Consul to get the preferred Envoy version, etc.)
func (h *envoyVersionHook) needsVersion(config map[string]interface{}) bool {
	if len(config) == 0 {
		return false
	}

	if _, exists := config["image"]; !exists {
		return false
	}

	image := h.taskImage(config)

	return strings.Contains(image, envoy.VersionVar)
}

// tweakImage determines the best Envoy version to use. If no supported versions were
// detected from the Consul API just return an error.
func (h *envoyVersionHook) tweakImage(configured string, supported map[string][]string) (string, error) {
	versions := supported["envoy"]
	if len(versions) == 0 {
		return "", errors.New("Consul did not report any supported envoy versions")
	}

	latest, err := semver(versions[0])
	if err != nil {
		return "", err
	}

	return strings.ReplaceAll(configured, envoy.VersionVar, latest), nil
}

// semver sanitizes the envoy version string coming from Consul into the format
// used by the Envoy project when publishing images (i.e. proper semver). This
// resulting string value does NOT contain the 'v' prefix for 2 reasons:
//  1. the version library does not include the 'v'
//  2. its plausible unofficial images use the 3 numbers without the prefix for
//     tagging their own images
func semver(chosen string) (string, error) {
	v, err := version.NewVersion(chosen)
	if err != nil {
		return "", fmt.Errorf("unexpected envoy version format: %w", err)
	}
	return v.String(), nil
}
