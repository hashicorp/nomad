// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	flagHelper "github.com/hashicorp/nomad/helper/flags"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// Ensure OperatorSchedulerSetConfig satisfies the cli.Command interface.
var _ cli.Command = &OperatorSchedulerSetConfig{}

type OperatorSchedulerSetConfig struct {
	Meta

	// The scheduler configuration flags allow us to tell whether the user set
	// a value or not. This means we can safely merge the current configuration
	// with user supplied, selective updates.
	checkIndex               string
	schedulerAlgorithm       string
	memoryOversubscription   flagHelper.BoolValue
	rejectJobRegistration    flagHelper.BoolValue
	pauseEvalBroker          flagHelper.BoolValue
	preemptBatchScheduler    flagHelper.BoolValue
	preemptServiceScheduler  flagHelper.BoolValue
	preemptSysBatchScheduler flagHelper.BoolValue
	preemptSystemScheduler   flagHelper.BoolValue
}

func (o *OperatorSchedulerSetConfig) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(o.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-check-index": complete.PredictAnything,
			"-scheduler-algorithm": complete.PredictSet(
				string(api.SchedulerAlgorithmBinpack),
				string(api.SchedulerAlgorithmSpread),
			),
			"-memory-oversubscription":    complete.PredictSet("true", "false"),
			"-reject-job-registration":    complete.PredictSet("true", "false"),
			"-pause-eval-broker":          complete.PredictSet("true", "false"),
			"-preempt-batch-scheduler":    complete.PredictSet("true", "false"),
			"-preempt-service-scheduler":  complete.PredictSet("true", "false"),
			"-preempt-sysbatch-scheduler": complete.PredictSet("true", "false"),
			"-preempt-system-scheduler":   complete.PredictSet("true", "false"),
		},
	)
}

func (o *OperatorSchedulerSetConfig) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (o *OperatorSchedulerSetConfig) Name() string { return "operator scheduler set-config" }

func (o *OperatorSchedulerSetConfig) Run(args []string) int {

	flags := o.Meta.FlagSet("set-config", FlagSetClient)
	flags.Usage = func() { o.Ui.Output(o.Help()) }

	flags.StringVar(&o.checkIndex, "check-index", "", "")
	flags.StringVar(&o.schedulerAlgorithm, "scheduler-algorithm", "", "")
	flags.Var(&o.memoryOversubscription, "memory-oversubscription", "")
	flags.Var(&o.rejectJobRegistration, "reject-job-registration", "")
	flags.Var(&o.pauseEvalBroker, "pause-eval-broker", "")
	flags.Var(&o.preemptBatchScheduler, "preempt-batch-scheduler", "")
	flags.Var(&o.preemptServiceScheduler, "preempt-service-scheduler", "")
	flags.Var(&o.preemptSysBatchScheduler, "preempt-sysbatch-scheduler", "")
	flags.Var(&o.preemptSystemScheduler, "preempt-system-scheduler", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Set up a client.
	client, err := o.Meta.Client()
	if err != nil {
		o.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Check that we got no arguments.
	args = flags.Args()
	if l := len(args); l != 0 {
		o.Ui.Error("This command takes no arguments")
		o.Ui.Error(commandErrorText(o))
		return 1
	}

	// Convert the check index string and handle any errors before adding this
	// to our request. This parsing handles empty values correctly.
	checkIndex, _, err := parseCheckIndex(o.checkIndex)
	if err != nil {
		o.Ui.Error(fmt.Sprintf("Error parsing check-index value %q: %v", o.checkIndex, err))
		return 1
	}

	// Fetch the current configuration. This will be used as a base to merge
	// user configuration onto.
	resp, _, err := client.Operator().SchedulerGetConfiguration(nil)
	if err != nil {
		o.Ui.Error(fmt.Sprintf("Error querying for scheduler configuration: %s", err))
		return 1
	}

	if checkIndex > 0 && resp.SchedulerConfig.ModifyIndex != checkIndex {
		errMsg := fmt.Sprintf("check-index %v does not match does not match current state value %v",
			checkIndex, resp.SchedulerConfig.ModifyIndex)
		o.Ui.Error(fmt.Sprintf("Error performing check index set: %s", errMsg))
		return 1
	}

	schedulerConfig := resp.SchedulerConfig

	// Overwrite the modification index if the user supplied one, otherwise we
	// use what was included within the read response.
	if checkIndex > 0 {
		schedulerConfig.ModifyIndex = checkIndex
	}

	// Merge the current configuration with any values set by the operator.
	if o.schedulerAlgorithm != "" {
		schedulerConfig.SchedulerAlgorithm = api.SchedulerAlgorithm(o.schedulerAlgorithm)
	}
	o.memoryOversubscription.Merge(&schedulerConfig.MemoryOversubscriptionEnabled)
	o.rejectJobRegistration.Merge(&schedulerConfig.RejectJobRegistration)
	o.pauseEvalBroker.Merge(&schedulerConfig.PauseEvalBroker)
	o.preemptBatchScheduler.Merge(&schedulerConfig.PreemptionConfig.BatchSchedulerEnabled)
	o.preemptServiceScheduler.Merge(&schedulerConfig.PreemptionConfig.ServiceSchedulerEnabled)
	o.preemptSysBatchScheduler.Merge(&schedulerConfig.PreemptionConfig.SysBatchSchedulerEnabled)
	o.preemptSystemScheduler.Merge(&schedulerConfig.PreemptionConfig.SystemSchedulerEnabled)

	// Check-and-set the new configuration.
	result, _, err := client.Operator().SchedulerCASConfiguration(schedulerConfig, nil)
	if err != nil {
		o.Ui.Error(fmt.Sprintf("Error setting scheduler configuration: %s", err))
		return 1
	}
	if result.Updated {
		o.Ui.Output("Scheduler configuration updated!")
		return 0
	}
	o.Ui.Output("Scheduler configuration could not be atomically updated, please try again")
	return 1
}

func (o *OperatorSchedulerSetConfig) Synopsis() string {
	return "Modify the current scheduler configuration"
}

func (o *OperatorSchedulerSetConfig) Help() string {
	helpText := `
Usage: nomad operator scheduler set-config [options]

  Modifies the current scheduler configuration.

  If ACLs are enabled, this command requires a token with the 'operator:write'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Scheduler Set Config Options:

  -check-index
    If set, the scheduler config is only updated if the passed modify index
    matches the current server side version. If a non-zero value is passed, it
    ensures that the scheduler config is being updated from a known state.

  -scheduler-algorithm=["binpack"|"spread"]
    Specifies whether scheduler binpacks or spreads allocations on available
    nodes.

  -memory-oversubscription=[true|false]
    When true, tasks may exceed their reserved memory limit, if the client has
    excess memory capacity. Tasks must specify memory_max to take advantage of
    memory oversubscription.

  -reject-job-registration=[true|false]
    When true, the server will return permission denied errors for job registration,
    job dispatch, and job scale APIs, unless the ACL token for the request is a
    management token. If ACLs are disabled, no user will be able to register jobs.
    This allows operators to shed load from automated processes during incident
    response.

  -pause-eval-broker=[true|false]
    When set to true, the eval broker which usually runs on the leader will be
    disabled. This will prevent the scheduler workers from receiving new work.

  -preempt-batch-scheduler=[true|false]
    Specifies whether preemption for batch jobs is enabled. Note that if this
    is set to true, then batch jobs can preempt any other jobs.

  -preempt-service-scheduler=[true|false]
    Specifies whether preemption for service jobs is enabled. Note that if this
    is set to true, then service jobs can preempt any other jobs.

  -preempt-sysbatch-scheduler=[true|false]
    Specifies whether preemption for system batch jobs is enabled. Note that if
    this is set to true, then system batch jobs can preempt any other jobs.

  -preempt-system-scheduler=[true|false]
    Specifies whether preemption for system jobs is enabled. Note that if this
    is set to true, then system jobs can preempt any other jobs.
`
	return strings.TrimSpace(helpText)
}
