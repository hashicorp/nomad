package framer

import (
	"io"
)

type WriteCloseChecker struct {
	io.WriteCloser
	Closed bool
}

func (w *WriteCloseChecker) Close() error {
	w.Closed = true
	return w.WriteCloser.Close()
}

/*
// This test checks, that even if the frame size has not been hit, a flush will
// periodically occur.
func TestStreamFramer_Flush(t *testing.T) {
	// Create the stream framer
	r, w := io.Pipe()
	wrappedW := &WriteCloseChecker{WriteCloser: w}
	hRate, bWindow := 100*time.Millisecond, 100*time.Millisecond
	sf := NewStreamFramer(wrappedW, false, hRate, bWindow, 100)
	sf.Run()

	// Create a decoder
	dec := codec.NewDecoder(r, structs.JsonHandle)

	f := "foo"
	fe := "bar"
	d := []byte{0xa}
	o := int64(10)

	// Start the reader
	resultCh := make(chan struct{})
	go func() {
		for {
			var frame StreamFrame
			if err := dec.Decode(&frame); err != nil {
				t.Fatalf("failed to decode")
			}

			if frame.IsHeartbeat() {
				continue
			}

			if reflect.DeepEqual(frame.Data, d) && frame.Offset == o && frame.File == f && frame.FileEvent == fe {
				resultCh <- struct{}{}
				return
			}

		}
	}()

	// Write only 1 byte so we do not hit the frame size
	if err := sf.Send(f, fe, d, o); err != nil {
		t.Fatalf("Send() failed %v", err)
	}

	select {
	case <-resultCh:
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * bWindow):
		t.Fatalf("failed to flush")
	}

	// Close the reader and wait. This should cause the runner to exit
	if err := r.Close(); err != nil {
		t.Fatalf("failed to close reader")
	}

	select {
	case <-sf.ExitCh():
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * hRate):
		t.Fatalf("exit channel should close")
	}

	sf.Destroy()
	if !wrappedW.Closed {
		t.Fatalf("writer not closed")
	}
}

// This test checks that frames will be batched till the frame size is hit (in
// the case that is before the flush).
func TestStreamFramer_Batch(t *testing.T) {
	// Create the stream framer
	r, w := io.Pipe()
	wrappedW := &WriteCloseChecker{WriteCloser: w}
	// Ensure the batch window doesn't get hit
	hRate, bWindow := 100*time.Millisecond, 500*time.Millisecond
	sf := NewStreamFramer(wrappedW, false, hRate, bWindow, 3)
	sf.Run()

	// Create a decoder
	dec := codec.NewDecoder(r, structs.JsonHandle)

	f := "foo"
	fe := "bar"
	d := []byte{0xa, 0xb, 0xc}
	o := int64(10)

	// Start the reader
	resultCh := make(chan struct{})
	go func() {
		for {
			var frame StreamFrame
			if err := dec.Decode(&frame); err != nil {
				t.Fatalf("failed to decode")
			}

			if frame.IsHeartbeat() {
				continue
			}

			if reflect.DeepEqual(frame.Data, d) && frame.Offset == o && frame.File == f && frame.FileEvent == fe {
				resultCh <- struct{}{}
				return
			}
		}
	}()

	// Write only 1 byte so we do not hit the frame size
	if err := sf.Send(f, fe, d[:1], o); err != nil {
		t.Fatalf("Send() failed %v", err)
	}

	// Ensure we didn't get any data
	select {
	case <-resultCh:
		t.Fatalf("Got data before frame size reached")
	case <-time.After(bWindow / 2):
	}

	// Write the rest so we hit the frame size
	if err := sf.Send(f, fe, d[1:], o); err != nil {
		t.Fatalf("Send() failed %v", err)
	}

	// Ensure we get data
	select {
	case <-resultCh:
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * bWindow):
		t.Fatalf("Did not receive data after batch size reached")
	}

	// Close the reader and wait. This should cause the runner to exit
	if err := r.Close(); err != nil {
		t.Fatalf("failed to close reader")
	}

	select {
	case <-sf.ExitCh():
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * hRate):
		t.Fatalf("exit channel should close")
	}

	sf.Destroy()
	if !wrappedW.Closed {
		t.Fatalf("writer not closed")
	}
}

func TestStreamFramer_Heartbeat(t *testing.T) {
	// Create the stream framer
	r, w := io.Pipe()
	wrappedW := &WriteCloseChecker{WriteCloser: w}
	hRate, bWindow := 100*time.Millisecond, 100*time.Millisecond
	sf := NewStreamFramer(wrappedW, false, hRate, bWindow, 100)
	sf.Run()

	// Create a decoder
	dec := codec.NewDecoder(r, structs.JsonHandle)

	// Start the reader
	resultCh := make(chan struct{})
	go func() {
		for {
			var frame StreamFrame
			if err := dec.Decode(&frame); err != nil {
				t.Fatalf("failed to decode")
			}

			if frame.IsHeartbeat() {
				resultCh <- struct{}{}
				return
			}
		}
	}()

	select {
	case <-resultCh:
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * hRate):
		t.Fatalf("failed to heartbeat")
	}

	// Close the reader and wait. This should cause the runner to exit
	if err := r.Close(); err != nil {
		t.Fatalf("failed to close reader")
	}

	select {
	case <-sf.ExitCh():
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * hRate):
		t.Fatalf("exit channel should close")
	}

	sf.Destroy()
	if !wrappedW.Closed {
		t.Fatalf("writer not closed")
	}
}

// This test checks that frames are received in order
func TestStreamFramer_Order(t *testing.T) {
	// Create the stream framer
	r, w := io.Pipe()
	wrappedW := &WriteCloseChecker{WriteCloser: w}
	// Ensure the batch window doesn't get hit
	hRate, bWindow := 100*time.Millisecond, 10*time.Millisecond
	sf := NewStreamFramer(wrappedW, false, hRate, bWindow, 10)
	sf.Run()

	// Create a decoder
	dec := codec.NewDecoder(r, structs.JsonHandle)

	files := []string{"1", "2", "3", "4", "5"}
	input := bytes.NewBuffer(make([]byte, 0, 100000))
	for i := 0; i <= 1000; i++ {
		str := strconv.Itoa(i) + ","
		input.WriteString(str)
	}

	expected := bytes.NewBuffer(make([]byte, 0, 100000))
	for range files {
		expected.Write(input.Bytes())
	}
	receivedBuf := bytes.NewBuffer(make([]byte, 0, 100000))

	// Start the reader
	resultCh := make(chan struct{})
	go func() {
		for {
			var frame StreamFrame
			if err := dec.Decode(&frame); err != nil {
				t.Fatalf("failed to decode")
			}

			if frame.IsHeartbeat() {
				continue
			}

			receivedBuf.Write(frame.Data)

			if reflect.DeepEqual(expected, receivedBuf) {
				resultCh <- struct{}{}
				return
			}
		}
	}()

	// Send the data
	b := input.Bytes()
	shards := 10
	each := len(b) / shards
	for _, f := range files {
		for i := 0; i < shards; i++ {
			l, r := each*i, each*(i+1)
			if i == shards-1 {
				r = len(b)
			}

			if err := sf.Send(f, "", b[l:r], 0); err != nil {
				t.Fatalf("Send() failed %v", err)
			}
		}
	}

	// Ensure we get data
	select {
	case <-resultCh:
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * bWindow):
		if reflect.DeepEqual(expected, receivedBuf) {
			got := receivedBuf.String()
			want := expected.String()
			t.Fatalf("Got %v; want %v", got, want)
		}
	}

	// Close the reader and wait. This should cause the runner to exit
	if err := r.Close(); err != nil {
		t.Fatalf("failed to close reader")
	}

	select {
	case <-sf.ExitCh():
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * hRate):
		t.Fatalf("exit channel should close")
	}

	sf.Destroy()
	if !wrappedW.Closed {
		t.Fatalf("writer not closed")
	}
}

// This test checks that frames are received in order
func TestStreamFramer_Order_PlainText(t *testing.T) {
	// Create the stream framer
	r, w := io.Pipe()
	wrappedW := &WriteCloseChecker{WriteCloser: w}
	// Ensure the batch window doesn't get hit
	hRate, bWindow := 100*time.Millisecond, 10*time.Millisecond
	sf := NewStreamFramer(wrappedW, true, hRate, bWindow, 10)
	sf.Run()

	files := []string{"1", "2", "3", "4", "5"}
	input := bytes.NewBuffer(make([]byte, 0, 100000))
	for i := 0; i <= 1000; i++ {
		str := strconv.Itoa(i) + ","
		input.WriteString(str)
	}

	expected := bytes.NewBuffer(make([]byte, 0, 100000))
	for range files {
		expected.Write(input.Bytes())
	}
	receivedBuf := bytes.NewBuffer(make([]byte, 0, 100000))

	// Start the reader
	resultCh := make(chan struct{})
	go func() {
	OUTER:
		for {
			if _, err := receivedBuf.ReadFrom(r); err != nil {
				if strings.Contains(err.Error(), "closed pipe") {
					resultCh <- struct{}{}
					return
				}
				t.Fatalf("bad read: %v", err)
			}

			if expected.Len() != receivedBuf.Len() {
				continue
			}
			expectedBytes := expected.Bytes()
			actualBytes := receivedBuf.Bytes()
			for i, e := range expectedBytes {
				if a := actualBytes[i]; a != e {
					continue OUTER
				}
			}
			resultCh <- struct{}{}
			return

		}
	}()

	// Send the data
	b := input.Bytes()
	shards := 10
	each := len(b) / shards
	for _, f := range files {
		for i := 0; i < shards; i++ {
			l, r := each*i, each*(i+1)
			if i == shards-1 {
				r = len(b)
			}

			if err := sf.Send(f, "", b[l:r], 0); err != nil {
				t.Fatalf("Send() failed %v", err)
			}
		}
	}

	// Ensure we get data
	select {
	case <-resultCh:
	case <-time.After(10 * time.Duration(testutil.TestMultiplier()) * bWindow):
		if expected.Len() != receivedBuf.Len() {
			t.Fatalf("Got %v; want %v", expected.Len(), receivedBuf.Len())
		}
		expectedBytes := expected.Bytes()
		actualBytes := receivedBuf.Bytes()
		for i, e := range expectedBytes {
			if a := actualBytes[i]; a != e {
				t.Fatalf("Index %d; Got %q; want %q", i, a, e)
			}
		}
	}

	// Close the reader and wait. This should cause the runner to exit
	if err := r.Close(); err != nil {
		t.Fatalf("failed to close reader")
	}

	sf.Destroy()
	if !wrappedW.Closed {
		t.Fatalf("writer not closed")
	}
}
*/
