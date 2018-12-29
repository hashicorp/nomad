package hclog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	_levelToBracket = map[Level]string{
		Debug: "[DEBUG]",
		Trace: "[TRACE]",
		Info:  "[INFO ]",
		Warn:  "[WARN ]",
		Error: "[ERROR]",
	}
)

// Given the options (nil for defaults), create a new Logger
func New(opts *LoggerOptions) Logger {
	if opts == nil {
		opts = &LoggerOptions{}
	}

	output := opts.Output
	if output == nil {
		output = os.Stderr
	}

	level := opts.Level
	if level == NoLevel {
		level = DefaultLevel
	}

	return &intLogger{
		m:      new(sync.Mutex),
		json:   opts.JSONFormat,
		caller: opts.IncludeLocation,
		name:   opts.Name,
		w:      bufio.NewWriter(output),
		level:  level,
	}
}

// The internal logger implementation. Internal in that it is defined entirely
// by this package.
type intLogger struct {
	json   bool
	caller bool
	name   string

	// this is a pointer so that it's shared by any derived loggers, since
	// those derived loggers share the bufio.Writer as well.
	m     *sync.Mutex
	w     *bufio.Writer
	level Level

	implied []interface{}
}

// Make sure that intLogger is a Logger
var _ Logger = &intLogger{}

// The time format to use for logging. This is a version of RFC3339 that
// contains millisecond precision
const TimeFormat = "2006-01-02T15:04:05.000Z0700"

// Log a message and a set of key/value pairs if the given level is at
// or more severe that the threshold configured in the Logger.
func (z *intLogger) Log(level Level, msg string, args ...interface{}) {
	if level < z.level {
		return
	}

	t := time.Now()

	z.m.Lock()
	defer z.m.Unlock()

	if z.json {
		z.logJson(t, level, msg, args...)
	} else {
		z.log(t, level, msg, args...)
	}

	z.w.Flush()
}

// Cleanup a path by returning the last 2 segments of the path only.
func trimCallerPath(path string) string {
	// lovely borrowed from zap
	// nb. To make sure we trim the path correctly on Windows too, we
	// counter-intuitively need to use '/' and *not* os.PathSeparator here,
	// because the path given originates from Go stdlib, specifically
	// runtime.Caller() which (as of Mar/17) returns forward slashes even on
	// Windows.
	//
	// See https://github.com/golang/go/issues/3335
	// and https://github.com/golang/go/issues/18151
	//
	// for discussion on the issue on Go side.
	//

	// Find the last separator.
	//
	idx := strings.LastIndexByte(path, '/')
	if idx == -1 {
		return path
	}

	// Find the penultimate separator.
	idx = strings.LastIndexByte(path[:idx], '/')
	if idx == -1 {
		return path
	}

	return path[idx+1:]
}

// Non-JSON logging format function
func (z *intLogger) log(t time.Time, level Level, msg string, args ...interface{}) {
	z.w.WriteString(t.Format(TimeFormat))
	z.w.WriteByte(' ')

	s, ok := _levelToBracket[level]
	if ok {
		z.w.WriteString(s)
	} else {
		z.w.WriteString("[UNKN ]")
	}

	if z.caller {
		if _, file, line, ok := runtime.Caller(3); ok {
			z.w.WriteByte(' ')
			z.w.WriteString(trimCallerPath(file))
			z.w.WriteByte(':')
			z.w.WriteString(strconv.Itoa(line))
			z.w.WriteByte(':')
		}
	}

	z.w.WriteByte(' ')

	if z.name != "" {
		z.w.WriteString(z.name)
		z.w.WriteString(": ")
	}

	z.w.WriteString(msg)

	args = append(z.implied, args...)

	var stacktrace CapturedStacktrace

	if args != nil && len(args) > 0 {
		if len(args)%2 != 0 {
			cs, ok := args[len(args)-1].(CapturedStacktrace)
			if ok {
				args = args[:len(args)-1]
				stacktrace = cs
			} else {
				args = append(args, "<unknown>")
			}
		}

		z.w.WriteByte(':')

	FOR:
		for i := 0; i < len(args); i = i + 2 {
			var val string

			switch st := args[i+1].(type) {
			case string:
				val = st
			case int:
				val = strconv.FormatInt(int64(st), 10)
			case int64:
				val = strconv.FormatInt(int64(st), 10)
			case int32:
				val = strconv.FormatInt(int64(st), 10)
			case int16:
				val = strconv.FormatInt(int64(st), 10)
			case int8:
				val = strconv.FormatInt(int64(st), 10)
			case uint:
				val = strconv.FormatUint(uint64(st), 10)
			case uint64:
				val = strconv.FormatUint(uint64(st), 10)
			case uint32:
				val = strconv.FormatUint(uint64(st), 10)
			case uint16:
				val = strconv.FormatUint(uint64(st), 10)
			case uint8:
				val = strconv.FormatUint(uint64(st), 10)
			case CapturedStacktrace:
				stacktrace = st
				continue FOR
			default:
				val = fmt.Sprintf("%v", st)
			}

			z.w.WriteByte(' ')
			z.w.WriteString(args[i].(string))
			z.w.WriteByte('=')

			if strings.ContainsAny(val, " \t\n\r") {
				z.w.WriteByte('"')
				z.w.WriteString(val)
				z.w.WriteByte('"')
			} else {
				z.w.WriteString(val)
			}
		}
	}

	z.w.WriteString("\n")

	if stacktrace != "" {
		z.w.WriteString(string(stacktrace))
	}
}

// JSON logging function
func (z *intLogger) logJson(t time.Time, level Level, msg string, args ...interface{}) {
	vals := map[string]interface{}{
		"@message":   msg,
		"@timestamp": t.Format("2006-01-02T15:04:05.000000Z07:00"),
	}

	var levelStr string
	switch level {
	case Error:
		levelStr = "error"
	case Warn:
		levelStr = "warn"
	case Info:
		levelStr = "info"
	case Debug:
		levelStr = "debug"
	case Trace:
		levelStr = "trace"
	default:
		levelStr = "all"
	}

	vals["@level"] = levelStr

	if z.name != "" {
		vals["@module"] = z.name
	}

	if z.caller {
		if _, file, line, ok := runtime.Caller(3); ok {
			vals["@caller"] = fmt.Sprintf("%s:%d", file, line)
		}
	}

	if args != nil && len(args) > 0 {
		if len(args)%2 != 0 {
			cs, ok := args[len(args)-1].(CapturedStacktrace)
			if ok {
				args = args[:len(args)-1]
				vals["stacktrace"] = cs
			} else {
				args = append(args, "<unknown>")
			}
		}

		for i := 0; i < len(args); i = i + 2 {
			if _, ok := args[i].(string); !ok {
				// As this is the logging function not much we can do here
				// without injecting into logs...
				continue
			}
			vals[args[i].(string)] = args[i+1]
		}
	}

	err := json.NewEncoder(z.w).Encode(vals)
	if err != nil {
		panic(err)
	}
}

// Emit the message and args at DEBUG level
func (z *intLogger) Debug(msg string, args ...interface{}) {
	z.Log(Debug, msg, args...)
}

// Emit the message and args at TRACE level
func (z *intLogger) Trace(msg string, args ...interface{}) {
	z.Log(Trace, msg, args...)
}

// Emit the message and args at INFO level
func (z *intLogger) Info(msg string, args ...interface{}) {
	z.Log(Info, msg, args...)
}

// Emit the message and args at WARN level
func (z *intLogger) Warn(msg string, args ...interface{}) {
	z.Log(Warn, msg, args...)
}

// Emit the message and args at ERROR level
func (z *intLogger) Error(msg string, args ...interface{}) {
	z.Log(Error, msg, args...)
}

// Indicate that the logger would emit TRACE level logs
func (z *intLogger) IsTrace() bool {
	return z.level == Trace
}

// Indicate that the logger would emit DEBUG level logs
func (z *intLogger) IsDebug() bool {
	return z.level <= Debug
}

// Indicate that the logger would emit INFO level logs
func (z *intLogger) IsInfo() bool {
	return z.level <= Info
}

// Indicate that the logger would emit WARN level logs
func (z *intLogger) IsWarn() bool {
	return z.level <= Warn
}

// Indicate that the logger would emit ERROR level logs
func (z *intLogger) IsError() bool {
	return z.level <= Error
}

// Return a sub-Logger for which every emitted log message will contain
// the given key/value pairs. This is used to create a context specific
// Logger.
func (z *intLogger) With(args ...interface{}) Logger {
	var nz intLogger = *z

	nz.implied = append(nz.implied, args...)

	return &nz
}

// Create a new sub-Logger that a name decending from the current name.
// This is used to create a subsystem specific Logger.
func (z *intLogger) Named(name string) Logger {
	var nz intLogger = *z

	if nz.name != "" {
		nz.name = nz.name + "." + name
	}

	return &nz
}

// Create a new sub-Logger with an explicit name. This ignores the current
// name. This is used to create a standalone logger that doesn't fall
// within the normal hierarchy.
func (z *intLogger) ResetNamed(name string) Logger {
	var nz intLogger = *z

	nz.name = name

	return &nz
}

// Create a *log.Logger that will send it's data through this Logger. This
// allows packages that expect to be using the standard library log to actually
// use this logger.
func (z *intLogger) StandardLogger(opts *StandardLoggerOptions) *log.Logger {
	if opts == nil {
		opts = &StandardLoggerOptions{}
	}

	return log.New(&stdlogAdapter{z, opts.InferLevels}, "", 0)
}
