package command

import (
  "context"

  "github.com/mitchellh/go-glint"

  cc "github.com/hashicorp/nomad/cli/components"
)

type DevCommand struct {
  UI *glint.Document
  Meta
}

func (c *DevCommand) Help() string { return ":)" }

func (c *DevCommand) Name() string { return "dev" }

func (c *DevCommand) Synopsis() string { return ":D" }

func (c *DevCommand) Run(_ []string) int {
  c.UI.Append(glint.Layout(
    cc.IconRunning(),
    cc.Text("Running that sweet sweet dev command"),
  ).Row())

  c.UI.Render(context.Background())

  return 0
}
