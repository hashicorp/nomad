package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWait_WaitForFilesUntil(t *testing.T) {

	var files []string
	for i := 1; i < 10; i++ {
		filename := fmt.Sprintf("test%d.txt", i)
		filepath := filepath.Join(os.TempDir(), filename)
		files = append(files, filepath)

		defer os.Remove(filepath)
	}

	go func() {
		for _, filepath := range files {
			t.Logf("Creating file %s...", filepath)
			fh, err := os.Create(filepath)
			fh.Close()

			require.NoError(t, err, "error creating test file")
			require.FileExists(t, filepath)

			time.Sleep(250 * time.Millisecond)
		}
	}()

	duration := 5 * time.Second
	t.Log("Waiting 5 seconds for files...")
	WaitForFilesUntil(t, files, duration)

	t.Log("done")

}
