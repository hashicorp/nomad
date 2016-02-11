package logcollector

import (
	"bufio"
	"fmt"
	"log"
	"log/syslog"
	"strconv"
	"time"

	"github.com/jeromer/syslogparser"
)

// Errors related to parsing priority
var (
	ErrPriorityNoStart  = fmt.Errorf("No start char found for priority")
	ErrPriorityEmpty    = fmt.Errorf("Priority field empty")
	ErrPriorityNoEnd    = fmt.Errorf("No end char found for priority")
	ErrPriorityTooShort = fmt.Errorf("Priority field too short")
	ErrPriorityTooLong  = fmt.Errorf("Priority field too long")
	ErrPriorityNonDigit = fmt.Errorf("Non digit found in priority")
)

// Priority header and ending characters
const (
	PRI_PART_START = '<'
	PRI_PART_END   = '>'
)

// Priority holds all the priority bits in a syslog log line
type Priority struct {
	Pri      int
	Facility syslog.Priority
	Severity syslog.Priority
}

// DockerLogParser parses a line of log message that the docker daemon ships
type DockerLogParser struct {
	line     []byte
	content  []byte
	severity Priority

	log *log.Logger
}

// NewDockerLogParser creates a new DockerLogParser
func NewDockerLogParser(line []byte) *DockerLogParser {
	return &DockerLogParser{line: line}
}

// Parse parses a syslog log line
func (d *DockerLogParser) Parse() error {
	severity, _, _ := d.parsePriority(d.line)
	msgIdx := d.logContentIndex(d.line)
	d.severity = severity
	d.content = d.line[msgIdx:]
	return nil
}

// Dump creates a map of the parsed log line and severity
func (d *DockerLogParser) Dump() syslogparser.LogParts {
	return map[string]interface{}{
		"content":  d.content,
		"severity": d.severity,
	}
}

// logContentIndex finds out the index of the start index of the content in a
// syslog line
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

// parsePriority parses the priority in a syslog message
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

// isDigit checks if a byte is a numeric char
func (d *DockerLogParser) isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

// newPriority creates a new default priority
func (d *DockerLogParser) newPriority(p int) Priority {
	// The Priority value is calculated by first multiplying the Facility
	// number by 8 and then adding the numerical value of the Severity.
	return Priority{
		Pri:      p,
		Facility: syslog.Priority(p / 8),
		Severity: syslog.Priority(p % 8),
	}
}

func (d *DockerLogParser) Location(location *time.Location) {
}

// CustomParser is a parser to parse docker syslog lines
type CustomParser struct {
	logger *log.Logger
}

func (c *CustomParser) GetParser(line []byte) syslogparser.LogParser {
	return NewDockerLogParser(line)
}

func (c *CustomParser) GetSplitFunc() bufio.SplitFunc {
	return nil
}
