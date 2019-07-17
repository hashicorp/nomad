package client

import (
	"fmt"
	"path/filepath"
	"runtime"

	getter "github.com/hashicorp/go-getter"
	hclog "github.com/hashicorp/go-hclog"
)

const (
	nomadCNIBinDir = "cnibin"
)

var (

	// checksums are copied from https://github.com/containernetworking/plugins/releases
	defaultCNIGetterChecksums = map[string]string{
		"linux-amd64":   "sha256:e9bfc78acd3ae71be77eb8f3e890cc9078a33cc3797703b8ff2fc3077a232252",
		"linux-arm":     "sha256:ae6ddbd87c05a79aceb92e1c8c32d11e302f6fc55045f87f6a3ea7e0268b2fda",
		"linux-arm64":   "sha256:acde854e3def3c776c532ae521c19d8784534918cc56449ff16945a2909bff6d",
		"windows-amd64": "sha256:a8a24e9cf93f4db92321afca3fe53bd3ccdf2b7117c403c55a5bac162d8d79cc",
	}
	defaultCNIPluginVersion = "0.8.1"
	defaultCNIGetterSrc     = fmt.Sprintf("https://github.com/containernetworking/plugins/releases/download/v%s/cni-plugins-%s-%s-v%s.tgz?checksum=%s",
		defaultCNIPluginVersion, runtime.GOOS, runtime.GOARCH, defaultCNIPluginVersion,
		defaultCNIGetterChecksums[runtime.GOOS+"-"+runtime.GOARCH])
)

// FetchCNIPlugins downloads the standard set of CNI plugins to the client's
// data directory and returns the path to be used when setting up the CNI_PATH
// environment variable. If an error occures during download, it is logged and
// an empty path is returned
func FetchCNIPlugins(logger hclog.Logger, src string, dataDir string) string {
	if src == "" {
		src = defaultCNIGetterSrc
	}

	logger.Info("downloading CNI plugins", "url", src)
	dst := filepath.Join(dataDir, nomadCNIBinDir)
	if err := getter.Get(dst, src); err != nil {
		logger.Warn("failed to fetch CNI plugins", "url", src, "error", err)
		return ""
	}

	return dst
}
