// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package getter

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode"

	"github.com/hashicorp/go-getter"
	"github.com/hashicorp/nomad/client/interfaces"
	"github.com/hashicorp/nomad/helper/subproc"
	"github.com/hashicorp/nomad/helper/users"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// githubPrefixSSH is the prefix for downloading via git using ssh from GitHub.
	githubPrefixSSH = "git@github.com:"
)

func getURL(taskEnv interfaces.EnvReplacer, artifact *structs.TaskArtifact) (string, error) {
	source := taskEnv.ReplaceEnv(artifact.GetterSource)

	// fixup GitHub SSH URL such as git@github.com:hashicorp/nomad.git
	gitSSH := false
	if strings.HasPrefix(source, githubPrefixSSH) {
		gitSSH = true
		source = source[len(githubPrefixSSH):]
	}

	u, err := url.Parse(source)
	if err != nil {
		return "", &Error{
			URL:         artifact.GetterSource,
			Err:         fmt.Errorf("failed to parse source URL %q: %v", artifact.GetterSource, err),
			Recoverable: false,
		}
	}

	// build the URL by substituting as necessary
	q := u.Query()
	for k, v := range artifact.GetterOptions {
		q.Set(k, taskEnv.ReplaceEnv(v))
	}
	u.RawQuery = q.Encode()

	// add the prefix back if necessary
	sourceURL := u.String()
	if gitSSH {
		sourceURL = fmt.Sprintf("%s%s", githubPrefixSSH, sourceURL)
	}

	return sourceURL, nil
}

func getDestination(env interfaces.EnvReplacer, artifact *structs.TaskArtifact) (string, error) {
	destination, escapes := env.ClientPath(artifact.RelativeDest, true)
	if escapes {
		return "", &Error{
			URL:         artifact.GetterSource,
			Err:         fmt.Errorf("artifact destination path escapes alloc directory"),
			Recoverable: false,
		}
	}
	return destination, nil
}

func getMode(artifact *structs.TaskArtifact) getter.ClientMode {
	switch artifact.GetterMode {
	case structs.GetterModeFile:
		return getter.ClientModeFile
	case structs.GetterModeDir:
		return getter.ClientModeDir
	default:
		return getter.ClientModeAny
	}
}

func chownDestination(destination, username string) error {
	if destination == "" || username == "" {
		return nil
	}

	if os.Geteuid() != 0 {
		return nil
	}

	if runtime.GOOS == "windows" {
		return nil
	}

	uid, gid, _, err := users.LookupUnix(username)
	if err != nil {
		return err
	}

	return filepath.Walk(destination, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(path, uid, gid)
	})
}

func isInsecure(artifact *structs.TaskArtifact) bool {
	return artifact.GetterInsecure
}

func getHeaders(env interfaces.EnvReplacer, artifact *structs.TaskArtifact) map[string][]string {
	m := artifact.GetterHeaders
	if len(m) == 0 {
		return nil
	}
	headers := make(http.Header, len(m))
	for k, v := range m {
		headers.Set(k, env.ReplaceEnv(v))
	}
	return headers
}

// getWritableDirs returns host paths to the task's allocation and task specific
// directories - the locations into which a Task is allowed to download an artifact.
func getWritableDirs(env interfaces.EnvReplacer) (string, string) {
	stub, _ := env.ClientPath("stub", false)
	taskDir := filepath.Dir(stub)
	allocDir := filepath.Dir(taskDir)
	return allocDir, taskDir
}

// environment merges the default minimal environment per-OS with the set of
// environment variables configured to be inherited from the Client
func environment(taskDir string, inherit string) []string {
	chomp := func(s string) []string {
		return strings.FieldsFunc(s, func(c rune) bool {
			return c == ',' || unicode.IsSpace(c)
		})
	}
	env := defaultEnvironment(taskDir)
	for _, name := range chomp(inherit) {
		env[name] = os.Getenv(name)
	}
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(result)
	return result
}

func (s *Sandbox) runCmd(env *parameters) error {
	// find the nomad process
	bin := subproc.Self()

	// final method of ensuring subprocess termination
	ctx, cancel := subproc.Context(env.deadline())
	defer cancel()

	// start the subprocess, passing in parameters via stdin
	output := new(bytes.Buffer)
	cmd := exec.CommandContext(ctx, bin, SubCommand)
	cmd.Env = environment(env.TaskDir, env.SetEnvironmentVariables)
	cmd.Stdin = env.reader()
	cmd.Stdout = output
	cmd.Stderr = output

	// start & wait for the subprocess to terminate
	if err := cmd.Run(); err != nil {
		msg := subproc.Log(output, s.logger.Error)

		return &Error{
			URL:         env.Source,
			Err:         fmt.Errorf("getter subprocess failed: %v: %v", err, msg),
			Recoverable: true,
		}
	}
	subproc.Log(output, s.logger.Debug)
	return nil
}
