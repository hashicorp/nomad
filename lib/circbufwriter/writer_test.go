package circbufwriter

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestWriter_NonBlockingWrite(t *testing.T) {
	require := require.New(t)
	var buf bytes.Buffer
	w := New(&buf, 64)
	n, err := w.Write([]byte("test"))
	require.Equal(4, n)
	require.NoError(err)

	n, err = w.Write([]byte("test"))
	require.Equal(4, n)
	require.NoError(err)

	testutil.WaitForResult(func() (bool, error) {
		return "testtest" == buf.String(), fmt.Errorf("expected both writes")
	}, func(err error) {
		require.NoError(err)
	})
}

type blockingWriter struct {
	buf     bytes.Buffer
	unblock <-chan struct{}
}

func (b *blockingWriter) Write(p []byte) (nn int, err error) {
	<-b.unblock
	return b.buf.Write(p)
}

func TestWriter_BlockingWrite(t *testing.T) {
	require := require.New(t)
	blockCh := make(chan struct{})
	bw := &blockingWriter{unblock: blockCh}
	w := New(bw, 64)

	n, err := w.Write([]byte("test"))
	require.Equal(4, n)
	require.NoError(err)
	require.Empty(bw.buf.Bytes())

	n, err = w.Write([]byte("test"))
	require.Equal(4, n)
	require.NoError(err)
	require.Empty(bw.buf.Bytes())
	close(blockCh)

	testutil.WaitForResult(func() (bool, error) {
		return "testtest" == bw.buf.String(), fmt.Errorf("expected both writes")
	}, func(err error) {
		require.NoError(err)
	})
}

func TestWriter_CloseClose(t *testing.T) {
	require := require.New(t)
	w := New(ioutil.Discard, 64)
	require.NoError(w.Close())
	require.NoError(w.Close())
}
