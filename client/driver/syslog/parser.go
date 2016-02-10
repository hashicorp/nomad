package syslog

import (
	"bufio"
	"fmt"
	"log"
	"log/syslog"
	"strconv"
	"time"

	"github.com/jeromer/syslogparser"
)

var (
	ErrPriorityNoStart  = fmt.Errorf("No start char found for priority")
	ErrPriorityEmpty    = fmt.Errorf("Priority field empty")
	ErrPriorityNoEnd    = fmt.Errorf("No end char found for priority")
	ErrPriorityTooShort = fmt.Errorf("Priority field too short")
	ErrPriorityTooLong  = fmt.Errorf("Priority field too long")
	ErrPriorityNonDigit = fmt.Errorf("Non digit found in priority")
)

const (
	PRI_PART_START = '<'
	PRI_PART_END   = '>'
)

type Priority struct {
	P syslog.Priority
	F syslog.Priority
	S syslog.Priority
}

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
	severity, _, _ := d.parsePriority(d.line)
	msgIdx := d.logContentIndex(d.line)
	return map[string]interface{}{
		"content":  d.line[msgIdx:],
		"severity": severity.S,
	}
}

func (d *DockerLogParser) logContentIndex(line []byte) int {
	cursor := 0
	numSpace := 0
	for i := 0; i < len(line); i++ {
		if line[i] == ' ' {
			numSpace += 1
			if numSpace == 1 {
				cursor = i
				break
			}
		}
	}
	for i := cursor; i < len(line); i++ {
		if line[i] == ':' {
			cursor = i
			break
		}
	}
	return cursor + 1
}

func (d *DockerLogParser) parsePriority(line []byte) (Priority, int, error) {
	cursor := 0
	pri := d.newPriority(0)
	if len(line) <= 0 {
		return pri, cursor, ErrPriorityEmpty
	}
	if line[cursor] != PRI_PART_START {
		return pri, cursor, ErrPriorityNoStart
	}
	i := 1
	priDigit := 0
	for i < len(line) {
		if i >= 5 {
			return pri, cursor, ErrPriorityTooLong
		}
		c := line[i]
		if c == PRI_PART_END {
			if i == 1 {
				return pri, cursor, ErrPriorityTooShort
			}
			cursor = i + 1
			return d.newPriority(priDigit), cursor, nil
		}
		if d.isDigit(c) {
			v, e := strconv.Atoi(string(c))
			if e != nil {
				return pri, cursor, e
			}
			priDigit = (priDigit * 10) + v
		} else {
			return pri, cursor, ErrPriorityNonDigit
		}
		i++
	}
	return pri, cursor, ErrPriorityNoEnd
}

func (d *DockerLogParser) isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func (d *DockerLogParser) newPriority(p int) Priority {
	// The Priority value is calculated by first multiplying the Facility
	// number by 8 and then adding the numerical value of the Severity.
	return Priority{
		P: syslog.Priority(p),
		F: syslog.Priority(p / 8),
		S: syslog.Priority(p % 8),
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
