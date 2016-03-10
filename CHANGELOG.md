## 0.3.1 (UNRELEASED)

IMPROVEMENTS:
  * client: Added the allocation index environment variable [GH-896]
  * client: Add environment variables for task name, allocation ID/Name [GH-869]
  * client: Starting task is retried under the restart policy if the error is
    recoverable [GH-859]
  * driver/docker: Support for ECR [GH-858]
  * driver/docker: Periodic Fingerprinting [GH-893]

BUG FIXES:
  * core: No longer cancel evaluations that are delayed in the plan queue
    [GH-884]
  * api: Guard client/fs/ APIs from being accessed on a non-client node [GH-890]
  * client: Allow dashes in variable names during interprelation [GH-857]
  * client: Updating kill timeout adheres to operator specified maximum value [GH-878]
  * consul: Remove concurrent map access [GH-874]
  * driver/exec: Stopping tasks with more than one pid in a cgroup [GH-855]

## 0.3.0

__BACKWARDS INCOMPATIBILITIES:__
  * Stdout and Stderr log files of tasks have moved from task/local to
    alloc/logs [GH-851]
  * Any users of the runtime environment variable `$NOMAD_PORT_` will need to
    update to the new `${NOMAD_ADDR_}` varriable [GH-704]
  * Service names that include periods will fail validation. To fix, remove any
    periods from the service name before running the job [GH-770]
  * Task resources are now validated and enforce minimum resources. If a job
    specifies resources below the minimum they will need to be updated [GH-739]
  * Node ID is no longer specifiable. For users who have set a custom Node
    ID, the node should be drained before Nomad is updated and the data_dir
    should be deleted before starting for the first time [GH-675]
  * Users of custom restart policies should update to the new syntax which adds
    a `mode` field. The `mode` can be either `fail` or `delay`. The default for
    `batch` and `service` jobs is `fail` and `delay` respectively [GH-594]
  * All jobs that interpret variables in constraints or driver configurations
    will need to be updated to the new syntax which wraps the interpreted
    variable in curly braces. (`$node.class` becomes `${node.class}`) [GH-760]

IMPROVEMENTS:
  * core: Populate job status [GH-663]
  * core: Cgroup fingerprinter [GH-712]
  * core: Node class constraint [GH-618]
  * core: User specifiable kill timeout [GH-624]
  * core: Job queueing via blocked evaluations  [GH-726]
  * core: Only reschedule failed batch allocations [GH-746]
  * core: Add available nodes by DC to AllocMetrics [GH-619]
  * core: Improve scheduler retry logic under contention [GH-787]
  * core: Computed node class and stack optimization [GH-691, GH-708]
  * core: Improved restart policy with more user configuration [GH-594]
  * core: Periodic specification for jobs [GH-540, GH-657, GH-659, GH-668]
  * core: Batch jobs are garbage collected from the Nomad Servers [GH-586]
  * core: Free half the CPUs on leader node for use in plan queue and evaluation
    broker [GH-812]
  * core: Seed random number generator used to randomize node traversal order
    during scheduling [GH-808]
  * core: Performance improvements [GH-823, GH-825, GH-827, GH-830, GH-832,
    GH-833, GH-834, GH-839]
  * core/api: System garbage collection endpoint [GH-828]
  * core/api: Allow users to set arbitrary headers via agent config [GH-699]
  * core/cli: Prefix based lookups of allocs/nodes/evals/jobs [GH-575]
  * core/cli: Print short identifiers and UX cleanup [GH-675, GH-693, GH-692]
  * core/client: Client pulls minimum set of required allocations [GH-731]
  * cli: Output of agent-info is sorted [GH-617]
  * cli: Eval monitor detects zero wait condition [GH-776]
  * cli: Ability to navigate allocation directories [GH-709, GH-798]
  * client: Batch allocation updates to the server [GH-835]
  * client: Log rotation for all drivers [GH-685, GH-763, GH-819]
  * client: Only download artifacts from http, https, and S3 [GH-841]
  * client: Create a tmp/ directory inside each task directory [GH-757]
  * client: Store when an allocation was received by the client [GH-821]
  * client: Heartbeating and saving state resilient under high load [GH-811]
  * client: Handle updates to tasks Restart Policy and KillTimeout [GH-751]
  * client: Killing a driver handle is retried with an exponential backoff
    [GH-809]
  * client: Send Node to server when periodic fingerprinters change Node
    attributes/metadata [GH-749]
  * client/api: File-system access to allocation directories [GH-669]
  * drivers: Validate the "command" field contains a single value [GH-842]
  * drivers: Interpret Nomad variables in environment variables/args [GH-653]
  * driver/rkt: Add support for CPU/Memory isolation [GH-610]
  * driver/rkt: Add support for mounting alloc/task directory [GH-645]
  * driver/docker: Support for .dockercfg based auth for private registries
    [GH-773]

BUG FIXES:
  * core: Node drain could only be partially applied [GH-750]
  * core: Fix panic when eval Ack occurs at delivery limit [GH-790]
  * cli: Handle parsing of un-named ports [GH-604]
  * cli: Enforce absolute paths for data directories [GH-622]
  * client: Cleanup of the allocation directory [GH-755]
  * client: Improved stability under high contention [GH-789]
  * client: Handle non-200 codes when parsing AWS metadata [GH-614]
  * client: Unmounted of shared alloc dir when client is rebooted [GH-755]
  * client/consul: Service name changes handled properly [GH-766]
  * driver/rkt: handle broader format of rkt version outputs [GH-745]
  * driver/qemu: failed to load image and kvm accelerator fixes [GH-656]

## 0.2.3 (December 17, 2015)

BUG FIXES:
  * core: Task States not being properly updated [GH-600]
  * client: Fixes for user lookup to support CoreOS [GH-591]
  * discovery: Using a random prefix for nomad managed services [GH-579]
  * discovery: De-Registering Tasks while Nomad sleeps before failed tasks are
    restarted.
  * discovery: Fixes for service registration when multiple allocations are bin
    packed on a node [GH-583]
  * configuration: Sort configuration files [GH-588]
  * cli: RetryInterval was not being applied properly [GH-601]

## 0.2.2 (December 11, 2015)

IMPROVEMENTS:
  * core: Enable `raw_exec` driver in dev mode [GH-558]
  * cli: Server join/retry-join command line and config options [GH-527]
  * cli: Nomad reports which config files are loaded at start time, or if none
    are loaded [GH-536], [GH-553]

BUG FIXES:
  * core: Send syslog to `LOCAL0` by default as previously documented [GH-547]
  * client: remove all calls to default logger [GH-570]
  * consul: Nomad is less noisy when Consul is not running [GH-567]
  * consul: Nomad only deregisters services that it created [GH-568]
  * driver/exec: Shutdown a task now sends the interrupt signal first to the
    process before forcefully killing it. [GH-543]
  * driver/docker: Docker driver no longer leaks unix domain socket connections
    [GH-556]
  * fingerprint/network: Now correctly detects interfaces on Windows [GH-382]

## 0.2.1 (November 28, 2015)

IMPROVEMENTS:

  * core: Can specify a whitelist for activating drivers [GH-467]
  * core: Can specify a whitelist for activating fingerprinters [GH-488]
  * core/api: Can list all known regions in the cluster [GH-495]
  * client/spawn: spawn package tests made portable (work on Windows) [GH-442]
  * client/executor: executor package tests made portable (work on Windows) [GH-497]
  * client/driver: driver package tests made portable (work on windows) [GH-502]
  * client/discovery: Added more consul client api configuration options [GH-503]
  * driver/docker: Added TLS client options to the config file [GH-480]
  * jobspec: More flexibility in naming Services [GH-509]

BUG FIXES:

  * core: Shared reference to DynamicPorts caused port conflicts when scheduling
    count > 1 [GH-494]
  * client/restart policy: Not restarting Batch Jobs if the exit code is 0 [GH-491]
  * client/service discovery: Make Service IDs unique [GH-479]
  * client/service: Fixes update to check definitions and services which are already registered [GH-498]
  * driver/docker: Expose the container port instead of the host port [GH-466]
  * driver/docker: Support `port_map` for static ports [GH-476]
  * driver/docker: Pass 0.2.0-style port environment variables to the docker container [GH-476]
  * jobspec: distinct_hosts constraint can be specified as a boolean (previously panicked) [GH-501]

## 0.2.0 (November 18, 2015)

__BACKWARDS INCOMPATIBILITIES:__

  * core: HTTP API `/v1/node/<id>/allocations` returns full Allocation and not
    stub [GH-402]
  * core: Removed weight and hard/soft fields in constraints [GH-351]
  * drivers: Qemu and Java driver configurations have been updated to both use
    `artifact_source` as the source for external images/jars to be ran
  * jobspec: New reserved and dynamic port specification [GH-415]
  * jobspec/drivers: Driver configuration supports arbitrary struct to be
    passed in jobspec [GH-415]

FEATURES:

  * core: Blocking queries supported in API [GH-366]
  * core: System Scheduler that runs tasks on every node [GH-287]
  * core: Regexp, version and lexical ordering constraints [GH-271]
  * core: distinctHost constraint ensures Task Groups are running on distinct
    clients [GH-321]
  * core: Service block definition with Consul registration [GH-463, GH-460,
    GH-458, GH-455, GH-446, GH-425]
  * client: GCE Fingerprinting [GH-215]
  * client: Restart policy for task groups enforced by the client [GH-369,
    GH-393]
  * driver/rawexec: Raw Fork/Exec Driver [GH-237]
  * driver/rkt: Experimental Rkt Driver [GH-165, GH-247]
  * drivers: Add support for downloading external artifacts to execute for
    Exec, Raw exec drivers [GH-381]

IMPROVEMENTS:

  * core: Configurable Node GC threshold [GH-362]
  * core: Overlap plan verification and plan application for increased
    throughput [GH-272]
  * cli: Output of `alloc-status` also displays task state [GH-424]
  * cli: Output of `server-members` is sorted [GH-323]
  * cli: Show node attributes in `node-status` [GH-313]
  * client/fingerprint: Network fingerprinter detects interface suitable for
    use, rather than defaulting to eth0 [GH-334, GH-356]
  * client: Client Restore State properly reattaches to tasks and recreates
    them as needed [GH-364, GH-380, GH-388, GH-392, GH-394, GH-397, GH-408]
  * client: Periodic Fingerprinting [GH-391]
  * client: Precise snapshotting of TaskRunner and AllocRunner [GH-403, GH-411]
  * client: Task State is tracked by client [GH-416]
  * client: Test Skip Detection [GH-221]
  * driver/docker: Can now specify auth for docker pull [GH-390]
  * driver/docker: Can now specify DNS and DNSSearch options [GH-390]
  * driver/docker: Can now specify the container's hostname [GH-426]
  * driver/docker: Containers now have names based on the task name. [GH-389]
  * driver/docker: Mount task local and alloc directory to docker containers [GH-290]
  * driver/docker: Now accepts any value for `network_mode` to support userspace networking plugins in docker 1.9
  * driver/java: Pass JVM options in java driver [GH-293, GH-297]
  * drivers: Use BlkioWeight rather than BlkioThrottleReadIopsDevice [GH-222]
  * jobspec and drivers: Driver configuration supports arbitrary struct to be passed in jobspec [GH-415]

BUG FIXES:

  * core: Nomad Client/Server RPC codec encodes strings properly [GH-420]
  * core: Reset Nack timer in response to scheduler operations [GH-325]
  * core: Scheduler checks for updates to environment variables [GH-327]
  * cli: Fix crash when -config was given a directory or empty path [GH-119]
  * client/fingerprint: Use correct local interface on OS X [GH-361, GH-365]
  * client: Nomad Client doesn't restart failed containers [GH-198]
  * client: Reap spawn-daemon process, avoiding a zombie process [GH-240]
  * client: Resource exhausted errors because of link-speed zero [GH-146,
    GH-205]
  * client: Restarting Nomad Client leads to orphaned containers [GH-159]
  * driver/docker: Apply SELinux label for mounting directories in docker
    [GH-377]
  * driver/docker: Docker driver exposes ports when creating container [GH-212,
    GH-412]
  * driver/docker: Docker driver uses docker environment variables correctly
    [GH-407]
  * driver/qemu: Qemu fingerprint and tests work on both windows/linux [GH-352]

## 0.1.2 (October 6, 2015)

IMPROVEMENTS:

  * client: Nomad client cleans allocations on exit when in dev mode [GH-214]
  * drivers: Use go-getter for artifact retrieval, add artifact support to
    Exec, Raw Exec drivers [GH-288]

## 0.1.1 (October 5, 2015)

IMPROVEMENTS:

  * cli: Nomad Client configurable from command-line [GH-191]
  * client/fingerprint: Native IP detection and user specifiable network
    interface for fingerprinting [GH-189]
  * driver/docker: Docker networking mode is configurable [GH-184]
  * drivers: Set task environment variables [GH-206]

BUG FIXES:

  * client/fingerprint: Network fingerprinting failed if default network
    interface did not exist [GH-189]
  * client: Fixed issue where network resources throughput would be set to 0
    MBits if the link speed could not be determined [GH-205]
  * client: Improved detection of Nomad binary [GH-181]
  * driver/docker: Docker dynamic port mapping were not being set properly
    [GH-199]

## 0.1.0 (September 28, 2015)

  * Initial release

