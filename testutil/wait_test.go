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

	N := 10

	tmpDir, err := os.MkdirTemp("", "waiter")
	require.NoError(t, err)

	defer func() {
		require.NoError(t, os.RemoveAll(tmpDir))
	}()

	var files []string
	for i := 1; i < N; i++ {
		files = append(files, filepath.Join(
			tmpDir, fmt.Sprintf("test%d.txt", i),
		))
	}

	go func() {
		for _, file := range files {
			t.Logf("Creating file %s ...", file)
			fh, createErr := os.Create(file)
			require.NoError(t, createErr)

			closeErr := fh.Close()
			require.NoError(t, closeErr)
			require.FileExists(t, file)

			time.Sleep(250 * time.Millisecond)
		}
	}()

	duration := 5 * time.Second
	t.Log("Waiting 5 seconds for files ...")
	WaitForFilesUntil(t, files, duration)
}
