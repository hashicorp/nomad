package command

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/hashicorp/nomad/jobspec"
)

type ValidateCommand struct {
	Meta

	// The fields below can be overwritten for tests
	testStdin io.Reader
}

func (c *ValidateCommand) Help() string {
	helpText := `
Usage: nomad validate [options] <file>

  Checks if a given HCL job file has a valid specification. This can be used to
  check for any syntax errors or validation problems with a job.

  If the supplied path is "-", the jobfile is read from stdin. Otherwise
  it is read from the file at the supplied path.

   -k
     Allow insecure SSL connections to access jobfile.

`
	return strings.TrimSpace(helpText)
}

func (c *ValidateCommand) Synopsis() string {
	return "Checks if a given job specification is valid"
}

func (c *ValidateCommand) Run(args []string) int {
	var insecure bool

	flags := c.Meta.FlagSet("validate", FlagSetNone)
	flags.BoolVar(&insecure, "k", false, "")
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one node
	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error(c.Help())
		return 1
	}

	// Read the Jobfile
	var path, url string
	// If prefix has http(s)://, read the Jobfile from the URL
	if strings.Index(args[0], "http://") == 0 || strings.Index(args[0], "https://") == 0 {
		path = "_url"
		url = args[0]
	} else {
		// Read the Jobfile
		path = args[0]
	}

	var f io.Reader
	switch path {
	case "-":
		if c.testStdin != nil {
			f = c.testStdin
		} else {
			f = os.Stdin
		}
		path = "stdin"
	case "_url":
		if len(url) == 0 {
			c.Ui.Error(fmt.Sprintf("Error invalid Jobfile name"))
		}
		var resp *http.Response
		var err error
		if insecure == true {
			tr := &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
			client := &http.Client{Transport: tr}
			resp, err = client.Get(url)
		} else {
			resp, err = http.Get(url)
		}
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error accessing URL %s: %v", url, err))
			return 1
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			c.Ui.Error(fmt.Sprintf("Error reading URL (%d) : %s", resp.StatusCode, resp.Status))
			return 1
		}
		f = resp.Body
		path = url
	default:
		file, err := os.Open(path)
		defer file.Close()
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error opening file %q: %v", path, err))
			return 1
		}
		f = file
	}

	// Parse the JobFile
	job, err := jobspec.Parse(f)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing job file from %s: %v", path, err))
		return 1
	}

	// Initialize any fields that need to be.
	job.Canonicalize()

	// Check that the job is valid
	if err := job.Validate(); err != nil {
		c.Ui.Error(fmt.Sprintf("Error validating job: %s", err))
		return 1
	}

	// Done!
	c.Ui.Output("Job validation successful")
	return 0
}
