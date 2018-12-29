package main

import (
	"os"

	"github.com/hashicorp/nomad/command"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/version"
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
		"acl": func() (cli.Command, error) {
			return &command.ACLCommand{
				Meta: meta,
			}, nil
		},
		"acl bootstrap": func() (cli.Command, error) {
			return &command.ACLBootstrapCommand{
				Meta: meta,
			}, nil
		},
		"acl policy": func() (cli.Command, error) {
			return &command.ACLPolicyCommand{
				Meta: meta,
			}, nil
		},
		"acl policy apply": func() (cli.Command, error) {
			return &command.ACLPolicyApplyCommand{
				Meta: meta,
			}, nil
		},
		"acl policy delete": func() (cli.Command, error) {
			return &command.ACLPolicyDeleteCommand{
				Meta: meta,
			}, nil
		},
		"acl policy info": func() (cli.Command, error) {
			return &command.ACLPolicyInfoCommand{
				Meta: meta,
			}, nil
		},
		"acl policy list": func() (cli.Command, error) {
			return &command.ACLPolicyListCommand{
				Meta: meta,
			}, nil
		},
		"acl token": func() (cli.Command, error) {
			return &command.ACLTokenCommand{
				Meta: meta,
			}, nil
		},
		"acl token create": func() (cli.Command, error) {
			return &command.ACLTokenCreateCommand{
				Meta: meta,
			}, nil
		},
		"acl token update": func() (cli.Command, error) {
			return &command.ACLTokenUpdateCommand{
				Meta: meta,
			}, nil
		},
		"acl token delete": func() (cli.Command, error) {
			return &command.ACLTokenDeleteCommand{
				Meta: meta,
			}, nil
		},
		"acl token info": func() (cli.Command, error) {
			return &command.ACLTokenInfoCommand{
				Meta: meta,
			}, nil
		},
		"acl token self": func() (cli.Command, error) {
			return &command.ACLTokenSelfCommand{
				Meta: meta,
			}, nil
		},
		"alloc-status": func() (cli.Command, error) {
			return &command.AllocStatusCommand{
				Meta: meta,
			}, nil
		},
		"agent": func() (cli.Command, error) {
			return &agent.Command{
				Version:    version.GetVersion(),
				Ui:         meta.Ui,
				ShutdownCh: make(chan struct{}),
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
		"job status": func() (cli.Command, error) {
			return &command.JobStatusCommand{
				Meta: meta,
			}, nil
		},
		"logs": func() (cli.Command, error) {
			return &command.LogsCommand{
				Meta: meta,
			}, nil
		},
		"namespace": func() (cli.Command, error) {
			return &command.NamespaceCommand{
				Meta: meta,
			}, nil
		},
		"namespace apply": func() (cli.Command, error) {
			return &command.NamespaceApplyCommand{
				Meta: meta,
			}, nil
		},
		"namespace delete": func() (cli.Command, error) {
			return &command.NamespaceDeleteCommand{
				Meta: meta,
			}, nil
		},
		"namespace inspect": func() (cli.Command, error) {
			return &command.NamespaceInspectCommand{
				Meta: meta,
			}, nil
		},
		"namespace list": func() (cli.Command, error) {
			return &command.NamespaceListCommand{
				Meta: meta,
			}, nil
		},
		"namespace status": func() (cli.Command, error) {
			return &command.NamespaceStatusCommand{
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

		"quota": func() (cli.Command, error) {
			return &command.QuotaCommand{
				Meta: meta,
			}, nil
		},

		"quota apply": func() (cli.Command, error) {
			return &command.QuotaApplyCommand{
				Meta: meta,
			}, nil
		},

		"quota delete": func() (cli.Command, error) {
			return &command.QuotaDeleteCommand{
				Meta: meta,
			}, nil
		},

		"quota init": func() (cli.Command, error) {
			return &command.QuotaInitCommand{
				Meta: meta,
			}, nil
		},

		"quota inspect": func() (cli.Command, error) {
			return &command.QuotaInspectCommand{
				Meta: meta,
			}, nil
		},

		"quota list": func() (cli.Command, error) {
			return &command.QuotaListCommand{
				Meta: meta,
			}, nil
		},

		"quota status": func() (cli.Command, error) {
			return &command.QuotaStatusCommand{
				Meta: meta,
			}, nil
		},

		"run": func() (cli.Command, error) {
			return &command.RunCommand{
				Meta: meta,
			}, nil
		},
		"sentinel": func() (cli.Command, error) {
			return &command.SentinelCommand{
				Meta: meta,
			}, nil
		},
		"sentinel list": func() (cli.Command, error) {
			return &command.SentinelListCommand{
				Meta: meta,
			}, nil
		},
		"sentinel apply": func() (cli.Command, error) {
			return &command.SentinelApplyCommand{
				Meta: meta,
			}, nil
		},
		"sentinel delete": func() (cli.Command, error) {
			return &command.SentinelDeleteCommand{
				Meta: meta,
			}, nil
		},
		"sentinel read": func() (cli.Command, error) {
			return &command.SentinelReadCommand{
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
		"ui": func() (cli.Command, error) {
			return &command.UiCommand{
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
				Version: version.GetVersion(),
				Ui:      meta.Ui,
			}, nil
		},
	}
}
