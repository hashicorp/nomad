package agent

import (
	"errors"
	"fmt"
	"io"

	"github.com/fatih/color"
)

// ErrNonInteractive is returned when Input is called on a non-Interactive UI.
var ErrNonInteractive = errors.New("noninteractive UI doesn't support this operation")

type UI interface {
	// Input asks the user for input. This will immediately return an error
	// if the UI doesn't support interaction. You can test for interaction
	// ahead of time with Interactive().
	Input(*Input) (string, error)

	// Interactive returns true if this prompt supports user interaction.
	// If this is false, Input will always error.
	Interactive() bool

	// Output outputs a message directly to the terminal. The remaining
	// arguments should be interpolations for the format string. After the
	// interpolations you may add Options.
	Output(string, ...interface{})

	// Output data as a table of data. Each entry is a row which will be output
	// with the columns lined up nicely.
	NamedValues([]NamedValue, ...Option)

	// OutputWriters returns stdout and stderr writers. These are usually
	// but not always TTYs. This is useful for subprocesses, network requests,
	// etc. Note that writing to these is not thread-safe by default so
	// you must take care that there is only ever one writer.
	OutputWriters() (stdout, stderr io.Writer, err error)

	// Status returns a live-updating status that can be used for single-line
	// status updates that typically have a spinner or some similar style.
	// While a Status is live (Close isn't called), other methods on UI should
	// NOT be called.
	Status() Status

	// Table outputs the information formatted into a Table structure.
	Table(*Table, ...Option)

	// StepGroup returns a value that can be used to output individual (possibly
	// parallel) steps that have their own message, status indicator, spinner, and
	// body. No other output mechanism (Output, Input, Status, etc.) may be
	// called until the StepGroup is complete.
	// StepGroup() StepGroup
}

// Interpret decomposes the msg and arguments into the message, style, and writer
func Interpret(msg string, raw ...interface{}) (string, string, io.Writer) {
	// Build our args and options
	var args []interface{}
	var opts []Option
	for _, r := range raw {
		if opt, ok := r.(Option); ok {
			opts = append(opts, opt)
		} else {
			args = append(args, r)
		}
	}

	// Build our message
	msg = fmt.Sprintf(msg, args...)

	// Build our config and set our options
	cfg := &config{Writer: color.Output}
	for _, opt := range opts {
		opt(cfg)
	}

	return msg, cfg.Style, cfg.Writer
}

const (
	HeaderStyle      = "header"
	ErrorStyle       = "error"
	ErrorBoldStyle   = "error-bold"
	WarningStyle     = "warning"
	WarningBoldStyle = "warning-bold"
	InfoStyle        = "info"
	SuccessStyle     = "success"
	SuccessBoldStyle = "success-bold"
)

type config struct {
	// Writer is where the message will be written to.
	Writer io.Writer

	// The style the output should take on
	Style string
}

// Option controls output styling.
type Option func(*config)

// WithHeaderStyle styles the output like a header denoting a new section
// of execution. This should only be used with single-line output. Multi-line
// output will not look correct.
func WithHeaderStyle() Option {
	return func(c *config) {
		c.Style = HeaderStyle
	}
}

// WithInfoStyle styles the output like it's formatted information.
func WithInfoStyle() Option {
	return func(c *config) {
		c.Style = InfoStyle
	}
}

// WithErrorStyle styles the output as an error message.
func WithErrorStyle() Option {
	return func(c *config) {
		c.Style = ErrorStyle
	}
}

// WithWarningStyle styles the output as an error message.
func WithWarningStyle() Option {
	return func(c *config) {
		c.Style = WarningStyle
	}
}

// WithSuccessStyle styles the output as a success message.
func WithSuccessStyle() Option {
	return func(c *config) {
		c.Style = SuccessStyle
	}
}

func WithStyle(style string) Option {
	return func(c *config) {
		c.Style = style
	}
}

// WithWriter specifies the writer for the output.
func WithWriter(w io.Writer) Option {
	return func(c *config) { c.Writer = w }
}

// Input is the configuration for an input.
type Input struct {
	// Prompt is a single-line prompt to give the user such as "Continue?"
	// The user will input their answer after this prompt.
	Prompt string

	// Style is the style to apply to the input. If this is blank,
	// the output won't be colorized in any way.
	Style string

	// True if this input is a secret. The input will be masked.
	Secret bool
}

// Passed to UI.NamedValues to provide a nicely formatted key: value output
type NamedValue struct {
	Name  string
	Value interface{}
}

// Passed to UI.Table to provide a nicely formatted table.
type Table struct {
	Headers []string
	Rows    [][]TableEntry
}

// TableEntry is a single entry for a table.
type TableEntry struct {
	Value string
	Color string
}

var (
	colorHeader      = color.New(color.Bold)
	colorInfo        = color.New()
	colorError       = color.New(color.FgRed)
	colorErrorBold   = color.New(color.FgRed, color.Bold)
	colorSuccess     = color.New(color.FgGreen)
	colorSuccessBold = color.New(color.FgGreen, color.Bold)
	colorWarning     = color.New(color.FgYellow)
	colorWarningBold = color.New(color.FgYellow, color.Bold)
)
