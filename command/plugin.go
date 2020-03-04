package command

type PluginCommand struct {
	Meta
}

func (c *PluginCommand) Help() string {
	helpText := `
Usage nomad plugin status [options] [plugin]

    This command groups subcommands for interacting with plugins.
`
}
