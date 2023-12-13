// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-version"
)

const (
	cniPluginAttribute = "plugins.cni.version"
)

// PluginsCNIFingerprint creates a fingerprint of the CNI plugins present on the
// CNI plugin path specified for the Nomad client.
type PluginsCNIFingerprint struct {
	StaticFingerprinter
	logger hclog.Logger
	lister func(string) ([]os.DirEntry, error)
}

func NewPluginsCNIFingerprint(logger hclog.Logger) Fingerprint {
	return &PluginsCNIFingerprint{
		logger: logger.Named("cni_plugins"),
		lister: os.ReadDir,
	}
}

func (f *PluginsCNIFingerprint) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {
	cniPath := req.Config.CNIPath
	if cniPath == "" {
		// this will be set to default by client; if empty then lets just do
		// nothing rather than re-assume a default of our own
		return nil
	}

	// cniPath could be a multi-path, e.g. /opt/cni/bin:/custom/cni/bin
	cniPathList := filepath.SplitList(cniPath)
	for _, cniPath = range cniPathList {
		// list the cni_path directory
		entries, err := f.lister(cniPath)
		switch {
		case err != nil:
			f.logger.Warn("failed to read CNI plugins directory", "cni_path", cniPath, "error", err)
			resp.Detected = false
			return nil
		case len(entries) == 0:
			f.logger.Debug("no CNI plugins found", "cni_path", cniPath)
			resp.Detected = true
			return nil
		}

		// for each file in cni_path, detect executables and try to get their version
		for _, entry := range entries {
			v, ok := f.detectOnePlugin(cniPath, entry)
			if ok {
				resp.AddAttribute(f.attribute(entry.Name()), v)
			}
		}
	}

	// detection complete, regardless of results
	resp.Detected = true
	return nil
}

func (f *PluginsCNIFingerprint) attribute(filename string) string {
	return fmt.Sprintf("%s.%s", cniPluginAttribute, filename)
}

func (f *PluginsCNIFingerprint) detectOnePlugin(pluginPath string, entry os.DirEntry) (string, bool) {
	fi, err := entry.Info()
	if err != nil {
		f.logger.Debug("failed to read cni directory entry", "error", err)
		return "", false
	}

	if fi.Mode()&0o111 == 0 {
		f.logger.Debug("unexpected non-executable in cni plugin directory", "name", fi.Name())
		return "", false // not executable
	}

	exePath := filepath.Join(pluginPath, fi.Name())
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// best effort attempt to get a version from the executable, otherwise
	// the version will be "unknown"
	// execute with no args; at least container-networking plugins respond with
	// version string in this case, which makes Windows support simpler
	cmd := exec.CommandContext(ctx, exePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		f.logger.Debug("failed to detect CNI plugin version", "name", fi.Name(), "error", err)
		return "unknown", false
	}

	// try to find semantic versioning string
	// e.g.
	//  /opt/cni/bin/bridge <no args>
	//  CNI bridge plugin v1.0.0
	//  (and optionally another line that contains the supported CNI protocol versions)
	tokens := strings.Fields(string(output))
	for _, token := range tokens {
		if _, parseErr := version.NewSemver(token); parseErr == nil {
			return token, true
		}
	}

	f.logger.Debug("failed to parse CNI plugin version", "name", fi.Name())
	return "unknown", false
}
