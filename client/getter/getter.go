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
        // We use go-getter to support a variety of protocols, but need to change
        // file permissions of the resulted download to be executable

        u, err := url.Parse(source)
        if err != nil {
                return "", err
        }

        // look for checksum, apply to URL
        if checksum != "" {
                source = strings.Join([]string{source, fmt.Sprintf("checksum=%s", checksum)}, "?")
                logger.Printf("[DEBUG] Applying checksum to Artifact Source URL, new url: %s", source)
        }

        artifactName := path.Base(u.Path)
        artifactFile := filepath.Join(destDir, artifactName)
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
