package agent

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/logutils"
	"github.com/stretchr/testify/require"
)

const (
	testFileName = "Nomad.log"
	testDuration = 2 * time.Second
	testBytes    = 10
)

func TestLogFile_timeRotation(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	tempDir, err := ioutil.TempDir("", "LogWriterTimeTest")
	require.NoError(err)

	defer os.Remove(tempDir)

	filt := LevelFilter()
	logFile := logFile{
		logFilter: filt,
		fileName:  testFileName,
		logPath:   tempDir,
		duration:  testDuration,
	}
	logFile.Write([]byte("Hello World"))
	time.Sleep(2 * time.Second)
	logFile.Write([]byte("Second File"))
	want := 2
	if got, _ := ioutil.ReadDir(tempDir); len(got) != want {
		t.Errorf("Expected %d files, got %v file(s)", want, len(got))
	}
}

func TestLogFile_openNew(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	tempDir, err := ioutil.TempDir("", "LogWriterOpenTest")
	require.NoError(err)
	defer os.Remove(tempDir)

	logFile := logFile{fileName: testFileName, logPath: tempDir, duration: testDuration}
	require.NoError(logFile.openNew())

	_, err = ioutil.ReadFile(logFile.FileInfo.Name())
	require.NoError(err)

	require.Equal(logFile.FileInfo.Name(), filepath.Join(tempDir, testFileName))
}

func TestLogFile_byteRotation(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	tempDir, err := ioutil.TempDir("", "LogWriterByteTest")
	require.NoError(err)
	defer os.Remove(tempDir)

	filt := LevelFilter()
	filt.MinLevel = logutils.LogLevel("INFO")
	logFile := logFile{
		logFilter: filt,
		fileName:  testFileName,
		logPath:   tempDir,
		MaxBytes:  testBytes,
		duration:  24 * time.Hour,
	}
	logFile.Write([]byte("Hello World"))
	logFile.Write([]byte("Second File"))
	want := 2
	tempFiles, _ := ioutil.ReadDir(tempDir)
	require.Equal(want, len(tempFiles))
}

func TestLogFile_logLevelFiltering(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	tempDir, err := ioutil.TempDir("", "LogWriterFilterTest")
	require.NoError(err)
	defer os.Remove(tempDir)
	filt := LevelFilter()
	logFile := logFile{
		logFilter: filt,
		fileName:  testFileName,
		logPath:   tempDir,
		MaxBytes:  testBytes,
		duration:  testDuration,
	}
	logFile.Write([]byte("[INFO] This is an info message"))
	logFile.Write([]byte("[DEBUG] This is a debug message"))
	logFile.Write([]byte("[ERR] This is an error message"))
	want := 2
	tempFiles, _ := ioutil.ReadDir(tempDir)
	require.Equal(want, len(tempFiles))
}

func TestLogFile_deleteArchives(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	tempDir, err := ioutil.TempDir("", "LogWriterDeleteArchivesTest")
	require.NoError(err)
	defer os.Remove(tempDir)

	filt := LevelFilter()
	filt.MinLevel = logutils.LogLevel("INFO")
	logFile := logFile{
		logFilter: filt,
		fileName:  testFileName,
		logPath:   tempDir,
		MaxBytes:  testBytes,
		duration:  24 * time.Hour,
		MaxFiles:  1,
	}
	logFile.Write([]byte("[INFO] Hello World"))
	logFile.Write([]byte("[INFO] Second File"))
	logFile.Write([]byte("[INFO] Third File"))
	want := 2
	tempFiles, _ := ioutil.ReadDir(tempDir)

	require.Equal(want, len(tempFiles))

	for _, tempFile := range tempFiles {
		var bytes []byte
		var err error
		path := filepath.Join(tempDir, tempFile.Name())
		if bytes, err = ioutil.ReadFile(path); err != nil {
			t.Errorf(err.Error())
			return
		}
		contents := string(bytes)

		require.NotEqual("[INFO] Hello World", contents, "oldest log should have been deleted")
	}
}

func TestLogFile_deleteArchivesDisabled(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	tempDir, err := ioutil.TempDir("", "LogWriterDeleteArchivesDisabledTest")
	require.NoError(err)
	defer os.Remove(tempDir)

	filt := LevelFilter()
	filt.MinLevel = logutils.LogLevel("INFO")
	logFile := logFile{
		logFilter: filt,
		fileName:  testFileName,
		logPath:   tempDir,
		MaxBytes:  testBytes,
		duration:  24 * time.Hour,
		MaxFiles:  0,
	}
	logFile.Write([]byte("[INFO] Hello World"))
	logFile.Write([]byte("[INFO] Second File"))
	logFile.Write([]byte("[INFO] Third File"))
	want := 3
	tempFiles, _ := ioutil.ReadDir(tempDir)
	require.Equal(want, len(tempFiles))
}
