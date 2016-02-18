package getter

import (
	"fmt"
	"log"
	"net/url"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"

	gg "github.com/hashicorp/go-getter"
)

var (
	// getters is the map of getters suitable for Nomad. It is initialized once
	// and the lock is used to guard access to it.
	getters map[string]gg.Getter
	lock    sync.Mutex
)

// getClient returns a client that is suitable for Nomad.
func getClient(src, dst string) *gg.Client {
	lock.Lock()
	defer lock.Unlock()

	// Return the pre-initialized client
	if getters == nil {
		getters = make(map[string]gg.Getter, len(gg.Getters))
		for k, v := range gg.Getters {
			getters[k] = v
		}

		getters["file"] = &gg.FileGetter{Copy: true}
	}

	return &gg.Client{
		Src:     src,
		Dst:     dst,
		Dir:     false, // Only support a single file for now.
		Getters: getters,
	}
}

func GetArtifact(destDir, source, checksum string, logger *log.Logger) (string, error) {
	if source == "" {
		return "", fmt.Errorf("Source url is empty in Artifact Getter")
	}
	u, err := url.Parse(source)
	if err != nil {
		return "", err
	}

	// if checksum is seperate, apply to source
	if checksum != "" {
		source = strings.Join([]string{source, fmt.Sprintf("checksum=%s", checksum)}, "?")
		logger.Printf("[DEBUG] client.getter: Applying checksum to Artifact Source URL, new url: %s", source)
	}

	artifactFile := filepath.Join(destDir, path.Base(u.Path))
	if err := getClient(source, artifactFile).Get(); err != nil {
		return "", fmt.Errorf("Error downloading artifact: %s", err)
	}

	// Add execution permissions to the newly downloaded artifact
	if runtime.GOOS != "windows" {
		if err := syscall.Chmod(artifactFile, 0755); err != nil {
			logger.Printf("[ERR] driver.raw_exec: Error making artifact executable: %s", err)
		}
	}
	return artifactFile, nil
}
