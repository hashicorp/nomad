package getter

import (
	"fmt"
	"log"
	"net/url"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	gg "github.com/hashicorp/go-getter"
)

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
	if err := gg.GetFile(artifactFile, source); err != nil {
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
