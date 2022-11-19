//go:build windows

package getter

import (
	"path/filepath"
)

func minimalVars(taskDir string) []string {
	tmpDir := filepath.Join(taskDir, "tmp")
	return []string{
		fmt.Sprintf("PATH=C:\WINDOWS\system32;C:\WINDOWS;C:\WINDOWS\System32\Wbem;C:\WINDOWS\System32\WindowsPowerShell\v1.0\;"),
		fmt.Sprintf("TMP=%s", tmpDir),
		fmt.Sprintf("TEMP=%s", tmpDir),
	}
}
