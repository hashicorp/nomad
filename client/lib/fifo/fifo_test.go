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

	"github.com/hashicorp/nomad/helper/uuid"
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

	writer, err := Open(path)
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

func TestWriteClose(t *testing.T) {
	require := require.New(t)
	var path string

	if runtime.GOOS == "windows" {
		path = "//./pipe/" + uuid.Generate()[:4]
	} else {
		dir, err := ioutil.TempDir("", "")
		require.NoError(err)
		defer os.RemoveAll(dir)

		path = filepath.Join(dir, "fifo")
	}

	reader, err := New(path)
	require.NoError(err)

	var readBuf bytes.Buffer
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()
		io.Copy(&readBuf, reader)
	}()

	writer, err := Open(path)
	require.NoError(err)

	var count int
	wait.Add(1)
	go func() {
		defer wait.Done()
		for count = 0; count < int(^uint16(0)); count++ {
			_, err := writer.Write([]byte(","))
			if err != nil && IsClosedErr(err) {
				break
			}
			require.NoError(err)
			time.Sleep(5 * time.Millisecond)
		}
	}()

	time.Sleep(500 * time.Millisecond)
	require.NoError(writer.Close())
	wait.Wait()

	require.Equal(count, len(readBuf.String()))
}
