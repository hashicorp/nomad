// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/types"
	"github.com/docker/distribution/reference"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/registry"
	docker "github.com/fsouza/go-dockerclient"
)

func parseDockerImage(image string) (repo, tag string) {
	repo, tag = docker.ParseRepositoryTag(image)
	if tag != "" {
		return repo, tag
	}
	if i := strings.IndexRune(image, '@'); i > -1 { // Has digest (@sha256:...)
		// when pulling images with a digest, the repository contains the sha hash, and the tag is empty
		// see: https://github.com/fsouza/go-dockerclient/blob/master/image_test.go#L471
		repo = image
	} else {
		tag = "latest"
	}
	return repo, tag
}

func dockerImageRef(repo string, tag string) string {
	if tag == "" {
		return repo
	}
	return fmt.Sprintf("%s:%s", repo, tag)
}

// loadDockerConfig loads the docker config at the specified path, returning an
// error if it couldn't be read.
func loadDockerConfig(file string) (*configfile.ConfigFile, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("Failed to open auth config file: %v, error: %v", file, err)
	}
	defer f.Close()

	cfile := new(configfile.ConfigFile)
	if err = cfile.LoadFromReader(f); err != nil {
		return nil, fmt.Errorf("Failed to parse auth config file: %v", err)
	}
	return cfile, nil
}

// parseRepositoryInfo takes a repo and returns the Docker RepositoryInfo. This
// is useful for interacting with a Docker config object.
func parseRepositoryInfo(repo string) (*registry.RepositoryInfo, error) {
	name, err := reference.ParseNormalizedNamed(repo)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse named repo %q: %v", repo, err)
	}

	repoInfo, err := registry.ParseRepositoryInfo(name)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse repository: %v", err)
	}

	return repoInfo, nil
}

// firstValidAuth tries a list of auth backends, returning first error or AuthConfiguration
func firstValidAuth(repo string, backends []authBackend) (*docker.AuthConfiguration, error) {
	for _, backend := range backends {
		auth, err := backend(repo)
		if auth != nil || err != nil {
			return auth, err
		}
	}
	return nil, nil
}

// authFromTaskConfig generates an authBackend for any auth given in the task-configuration
func authFromTaskConfig(driverConfig *TaskConfig) authBackend {
	return func(string) (*docker.AuthConfiguration, error) {
		// If all auth fields are empty, return
		if len(driverConfig.Auth.Username) == 0 && len(driverConfig.Auth.Password) == 0 && len(driverConfig.Auth.Email) == 0 && len(driverConfig.Auth.ServerAddr) == 0 {
			return nil, nil
		}
		return &docker.AuthConfiguration{
			Username:      driverConfig.Auth.Username,
			Password:      driverConfig.Auth.Password,
			Email:         driverConfig.Auth.Email,
			ServerAddress: driverConfig.Auth.ServerAddr,
		}, nil
	}
}

// authFromDockerConfig generate an authBackend for a dockercfg-compatible file.
// The authBackend can either be from explicit auth definitions or via credential
// helpers
func authFromDockerConfig(file string) authBackend {
	return func(repo string) (*docker.AuthConfiguration, error) {
		if file == "" {
			return nil, nil
		}
		repoInfo, err := parseRepositoryInfo(repo)
		if err != nil {
			return nil, err
		}

		cfile, err := loadDockerConfig(file)
		if err != nil {
			return nil, err
		}

		return firstValidAuth(repo, []authBackend{
			func(string) (*docker.AuthConfiguration, error) {
				dockerAuthConfig := registryResolveAuthConfig(cfile.AuthConfigs, repoInfo.Index)
				auth := &docker.AuthConfiguration{
					Username:      dockerAuthConfig.Username,
					Password:      dockerAuthConfig.Password,
					Email:         dockerAuthConfig.Email,
					ServerAddress: dockerAuthConfig.ServerAddress,
					IdentityToken: dockerAuthConfig.IdentityToken,
					RegistryToken: dockerAuthConfig.RegistryToken,
				}
				if authIsEmpty(auth) {
					return nil, nil
				}
				return auth, nil
			},
			authFromHelper(cfile.CredentialHelpers[registry.GetAuthConfigKey(repoInfo.Index)]),
			authFromHelper(cfile.CredentialsStore),
		})
	}
}

// authFromHelper generates an authBackend for a docker-credentials-helper;
// A script taking the requested domain on input, outputting JSON with
// "Username" and "Secret"
func authFromHelper(helperName string) authBackend {
	return func(repo string) (*docker.AuthConfiguration, error) {
		if helperName == "" {
			return nil, nil
		}
		helper := dockerAuthHelperPrefix + helperName
		cmd := exec.Command(helper, "get")

		repoInfo, err := parseRepositoryInfo(repo)
		if err != nil {
			return nil, err
		}

		cmd.Stdin = strings.NewReader(repoInfo.Index.Name)
		output, err := cmd.Output()
		if err != nil {
			exitErr, ok := err.(*exec.ExitError)
			if ok {
				return nil, fmt.Errorf(
					"%s with input %q failed with stderr: %s", helper, repo, exitErr.Stderr)
			}
			return nil, err
		}

		var response map[string]string
		if err := json.Unmarshal(output, &response); err != nil {
			return nil, err
		}

		auth := &docker.AuthConfiguration{
			Username: response["Username"],
			Password: response["Secret"],
		}

		if authIsEmpty(auth) {
			return nil, nil
		}
		return auth, nil
	}
}

// authIsEmpty returns if auth is nil or an empty structure
func authIsEmpty(auth *docker.AuthConfiguration) bool {
	if auth == nil {
		return false
	}
	return auth.Username == "" &&
		auth.Password == "" &&
		auth.Email == "" &&
		auth.ServerAddress == ""
}

func validateCgroupPermission(s string) bool {
	for _, c := range s {
		switch c {
		case 'r', 'w', 'm':
		default:
			return false
		}
	}

	return true
}

// expandPath returns the absolute path of dir, relative to base if dir is relative path.
// base is expected to be an absolute path
func expandPath(base, dir string) string {
	if runtime.GOOS == "windows" {
		pipeExp := regexp.MustCompile(`^` + rxPipe + `$`)
		match := pipeExp.FindStringSubmatch(strings.ToLower(dir))

		if len(match) == 1 {
			// avoid resolving dot-segment in named pipe
			return dir
		}
	}

	if filepath.IsAbs(dir) {
		return filepath.Clean(dir)
	}

	return filepath.Clean(filepath.Join(base, dir))
}

// isParentPath returns true if path is a child or a descendant of parent path.
// Both inputs need to be absolute paths.
func isParentPath(parent, path string) bool {
	rel, err := filepath.Rel(parent, path)
	return err == nil && !strings.HasPrefix(rel, "..")
}

func parseVolumeSpec(volBind, os string) (hostPath string, containerPath string, mode string, err error) {
	if os == "windows" {
		return parseVolumeSpecWindows(volBind)
	}
	return parseVolumeSpecLinux(volBind)
}

func parseVolumeSpecWindows(volBind string) (hostPath string, containerPath string, mode string, err error) {
	parts, err := windowsSplitRawSpec(volBind, rxDestination)
	if err != nil {
		return "", "", "", fmt.Errorf("not <src>:<destination> format")
	}

	if len(parts) < 2 {
		return "", "", "", fmt.Errorf("not <src>:<destination> format")
	}

	// Convert host mount path separators to match the host OS's separator
	// so that relative paths are supported cross-platform regardless of
	// what slash is used in the jobspec.
	hostPath = filepath.FromSlash(parts[0])
	containerPath = parts[1]

	if len(parts) > 2 {
		mode = parts[2]
	}

	return
}

func parseVolumeSpecLinux(volBind string) (hostPath string, containerPath string, mode string, err error) {
	// using internal parser to preserve old parsing behavior.  Docker
	// parser has additional validators (e.g. mode validity) and accepts invalid output (per Nomad),
	// e.g. single path entry to be treated as a container path entry with an auto-generated host-path.
	//
	// Reconsider updating to use Docker parser when ready to make incompatible changes.
	parts := strings.Split(volBind, ":")
	if len(parts) < 2 {
		return "", "", "", fmt.Errorf("not <src>:<destination> format")
	}

	m := ""
	if len(parts) > 2 {
		m = parts[2]
	}

	return parts[0], parts[1], m, nil
}

// ResolveAuthConfig matches an auth configuration to a server address or a URL
// copied from https://github.com/moby/moby/blob/ca20bc4214e6a13a5f134fb0d2f67c38065283a8/registry/auth.go#L217-L235
// but with the CLI types.AuthConfig type rather than api/types
func registryResolveAuthConfig(authConfigs map[string]types.AuthConfig, index *registrytypes.IndexInfo) types.AuthConfig {
	configKey := registry.GetAuthConfigKey(index)
	// First try the happy case
	if c, found := authConfigs[configKey]; found || index.Official {
		return c
	}

	// Maybe they have a legacy config file, we will iterate the keys converting
	// them to the new format and testing
	for r, ac := range authConfigs {
		if configKey == registry.ConvertToHostname(r) {
			return ac
		}
	}

	// When all else fails, return an empty auth config
	return types.AuthConfig{}
}
