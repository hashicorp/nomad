// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package host

import (
	"io"
	"os"
	"strings"
)

type HostData struct {
	OS          string
	Network     []map[string]string
	ResolvConf  string
	Hosts       string
	Environment map[string]string
	Disk        map[string]DiskUsage
}

type DiskUsage struct {
	DiskMB int64
	UsedMB int64
}

func MakeHostData() (*HostData, error) {
	du := make(map[string]DiskUsage)
	for _, path := range mountedPaths() {
		u, err := diskUsage(path)
		if err != nil {
			continue
		}
		du[path] = u
	}

	return &HostData{
		OS:          uname(),
		Network:     network(),
		ResolvConf:  resolvConf(),
		Hosts:       etcHosts(),
		Environment: environment(),
		Disk:        du,
	}, nil
}

// diskUsage calculates the DiskUsage
func diskUsage(path string) (du DiskUsage, err error) {
	s, err := makeDf(path)
	if err != nil {
		return du, err
	}

	disk := float64(s.total())
	// Bavail is blocks available to unprivileged users, Bfree includes reserved blocks
	free := float64(s.available())
	used := disk - free
	mb := float64(1048576)

	disk = disk / mb
	used = used / mb

	du.DiskMB = int64(disk)
	du.UsedMB = int64(used)
	return du, nil
}

var (
	envRedactSet = makeEnvRedactSet()
)

// environment returns the process environment in a map
func environment() map[string]string {
	env := make(map[string]string)

	for _, e := range os.Environ() {
		s := strings.SplitN(e, "=", 2)
		k := s[0]
		up := strings.ToUpper(k)
		v := s[1]

		_, redact := envRedactSet[k]
		if redact ||
			strings.Contains(up, "TOKEN") ||
			strings.Contains(up, "SECRET") {
			v = "<redacted>"
		}

		env[k] = v
	}
	return env
}

// DefaultEnvDenyList is the default set of environment variables that are
// filtered when passing the environment variables of the host to the task.
//
// Update https://www.nomadproject.io/docs/configuration/client#env-denylist
// whenever this is changed.
var DefaultEnvDenyList = []string{
	"CONSUL_TOKEN",
	"CONSUL_HTTP_TOKEN",
	"VAULT_TOKEN",
	"NOMAD_LICENSE",
	"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN",
	"GOOGLE_APPLICATION_CREDENTIALS",
}

// makeEnvRedactSet creates a set of well known environment variables that should be
// redacted in the output
func makeEnvRedactSet() map[string]struct{} {
	set := make(map[string]struct{})
	for _, e := range DefaultEnvDenyList {
		set[e] = struct{}{}
	}

	return set
}

// slurp returns the file contents as a string, returning an error string
func slurp(path string) string {
	fh, err := os.Open(path)
	if err != nil {
		return err.Error()
	}

	bs, err := io.ReadAll(fh)
	if err != nil {
		return err.Error()
	}

	return string(bs)
}
