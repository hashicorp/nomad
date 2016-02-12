package format

import (
	"bufio"
	"bytes"
	"strconv"

	"github.com/jeromer/syslogparser"
	"github.com/jeromer/syslogparser/rfc5424"
)

type RFC6587 struct{}

func (f *RFC6587) GetParser(line []byte) syslogparser.LogParser {
	return rfc5424.NewParser(line)
}

func (f *RFC6587) GetSplitFunc() bufio.SplitFunc {
	return rfc6587ScannerSplit
}

func rfc6587ScannerSplit(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	if i := bytes.IndexByte(data, ' '); i > 0 {
		pLength := data[0:i]
		length, err := strconv.Atoi(string(pLength))
		if err != nil {
			return 0, nil, err
		}
		end := length + i + 1
		if len(data) >= end {
			//Return the frame with the length removed
			return end, data[i+1 : end], nil
		}
	}

	// Request more data
	return 0, nil, nil
}
