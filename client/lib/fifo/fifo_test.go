package fifo

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFIFO(t *testing.T) {
	require := require.New(t)
	var path string

	if runtime.GOOS == "windows" {
		path = "//./pipe/fifo"
	} else {
		dir, err := ioutil.TempDir("", "")
		require.NoError(err)
		defer os.RemoveAll(dir)

		path = filepath.Join(dir, "fifo")
	}

	reader, err := New(path)
	require.NoError(err)

	toWrite := [][]byte{
		[]byte("abc\n"),
		[]byte(""),
		[]byte("def\n"),
		[]byte("nomad"),
		[]byte("\n"),
	}

	var readBuf bytes.Buffer
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()
		io.Copy(&readBuf, reader)
	}()

	writer, err := OpenWriter(path)
	require.NoError(err)
	for _, b := range toWrite {
		n, err := writer.Write(b)
		require.NoError(err)
		require.Equal(n, len(b))
	}
	require.NoError(writer.Close())
	time.Sleep(500 * time.Millisecond)
	require.NoError(reader.Close())

	wait.Wait()

	expected := "abc\ndef\nnomad\n"
	require.Equal(expected, readBuf.String())

	require.NoError(Remove(path))
}
