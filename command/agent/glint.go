package agent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/mitchellh/go-glint"
	"github.com/olekukonko/tablewriter"
)

type glintUI struct {
	d *glint.Document
}

func GlintUI(ctx context.Context) UI {
	result := &glintUI{
		d: glint.New(),
	}

	go result.d.Render(ctx)

	return result
}

func (ui *glintUI) Ask(s string) (string, error) {
	panic("not implemented")
}

func (ui *glintUI) AskSecret(s string) (string, error) {
	panic("oops")
}
func (ui *glintUI) Error(s string) {}

func (ui *glintUI) Close() error {
	return ui.d.Close()
}

func (ui *glintUI) Info(s string) {}
func (ui *glintUI) Warn(s string) {}

func (ui *glintUI) Input(input *Input) (string, error) {
	return "", ErrNonInteractive
}

// Interactive implements UI
func (ui *glintUI) Interactive() bool {
	// TODO(mitchellh): We can make this interactive later but Glint itself
	// doesn't support input yet. We can pause the document, do some input,
	// then resume potentially.
	return false
}

// Output implements UI
func (ui *glintUI) Output(msg string, raw ...interface{}) {
	msg, style, _ := Interpret(msg, raw...)

	var cs []glint.StyleOption
	switch style {
	case HeaderStyle:
		cs = append(cs, glint.Bold())
		msg = "\n» " + msg
	case ErrorStyle, ErrorBoldStyle:
		cs = append(cs, glint.Color("lightRed"))
		if style == ErrorBoldStyle {
			cs = append(cs, glint.Bold())
		}

		lines := strings.Split(msg, "\n")
		if len(lines) > 0 {
			ui.d.Append(glint.Finalize(
				glint.Style(
					glint.Text("! "+lines[0]),
					cs...,
				),
			))

			for _, line := range lines[1:] {
				ui.d.Append(glint.Finalize(
					glint.Text("  " + line),
				))
			}
		}

		return

	case WarningStyle, WarningBoldStyle:
		cs = append(cs, glint.Color("lightYellow"))
		if style == WarningBoldStyle {
			cs = append(cs, glint.Bold())
		}

	case SuccessStyle, SuccessBoldStyle:
		cs = append(cs, glint.Color("lightGreen"))
		if style == SuccessBoldStyle {
			cs = append(cs, glint.Bold())
		}

		msg = colorSuccess.Sprint(msg)

	case InfoStyle:
		lines := strings.Split(msg, "\n")
		for i, line := range lines {
			lines[i] = colorInfo.Sprintf("  %s", line)
		}

		msg = strings.Join(lines, "\n")
	}

	ui.d.Append(glint.Finalize(
		glint.Style(
			glint.Text(msg),
			cs...,
		),
	))
}

// NamedValues implements UI
func (ui *glintUI) NamedValues(rows []NamedValue, opts ...Option) {
	cfg := &uiconfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	var buf bytes.Buffer
	tr := tabwriter.NewWriter(&buf, 1, 8, 0, ' ', tabwriter.AlignRight)
	for _, row := range rows {
		switch v := row.Value.(type) {
		case int, uint, int8, uint8, int16, uint16, int32, uint32, int64, uint64:
			fmt.Fprintf(tr, "  %s: \t%d\n", row.Name, row.Value)
		case float32, float64:
			fmt.Fprintf(tr, "  %s: \t%f\n", row.Name, row.Value)
		case bool:
			fmt.Fprintf(tr, "  %s: \t%v\n", row.Name, row.Value)
		case string:
			if v == "" {
				continue
			}
			fmt.Fprintf(tr, "  %s: \t%s\n", row.Name, row.Value)
		default:
			fmt.Fprintf(tr, "  %s: \t%s\n", row.Name, row.Value)
		}
	}
	tr.Flush()

	// We want to trim the trailing newline
	text := buf.String()
	if len(text) > 0 && text[len(text)-1] == '\n' {
		text = text[:len(text)-1]
	}

	ui.d.Append(glint.Finalize(glint.Text(text)))
}

// OutputWriters implements UI
func (ui *glintUI) OutputWriters() (io.Writer, io.Writer, error) {
	return os.Stdout, os.Stderr, nil
}

// Status implements UI
func (ui *glintUI) Status() Status {
	st := newGlintStatus()
	ui.d.Append(st)
	return st
}

func (ui *glintUI) StepGroup() StepGroup {
	ctx, cancel := context.WithCancel(context.Background())
	sg := &glintStepGroup{ctx: ctx, cancel: cancel}
	ui.d.Append(sg)
	return sg
}

// Table implements UI
func (ui *glintUI) Table(tbl *Table, opts ...Option) {
	var buf bytes.Buffer
	table := tablewriter.NewWriter(&buf)
	table.SetHeader(tbl.Headers)
	table.SetBorder(false)
	table.SetAutoWrapText(false)

	for _, row := range tbl.Rows {
		colors := make([]tablewriter.Colors, len(row))
		entries := make([]string, len(row))

		for i, ent := range row {
			entries[i] = ent.Value

			color, ok := colorMapping[ent.Color]
			if ok {
				colors[i] = tablewriter.Colors{color}
			}
		}

		table.Rich(entries, colors)
	}

	table.Render()

	ui.d.Append(glint.Finalize(glint.Text(buf.String())))
}

const (
	Yellow = "yellow"
	Green  = "green"
	Red    = "red"
)

var colorMapping = map[string]int{
	Green:  tablewriter.FgGreenColor,
	Yellow: tablewriter.FgYellowColor,
	Red:    tablewriter.FgRedColor,
}
