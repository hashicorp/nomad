package main

import (
	"os"

	"github.com/hashicorp/nomad/command"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/mitchellh/cli"
)

// Commands returns the mapping of CLI commands for Vault. The meta
// parameter lets you set meta options for all commands.
func Commands(metaPtr *command.Meta) map[string]cli.CommandFactory {
	if metaPtr == nil {
		metaPtr = new(command.Meta)
	}

	meta := *metaPtr
	if meta.Ui == nil {
		meta.Ui = &cli.BasicUi{
			Writer:      os.Stdout,
			ErrorWriter: os.Stderr,
		}
	}

	return map[string]cli.CommandFactory{
		"agent": func() (cli.Command, error) {
			return &agent.Command{
				Revision:          GitCommit,
				Version:           Version,
				VersionPrerelease: VersionPrerelease,
				Ui:                meta.Ui,
				ShutdownCh:        make(chan struct{}),
			}, nil
		},

		"agent-force-leave": func() (cli.Command, error) {
			return &command.AgentForceLeaveCommand{
				Ui: meta.Ui,
			}, nil
		},

		"agent-info": func() (cli.Command, error) {
			return &command.AgentInfoCommand{
				Ui: meta.Ui,
			}, nil
		},

		"agent-join": func() (cli.Command, error) {
			return &command.AgentJoinCommand{
				Ui: meta.Ui,
			}, nil
		},

		"agent-members": func() (cli.Command, error) {
			return &command.AgentMembersCommand{
				Ui: meta.Ui,
			}, nil
		},

		"node-drain": func() (cli.Command, error) {
			return &command.NodeDrainCommand{
				Ui: meta.Ui,
			}, nil
		},

		"node-status": func() (cli.Command, error) {
			return &command.NodeStatusCommand{
				Ui: meta.Ui,
			}, nil
		},

		"status": func() (cli.Command, error) {
			return &command.StatusCommand{
				Ui: meta.Ui,
			}, nil
		},

		"version": func() (cli.Command, error) {
			ver := Version
			rel := VersionPrerelease
			if GitDescribe != "" {
				ver = GitDescribe
			}
			if GitDescribe == "" && rel == "" && VersionPrerelease != "" {
				rel = "dev"
			}

			return &command.VersionCommand{
				Revision:          GitCommit,
				Version:           ver,
				VersionPrerelease: rel,
				Ui:                meta.Ui,
			}, nil
		},
	}
}
