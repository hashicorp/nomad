//go:build windows

package getter

import (
	"fmt"
	"path/filepath"
)

// credentials returns 0, 0 on windows
func credentials() (uint32, uint32) {
	return 0, 0
}

// lockdown has no effect on windows
func lockdown(string, bool) error {
	return nil
}

func minimalVars(taskDir string) []string {
	tmpDir := filepath.Join(taskDir, "tmp")
	return []string{
		fmt.Sprintf(`PATH=C:\WINDOWS\system32;C:\WINDOWS;C:\WINDOWS\System32\Wbem;C:\WINDOWS\System32\WindowsPowerShell\v1.0\;`),
		fmt.Sprintf("TMP=%s", tmpDir),
		fmt.Sprintf("TEMP=%s", tmpDir),
	}
}
