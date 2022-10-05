package command

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/posener/complete"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	stdinArg  = "-"
	stdinPath = "<stdin>"
)

type FormatCommand struct {
	Meta

	diagWr hcl.DiagnosticWriter

	parser   *hclparse.Parser
	hclDiags hcl.Diagnostics

	errs *multierror.Error

	list      bool
	check     bool
	recursive bool
	write     bool
	paths     []string

	stdin io.Reader
}

func (*FormatCommand) Help() string {
	helpText := `
Usage: nomad fmt [flags] paths ...

	Formats Nomad agent configuration and job file to a canonical format.
	If a path is a directory, it will recursively format all files
	with .nomad and .hcl extensions in the directory.
	
	If you provide a single dash (-) as argument, fmt will read from standard
	input (STDIN) and output the processed output to standard output (STDOUT).

Format Options:

	-list=false
		Don't list the files, which contain formatting inconsistencies.

	-check
		Check if the files are valid HCL files. If not, exit status of the command
		will be 1 and the incorrect files will not be formatted.

	-write=false
		Don't overwrite the input files.

	-recursive
		Process also files in subdirectories. By default only the given (or current) directory is processed.
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
		"-check":     complete.PredictNothing,
		"-write":     complete.PredictNothing,
		"-list":      complete.PredictNothing,
		"-recursive": complete.PredictNothing,
	}
}

func (f *FormatCommand) Name() string { return "fmt" }

func (f *FormatCommand) Run(args []string) int {
	if f.stdin == nil {
		f.stdin = os.Stdin
	}

	flags := f.Meta.FlagSet(f.Name(), FlagSetClient)
	flags.Usage = func() { f.Ui.Output(f.Help()) }
	flags.BoolVar(&f.check, "check", false, "")
	flags.BoolVar(&f.write, "write", true, "")
	flags.BoolVar(&f.list, "list", true, "")
	flags.BoolVar(&f.recursive, "recursive", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	f.parser = hclparse.NewParser()

	color := terminal.IsTerminal(int(os.Stderr.Fd()))
	w, _, err := terminal.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		w = 80
	}

	f.diagWr = hcl.NewDiagnosticTextWriter(os.Stderr, f.parser.Files(), uint(w), color)

	if len(flags.Args()) == 0 {
		f.paths = []string{"."}
	} else if flags.Args()[0] == stdinArg {
		f.write = false
		f.list = false
	} else {
		f.paths = flags.Args()
	}

	f.fmt()

	if f.hclDiags.HasErrors() {
		f.diagWr.WriteDiagnostics(f.hclDiags)
	}

	if f.errs != nil {
		f.Ui.Error(f.errs.Error())
		f.Ui.Error(commandErrorText(f))
	}

	if f.hclDiags.HasErrors() || f.errs != nil {
		return 1
	}

	return 0
}

func (f *FormatCommand) fmt() {
	if len(f.paths) == 0 {
		f.processFile(stdinPath, f.stdin)
		return
	}

	for _, path := range f.paths {
		info, err := os.Stat(path)
		if err != nil {
			f.appendError(fmt.Errorf("No file or directory at %s", path))
			continue
		}

		if info.IsDir() {
			f.processDir(path)
		} else {
			if isNomadFile(info) {
				fp, err := os.Open(path)
				if err != nil {
					f.appendError(fmt.Errorf("Failed to open file %s: %w", path, err))
					continue
				}

				f.processFile(path, fp)

				fp.Close()
			} else {
				f.appendError(fmt.Errorf("Only .nomad and .hcl files can be processed using nomad fmt"))
				continue
			}
		}
	}
}

func (f *FormatCommand) processDir(path string) {
	entries, err := os.ReadDir(path)
	if err != nil {
		f.appendError(fmt.Errorf("Failed to list directory %s", path))
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		subpath := filepath.Join(path, name)

		if entry.IsDir() {
			if f.recursive {
				f.processDir(subpath)
			}

			continue
		}

		info, err := entry.Info()
		if err != nil {
			f.appendError(err)
			continue
		}

		if isNomadFile(info) {
			fp, err := os.Open(subpath)
			if err != nil {
				f.appendError(fmt.Errorf("Failed to open file %s: %w", path, err))
				continue
			}

			f.processFile(subpath, fp)

			fp.Close()
		}
	}
}

func (f *FormatCommand) processFile(path string, r io.Reader) {
	src, err := io.ReadAll(r)
	if err != nil {
		f.appendError(fmt.Errorf("Failed to read file %s: %w", path, err))
		return
	}

	f.parser.AddFile(path, &hcl.File{
		Body:  hcl.EmptyBody(),
		Bytes: src,
	})

	_, syntaxDiags := hclsyntax.ParseConfig(src, path, hcl.InitialPos)
	if syntaxDiags.HasErrors() {
		f.hclDiags = append(f.hclDiags, syntaxDiags...)
		return
	}
	formattedFile, diags := hclwrite.ParseConfig(src, path, hcl.InitialPos)
	if diags.HasErrors() {
		f.hclDiags = append(f.hclDiags, diags...)
		return
	}

	out := formattedFile.Bytes()

	if !bytes.Equal(src, out) {
		if f.list {
			f.Ui.Output(path)
		}

		if f.write {
			if err := os.WriteFile(path, out, 0644); err != nil {
				f.appendError(fmt.Errorf("Failed to write file %s: %w", path, err))
				return
			}
		}
	}

	if !f.list && !f.write {
		f.Ui.Output(string(out))
	}
}

func isNomadFile(file fs.FileInfo) bool {
	return !file.IsDir() && (filepath.Ext(file.Name()) == ".nomad" || filepath.Ext(file.Name()) == ".hcl")
}

func (f *FormatCommand) appendError(err error) {
	f.errs = multierror.Append(f.errs, err)
}
