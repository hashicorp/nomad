// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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

	// EnvNomadCLIForceColor is an env var that forces colored UI output.
	EnvNomadCLIForceColor = `NOMAD_CLI_FORCE_COLOR`
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
			"instead. This command will be removed a later version of Nomad.",
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
		"acl auth-method": func() (cli.Command, error) {
			return &ACLAuthMethodCommand{
				Meta: meta,
			}, nil
		},
		"acl auth-method create": func() (cli.Command, error) {
			return &ACLAuthMethodCreateCommand{
				Meta: meta,
			}, nil
		},
		"acl auth-method delete": func() (cli.Command, error) {
			return &ACLAuthMethodDeleteCommand{
				Meta: meta,
			}, nil
		},
		"acl auth-method info": func() (cli.Command, error) {
			return &ACLAuthMethodInfoCommand{
				Meta: meta,
			}, nil
		},
		"acl auth-method list": func() (cli.Command, error) {
			return &ACLAuthMethodListCommand{
				Meta: meta,
			}, nil
		},
		"acl auth-method update": func() (cli.Command, error) {
			return &ACLAuthMethodUpdateCommand{
				Meta: meta,
			}, nil
		},
		"acl binding-rule": func() (cli.Command, error) {
			return &ACLBindingRuleCommand{
				Meta: meta,
			}, nil
		},
		"acl binding-rule create": func() (cli.Command, error) {
			return &ACLBindingRuleCreateCommand{
				Meta: meta,
			}, nil
		},
		"acl binding-rule delete": func() (cli.Command, error) {
			return &ACLBindingRuleDeleteCommand{
				Meta: meta,
			}, nil
		},
		"acl binding-rule info": func() (cli.Command, error) {
			return &ACLBindingRuleInfoCommand{
				Meta: meta,
			}, nil
		},
		"acl binding-rule list": func() (cli.Command, error) {
			return &ACLBindingRuleListCommand{
				Meta: meta,
			}, nil
		},
		"acl binding-rule update": func() (cli.Command, error) {
			return &ACLBindingRuleUpdateCommand{
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
		"acl role": func() (cli.Command, error) {
			return &ACLRoleCommand{
				Meta: meta,
			}, nil
		},
		"acl role create": func() (cli.Command, error) {
			return &ACLRoleCreateCommand{
				Meta: meta,
			}, nil
		},
		"acl role delete": func() (cli.Command, error) {
			return &ACLRoleDeleteCommand{
				Meta: meta,
			}, nil
		},
		"acl role info": func() (cli.Command, error) {
			return &ACLRoleInfoCommand{
				Meta: meta,
			}, nil
		},
		"acl role list": func() (cli.Command, error) {
			return &ACLRoleListCommand{
				Meta: meta,
			}, nil
		},
		"acl role update": func() (cli.Command, error) {
			return &ACLRoleUpdateCommand{
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
		"action": func() (cli.Command, error) {
			return &ActionCommand{
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
		"alloc checks": func() (cli.Command, error) {
			return &AllocChecksCommand{
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
		"config": func() (cli.Command, error) {
			return &ConfigCommand{
				Meta: meta,
			}, nil
		},
		"config validate": func() (cli.Command, error) {
			return &ConfigValidateCommand{
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
		"eval delete": func() (cli.Command, error) {
			return &EvalDeleteCommand{
				Meta: meta,
			}, nil
		},
		"eval list": func() (cli.Command, error) {
			return &EvalListCommand{
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
		"fmt": func() (cli.Command, error) {
			return &FormatCommand{
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
		"job": func() (cli.Command, error) {
			return &JobCommand{
				Meta: meta,
			}, nil
		},
		"job allocs": func() (cli.Command, error) {
			return &JobAllocsCommand{
				Meta: meta,
			}, nil
		},
		"job restart": func() (cli.Command, error) {
			// Use a *cli.ConcurrentUi because this command spawns several
			// goroutines that write to the terminal concurrently.
			meta.Ui = &cli.ConcurrentUi{Ui: meta.Ui}
			return &JobRestartCommand{
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
		"login": func() (cli.Command, error) {
			return &LoginCommand{
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
		"node meta": func() (cli.Command, error) {
			return &NodeMetaCommand{
				Meta: meta,
			}, nil
		},
		"node meta apply": func() (cli.Command, error) {
			return &NodeMetaApplyCommand{
				Meta: meta,
			}, nil
		},
		"node meta read": func() (cli.Command, error) {
			return &NodeMetaReadCommand{
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
		"node pool": func() (cli.Command, error) {
			return &NodePoolCommand{
				Meta: meta,
			}, nil
		},
		"node pool apply": func() (cli.Command, error) {
			return &NodePoolApplyCommand{
				Meta: meta,
			}, nil
		},
		"node pool delete": func() (cli.Command, error) {
			return &NodePoolDeleteCommand{
				Meta: meta,
			}, nil
		},
		"node pool info": func() (cli.Command, error) {
			return &NodePoolInfoCommand{
				Meta: meta,
			}, nil
		},
		"node pool init": func() (cli.Command, error) {
			return &NodePoolInitCommand{
				Meta: meta,
			}, nil
		},
		"node pool jobs": func() (cli.Command, error) {
			return &NodePoolJobsCommand{
				Meta: meta,
			}, nil
		},
		"node pool list": func() (cli.Command, error) {
			return &NodePoolListCommand{
				Meta: meta,
			}, nil
		},
		"node pool nodes": func() (cli.Command, error) {
			return &NodePoolNodesCommand{
				Meta: meta,
			}, nil
		},
		"operator": func() (cli.Command, error) {
			return &OperatorCommand{
				Meta: meta,
			}, nil
		},

		"operator api": func() (cli.Command, error) {
			return &OperatorAPICommand{
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

		"operator client-state": func() (cli.Command, error) {
			return &OperatorClientStateCommand{
				Meta: meta,
			}, nil
		},
		"operator debug": func() (cli.Command, error) {
			return &OperatorDebugCommand{
				Meta: meta,
			}, nil
		},
		"operator gossip keyring": func() (cli.Command, error) {
			return &OperatorGossipKeyringCommand{
				Meta: meta,
			}, nil
		},
		"operator gossip keyring install": func() (cli.Command, error) {
			return &OperatorGossipKeyringInstallCommand{
				Meta: meta,
			}, nil
		},
		"operator gossip keyring use": func() (cli.Command, error) {
			return &OperatorGossipKeyringUseCommand{
				Meta: meta,
			}, nil
		},
		"operator gossip keyring list": func() (cli.Command, error) {
			return &OperatorGossipKeyringListCommand{
				Meta: meta,
			}, nil
		},
		"operator gossip keyring remove": func() (cli.Command, error) {
			return &OperatorGossipKeyringRemoveCommand{
				Meta: meta,
			}, nil
		},
		"operator gossip keyring generate": func() (cli.Command, error) {
			return &OperatorGossipKeyringGenerateCommand{
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
		"operator raft transfer-leadership": func() (cli.Command, error) {
			return &OperatorRaftTransferLeadershipCommand{
				Meta: meta,
			}, nil
		},
		"operator raft info": func() (cli.Command, error) {
			return &OperatorRaftInfoCommand{
				Meta: meta,
			}, nil
		},
		"operator raft logs": func() (cli.Command, error) {
			return &OperatorRaftLogsCommand{
				Meta: meta,
			}, nil
		},
		"operator raft state": func() (cli.Command, error) {
			return &OperatorRaftStateCommand{
				Meta: meta,
			}, nil
		},
		"operator scheduler": func() (cli.Command, error) {
			return &OperatorSchedulerCommand{
				Meta: meta,
			}, nil
		},
		"operator scheduler get-config": func() (cli.Command, error) {
			return &OperatorSchedulerGetConfig{
				Meta: meta,
			}, nil
		},
		"operator scheduler set-config": func() (cli.Command, error) {
			return &OperatorSchedulerSetConfig{
				Meta: meta,
			}, nil
		},
		"operator root keyring": func() (cli.Command, error) {
			return &OperatorRootKeyringCommand{
				Meta: meta,
			}, nil
		},
		"operator root keyring list": func() (cli.Command, error) {
			return &OperatorRootKeyringListCommand{
				Meta: meta,
			}, nil
		},
		"operator root keyring remove": func() (cli.Command, error) {
			return &OperatorRootKeyringRemoveCommand{
				Meta: meta,
			}, nil
		},
		"operator root keyring rotate": func() (cli.Command, error) {
			return &OperatorRootKeyringRotateCommand{
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
		"operator snapshot state": func() (cli.Command, error) {
			return &OperatorSnapshotStateCommand{
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
		"service": func() (cli.Command, error) {
			return &ServiceCommand{
				Meta: meta,
			}, nil
		},
		"service list": func() (cli.Command, error) {
			return &ServiceListCommand{
				Meta: meta,
			}, nil
		},
		"service info": func() (cli.Command, error) {
			return &ServiceInfoCommand{
				Meta: meta,
			}, nil
		},
		"service delete": func() (cli.Command, error) {
			return &ServiceDeleteCommand{
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
		"tls": func() (cli.Command, error) {
			return &TLSCommand{
				Meta: meta,
			}, nil
		},
		"tls ca": func() (cli.Command, error) {
			return &TLSCACommand{
				Meta: meta,
			}, nil
		},
		"tls ca create": func() (cli.Command, error) {
			return &TLSCACreateCommand{
				Meta: meta,
			}, nil
		},
		"tls ca info": func() (cli.Command, error) {
			return &TLSCAInfoCommand{
				Meta: meta,
			}, nil
		},
		"tls cert": func() (cli.Command, error) {
			return &TLSCertCommand{
				Meta: meta,
			}, nil
		},
		"tls cert create": func() (cli.Command, error) {
			return &TLSCertCreateCommand{
				Meta: meta,
			}, nil
		},
		"tls cert info": func() (cli.Command, error) {
			return &TLSCertInfoCommand{
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
		"var": func() (cli.Command, error) {
			return &VarCommand{
				Meta: meta,
			}, nil
		},
		"var purge": func() (cli.Command, error) {
			return &VarPurgeCommand{
				Meta: meta,
			}, nil
		},
		"var init": func() (cli.Command, error) {
			return &VarInitCommand{
				Meta: meta,
			}, nil
		},
		"var list": func() (cli.Command, error) {
			return &VarListCommand{
				Meta: meta,
			}, nil
		},
		"var put": func() (cli.Command, error) {
			return &VarPutCommand{
				Meta: meta,
			}, nil
		},
		"var lock": func() (cli.Command, error) {
			return &VarLockCommand{
				varPutCommand: &VarPutCommand{
					Meta: meta,
				},
			}, nil
		},
		"var get": func() (cli.Command, error) {
			return &VarGetCommand{
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
		"volume create": func() (cli.Command, error) {
			return &VolumeCreateCommand{
				Meta: meta,
			}, nil
		},
		"volume delete": func() (cli.Command, error) {
			return &VolumeDeleteCommand{
				Meta: meta,
			}, nil
		},
		"volume snapshot": func() (cli.Command, error) {
			return &VolumeSnapshotCommand{
				Meta: meta,
			}, nil
		},
		"volume snapshot create": func() (cli.Command, error) {
			return &VolumeSnapshotCreateCommand{
				Meta: meta,
			}, nil
		},
		"volume snapshot delete": func() (cli.Command, error) {
			return &VolumeSnapshotDeleteCommand{
				Meta: meta,
			}, nil
		},
		"volume snapshot list": func() (cli.Command, error) {
			return &VolumeSnapshotListCommand{
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
