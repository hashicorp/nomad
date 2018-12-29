package docker

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/distribution/reference"
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
		if len(driverConfig.Auth.Email) == 0 {
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
// The authBacken can either be from explicit auth definitions or via credential
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
				dockerAuthConfig := registry.ResolveAuthConfig(cfile.AuthConfigs, repoInfo.Index)
				auth := &docker.AuthConfiguration{
					Username:      dockerAuthConfig.Username,
					Password:      dockerAuthConfig.Password,
					Email:         dockerAuthConfig.Email,
					ServerAddress: dockerAuthConfig.ServerAddress,
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

		// Ensure that the HTTPs prefix exists
		repoAddr := fmt.Sprintf("https://%s", repoInfo.Index.Name)

		cmd.Stdin = strings.NewReader(repoAddr)
		output, err := cmd.Output()
		if err != nil {
			switch err.(type) {
			default:
				return nil, err
			case *exec.ExitError:
				return nil, fmt.Errorf("%s with input %q failed with stderr: %s", helper, repo, output)
			}
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
