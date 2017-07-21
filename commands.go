package main

import (
	"os"

	"github.com/hashicorp/nomad/command"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/mitchellh/cli"
)

// Commands returns the mapping of CLI commands for Nomad. The meta
// parameter lets you set meta options for all commands.
func Commands(metaPtr *command.Meta) map[string]cli.CommandFactory {
	if metaPtr == nil {
		metaPtr = new(command.Meta)
	}

	meta := *metaPtr
	if meta.Ui == nil {
		meta.Ui = &cli.BasicUi{
			Reader:      os.Stdin,
			Writer:      os.Stdout,
			ErrorWriter: os.Stderr,
		}
	}

	return map[string]cli.CommandFactory{
		"alloc-status": func() (cli.Command, error) {
			return &command.AllocStatusCommand{
				Meta: meta,
			}, nil
		},
		"agent": func() (cli.Command, error) {
			return &agent.Command{
				Revision:          GitCommit,
				Version:           Version,
				VersionPrerelease: VersionPrerelease,
				Ui:                meta.Ui,
				ShutdownCh:        make(chan struct{}),
			}, nil
		},
		"agent-info": func() (cli.Command, error) {
			return &command.AgentInfoCommand{
				Meta: meta,
			}, nil
		},
		"check": func() (cli.Command, error) {
			return &command.AgentCheckCommand{
				Meta: meta,
			}, nil
		},
		"client-config": func() (cli.Command, error) {
			return &command.ClientConfigCommand{
				Meta: meta,
			}, nil
		},
		"deployment": func() (cli.Command, error) {
			return &command.DeploymentCommand{
				Meta: meta,
			}, nil
		},
		"deployment fail": func() (cli.Command, error) {
			return &command.DeploymentFailCommand{
				Meta: meta,
			}, nil
		},
		"deployment list": func() (cli.Command, error) {
			return &command.DeploymentListCommand{
				Meta: meta,
			}, nil
		},
		"deployment pause": func() (cli.Command, error) {
			return &command.DeploymentPauseCommand{
				Meta: meta,
			}, nil
		},
		"deployment promote": func() (cli.Command, error) {
			return &command.DeploymentPromoteCommand{
				Meta: meta,
			}, nil
		},
		"deployment resume": func() (cli.Command, error) {
			return &command.DeploymentResumeCommand{
				Meta: meta,
			}, nil
		},
		"deployment status": func() (cli.Command, error) {
			return &command.DeploymentStatusCommand{
				Meta: meta,
			}, nil
		},
		"eval-status": func() (cli.Command, error) {
			return &command.EvalStatusCommand{
				Meta: meta,
			}, nil
		},
		"executor": func() (cli.Command, error) {
			return &command.ExecutorPluginCommand{
				Meta: meta,
			}, nil
		},
		"fs": func() (cli.Command, error) {
			return &command.FSCommand{
				Meta: meta,
			}, nil
		},
		"init": func() (cli.Command, error) {
			return &command.InitCommand{
				Meta: meta,
			}, nil
		},
		"inspect": func() (cli.Command, error) {
			return &command.InspectCommand{
				Meta: meta,
			}, nil
		},
		"keygen": func() (cli.Command, error) {
			return &command.KeygenCommand{
				Meta: meta,
			}, nil
		},
		"keyring": func() (cli.Command, error) {
			return &command.KeyringCommand{
				Meta: meta,
			}, nil
		},
		"job": func() (cli.Command, error) {
			return &command.JobCommand{
				Meta: meta,
			}, nil
		},
		"job deployments": func() (cli.Command, error) {
			return &command.JobDeploymentsCommand{
				Meta: meta,
			}, nil
		},
		"job dispatch": func() (cli.Command, error) {
			return &command.JobDispatchCommand{
				Meta: meta,
			}, nil
		},
		"job history": func() (cli.Command, error) {
			return &command.JobHistoryCommand{
				Meta: meta,
			}, nil
		},
		"job promote": func() (cli.Command, error) {
			return &command.JobPromoteCommand{
				Meta: meta,
			}, nil
		},
		"job revert": func() (cli.Command, error) {
			return &command.JobRevertCommand{
				Meta: meta,
			}, nil
		},
		"logs": func() (cli.Command, error) {
			return &command.LogsCommand{
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

		"operator": func() (cli.Command, error) {
			return &command.OperatorCommand{
				Meta: meta,
			}, nil
		},

		"operator raft": func() (cli.Command, error) {
			return &command.OperatorRaftCommand{
				Meta: meta,
			}, nil
		},

		"operator raft list-peers": func() (cli.Command, error) {
			return &command.OperatorRaftListCommand{
				Meta: meta,
			}, nil
		},

		"operator raft remove-peer": func() (cli.Command, error) {
			return &command.OperatorRaftRemoveCommand{
				Meta: meta,
			}, nil
		},

		"plan": func() (cli.Command, error) {
			return &command.PlanCommand{
				Meta: meta,
			}, nil
		},

		"run": func() (cli.Command, error) {
			return &command.RunCommand{
				Meta: meta,
			}, nil
		},
		"server-force-leave": func() (cli.Command, error) {
			return &command.ServerForceLeaveCommand{
				Meta: meta,
			}, nil
		},
		"server-join": func() (cli.Command, error) {
			return &command.ServerJoinCommand{
				Meta: meta,
			}, nil
		},
		"server-members": func() (cli.Command, error) {
			return &command.ServerMembersCommand{
				Meta: meta,
			}, nil
		},
		"status": func() (cli.Command, error) {
			return &command.StatusCommand{
				Meta: meta,
			}, nil
		},
		"stop": func() (cli.Command, error) {
			return &command.StopCommand{
				Meta: meta,
			}, nil
		},
		"validate": func() (cli.Command, error) {
			return &command.ValidateCommand{
				Meta: meta,
			}, nil
		},
		"version": func() (cli.Command, error) {
			return &command.VersionCommand{
				Version: PrettyVersion(GetVersionParts()),
				Ui:      meta.Ui,
			}, nil
		},
	}
}
