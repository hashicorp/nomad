package command

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/posener/complete"
	"golang.org/x/crypto/ssh/terminal"
)

type FormatCommand struct {
	Meta

	diagWr hcl.DiagnosticWriter
	parser *hclparse.Parser
	files  []string
}

type FormatArgs struct {
	Paths     []string
	Check     bool
	Overwrite bool
}

func (f *FormatCommand) RunContext(ctx context.Context, args *FormatArgs) int {
	var errs error

	f.parser = hclparse.NewParser()

	color := terminal.IsTerminal(int(os.Stderr.Fd()))
	w, _, err := terminal.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		w = 80
	}

	f.diagWr = hcl.NewDiagnosticTextWriter(os.Stderr, f.parser.Files(), uint(w), color)

	if err := f.findFiles(args); err != nil {
		f.Ui.Error("Failed to find files to format:")
		f.Ui.Error(err.Error())
		f.Ui.Error(commandErrorText(f))
		return 1
	}

	for _, file := range f.files {
		if err := f.processFile(file, args); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	if errs != nil {
		f.Ui.Error(errs.Error())
		return 1
	}

	return 0
}

func (*FormatCommand) Help() string {
	helpText := `
Usage: nomad fmt [flags] paths ...

	Formats Nomad agent configuration and job file to a canonical format.
	If a path is a directory, it will recursively format all files
	with .nomad and .hcl extensions in the directory.

Format Options:

	-check
		Check if the files are valid HCL files. If not, exit status of the command
		will be 1 and the incorrect files will not be formatted.

	-overwrite=false
		Print the formatted files to stdout, instead of overwriting the file content.
`

	return strings.TrimSpace(helpText)
}

func (*FormatCommand) Synopsis() string {
	return "Rewrites Nomad config and job files to canonical format"
}

func (*FormatCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFiles("*")
}

func (*FormatCommand) AutocompleteFlags() complete.Flags {
	return complete.Flags{
		"-check":           complete.PredictNothing,
		"-overwrite=false": complete.PredictNothing,
	}
}

func (f *FormatCommand) Name() string { return "fmt" }

func (f *FormatCommand) Run(args []string) int {
	ctx := context.Background()

	fmtArgs := &FormatArgs{}

	flags := f.Meta.FlagSet(f.Name(), FlagSetClient)
	flags.Usage = func() { f.Ui.Output(f.Help()) }
	flags.BoolVar(&fmtArgs.Check, "check", false, "")
	flags.BoolVar(&fmtArgs.Overwrite, "overwrite", true, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	fmtArgs.Paths = flags.Args()

	return f.RunContext(ctx, fmtArgs)
}

func (f *FormatCommand) findFiles(args *FormatArgs) error {
	for _, path := range args.Paths {
		fi, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("while stating %s: %w", path, err)
		}

		if !fi.IsDir() {
			f.files = append(f.files, path)
			continue
		}

		if err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if !d.IsDir() && (filepath.Ext(path) == ".nomad" || filepath.Ext(path) == ".hcl") {
				f.files = append(f.files, path)
			}

			return nil
		}); err != nil {
			return fmt.Errorf("while walking through %s: %w", path, err)
		}
	}

	return nil
}

func (f *FormatCommand) processFile(filepath string, args *FormatArgs) error {
	bytes, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("while reading %s: %w", filepath, err)
	}

	if args.Check {
		_, diags := f.parser.ParseHCL(bytes, filepath)
		f.diagWr.WriteDiagnostics(diags)
		if diags.HasErrors() {
			return fmt.Errorf("error in HCL syntax: %s", filepath)
		}
	}

	fmtBytes := hclwrite.Format(bytes)

	if args.Overwrite {
		os.WriteFile(filepath, fmtBytes, 0644)
	} else {
		f.Ui.Output(string(fmtBytes))
	}

	return nil
}
