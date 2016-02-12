package format

import (
	"bufio"

	"github.com/jeromer/syslogparser"
)

type Format interface {
	GetParser([]byte) syslogparser.LogParser
	GetSplitFunc() bufio.SplitFunc
}
