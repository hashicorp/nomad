// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

const (
	testFileName = "Nomad.log"
	testDuration = 2 * time.Second
	testBytes    = 10
)

func TestLogFile_timeRotation(t *testing.T) {
	ci.Parallel(t)

	tempDir := t.TempDir()

	logFile := logFile{
		fileName: testFileName,
		logPath:  tempDir,
		duration: testDuration,
	}

	_, err := logFile.Write([]byte("Hello World"))
	must.NoError(t, err)
	time.Sleep(2 * time.Second)
	_, err = logFile.Write([]byte("Second File"))
	must.NoError(t, err)

	numEntries, err := os.ReadDir(tempDir)
	must.NoError(t, err)
	must.Len(t, 2, numEntries)
}

func TestLogFile_openNew(t *testing.T) {
	ci.Parallel(t)

	tempDir := t.TempDir()

	logFile := logFile{
		fileName: testFileName,
		logPath:  tempDir,
		MaxBytes: testBytes,
		duration: 24 * time.Hour,
	}
	must.NoError(t, logFile.openNew())

	_, err := os.ReadFile(logFile.FileInfo.Name())
	must.NoError(t, err)

	must.Eq(t, logFile.FileInfo.Name(), filepath.Join(tempDir, testFileName))

	// Check if create time and bytes written are kept when opening the active
	// log file again.
	bytesWritten, err := logFile.Write([]byte("test"))
	must.NoError(t, err)

	time.Sleep(2 * time.Second)
	must.NoError(t, err)

	timeDelta := time.Now().Sub(logFile.LastCreated)
	must.Greater(t, 2*time.Second, timeDelta)
	must.Eq(t, logFile.BytesWritten, int64(bytesWritten))
}

func TestLogFile_byteRotation(t *testing.T) {
	ci.Parallel(t)

	tempDir := t.TempDir()

	logFile := logFile{
		fileName: testFileName,
		logPath:  tempDir,
		MaxBytes: testBytes,
		duration: 24 * time.Hour,
	}
	_, err := logFile.Write([]byte("Hello World"))
	must.NoError(t, err)
	_, err = logFile.Write([]byte("Second File"))
	must.NoError(t, err)

	tempFiles, err := os.ReadDir(tempDir)
	must.NoError(t, err)
	must.Len(t, 2, tempFiles)
}

func TestLogFile_deleteArchives(t *testing.T) {
	ci.Parallel(t)

	tempDir := t.TempDir()

	logFile := logFile{
		fileName: testFileName,
		logPath:  tempDir,
		MaxBytes: testBytes,
		duration: 24 * time.Hour,
		MaxFiles: 1,
	}
	_, err := logFile.Write([]byte("[INFO] Hello World"))
	must.NoError(t, err)
	_, err = logFile.Write([]byte("[INFO] Second File"))
	must.NoError(t, err)
	_, err = logFile.Write([]byte("[INFO] Third File"))
	must.NoError(t, err)

	tempFiles, err := os.ReadDir(tempDir)
	must.NoError(t, err)
	must.Len(t, 2, tempFiles)

	for _, tempFile := range tempFiles {
		bytes, err := os.ReadFile(filepath.Join(tempDir, tempFile.Name()))
		must.NoError(t, err)
		must.StrNotEqFold(t, "[INFO] Hello World", string(bytes))
	}
}

func TestLogFile_deleteArchivesDisabled(t *testing.T) {
	ci.Parallel(t)

	tempDir := t.TempDir()

	logFile := logFile{
		fileName: testFileName,
		logPath:  tempDir,
		MaxBytes: testBytes,
		duration: 24 * time.Hour,
		MaxFiles: 0,
	}
	_, err := logFile.Write([]byte("[INFO] Hello World"))
	must.NoError(t, err)
	_, err = logFile.Write([]byte("[INFO] Second File"))
	must.NoError(t, err)
	_, err = logFile.Write([]byte("[INFO] Third File"))
	must.NoError(t, err)

	tempFiles, err := os.ReadDir(tempDir)
	must.NoError(t, err)
	must.Len(t, 3, tempFiles)
}
