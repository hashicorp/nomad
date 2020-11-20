package command

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	cflags "github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/posener/complete"
)

var (
	// enforceIndexRegex is a regular expression which extracts the enforcement error
	enforceIndexRegex = regexp.MustCompile(`\((Enforcing job modify index.*)\)`)
)

type JobRunCommand struct {
	Meta
	JobGetter
}

func (c *JobRunCommand) Help() string {
	helpText := `
Usage: nomad job run [options] <path>
Alias: nomad run

  Starts running a new job or updates an existing job using
  the specification located at <path>. This is the main command
  used to interact with Nomad.

  If the supplied path is "-", the jobfile is read from stdin. Otherwise
  it is read from the file at the supplied path or downloaded and
  read from URL specified.

  Upon successful job submission, this command will immediately
  enter an interactive monitor. This is useful to watch Nomad's
  internals make scheduling decisions and place the submitted work
  onto nodes. The monitor will end once job placement is done. It
  is safe to exit the monitor early using ctrl+c.

  On successful job submission and scheduling, exit code 0 will be
  returned. If there are job placement issues encountered
  (unsatisfiable constraints, resource exhaustion, etc), then the
  exit code will be 2. Any other errors, including client connection
  issues or internal errors, are indicated by exit code 1.

  If the job has specified the region, the -region flag and NOMAD_REGION
  environment variable are overridden and the job's region is used.

  The run command will set the consul_token of the job based on the following
  precedence, going from highest to lowest: the -consul-token flag, the
  $CONSUL_HTTP_TOKEN environment variable and finally the value in the job file.

  The run command will set the vault_token of the job based on the following
  precedence, going from highest to lowest: the -vault-token flag, the
  $VAULT_TOKEN environment variable and finally the value in the job file.

  When ACLs are enabled, this command requires a token with the 'submit-job'
  capability for the job's namespace. Jobs that mount CSI volumes require a
  token with the 'csi-mount-volume' capability for the volume's
  namespace. Jobs that mount host volumes require a token with the
  'host_volume' capability for that volume.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Run Options:

  -check-index
    If set, the job is only registered or updated if the passed
    job modify index matches the server side version. If a check-index value of
    zero is passed, the job is only registered if it does not yet exist. If a
    non-zero value is passed, it ensures that the job is being updated from a
    known state. The use of this flag is most common in conjunction with plan
    command.

  -detach
    Return immediately instead of entering monitor mode. After job submission,
    the evaluation ID will be printed to the screen, which can be used to
    examine the evaluation using the eval-status command.

  -hcl1
    Parses the job file as HCLv1.

  -output
    Output the JSON that would be submitted to the HTTP API without submitting
    the job.

  -policy-override
    Sets the flag to force override any soft mandatory Sentinel policies.

  -preserve-counts
    If set, the existing task group counts will be preserved when updating a job.

  -consul-token
    If set, the passed Consul token is stored in the job before sending to the
    Nomad servers. This allows passing the Consul token without storing it in
    the job file. This overrides the token found in $CONSUL_HTTP_TOKEN environment
    variable and that found in the job.

  -vault-token
    If set, the passed Vault token is stored in the job before sending to the
    Nomad servers. This allows passing the Vault token without storing it in
    the job file. This overrides the token found in $VAULT_TOKEN environment
    variable and that found in the job.

  -vault-namespace
    If set, the passed Vault namespace is stored in the job before sending to the
    Nomad servers.

  -var 'key=value'
    Variable for template, can be used multiple times.

  -var-file=path
    Path to HCL2 file containing user variables.

  -verbose
    Display full information.
`
	return strings.TrimSpace(helpText)
}

func (c *JobRunCommand) Synopsis() string {
	return "Run a new job or update an existing job"
}

func (c *JobRunCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-check-index":     complete.PredictNothing,
			"-detach":          complete.PredictNothing,
			"-verbose":         complete.PredictNothing,
			"-consul-token":    complete.PredictNothing,
			"-vault-token":     complete.PredictAnything,
			"-vault-namespace": complete.PredictAnything,
			"-output":          complete.PredictNothing,
			"-policy-override": complete.PredictNothing,
			"-preserve-counts": complete.PredictNothing,
			"-hcl1":            complete.PredictNothing,
			"-var":             complete.PredictAnything,
			"-var-file":        complete.PredictFiles("*.var"),
		})
}

func (c *JobRunCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictOr(complete.PredictFiles("*.nomad"), complete.PredictFiles("*.hcl"))
}

func (c *JobRunCommand) Name() string { return "job run" }

func (c *JobRunCommand) Run(args []string) int {
	var detach, verbose, output, override, preserveCounts bool
	var checkIndexStr, consulToken, vaultToken, vaultNamespace string
	var varArgs, varFiles cflags.AppendSliceValue

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&detach, "detach", false, "")
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&output, "output", false, "")
	flags.BoolVar(&override, "policy-override", false, "")
	flags.BoolVar(&preserveCounts, "preserve-counts", false, "")
	flags.BoolVar(&c.JobGetter.hcl1, "hcl1", false, "")
	flags.StringVar(&checkIndexStr, "check-index", "", "")
	flags.StringVar(&consulToken, "consul-token", "", "")
	flags.StringVar(&vaultToken, "vault-token", "", "")
	flags.StringVar(&vaultNamespace, "vault-namespace", "", "")
	flags.Var(&varArgs, "var", "")
	flags.Var(&varFiles, "var-file", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Truncate the id unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}

	// Check that we got exactly one argument
	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error("This command takes one argument: <path>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Get Job struct from Jobfile
	job, err := c.JobGetter.ApiJobWithArgs(args[0], varArgs, varFiles)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error getting job struct: %s", err))
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Force the region to be that of the job.
	if r := job.Region; r != nil {
		client.SetRegion(*r)
	}

	// Force the namespace to be that of the job.
	if n := job.Namespace; n != nil {
		client.SetNamespace(*n)
	}

	// Check if the job is periodic or is a parameterized job
	periodic := job.IsPeriodic()
	paramjob := job.IsParameterized()
	multiregion := job.IsMultiregion()

	// Parse the Consul token
	if consulToken == "" {
		// Check the environment variable
		consulToken = os.Getenv("CONSUL_HTTP_TOKEN")
	}

	if consulToken != "" {
		job.ConsulToken = helper.StringToPtr(consulToken)
	}

	// Parse the Vault token
	if vaultToken == "" {
		// Check the environment variable
		vaultToken = os.Getenv("VAULT_TOKEN")
	}

	if vaultToken != "" {
		job.VaultToken = helper.StringToPtr(vaultToken)
	}

	if vaultNamespace != "" {
		job.VaultNamespace = helper.StringToPtr(vaultNamespace)
	}

	if output {
		req := struct {
			Job *api.Job
		}{
			Job: job,
		}
		buf, err := json.MarshalIndent(req, "", "    ")
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error converting job: %s", err))
			return 1
		}

		c.Ui.Output(string(buf))
		return 0
	}

	// Parse the check-index
	checkIndex, enforce, err := parseCheckIndex(checkIndexStr)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing check-index value %q: %v", checkIndexStr, err))
		return 1
	}

	// Set the register options
	opts := &api.RegisterOptions{}
	if enforce {
		opts.EnforceIndex = true
		opts.ModifyIndex = checkIndex
	}
	opts.PolicyOverride = override
	opts.PreserveCounts = preserveCounts

	// Submit the job
	resp, _, err := client.Jobs().RegisterOpts(job, opts, nil)
	if err != nil {
		if strings.Contains(err.Error(), api.RegisterEnforceIndexErrPrefix) {
			// Format the error specially if the error is due to index
			// enforcement
			matches := enforceIndexRegex.FindStringSubmatch(err.Error())
			if len(matches) == 2 {
				c.Ui.Error(matches[1]) // The matched group
				c.Ui.Error("Job not updated")
				return 1
			}
		}

		c.Ui.Error(fmt.Sprintf("Error submitting job: %s", err))
		return 1
	}

	// Print any warnings if there are any
	if resp.Warnings != "" {
		c.Ui.Output(
			c.Colorize().Color(fmt.Sprintf("[bold][yellow]Job Warnings:\n%s[reset]\n", resp.Warnings)))
	}

	evalID := resp.EvalID

	// Check if we should enter monitor mode
	if detach || periodic || paramjob || multiregion {
		c.Ui.Output("Job registration successful")
		if periodic && !paramjob {
			loc, err := job.Periodic.GetLocation()
			if err == nil {
				now := time.Now().In(loc)
				next, err := job.Periodic.Next(now)
				if err != nil {
					c.Ui.Error(fmt.Sprintf("Error determining next launch time: %v", err))
				} else {
					c.Ui.Output(fmt.Sprintf("Approximate next launch time: %s (%s from now)",
						formatTime(next), formatTimeDifference(now, next, time.Second)))
				}
			}
		} else if !paramjob {
			c.Ui.Output("Evaluation ID: " + evalID)
		}

		return 0
	}

	// Detach was not specified, so start monitoring
	mon := newMonitor(c.Ui, client, length)
	return mon.monitor(evalID)

}

// parseCheckIndex parses the check-index flag and returns the index, whether it
// was set and potentially an error during parsing.
func parseCheckIndex(input string) (uint64, bool, error) {
	if input == "" {
		return 0, false, nil
	}

	u, err := strconv.ParseUint(input, 10, 64)
	return u, true, err
}
