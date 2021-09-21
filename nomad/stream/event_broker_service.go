package stream

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
)

const SockAddr = "/var/run/nomad/event-broker-service.sock"

func eventBrokerServer(c net.Conn) {
	log.Printf("Client connected [%s]", c.RemoteAddr().Network())
	io.Copy(c, c)
	c.Close()
}

func RegisterEventBrokerService(logger hclog.InterceptLogger, logOutput io.Writer, inmem *metrics.InmemSink) error {
	if err := os.RemoveAll(SockAddr); err != nil {
		log.Fatal(err)
	}

	// TODO: Add windows support https://github.com/Microsoft/go-winio
	l, err := net.Listen("unix", SockAddr)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	defer l.Close()

	logger.Info(fmt.Sprintf("EventBrokerService listening on %s", l.Addr()))

	for {
		// Accept new connections, dispatching them to echoServer
		// in a goroutine.
		conn, err := l.Accept()
		if err != nil {
			log.Fatal("accept error:", err)
		}

		go eventBrokerServer(conn)
	}

	return nil
}
