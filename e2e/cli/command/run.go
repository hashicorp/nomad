package command

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/mitchellh/cli"
)

func RunCommandFactory() (cli.Command, error) {
	return &Run{}, nil
}

type Run struct {
}

func (c *Run) Help() string {
	helpText := `
Usage: nomad-e2e run
`
	return strings.TrimSpace(helpText)
}

func (c *Run) Synopsis() string {
	return "Runs the e2e test suite"
}

func (c *Run) Run(args []string) int {
	if err := c.run(); err != nil {
		fmt.Println(err)
		return 1
	}
	return 0
}

func (c *Run) run() error {
	goBin, err := exec.LookPath("go")
	if err != nil {
		return err
	}

	goArgs := []string{
		"test",
		"-json",
		"github.com/hashicorp/nomad/e2e",
	}

	cmd := exec.Command(goBin, goArgs...)
	out, err := cmd.StdoutPipe()
	defer out.Close()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	dec := NewDecoder(out)
	report, err := dec.Decode()
	if err != nil {
		return err
	}

	fmt.Println(report.Summary())

	return nil
}
