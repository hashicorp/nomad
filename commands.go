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
				Meta: meta,
			}, nil
		},

		"agent-info": func() (cli.Command, error) {
			return &command.AgentInfoCommand{
				Meta: meta,
			}, nil
		},

		"agent-join": func() (cli.Command, error) {
			return &command.AgentJoinCommand{
				Meta: meta,
			}, nil
		},

		"agent-members": func() (cli.Command, error) {
			return &command.AgentMembersCommand{
				Meta: meta,
			}, nil
		},

		"node-drain": func() (cli.Command, error) {
			return &command.NodeDrainCommand{
				Meta: meta,
			}, nil
		},

		"node-status": func() (cli.Command, error) {
			return &command.NodeStatusCommand{
				Meta: meta,
			}, nil
		},

		"run": func() (cli.Command, error) {
			return &command.RunCommand{
				Meta: meta,
			}, nil
		},

		"status": func() (cli.Command, error) {
			return &command.StatusCommand{
				Meta: meta,
			}, nil
		},

		"version": func() (cli.Command, error) {
			ver := Version
			rel := VersionPrerelease
			if GitDescribe != "" {
				ver = GitDescribe
				// Trim off a leading 'v', we append it anyways.
				if ver[0] == 'v' {
					ver = ver[1:]
				}
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
