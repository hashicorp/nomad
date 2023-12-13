// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"strings"

	"github.com/muesli/reflow/wordwrap"
	"github.com/posener/complete"
)

const (
	// DefaultHclVarInitName is the default name we use when initializing the
	// example var file in HCL format
	DefaultHclVarInitName = "spec.nv.hcl"

	// DefaultHclVarInitName is the default name we use when initializing the
	// example var file in JSON format
	DefaultJsonVarInitName = "spec.nv.json"
)

// VarInitCommand generates a new variable specification
type VarInitCommand struct {
	Meta
}

func (c *VarInitCommand) Help() string {
	helpText := `
Usage: nomad var init <filename>

  Creates an example variable specification file that can be used as a starting
  point to customize further. When no filename is supplied, a default filename
  of "spec.nv.hcl" or "spec.nv.json" will be used depending on the output
  format.

Init Options:

  -out (hcl | json)
    Format of generated variable specification. Defaults to "hcl".

  -quiet
    Do not print success message.
`
	return strings.TrimSpace(helpText)
}

func (c *VarInitCommand) Synopsis() string {
	return "Create an example variable specification file"
}

func (c *VarInitCommand) AutocompleteFlags() complete.Flags {
	return complete.Flags{
		"-out":   complete.PredictSet("hcl", "json"),
		"-quiet": complete.PredictNothing,
	}
}

func (c *VarInitCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *VarInitCommand) Name() string { return "var init" }

func (c *VarInitCommand) Run(args []string) int {
	var outFmt string
	var quiet bool

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&outFmt, "out", "hcl", "")
	flags.BoolVar(&quiet, "quiet", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we get no arguments
	args = flags.Args()
	if l := len(args); l > 1 {
		c.Ui.Error("This command takes no arguments or one: <filename>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}
	var fileName, fileContent string
	switch outFmt {
	case "hcl":
		fileName = DefaultHclVarInitName
		fileContent = defaultHclVarSpec
	case "json":
		fileName = DefaultJsonVarInitName
		fileContent = defaultJsonVarSpec
	}

	if len(args) == 1 {
		fileName = args[0]
	}

	// Check if the file already exists
	_, err := os.Stat(fileName)
	if err == nil {
		c.Ui.Error(fmt.Sprintf("File %q already exists", fileName))
		return 1
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		c.Ui.Error(fmt.Sprintf("Failed to stat %q: %v", fileName, err))
		return 1
	}

	// Write out the example
	err = os.WriteFile(fileName, []byte(fileContent), 0660)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to write %q: %v", fileName, err))
		return 1
	}

	// Success
	if !quiet {
		if outFmt == "json" {
			c.Ui.Info(wrapString(tidyRawString(strings.ReplaceAll(msgOnlyItemsRequired, "items", "Items")), 70))
			c.Ui.Warn(wrapString(tidyRawString(strings.ReplaceAll(msgWarnKeys, "items", "Items")), 70))
		}
		c.Ui.Output(fmt.Sprintf("Example variable specification written to %s", fileName))
	}
	return 0
}

const (
	msgWarnKeys = `
	REMINDER: While keys in the items map can contain dots, using them in
	templates is easier when they do not. As a best practice, avoid dotted
	keys when possible.`
	msgOnlyItemsRequired = `
	The items map is the only strictly required part of a variable
	specification, since path and namespace can be set via other means. It
	contains the sensitive material to encrypt and store as a Nomad variable.
	The entire items map is encrypted and decrypted as a single unit.`
)

var defaultHclVarSpec = strings.TrimSpace(`
# A variable path can be specified in the specification file
# and will be used when writing the variable without specifying a
# path in the command or when writing JSON directly to the `+"`/var/`"+`
# HTTP API endpoint
# path = "path/to/variable"

# The Namespace to write the variable can be included in the specification. This
# value can be overridden by specifying the "-namespace" flag on the "put"
# command.
# namespace = "default"

`+makeHCLComment(msgOnlyItemsRequired)+`

`+makeHCLComment(msgWarnKeys)+`
items {
  key1 = "value 1"
  key2 = "value 2"
}
`) + "\n"

var defaultJsonVarSpec = strings.TrimSpace(`
{
  "Namespace": "default",
  "Path": "path/to/variable",
  "Items": {
    "key1": "value 1",
    "key2": "value 2"
  }
}
`) + "\n"

// makeHCLComment is a helper function that will take the contents of a raw
// string, tidy them, wrap them to 68 characters and add a leading comment
// marker plus a space.
func makeHCLComment(in string) string {
	return wrapAndPrepend(tidyRawString(in), 70, "# ")
}

// wrapString is a convenience func to abstract away the word wrapping
// implementation
func wrapString(input string, lineLen int) string {
	return wordwrap.String(input, lineLen)
}

// wrapAndPrepend will word wrap the input string to lineLen characters and
// prepend the provided prefix to every line. The total length of each returned
// line will be at most len(input[line])+len(prefix)
func wrapAndPrepend(input string, lineLen int, prefix string) string {
	ss := strings.Split(wrapString(input, lineLen-len(prefix)), "\n")
	prefixStringList(ss, prefix)
	return strings.Join(ss, "\n")
}

// tidyRawString will convert a wrapped and indented raw string into a single
// long string suitable for rewrapping with another tool. It trims leading and
// trailing whitespace and then consume groups of tabs, newlines, and spaces
// replacing them with a single space
func tidyRawString(raw string) string {
	re := regexp.MustCompile("[\t\n ]+")
	return re.ReplaceAllString(strings.TrimSpace(raw), " ")
}

// prefixStringList is a helper function that prepends each item in a slice of
// string with a provided prefix.
func prefixStringList(ss []string, prefix string) []string {
	for i, s := range ss {
		ss[i] = prefix + s
	}
	return ss
}
