package logging

import (
	"io/ioutil"
	"net"
	"os"
	"path"
	"testing"
	"time"
)

func TestSyslogServer_Start_Shutdown(t *testing.T) {
	t.Parallel()
	dir, err := ioutil.TempDir("", "sock")
	if err != nil {
		t.Fatalf("Failed to create temporary direcotry: %v", err)
	}

	sock := path.Join(dir, "socket")
	defer os.Remove(sock)

	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("Failed to listen unix socket: %v", err)
	}

	s := NewSyslogServer(l, make(chan *SyslogMessage, 2048), nil)

	go s.Start()
	if s.done {
		t.Fatalf("expected running SyslogServer, but not running")
	}

	received := false
	go func() {
		for _ = range s.messages {
			received = true
		}
	}()

	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("expected access to SyslogServer, but %v", err)
	}

	_, err = conn.Write([]byte("syslog server test\n"))
	if err != nil {
		t.Fatalf("expected send data to SyslogServer but: %v", err)
	}

	// Need to wait until SyslogServer received the data certainly
	time.Sleep(1000 * time.Millisecond)

	if !received {
		t.Fatalf("expected SyslogServer received data, but not received")
	}

	defer conn.Close()

	s.Shutdown()
	if !s.done {
		t.Fatalf("expected SyslogServer done, but running")
	}
}
