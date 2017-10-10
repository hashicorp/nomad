package command

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

const (
	// DefaultQuotaInitName is the default name we use when initializing the
	// example quota file
	DefaultQuotaInitName = "spec.json"
)

// QuotaInitCommand generates a new quota spec that you can customize to your
// liking, like vagrant init
type QuotaInitCommand struct {
	Meta
}

func (c *QuotaInitCommand) Help() string {
	helpText := `
Usage: nomad quota init

  Creates an example quota specification file that can be used as a starting
  point to customize further.
`
	return strings.TrimSpace(helpText)
}

func (c *QuotaInitCommand) Synopsis() string {
	return "Create an example quota specification file"
}

func (c *QuotaInitCommand) Run(args []string) int {
	// Check for misuse
	if len(args) != 0 {
		c.Ui.Error(c.Help())
		return 1
	}

	// Check if the file already exists
	_, err := os.Stat(DefaultQuotaInitName)
	if err != nil && !os.IsNotExist(err) {
		c.Ui.Error(fmt.Sprintf("Failed to stat %q: %v", DefaultQuotaInitName, err))
		return 1
	}
	if !os.IsNotExist(err) {
		c.Ui.Error(fmt.Sprintf("Quota specification %q already exists", DefaultQuotaInitName))
		return 1
	}

	// Write out the example
	err = ioutil.WriteFile(DefaultQuotaInitName, []byte(defaultQuotaSpec), 0660)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to write %q: %v", DefaultQuotaInitName, err))
		return 1
	}

	// Success
	c.Ui.Output(fmt.Sprintf("Example quota specification written to %s", DefaultQuotaInitName))
	return 0
}

var defaultQuotaSpec = strings.TrimSpace(`
{
	"Name": "default-quota",
	"Description": "Limit the shared default namespace",
	"Limits": [
		{
			"Region": "global",
			"RegionLimit": {
				"CPU": 2500,
				"MemoryMB": 1000
			}
		}
	]
}
`)
