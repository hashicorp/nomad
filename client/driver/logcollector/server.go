package logcollector

import (
	"bufio"
	"log"
	"net"
)

type SyslogServer struct {
	listener net.Listener
	messages chan *SyslogMessage
	parser   *DockerLogParser

	doneCh chan interface{}
	logger *log.Logger
}

func NewSyslogServer(l net.Listener, messages chan *SyslogMessage, logger *log.Logger) *SyslogServer {
	parser := NewDockerLogParser(logger)
	return &SyslogServer{
		listener: l,
		messages: messages,
		parser:   parser,
		logger:   logger,
		doneCh:   make(chan interface{}),
	}
}

func (s *SyslogServer) Start() {
	for {
		select {
		case <-s.doneCh:
			return
		default:
			connection, err := s.listener.Accept()
			s.logger.Printf("DIPTANU ACCEPTED CON")
			if err != nil {
				s.logger.Printf("[ERROR] logcollector.server: error in accepting connection: %v", err)
				continue
			}
			go s.Read(connection)
		}
	}
}

func (s *SyslogServer) Read(connection net.Conn) {
	defer connection.Close()
	scanner := bufio.NewScanner(bufio.NewReader(connection))

LOOP:
	for {
		select {
		case <-s.doneCh:
			break LOOP
		default:
		}
		if scanner.Scan() {
			b := scanner.Bytes()
			s.logger.Printf("DIPTANU READ BYTES %v", b)
			msg := s.parser.Parse(b)
			s.messages <- msg
		} else {
			break LOOP
		}
	}
}

func (s *SyslogServer) Shutdown() {
	close(s.doneCh)
}
