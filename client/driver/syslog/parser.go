package syslog

import (
	"bufio"
	"log"
	"time"

	"github.com/jeromer/syslogparser"
)

type DockerLogParser struct {
	line []byte

	log *log.Logger
}

func NewDockerLogParser(line []byte) *DockerLogParser {
	return &DockerLogParser{line: line}
}

func (d *DockerLogParser) Parse() error {
	return nil
}

func (d *DockerLogParser) Dump() syslogparser.LogParts {
	return map[string]interface{}{
		"content": string(d.line),
	}
}

func (d *DockerLogParser) Location(location *time.Location) {
}

type CustomParser struct {
	logger *log.Logger
}

func (c *CustomParser) GetParser(line []byte) syslogparser.LogParser {
	return NewDockerLogParser(line)
}

func (c *CustomParser) GetSplitFunc() bufio.SplitFunc {
	return nil
}
