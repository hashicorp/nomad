package client

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	getter "github.com/hashicorp/go-getter"
)

const (
	nomadCNIBinDir = "cnibin"
)

var (
	defaultCNIGetterChecksums = map[string]string{
		"linux-amd64":   "sha256:e9bfc78acd3ae71be77eb8f3e890cc9078a33cc3797703b8ff2fc3077a232252",
		"linux-arm":     "sha256:ae6ddbd87c05a79aceb92e1c8c32d11e302f6fc55045f87f6a3ea7e0268b2fda",
		"linux-arm64":   "sha256:acde854e3def3c776c532ae521c19d8784534918cc56449ff16945a2909bff6d",
		"windows-amd64": "sha256:a8a24e9cf93f4db92321afca3fe53bd3ccdf2b7117c403c55a5bac162d8d79cc",
	}
	defaultCNIGetterSrc = fmt.Sprintf("https://github.com/containernetworking/plugins/releases/download/v0.8.1/cni-plugins-%s-%s-v0.8.1.tgz?checksum=%s",
		runtime.GOOS, runtime.GOARCH, defaultCNIGetterChecksums[runtime.GOOS+"-"+runtime.GOARCH])
)

type CNIGetter struct {
	src string
	dst string
}

func NewCNIGetter(src, dataDir string) *CNIGetter {
	if src == "" {
		src = defaultCNIGetterSrc
	}
	return &CNIGetter{
		src: src,
		dst: filepath.Join(dataDir, nomadCNIBinDir),
	}
}

func (g *CNIGetter) Get() error {
	return getter.Get(g.dst, g.src)
}

func (g *CNIGetter) CNIPath(path string) string {
	if path == "" {
		return g.dst
	}
	return strings.Join([]string{path, g.dst}, ":")
}
