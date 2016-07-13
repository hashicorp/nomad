package command

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/ryanuber/columnize"
)

// formatKV takes a set of strings and formats them into properly
// aligned k = v pairs using the columnize library.
func formatKV(in []string) string {
	columnConf := columnize.DefaultConfig()
	columnConf.Empty = "<none>"
	columnConf.Glue = " = "
	return columnize.Format(in, columnConf)
}

// formatList takes a set of strings and formats them into properly
// aligned output, replacing any blank fields with a placeholder
// for awk-ability.
func formatList(in []string) string {
	columnConf := columnize.DefaultConfig()
	columnConf.Empty = "<none>"
	return columnize.Format(in, columnConf)
}

// formatListWithSpaces takes a set of strings and formats them into properly
// aligned output. It should be used sparingly since it doesn't replace empty
// values and hence not awk/sed friendly
func formatListWithSpaces(in []string) string {
	columnConf := columnize.DefaultConfig()
	return columnize.Format(in, columnConf)
}

// Limits the length of the string.
func limit(s string, length int) string {
	if len(s) < length {
		return s
	}

	return s[:length]
}

// formatTime formats the time to string based on RFC822
func formatTime(t time.Time) string {
	return t.Format("01/02/06 15:04:05 MST")
}

// formatTimeDifference takes two times and determines their duration difference
// truncating to a passed unit.
// E.g. formatTimeDifference(first=1m22s33ms, second=1m28s55ms, time.Second) -> 6s
func formatTimeDifference(first, second time.Time, d time.Duration) string {
	return second.Truncate(d).Sub(first.Truncate(d)).String()
}

// getLocalNodeID returns the node ID of the local Nomad Client and an error if
// it couldn't be determined or the Agent is not running in Client mode.
func getLocalNodeID(client *api.Client) (string, error) {
	info, err := client.Agent().Self()
	if err != nil {
		return "", fmt.Errorf("Error querying agent info: %s", err)
	}
	var stats map[string]interface{}
	stats, _ = info["stats"]
	clientStats, ok := stats["client"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("Nomad not running in client mode")
	}

	nodeID, ok := clientStats["node_id"].(string)
	if !ok {
		return "", fmt.Errorf("Failed to determine node ID")
	}

	return nodeID, nil
}

// evalFailureStatus returns whether the evaluation has failures and a string to
// display when presenting users with whether there are failures for the eval
func evalFailureStatus(eval *api.Evaluation) (string, bool) {
	if eval == nil {
		return "", false
	}

	hasFailures := len(eval.FailedTGAllocs) != 0
	text := strconv.FormatBool(hasFailures)
	if eval.Status == "blocked" {
		text = "N/A - In Progress"
	}

	return text, hasFailures
}

// LineLimitReader wraps another reader and provides `tail -n` like behavior.
// LineLimitReader buffers up to the searchLimit and returns `-n` number of
// lines. After those lines have been returned, LineLimitReader streams the
// underlying ReadCloser
type LineLimitReader struct {
	io.ReadCloser
	lines       int
	searchLimit int

	buffer     *bytes.Buffer
	bufFiled   bool
	foundLines bool
}

// NewLineLimitReader takes the ReadCloser to wrap, the number of lines to find
// searching backwards in the first searchLimit bytes.
func NewLineLimitReader(r io.ReadCloser, lines, searchLimit int) *LineLimitReader {
	return &LineLimitReader{
		ReadCloser:  r,
		searchLimit: searchLimit,
		lines:       lines,
		buffer:      bytes.NewBuffer(make([]byte, 0, searchLimit)),
	}
}

func (l *LineLimitReader) Read(p []byte) (n int, err error) {
	// Fill up the buffer so we can find the correct number of lines.
	if !l.bufFiled {
		_, err := l.buffer.ReadFrom(io.LimitReader(l.ReadCloser, int64(l.searchLimit)))
		if err != nil {
			return 0, err
		}

		l.bufFiled = true
	}

	if l.bufFiled && l.buffer.Len() != 0 {
		b := l.buffer.Bytes()

		// Find the lines
		if !l.foundLines {
			found := 0
			i := len(b) - 1
			sep := byte('\n')
			lastIndex := len(b) - 1
			for ; found < l.lines && i >= 0; i-- {
				if b[i] == sep {
					lastIndex = i

					// Skip the first one
					if i != len(b)-1 {
						found++
					}
				}
			}

			// We found them all
			if found == l.lines {
				// Clear the buffer until the last index
				l.buffer.Next(lastIndex + 1)
			}

			l.foundLines = true
		}

		// Read from the buffer
		n := copy(p, l.buffer.Next(len(p)))
		return n, nil
	}

	// Just stream from the underlying reader now
	return l.ReadCloser.Read(p)
}
