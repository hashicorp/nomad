package command

import (
	"fmt"
	"os"

	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/version"
	colorable "github.com/mattn/go-colorable"
	"github.com/mitchellh/cli"
)

const (
	// EnvNomadCLINoColor is an env var that toggles colored UI output.
	EnvNomadCLINoColor = `NOMAD_CLI_NO_COLOR`
)

// DeprecatedCommand is a command that wraps an existing command and prints a
// deprecation notice and points the user to the new command. Deprecated
// commands are always hidden from help output.
type DeprecatedCommand struct {
	cli.Command
	Meta

	// Old is the old command name, New is the new command name.
	Old, New string
}

// Help wraps the embedded Help command and prints a warning about deprecations.
func (c *DeprecatedCommand) Help() string {
	c.warn()
	return c.Command.Help()
}

// Run wraps the embedded Run command and prints a warning about deprecation.
func (c *DeprecatedCommand) Run(args []string) int {
	c.warn()
	return c.Command.Run(args)
}

func (c *DeprecatedCommand) warn() {
	c.Ui.Warn(wrapAtLength(fmt.Sprintf(
		"WARNING! The \"nomad %s\" command is deprecated. Please use \"nomad %s\" "+
			"instead. This command will be removed in Nomad 0.10 (or later).",
		c.Old,
		c.New)))
	c.Ui.Warn("")
}

// NamedCommand is a interface to denote a commmand's name.
type NamedCommand interface {
	Name() string
}

// Commands returns the mapping of CLI commands for Nomad. The meta
// parameter lets you set meta options for all commands.
func Commands(metaPtr *Meta, agentUi cli.Ui) map[string]cli.CommandFactory {
	if metaPtr == nil {
		metaPtr = new(Meta)
	}

	meta := *metaPtr
	if meta.Ui == nil {
		meta.Ui = &cli.BasicUi{
			Reader:      os.Stdin,
			Writer:      colorable.NewColorableStdout(),
			ErrorWriter: colorable.NewColorableStderr(),
		}
	}

	all := map[string]cli.CommandFactory{
		"acl": func() (cli.Command, error) {
			return &ACLCommand{
				Meta: meta,
			}, nil
		},
		"acl bootstrap": func() (cli.Command, error) {
			return &ACLBootstrapCommand{
				Meta: meta,
			}, nil
		},
		"acl policy": func() (cli.Command, error) {
			return &ACLPolicyCommand{
				Meta: meta,
			}, nil
		},
		"acl policy apply": func() (cli.Command, error) {
			return &ACLPolicyApplyCommand{
				Meta: meta,
			}, nil
		},
		"acl policy delete": func() (cli.Command, error) {
			return &ACLPolicyDeleteCommand{
				Meta: meta,
			}, nil
		},
		"acl policy info": func() (cli.Command, error) {
			return &ACLPolicyInfoCommand{
				Meta: meta,
			}, nil
		},
		"acl policy list": func() (cli.Command, error) {
			return &ACLPolicyListCommand{
				Meta: meta,
			}, nil
		},
		"acl token": func() (cli.Command, error) {
			return &ACLTokenCommand{
				Meta: meta,
			}, nil
		},
		"acl token create": func() (cli.Command, error) {
			return &ACLTokenCreateCommand{
				Meta: meta,
			}, nil
		},
		"acl token update": func() (cli.Command, error) {
			return &ACLTokenUpdateCommand{
				Meta: meta,
			}, nil
		},
		"acl token delete": func() (cli.Command, error) {
			return &ACLTokenDeleteCommand{
				Meta: meta,
			}, nil
		},
		"acl token info": func() (cli.Command, error) {
			return &ACLTokenInfoCommand{
				Meta: meta,
			}, nil
		},
		"acl token list": func() (cli.Command, error) {
			return &ACLTokenListCommand{
				Meta: meta,
			}, nil
		},
		"acl token self": func() (cli.Command, error) {
			return &ACLTokenSelfCommand{
				Meta: meta,
			}, nil
		},
		"alloc": func() (cli.Command, error) {
			return &AllocCommand{
				Meta: meta,
			}, nil
		},
		"alloc exec": func() (cli.Command, error) {
			return &AllocExecCommand{
				Meta: meta,
			}, nil
		},
		"alloc signal": func() (cli.Command, error) {
			return &AllocSignalCommand{
				Meta: meta,
			}, nil
		},
		"alloc stop": func() (cli.Command, error) {
			return &AllocStopCommand{
				Meta: meta,
			}, nil
		},
		"alloc fs": func() (cli.Command, error) {
			return &AllocFSCommand{
				Meta: meta,
			}, nil
		},
		"alloc logs": func() (cli.Command, error) {
			return &AllocLogsCommand{
				Meta: meta,
			}, nil
		},
		"alloc restart": func() (cli.Command, error) {
			return &AllocRestartCommand{
				Meta: meta,
			}, nil
		},
		"alloc status": func() (cli.Command, error) {
			return &AllocStatusCommand{
				Meta: meta,
			}, nil
		},
		"alloc-status": func() (cli.Command, error) {
			return &AllocStatusCommand{
				Meta: meta,
			}, nil
		},
		"agent": func() (cli.Command, error) {
			return &agent.Command{
				Version:    version.GetVersion(),
				Ui:         agentUi,
				ShutdownCh: make(chan struct{}),
			}, nil
		},
		"agent-info": func() (cli.Command, error) {
			return &AgentInfoCommand{
				Meta: meta,
			}, nil
		},
		"check": func() (cli.Command, error) {
			return &AgentCheckCommand{
				Meta: meta,
			}, nil
		},
		// operator debug was released in 0.12 as debug. This top-level alias preserves compatibility
		"debug": func() (cli.Command, error) {
			return &OperatorDebugCommand{
				Meta: meta,
			}, nil
		},
		"deployment": func() (cli.Command, error) {
			return &DeploymentCommand{
				Meta: meta,
			}, nil
		},
		"deployment fail": func() (cli.Command, error) {
			return &DeploymentFailCommand{
				Meta: meta,
			}, nil
		},
		"deployment list": func() (cli.Command, error) {
			return &DeploymentListCommand{
				Meta: meta,
			}, nil
		},
		"deployment pause": func() (cli.Command, error) {
			return &DeploymentPauseCommand{
				Meta: meta,
			}, nil
		},
		"deployment promote": func() (cli.Command, error) {
			return &DeploymentPromoteCommand{
				Meta: meta,
			}, nil
		},
		"deployment resume": func() (cli.Command, error) {
			return &DeploymentResumeCommand{
				Meta: meta,
			}, nil
		},
		"deployment status": func() (cli.Command, error) {
			return &DeploymentStatusCommand{
				Meta: meta,
			}, nil
		},
		"deployment unblock": func() (cli.Command, error) {
			return &DeploymentUnblockCommand{
				Meta: meta,
			}, nil
		},
		"eval": func() (cli.Command, error) {
			return &EvalCommand{
				Meta: meta,
			}, nil
		},
		"eval status": func() (cli.Command, error) {
			return &EvalStatusCommand{
				Meta: meta,
			}, nil
		},
		"eval-status": func() (cli.Command, error) {
			return &EvalStatusCommand{
				Meta: meta,
			}, nil
		},
		"exec": func() (cli.Command, error) {
			return &AllocExecCommand{
				Meta: meta,
			}, nil
		},
		"fs": func() (cli.Command, error) {
			return &AllocFSCommand{
				Meta: meta,
			}, nil
		},
		"init": func() (cli.Command, error) {
			return &JobInitCommand{
				Meta: meta,
			}, nil
		},
		"inspect": func() (cli.Command, error) {
			return &JobInspectCommand{
				Meta: meta,
			}, nil
		},
		"keygen": func() (cli.Command, error) {
			return &OperatorKeygenCommand{
				Meta: meta,
			}, nil
		},
		"keyring": func() (cli.Command, error) {
			return &OperatorKeyringCommand{
				Meta: meta,
			}, nil
		},
		"job": func() (cli.Command, error) {
			return &JobCommand{
				Meta: meta,
			}, nil
		},
		"job deployments": func() (cli.Command, error) {
			return &JobDeploymentsCommand{
				Meta: meta,
			}, nil
		},
		"job dispatch": func() (cli.Command, error) {
			return &JobDispatchCommand{
				Meta: meta,
			}, nil
		},
		"job eval": func() (cli.Command, error) {
			return &JobEvalCommand{
				Meta: meta,
			}, nil
		},
		"job history": func() (cli.Command, error) {
			return &JobHistoryCommand{
				Meta: meta,
			}, nil
		},
		"job init": func() (cli.Command, error) {
			return &JobInitCommand{
				Meta: meta,
			}, nil
		},
		"job inspect": func() (cli.Command, error) {
			return &JobInspectCommand{
				Meta: meta,
			}, nil
		},
		"job periodic": func() (cli.Command, error) {
			return &JobPeriodicCommand{
				Meta: meta,
			}, nil
		},
		"job periodic force": func() (cli.Command, error) {
			return &JobPeriodicForceCommand{
				Meta: meta,
			}, nil
		},
		"job plan": func() (cli.Command, error) {
			return &JobPlanCommand{
				Meta: meta,
			}, nil
		},
		"job promote": func() (cli.Command, error) {
			return &JobPromoteCommand{
				Meta: meta,
			}, nil
		},
		"job revert": func() (cli.Command, error) {
			return &JobRevertCommand{
				Meta: meta,
			}, nil
		},
		"job run": func() (cli.Command, error) {
			return &JobRunCommand{
				Meta: meta,
			}, nil
		},
		"job scale": func() (cli.Command, error) {
			return &JobScaleCommand{
				Meta: meta,
			}, nil
		},
		"job scaling-events": func() (cli.Command, error) {
			return &JobScalingEventsCommand{
				Meta: meta,
			}, nil
		},
		"job status": func() (cli.Command, error) {
			return &JobStatusCommand{
				Meta: meta,
			}, nil
		},
		"job stop": func() (cli.Command, error) {
			return &JobStopCommand{
				Meta: meta,
			}, nil
		},
		"job validate": func() (cli.Command, error) {
			return &JobValidateCommand{
				Meta: meta,
			}, nil
		},
		"license": func() (cli.Command, error) {
			return &LicenseCommand{
				Meta: meta,
			}, nil
		},
		"license get": func() (cli.Command, error) {
			return &LicenseGetCommand{
				Meta: meta,
			}, nil
		},
		"license put": func() (cli.Command, error) {
			return &LicensePutCommand{
				Meta: meta,
			}, nil
		},
		"logs": func() (cli.Command, error) {
			return &AllocLogsCommand{
				Meta: meta,
			}, nil
		},
		"monitor": func() (cli.Command, error) {
			return &MonitorCommand{
				Meta: meta,
			}, nil
		},
		"namespace": func() (cli.Command, error) {
			return &NamespaceCommand{
				Meta: meta,
			}, nil
		},
		"namespace apply": func() (cli.Command, error) {
			return &NamespaceApplyCommand{
				Meta: meta,
			}, nil
		},
		"namespace delete": func() (cli.Command, error) {
			return &NamespaceDeleteCommand{
				Meta: meta,
			}, nil
		},
		"namespace inspect": func() (cli.Command, error) {
			return &NamespaceInspectCommand{
				Meta: meta,
			}, nil
		},
		"namespace list": func() (cli.Command, error) {
			return &NamespaceListCommand{
				Meta: meta,
			}, nil
		},
		"namespace status": func() (cli.Command, error) {
			return &NamespaceStatusCommand{
				Meta: meta,
			}, nil
		},
		"node": func() (cli.Command, error) {
			return &NodeCommand{
				Meta: meta,
			}, nil
		},
		"node config": func() (cli.Command, error) {
			return &NodeConfigCommand{
				Meta: meta,
			}, nil
		},
		"node-drain": func() (cli.Command, error) {
			return &NodeDrainCommand{
				Meta: meta,
			}, nil
		},
		"node drain": func() (cli.Command, error) {
			return &NodeDrainCommand{
				Meta: meta,
			}, nil
		},
		"node eligibility": func() (cli.Command, error) {
			return &NodeEligibilityCommand{
				Meta: meta,
			}, nil
		},
		"node-status": func() (cli.Command, error) {
			return &NodeStatusCommand{
				Meta: meta,
			}, nil
		},
		"node status": func() (cli.Command, error) {
			return &NodeStatusCommand{
				Meta: meta,
			}, nil
		},
		"operator": func() (cli.Command, error) {
			return &OperatorCommand{
				Meta: meta,
			}, nil
		},

		"operator autopilot": func() (cli.Command, error) {
			return &OperatorAutopilotCommand{
				Meta: meta,
			}, nil
		},

		"operator autopilot get-config": func() (cli.Command, error) {
			return &OperatorAutopilotGetCommand{
				Meta: meta,
			}, nil
		},

		"operator autopilot set-config": func() (cli.Command, error) {
			return &OperatorAutopilotSetCommand{
				Meta: meta,
			}, nil
		},
		"operator debug": func() (cli.Command, error) {
			return &OperatorDebugCommand{
				Meta: meta,
			}, nil
		},
		"operator keygen": func() (cli.Command, error) {
			return &OperatorKeygenCommand{
				Meta: meta,
			}, nil
		},
		"operator keyring": func() (cli.Command, error) {
			return &OperatorKeyringCommand{
				Meta: meta,
			}, nil
		},
		"operator metrics": func() (cli.Command, error) {
			return &OperatorMetricsCommand{
				Meta: meta,
			}, nil
		},
		"operator raft": func() (cli.Command, error) {
			return &OperatorRaftCommand{
				Meta: meta,
			}, nil
		},

		"operator raft list-peers": func() (cli.Command, error) {
			return &OperatorRaftListCommand{
				Meta: meta,
			}, nil
		},

		"operator raft remove-peer": func() (cli.Command, error) {
			return &OperatorRaftRemoveCommand{
				Meta: meta,
			}, nil
		},
		"operator raft _info": func() (cli.Command, error) {
			return &OperatorRaftInfoCommand{
				Meta: meta,
			}, nil
		},
		"operator raft _logs": func() (cli.Command, error) {
			return &OperatorRaftLogsCommand{
				Meta: meta,
			}, nil
		},
		"operator raft _state": func() (cli.Command, error) {
			return &OperatorRaftStateCommand{
				Meta: meta,
			}, nil
		},

		"operator snapshot": func() (cli.Command, error) {
			return &OperatorSnapshotCommand{
				Meta: meta,
			}, nil
		},
		"operator snapshot save": func() (cli.Command, error) {
			return &OperatorSnapshotSaveCommand{
				Meta: meta,
			}, nil
		},
		"operator snapshot inspect": func() (cli.Command, error) {
			return &OperatorSnapshotInspectCommand{
				Meta: meta,
			}, nil
		},
		"operator snapshot restore": func() (cli.Command, error) {
			return &OperatorSnapshotRestoreCommand{
				Meta: meta,
			}, nil
		},

		"plan": func() (cli.Command, error) {
			return &JobPlanCommand{
				Meta: meta,
			}, nil
		},

		"plugin": func() (cli.Command, error) {
			return &PluginCommand{
				Meta: meta,
			}, nil
		},
		"plugin status": func() (cli.Command, error) {
			return &PluginStatusCommand{
				Meta: meta,
			}, nil
		},

		"quota": func() (cli.Command, error) {
			return &QuotaCommand{
				Meta: meta,
			}, nil
		},

		"quota apply": func() (cli.Command, error) {
			return &QuotaApplyCommand{
				Meta: meta,
			}, nil
		},

		"quota delete": func() (cli.Command, error) {
			return &QuotaDeleteCommand{
				Meta: meta,
			}, nil
		},

		"quota init": func() (cli.Command, error) {
			return &QuotaInitCommand{
				Meta: meta,
			}, nil
		},

		"quota inspect": func() (cli.Command, error) {
			return &QuotaInspectCommand{
				Meta: meta,
			}, nil
		},

		"quota list": func() (cli.Command, error) {
			return &QuotaListCommand{
				Meta: meta,
			}, nil
		},

		"quota status": func() (cli.Command, error) {
			return &QuotaStatusCommand{
				Meta: meta,
			}, nil
		},

		"recommendation": func() (cli.Command, error) {
			return &RecommendationCommand{
				Meta: meta,
			}, nil
		},
		"recommendation apply": func() (cli.Command, error) {
			return &RecommendationApplyCommand{
				RecommendationAutocompleteCommand: RecommendationAutocompleteCommand{
					Meta: meta,
				},
			}, nil
		},
		"recommendation dismiss": func() (cli.Command, error) {
			return &RecommendationDismissCommand{
				RecommendationAutocompleteCommand: RecommendationAutocompleteCommand{
					Meta: meta,
				},
			}, nil
		},
		"recommendation info": func() (cli.Command, error) {
			return &RecommendationInfoCommand{
				RecommendationAutocompleteCommand: RecommendationAutocompleteCommand{
					Meta: meta,
				},
			}, nil
		},
		"recommendation list": func() (cli.Command, error) {
			return &RecommendationListCommand{
				Meta: meta,
			}, nil
		},

		"run": func() (cli.Command, error) {
			return &JobRunCommand{
				Meta: meta,
			}, nil
		},
		"scaling": func() (cli.Command, error) {
			return &ScalingCommand{
				Meta: meta,
			}, nil
		},
		"scaling policy": func() (cli.Command, error) {
			return &ScalingPolicyCommand{
				Meta: meta,
			}, nil
		},
		"scaling policy info": func() (cli.Command, error) {
			return &ScalingPolicyInfoCommand{
				Meta: meta,
			}, nil
		},
		"scaling policy list": func() (cli.Command, error) {
			return &ScalingPolicyListCommand{
				Meta: meta,
			}, nil
		},
		"sentinel": func() (cli.Command, error) {
			return &SentinelCommand{
				Meta: meta,
			}, nil
		},
		"sentinel list": func() (cli.Command, error) {
			return &SentinelListCommand{
				Meta: meta,
			}, nil
		},
		"sentinel apply": func() (cli.Command, error) {
			return &SentinelApplyCommand{
				Meta: meta,
			}, nil
		},
		"sentinel delete": func() (cli.Command, error) {
			return &SentinelDeleteCommand{
				Meta: meta,
			}, nil
		},
		"sentinel read": func() (cli.Command, error) {
			return &SentinelReadCommand{
				Meta: meta,
			}, nil
		},
		"server": func() (cli.Command, error) {
			return &ServerCommand{
				Meta: meta,
			}, nil
		},
		"server force-leave": func() (cli.Command, error) {
			return &ServerForceLeaveCommand{
				Meta: meta,
			}, nil
		},
		"server join": func() (cli.Command, error) {
			return &ServerJoinCommand{
				Meta: meta,
			}, nil
		},
		"server members": func() (cli.Command, error) {
			return &ServerMembersCommand{
				Meta: meta,
			}, nil
		},
		"server-force-leave": func() (cli.Command, error) {
			return &ServerForceLeaveCommand{
				Meta: meta,
			}, nil
		},
		"server-join": func() (cli.Command, error) {
			return &ServerJoinCommand{
				Meta: meta,
			}, nil
		},
		"server-members": func() (cli.Command, error) {
			return &ServerMembersCommand{
				Meta: meta,
			}, nil
		},
		"status": func() (cli.Command, error) {
			return &StatusCommand{
				Meta: meta,
			}, nil
		},
		"stop": func() (cli.Command, error) {
			return &JobStopCommand{
				Meta: meta,
			}, nil
		},
		"system": func() (cli.Command, error) {
			return &SystemCommand{
				Meta: meta,
			}, nil
		},
		"system gc": func() (cli.Command, error) {
			return &SystemGCCommand{
				Meta: meta,
			}, nil
		},
		"system reconcile": func() (cli.Command, error) {
			return &SystemReconcileCommand{
				Meta: meta,
			}, nil
		},
		"system reconcile summaries": func() (cli.Command, error) {
			return &SystemReconcileSummariesCommand{
				Meta: meta,
			}, nil
		},
		"ui": func() (cli.Command, error) {
			return &UiCommand{
				Meta: meta,
			}, nil
		},
		"validate": func() (cli.Command, error) {
			return &JobValidateCommand{
				Meta: meta,
			}, nil
		},
		"version": func() (cli.Command, error) {
			return &VersionCommand{
				Version: version.GetVersion(),
				Ui:      meta.Ui,
			}, nil
		},
		"volume": func() (cli.Command, error) {
			return &VolumeCommand{
				Meta: meta,
			}, nil
		},
		"volume init": func() (cli.Command, error) {
			return &VolumeInitCommand{
				Meta: meta,
			}, nil
		},
		"volume status": func() (cli.Command, error) {
			return &VolumeStatusCommand{
				Meta: meta,
			}, nil
		},
		"volume register": func() (cli.Command, error) {
			return &VolumeRegisterCommand{
				Meta: meta,
			}, nil
		},
		"volume deregister": func() (cli.Command, error) {
			return &VolumeDeregisterCommand{
				Meta: meta,
			}, nil
		},
		"volume detach": func() (cli.Command, error) {
			return &VolumeDetachCommand{
				Meta: meta,
			}, nil
		},
	}

	deprecated := map[string]cli.CommandFactory{
		"client-config": func() (cli.Command, error) {
			return &DeprecatedCommand{
				Old:  "client-config",
				New:  "node config",
				Meta: meta,
				Command: &NodeConfigCommand{
					Meta: meta,
				},
			}, nil
		},

		"keygen": func() (cli.Command, error) {
			return &DeprecatedCommand{
				Old:  "keygen",
				New:  "operator keygen",
				Meta: meta,
				Command: &OperatorKeygenCommand{
					Meta: meta,
				},
			}, nil
		},

		"keyring": func() (cli.Command, error) {
			return &DeprecatedCommand{
				Old:  "keyring",
				New:  "operator keyring",
				Meta: meta,
				Command: &OperatorKeyringCommand{
					Meta: meta,
				},
			}, nil
		},

		"server-force-leave": func() (cli.Command, error) {
			return &DeprecatedCommand{
				Old:  "server-force-leave",
				New:  "server force-leave",
				Meta: meta,
				Command: &ServerForceLeaveCommand{
					Meta: meta,
				},
			}, nil
		},

		"server-join": func() (cli.Command, error) {
			return &DeprecatedCommand{
				Old:  "server-join",
				New:  "server join",
				Meta: meta,
				Command: &ServerJoinCommand{
					Meta: meta,
				},
			}, nil
		},

		"server-members": func() (cli.Command, error) {
			return &DeprecatedCommand{
				Old:  "server-members",
				New:  "server members",
				Meta: meta,
				Command: &ServerMembersCommand{
					Meta: meta,
				},
			}, nil
		},
	}

	for k, v := range deprecated {
		all[k] = v
	}

	for k, v := range EntCommands(metaPtr, agentUi) {
		all[k] = v
	}

	return all
}
