package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/bgentry/speakeasy"
	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/mitchellh/cli"
)

// LogUI is an implementation of the cli.Ui interface which can be used for
// logging outputs. It differs from cli.BasicUi in the only fact that it does
// not add a newline after each UI write.
type LogUI struct {
	reader      io.Reader
	writer      io.Writer
	errorWriter io.Writer

	isColor     bool
	outputColor cli.UiColor
	infoColor   cli.UiColor
	errorColor  cli.UiColor
	warnColor   cli.UiColor
}

// NewLogUI generates a new cli.Ui that can be used for commands that write log
// lines to the terminal. The caller is required to pass a cli.BasicUi so we
// have access to the underlying writers.
//
// Currently, the passed ui needs to be either *cli.ColoredUi or *cli.BasicUi
// to work correctly. If more are needed, please add them.
func NewLogUI(ui cli.Ui) (cli.Ui, error) {

	var found bool

	logUI := LogUI{}

	if coloredUI, ok := ui.(*cli.ColoredUi); ok {

		logUI.isColor = true
		logUI.outputColor = coloredUI.OutputColor
		logUI.infoColor = coloredUI.InfoColor
		logUI.errorColor = coloredUI.ErrorColor
		logUI.warnColor = coloredUI.WarnColor

		if basicUI, ok := coloredUI.Ui.(*cli.BasicUi); ok {
			logUI.reader = basicUI.Reader
			logUI.writer = basicUI.Writer
			logUI.errorWriter = basicUI.ErrorWriter
			found = true
		}
	} else if basicUI, ok := ui.(*cli.BasicUi); ok && !found {
		logUI.reader = basicUI.Reader
		logUI.writer = basicUI.Writer
		logUI.errorWriter = basicUI.ErrorWriter
		found = true
	}

	if !found {
		return nil, errors.New("failed to generate logging UI")
	}

	return &logUI, nil
}

// Ask implements the Ask function of the cli.Ui interface.
func (l *LogUI) Ask(query string) (string, error) {
	return l.ask(l.colorize(query, l.outputColor), false)
}

// AskSecret implements the AskSecret function of the cli.Ui interface.
func (l *LogUI) AskSecret(query string) (string, error) {
	return l.ask(l.colorize(query, l.outputColor), true)
}

func (l *LogUI) ask(query string, secret bool) (string, error) {
	if _, err := fmt.Fprint(l.writer, query+" "); err != nil {
		return "", err
	}

	// Register for interrupts so that we can catch it and immediately
	// return...
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	// Ask for input in a go-routine so that we can ignore it.
	errCh := make(chan error, 1)
	lineCh := make(chan string, 1)
	go func() {
		var line string
		var err error
		if secret && isatty.IsTerminal(os.Stdin.Fd()) {
			line, err = speakeasy.Ask("")
		} else {
			r := bufio.NewReader(l.reader)
			line, err = r.ReadString('\n')
		}
		if err != nil {
			errCh <- err
			return
		}

		lineCh <- strings.TrimRight(line, "\r\n")
	}()

	select {
	case err := <-errCh:
		return "", err
	case line := <-lineCh:
		return line, nil
	case <-sigCh:
		// Print a newline so that any further output starts properly
		// on a new line.
		_, _ = fmt.Fprintln(l.writer)
		return "", errors.New("interrupted")
	}
}

// Output implements the Output function of the cli.Ui interface.
func (l *LogUI) Output(message string) {
	_, _ = fmt.Fprint(l.writer, l.colorize(message, l.outputColor))
}

// Info implements the Info function of the cli.Ui interface.
func (l *LogUI) Info(message string) { l.Output(l.colorize(message, l.infoColor)) }

// Error implements the Error function of the cli.Ui interface.
func (l *LogUI) Error(message string) {
	w := l.writer
	if l.errorWriter != nil {
		w = l.errorWriter
	}
	_, _ = fmt.Fprint(w, l.colorize(message, l.errorColor))
}

// Warn implements the Warn function of the cli.Ui interface.
func (l *LogUI) Warn(message string) { l.Error(l.colorize(message, l.warnColor)) }

func (l *LogUI) colorize(message string, uc cli.UiColor) string {
	if !l.isColor {
		return message
	}

	attr := []color.Attribute{color.Attribute(uc.Code)}
	if uc.Bold {
		attr = append(attr, color.Bold)
	}

	return color.New(attr...).SprintFunc()(message)
}
