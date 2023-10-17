// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	gg "github.com/hashicorp/go-getter"
	"github.com/hashicorp/nomad/api"
	flaghelper "github.com/hashicorp/nomad/helper/flags"
	"github.com/hashicorp/nomad/jobspec"
	"github.com/hashicorp/nomad/jobspec2"
	"github.com/kr/text"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/ryanuber/columnize"
)

const (
	formatJSON = "json"
	formatHCL1 = "hcl1"
	formatHCL2 = "hcl2"
)

// maxLineLength is the maximum width of any line.
const maxLineLength int = 78

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

// indentString returns the string s padded with the given number of empty
// spaces before each line except for the first one.
func indentString(s string, pad int) string {
	prefix := strings.Repeat(" ", pad)
	return strings.Join(strings.Split(s, "\n"), fmt.Sprintf("\n%s", prefix))
}

// wrapAtLengthWithPadding wraps the given text at the maxLineLength, taking
// into account any provided left padding.
func wrapAtLengthWithPadding(s string, pad int) string {
	wrapped := text.Wrap(s, maxLineLength-pad)
	lines := strings.Split(wrapped, "\n")
	for i, line := range lines {
		lines[i] = strings.Repeat(" ", pad) + line
	}
	return strings.Join(lines, "\n")
}

// wrapAtLength wraps the given text to maxLineLength.
func wrapAtLength(s string) string {
	return wrapAtLengthWithPadding(s, 0)
}

// formatTime formats the time to string based on RFC822
func formatTime(t time.Time) string {
	if t.Unix() < 1 {
		// It's more confusing to display the UNIX epoch or a zero value than nothing
		return ""
	}
	// Return ISO_8601 time format GH-3806
	return t.Format("2006-01-02T15:04:05Z07:00")
}

// formatUnixNanoTime is a helper for formatting time for output.
func formatUnixNanoTime(nano int64) string {
	t := time.Unix(0, nano)
	return formatTime(t)
}

// formatTimeDifference takes two times and determines their duration difference
// truncating to a passed unit.
// E.g. formatTimeDifference(first=1m22s33ms, second=1m28s55ms, time.Second) -> 6s
func formatTimeDifference(first, second time.Time, d time.Duration) string {
	return second.Truncate(d).Sub(first.Truncate(d)).String()
}

// fmtInt formats v into the tail of buf.
// It returns the index where the output begins.
func fmtInt(buf []byte, v uint64) int {
	w := len(buf)
	for v > 0 {
		w--
		buf[w] = byte(v%10) + '0'
		v /= 10
	}
	return w
}

// prettyTimeDiff prints a human readable time difference.
// It uses abbreviated forms for each period - s for seconds, m for minutes, h for hours,
// d for days, mo for months, and y for years. Time difference is rounded to the nearest second,
// and the top two least granular periods are returned. For example, if the time difference
// is 10 months, 12 days, 3 hours and 2 seconds, the string "10mo12d" is returned. Zero values return the empty string
func prettyTimeDiff(first, second time.Time) string {
	// handle zero values
	if first.IsZero() || first.UnixNano() == 0 {
		return ""
	}
	// round to the nearest second
	first = first.Round(time.Second)
	second = second.Round(time.Second)

	// calculate time difference in seconds
	var d time.Duration
	messageSuffix := "ago"
	if second.Equal(first) || second.After(first) {
		d = second.Sub(first)
	} else {
		d = first.Sub(second)
		messageSuffix = "from now"
	}

	u := uint64(d.Seconds())

	var buf [32]byte
	w := len(buf)
	secs := u % 60

	// track indexes of various periods
	var indexes []int

	if secs > 0 {
		w--
		buf[w] = 's'
		// u is now seconds
		w = fmtInt(buf[:w], secs)
		indexes = append(indexes, w)
	}
	u /= 60
	// u is now minutes
	if u > 0 {
		mins := u % 60
		if mins > 0 {
			w--
			buf[w] = 'm'
			w = fmtInt(buf[:w], mins)
			indexes = append(indexes, w)
		}
		u /= 60
		// u is now hours
		if u > 0 {
			hrs := u % 24
			if hrs > 0 {
				w--
				buf[w] = 'h'
				w = fmtInt(buf[:w], hrs)
				indexes = append(indexes, w)
			}
			u /= 24
		}
		// u is now days
		if u > 0 {
			days := u % 30
			if days > 0 {
				w--
				buf[w] = 'd'
				w = fmtInt(buf[:w], days)
				indexes = append(indexes, w)
			}
			u /= 30
		}
		// u is now months
		if u > 0 {
			months := u % 12
			if months > 0 {
				w--
				buf[w] = 'o'
				w--
				buf[w] = 'm'
				w = fmtInt(buf[:w], months)
				indexes = append(indexes, w)
			}
			u /= 12
		}
		// u is now years
		if u > 0 {
			w--
			buf[w] = 'y'
			w = fmtInt(buf[:w], u)
			indexes = append(indexes, w)
		}
	}
	start := w
	end := len(buf)

	// truncate to the first two periods
	num_periods := len(indexes)
	if num_periods > 2 {
		end = indexes[num_periods-3]
	}
	if start == end { //edge case when time difference is less than a second
		return "0s " + messageSuffix
	} else {
		return string(buf[start:end]) + " " + messageSuffix
	}

}

// getLocalNodeID returns the node ID of the local Nomad Client and an error if
// it couldn't be determined or the Agent is not running in Client mode.
func getLocalNodeID(client *api.Client) (string, error) {
	info, err := client.Agent().Self()
	if err != nil {
		return "", fmt.Errorf("Error querying agent info: %s", err)
	}
	clientStats, ok := info.Stats["client"]
	if !ok {
		return "", fmt.Errorf("Nomad not running in client mode")
	}

	nodeID, ok := clientStats["node_id"]
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

	timeLimit time.Duration
	lastRead  time.Time

	buffer     *bytes.Buffer
	bufFiled   bool
	foundLines bool
}

// NewLineLimitReader takes the ReadCloser to wrap, the number of lines to find
// searching backwards in the first searchLimit bytes. timeLimit can optionally
// be specified by passing a non-zero duration. When set, the search for the
// last n lines is aborted if no data has been read in the duration. This
// can be used to flush what is had if no extra data is being received. When
// used, the underlying reader must not block forever and must periodically
// unblock even when no data has been read.
func NewLineLimitReader(r io.ReadCloser, lines, searchLimit int, timeLimit time.Duration) *LineLimitReader {
	return &LineLimitReader{
		ReadCloser:  r,
		searchLimit: searchLimit,
		timeLimit:   timeLimit,
		lines:       lines,
		buffer:      bytes.NewBuffer(make([]byte, 0, searchLimit)),
	}
}

func (l *LineLimitReader) Read(p []byte) (n int, err error) {
	// Fill up the buffer so we can find the correct number of lines.
	if !l.bufFiled {
		b := make([]byte, len(p))
		n, err := l.ReadCloser.Read(b)
		if n > 0 {
			if _, err := l.buffer.Write(b[:n]); err != nil {
				return 0, err
			}
		}

		if err != nil {
			if err != io.EOF {
				return 0, err
			}

			l.bufFiled = true
			goto READ
		}

		if l.buffer.Len() >= l.searchLimit {
			l.bufFiled = true
			goto READ
		}

		if l.timeLimit.Nanoseconds() > 0 {
			if l.lastRead.IsZero() {
				l.lastRead = time.Now()
				return 0, nil
			}

			now := time.Now()
			if n == 0 {
				// We hit the limit
				if l.lastRead.Add(l.timeLimit).Before(now) {
					l.bufFiled = true
					goto READ
				} else {
					return 0, nil
				}
			} else {
				l.lastRead = now
			}
		}

		return 0, nil
	}

READ:
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

// JobGetter provides helpers for retrieving and parsing a jobpsec.
type JobGetter struct {
	HCL1     bool
	Vars     flaghelper.StringFlag
	VarFiles flaghelper.StringFlag
	Strict   bool
	JSON     bool

	// The fields below can be overwritten for tests
	testStdin io.Reader
}

func (j *JobGetter) Validate() error {
	if j.HCL1 && j.Strict {
		return fmt.Errorf("cannot parse job file as HCLv1 and HCLv2 strict.")
	}
	if j.HCL1 && j.JSON {
		return fmt.Errorf("cannot parse job file as HCL and JSON.")
	}
	if len(j.Vars) > 0 && j.JSON {
		return fmt.Errorf("cannot use variables with JSON files.")
	}
	if len(j.VarFiles) > 0 && j.JSON {
		return fmt.Errorf("cannot use variables with JSON files.")
	}
	if len(j.Vars) > 0 && j.HCL1 {
		return fmt.Errorf("cannot use variables with HCLv1.")
	}
	if len(j.VarFiles) > 0 && j.HCL1 {
		return fmt.Errorf("cannot use variables with HCLv1.")
	}
	return nil
}

// ApiJob returns the Job struct from jobfile.
func (j *JobGetter) ApiJob(jpath string) (*api.JobSubmission, *api.Job, error) {
	return j.Get(jpath)
}

func (j *JobGetter) Get(jpath string) (*api.JobSubmission, *api.Job, error) {
	var jobfile io.Reader
	pathName := filepath.Base(jpath)
	switch jpath {
	case "-":
		if j.testStdin != nil {
			jobfile = j.testStdin
		} else {
			jobfile = os.Stdin
		}
		pathName = "stdin"
	default:
		if len(jpath) == 0 {
			return nil, nil, fmt.Errorf("Error jobfile path has to be specified.")
		}

		jobFile, err := os.CreateTemp("", "jobfile")
		if err != nil {
			return nil, nil, err
		}
		defer os.Remove(jobFile.Name())

		if err := jobFile.Close(); err != nil {
			return nil, nil, err
		}

		// Get the pwd
		pwd, err := os.Getwd()
		if err != nil {
			return nil, nil, err
		}

		client := &gg.Client{
			Src: jpath,
			Pwd: pwd,
			Dst: jobFile.Name(),

			// This will prevent copying or writing files through symlinks
			DisableSymlinks: true,
		}

		if err := client.Get(); err != nil {
			return nil, nil, fmt.Errorf("Error getting jobfile from %q: %v", jpath, err)
		} else {
			file, err := os.Open(jobFile.Name())
			if err != nil {
				return nil, nil, fmt.Errorf("Error opening file %q: %v", jpath, err)
			}
			defer file.Close()
			jobfile = file
		}
	}

	// Parse the JobFile
	var jobStruct *api.Job               // deserialized destination
	var source bytes.Buffer              // tee the original
	var jobSubmission *api.JobSubmission // store the original and format
	jobfile = io.TeeReader(jobfile, &source)
	var err error
	switch {
	case j.HCL1:
		jobStruct, err = jobspec.Parse(jobfile)

		// include the hcl1 source as the submission
		jobSubmission = &api.JobSubmission{
			Source: source.String(),
			Format: formatHCL1,
		}
	case j.JSON:

		// Support JSON files with both a top-level Job key as well as
		// ones without.
		eitherJob := struct {
			NestedJob *api.Job `json:"Job"`
			api.Job
		}{}

		if err := json.NewDecoder(jobfile).Decode(&eitherJob); err != nil {
			return nil, nil, fmt.Errorf("Failed to parse JSON job: %w", err)
		}

		if eitherJob.NestedJob != nil {
			jobStruct = eitherJob.NestedJob
		} else {
			jobStruct = &eitherJob.Job
		}

		// include the json source as the submission
		jobSubmission = &api.JobSubmission{
			Source: source.String(),
			Format: formatJSON,
		}
	default:
		// we are parsing HCL2

		// make a copy of the job file (or stdio)
		if _, err = io.Copy(&source, jobfile); err != nil {
			return nil, nil, fmt.Errorf("Failed to parse HCL job: %w", err)
		}

		// we are parsing HCL2, whether from a file or stdio
		jobStruct, err = jobspec2.ParseWithConfig(&jobspec2.ParseConfig{
			Path:     pathName,
			Body:     source.Bytes(),
			ArgVars:  j.Vars,
			AllowFS:  true,
			VarFiles: j.VarFiles,
			Envs:     os.Environ(),
			Strict:   j.Strict,
		})

		var varFileCat string
		var readVarFileErr error
		if err == nil {
			// combine any -var-file data into one big blob
			varFileCat, readVarFileErr = extractVarFiles([]string(j.VarFiles))
			if readVarFileErr != nil {
				return nil, nil, fmt.Errorf("Failed to read var file(s): %w", readVarFileErr)
			}
		}

		// submit the job with the submission with content from -var flags
		jobSubmission = &api.JobSubmission{
			VariableFlags: extractVarFlags(j.Vars),
			Variables:     varFileCat,
			Source:        source.String(),
			Format:        formatHCL2,
		}
		if err != nil {
			if _, merr := jobspec.Parse(&source); merr == nil {
				return nil, nil, fmt.Errorf("Failed to parse using HCL 2. Use the HCL 1 parser with `nomad run -hcl1`, or address the following issues:\n%v", err)
			}
		}
	}

	if err != nil {
		return nil, nil, fmt.Errorf("Error parsing job file from %s:\n%v", jpath, err)
	}

	return jobSubmission, jobStruct, nil
}

// extractVarFiles concatenates the content of each file in filenames and
// returns it all as one big content blob
func extractVarFiles(filenames []string) (string, error) {
	var sb strings.Builder
	for _, filename := range filenames {
		b, err := os.ReadFile(filename)
		if err != nil {
			return "", err
		}
		sb.WriteString(string(b))
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// extractVarFlags is used to parse the values of -var command line arguments
// and turn them into a map to be used for submission. The result is never
// nil for convenience.
func extractVarFlags(slice []string) map[string]string {
	m := make(map[string]string, len(slice))
	for _, s := range slice {
		if tokens := strings.SplitN(s, "=", 2); len(tokens) == 1 {
			m[tokens[0]] = ""
		} else {
			m[tokens[0]] = tokens[1]
		}
	}
	return m
}

// mergeAutocompleteFlags is used to join multiple flag completion sets.
func mergeAutocompleteFlags(flags ...complete.Flags) complete.Flags {
	merged := make(map[string]complete.Predictor, len(flags))
	for _, f := range flags {
		for k, v := range f {
			merged[k] = v
		}
	}
	return merged
}

// sanitizeUUIDPrefix is used to sanitize a UUID prefix. The returned result
// will be a truncated version of the prefix if the prefix would not be
// queryable.
func sanitizeUUIDPrefix(prefix string) string {
	hyphens := strings.Count(prefix, "-")
	length := len(prefix) - hyphens
	remainder := length % 2
	return prefix[:len(prefix)-remainder]
}

// commandErrorText is used to easily render the same messaging across commands
// when an error is printed.
func commandErrorText(cmd NamedCommand) string {
	return fmt.Sprintf("For additional help try 'nomad %s -help'", cmd.Name())
}

// uiErrorWriter is a io.Writer that wraps underlying ui.ErrorWriter().
// ui.ErrorWriter expects full lines as inputs and it emits its own line breaks.
//
// uiErrorWriter scans input for individual lines to pass to ui.ErrorWriter. If data
// doesn't contain a new line, it buffers result until next new line or writer is closed.
type uiErrorWriter struct {
	ui  cli.Ui
	buf bytes.Buffer
}

func (w *uiErrorWriter) Write(data []byte) (int, error) {
	read := 0
	for len(data) != 0 {
		a, token, err := bufio.ScanLines(data, false)
		if err != nil {
			return read, err
		}

		if a == 0 {
			r, err := w.buf.Write(data)
			return read + r, err
		}

		w.ui.Error(w.buf.String() + string(token))
		data = data[a:]
		w.buf.Reset()
		read += a
	}

	return read, nil
}

func (w *uiErrorWriter) Close() error {
	// emit what's remaining
	if w.buf.Len() != 0 {
		w.ui.Error(w.buf.String())
		w.buf.Reset()
	}
	return nil
}

func loadDataSource(data string, testStdin io.Reader) (string, error) {
	// Handle empty quoted shell parameters
	if len(data) == 0 {
		return "", nil
	}

	switch data[0] {
	case '@':
		return loadFromFile(data[1:])
	case '-':
		if len(data) > 1 {
			return data, nil
		}
		return loadFromStdin(testStdin)
	default:
		return data, nil
	}
}

func loadFromFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("Failed to read file: %v", err)
	}
	return string(data), nil
}

func loadFromStdin(testStdin io.Reader) (string, error) {
	var stdin io.Reader = os.Stdin
	if testStdin != nil {
		stdin = testStdin
	}

	var b bytes.Buffer
	if _, err := io.Copy(&b, stdin); err != nil {
		return "", fmt.Errorf("Failed to read stdin: %v", err)
	}
	return b.String(), nil
}
