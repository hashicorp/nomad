package docker

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/registry"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/pkg/errors"
)

const (
	// Spec should be in the format [source:]destination[:mode]
	//
	// Examples: c:\foo bar:d:rw
	//           c:\foo:d:\bar
	//           myname:d:
	//           d:\
	//
	// Explanation of this regex! Thanks @thaJeztah on IRC and gist for help. See
	// https://gist.github.com/thaJeztah/6185659e4978789fb2b2. A good place to
	// test is https://regex-golang.appspot.com/assets/html/index.html
	//
	// Useful link for referencing named capturing groups:
	// http://stackoverflow.com/questions/20750843/using-named-matches-from-go-regex
	//
	// There are three match groups: source, destination and mode.
	//

	// rxHostDir is the first option of a source
	rxHostDir = `(?:\\\\\?\\)?[a-z]:[\\/](?:[^\\/:*?"<>|\r\n]+[\\/]?)*`
	// rxName is the second option of a source
	rxName = `[^\\/:*?"<>|\r\n]+`

	// RXReservedNames are reserved names not possible on Windows
	rxReservedNames = `(con)|(prn)|(nul)|(aux)|(com[1-9])|(lpt[1-9])`

	// rxPipe is a named path pipe (starts with `\\.\pipe\`, possibly with / instead of \)
	rxPipe = `[/\\]{2}.[/\\]pipe[/\\][^:*?"<>|\r\n]+`
	// rxSource is the combined possibilities for a source
	rxSource = `((?P<source>((` + rxHostDir + `)|(` + rxName + `)|(` + rxPipe + `))):)?`

	// Source. Can be either a host directory, a name, or omitted:
	//  HostDir:
	//    -  Essentially using the folder solution from
	//       https://www.safaribooksonline.com/library/view/regular-expressions-cookbook/9781449327453/ch08s18.html
	//       but adding case insensitivity.
	//    -  Must be an absolute path such as c:\path
	//    -  Can include spaces such as `c:\program files`
	//    -  And then followed by a colon which is not in the capture group
	//    -  And can be optional
	//  Name:
	//    -  Must not contain invalid NTFS filename characters (https://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx)
	//    -  And then followed by a colon which is not in the capture group
	//    -  And can be optional

	// rxDestination is the regex expression for the mount destination
	rxDestination = `(?P<destination>((?:\\\\\?\\)?([a-z]):((?:[\\/][^\\/:*?"<>\r\n]+)*[\\/]?))|(` + rxPipe + `))`

	// Destination (aka container path):
	//    -  Variation on hostdir but can be a drive followed by colon as well
	//    -  If a path, must be absolute. Can include spaces
	//    -  Drive cannot be c: (explicitly checked in code, not RegEx)

	// rxMode is the regex expression for the mode of the mount
	// Mode (optional):
	//    -  Hopefully self explanatory in comparison to above regex's.
	//    -  Colon is not in the capture group
	rxMode = `(:(?P<mode>(?i)ro|rw))?`
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

func errInvalidSpec(spec string) error {
	return errors.Errorf("invalid volume specification: '%s'", spec)
}

func parseVolumeSpec(volBind, os string) (hostPath string, containerPath string, mode string, err error) {
	if os == "windows" {
		return parseVolumeSpecWindows(volBind)
	}
	return parseVolumeSpecLinux(volBind)
}

type fileInfoProvider interface {
	fileInfo(path string) (exist, isDir bool, err error)
}

type defaultFileInfoProvider struct {
}

func (defaultFileInfoProvider) fileInfo(path string) (exist, isDir bool, err error) {
	fi, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return false, false, err
		}
		return false, false, nil
	}
	return true, fi.IsDir(), nil
}

var currentFileInfoProvider fileInfoProvider = defaultFileInfoProvider{}

func windowsSplitRawSpec(raw, destRegex string) ([]string, error) {
	specExp := regexp.MustCompile(`^` + rxSource + destRegex + rxMode + `$`)
	match := specExp.FindStringSubmatch(strings.ToLower(raw))

	// Must have something back
	if len(match) == 0 {
		return nil, errInvalidSpec(raw)
	}

	var split []string
	matchgroups := make(map[string]string)
	// Pull out the sub expressions from the named capture groups
	for i, name := range specExp.SubexpNames() {
		matchgroups[name] = strings.ToLower(match[i])
	}
	if source, exists := matchgroups["source"]; exists {
		if source != "" {
			split = append(split, source)
		}
	}
	if destination, exists := matchgroups["destination"]; exists {
		if destination != "" {
			split = append(split, destination)
		}
	}
	if mode, exists := matchgroups["mode"]; exists {
		if mode != "" {
			split = append(split, mode)
		}
	}
	// Fix #26329. If the destination appears to be a file, and the source is null,
	// it may be because we've fallen through the possible naming regex and hit a
	// situation where the user intention was to map a file into a container through
	// a local volume, but this is not supported by the platform.
	if matchgroups["source"] == "" && matchgroups["destination"] != "" {
		volExp := regexp.MustCompile(`^` + rxName + `$`)
		reservedNameExp := regexp.MustCompile(`^` + rxReservedNames + `$`)

		if volExp.MatchString(matchgroups["destination"]) {
			if reservedNameExp.MatchString(matchgroups["destination"]) {
				return nil, fmt.Errorf("volume name %q cannot be a reserved word for Windows filenames", matchgroups["destination"])
			}
		} else {

			exists, isDir, _ := currentFileInfoProvider.fileInfo(matchgroups["destination"])
			if exists && !isDir {
				return nil, fmt.Errorf("file '%s' cannot be mapped. Only directories can be mapped on this platform", matchgroups["destination"])

			}
		}
	}
	return split, nil
}

func parseVolumeSpecWindows(volBind string) (hostPath string, containerPath string, mode string, err error) {
	parts, err := windowsSplitRawSpec(volBind, rxDestination)
	if err != nil {
		return "", "", "", fmt.Errorf("not <src>:<destination> format")
	}

	if len(parts) < 2 {
		return "", "", "", fmt.Errorf("not <src>:<destination> format")
	}

	hostPath = parts[0]
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
