## 0.9.0 (Unreleased)

IMPROVEMENTS:
 * core: Added advertise address to client node meta data [[GH-4390](https://github.com/hashicorp/nomad/issues/4390)]
 * client: Extend timeout to 60 seconds for Windows CPU fingerprinting [[GH-4441](https://github.com/hashicorp/nomad/pull/4441)]
 * driver/docker: Add support for specifying `cpu_cfs_period` in the Docker driver [[GH-4462](https://github.com/hashicorp/nomad/issues/4462)]
 * telemetry: All client metrics include a new `node_class` tag [[GH-3882](https://github.com/hashicorp/nomad/issues/3882)]
 * telemetry: Added new tags with value of child job id and parent job id for
   parameterized and periodic jobs [[GH-4392](https://github.com/hashicorp/nomad/issues/4392)]
 * vendor: Removed library obsoleted by go 1.8 [[GH-4469](https://github.com/hashicorp/nomad/issues/4469)]


BUG FIXES:
 * core: Reset queued allocation summary to zero when job stopped [[GH-4414](https://github.com/hashicorp/nomad/issues/4414)]
 * driver/docker: Fix kill timeout not being respected when timeout is over five
   minutes [[GH-4599](https://github.com/hashicorp/nomad/issues/4599)]
 * scheduler: Fix nil pointer dereference [[GH-4463](https://github.com/hashicorp/nomad/issues/4463)]

## 0.8.4 (June 11, 2018)

IMPROVEMENTS:
 * core: Updated serf library to improve how leave intents are handled [[GH-4278](https://github.com/hashicorp/nomad/issues/4278)]
 * core: Add more descriptive errors when parsing agent TLS certificates [[GH-4340](https://github.com/hashicorp/nomad/issues/4340)]
 * core: Added TLS configuration option to prefer server's ciphersuites over clients[[GH-4338](https://github.com/hashicorp/nomad/issues/4338)]
 * core: Add the option for operators to configure TLS versions and allowed
   cipher suites. Default is a subset of safe ciphers and TLS 1.2 [[GH-4269](https://github.com/hashicorp/nomad/pull/4269)]
 * core: Add a new [progress_deadline](https://www.nomadproject.io/docs/job-specification/update.html#progress_deadline) parameter to
   support rescheduling failed allocations during a deployment. This allows operators to specify a configurable deadline before which
   a deployment should see healthy allocations [[GH-4259](https://github.com/hashicorp/nomad/issues/4259)]
 * core: Add a new [job eval](https://www.nomadproject.io/docs/commands/job/eval.html) CLI and API
   for forcing an evaluation of a job, given the job ID. The new CLI also includes an option to force
   reschedule failed allocations [[GH-4274](https://github.com/hashicorp/nomad/issues/4274)]
 * core: Canary allocations are tagged in Consul to enable using service tags to
   isolate canary instances during deployments [[GH-4259](https://github.com/hashicorp/nomad/issues/4259)]
 * core: Emit Node events for drain and eligibility operations as well as for
   missed heartbeats [[GH-4284](https://github.com/hashicorp/nomad/issues/4284)], [[GH-4291](https://github.com/hashicorp/nomad/issues/4291)], [[GH-4292](https://github.com/hashicorp/nomad/issues/4292)]
 * agent: Support go-discover for auto-joining clusters based on cloud metadata
   [[GH-4277](https://github.com/hashicorp/nomad/issues/4277)]
 * cli: Add node drain monitoring with new `-monitor` flag on node drain
   command [[GH-4260](https://github.com/hashicorp/nomad/issues/4260)]
 * cli: Add node drain details to node status [[GH-4247](https://github.com/hashicorp/nomad/issues/4247)]
 * client: Avoid splitting log line across two files [[GH-4282](https://github.com/hashicorp/nomad/issues/4282)]
 * command: Add -short option to init command that emits a minimal
   jobspec [[GH-4239](https://github.com/hashicorp/nomad/issues/4239)]
 * discovery: Support Consul gRPC health checks. [[GH-4251](https://github.com/hashicorp/nomad/issues/4251)]
 * driver/docker: OOM kill metric [[GH-4185](https://github.com/hashicorp/nomad/issues/4185)]
 * driver/docker: Pull image with digest [[GH-4298](https://github.com/hashicorp/nomad/issues/4298)]
 * driver/docker: Support Docker pid limits [[GH-4341](https://github.com/hashicorp/nomad/issues/4341)]
 * driver/docker: Add progress monitoring and inactivity detection to docker
   image pulls [[GH-4192](https://github.com/hashicorp/nomad/issues/4192)]
 * driver/raw_exec: Use cgroups to manage process tree for precise cleanup of
   launched processes [[GH-4350](https://github.com/hashicorp/nomad/issues/4350)]
 * env: Default interpolation of optional meta fields of parameterized jobs to
   an empty string rather than the field key. [[GH-3720](https://github.com/hashicorp/nomad/issues/3720)]
 * ui: Show node drain, node eligibility, and node drain strategy information in the Client list and Client detail pages [[GH-4353](https://github.com/hashicorp/nomad/issues/4353)]
 * ui: Show reschedule-event information for allocations that were server-side rescheduled [[GH-4254](https://github.com/hashicorp/nomad/issues/4254)]
 * ui: Show the running deployment Progress Deadlines on the Job Detail Page [[GH-4388](https://github.com/hashicorp/nomad/issues/4388)]
 * ui: Show driver health status and node events on the Client Detail Page [[GH-4294](https://github.com/hashicorp/nomad/issues/4294)]
 * ui: Fuzzy and tokenized search on the Jobs List Page [[GH-4201](https://github.com/hashicorp/nomad/issues/4201)]
 * ui: The stop job button looks more dangerous [[GH-4339](https://github.com/hashicorp/nomad/issues/4339)]

BUG FIXES:
 * core: Clean up leaked deployments on restoration [[GH-4329](https://github.com/hashicorp/nomad/issues/4329)]
 * core: Fix regression to allow for dynamic Vault configuration reload [[GH-4395](https://github.com/hashicorp/nomad/issues/4395)]
 * core: Fix bug where older failed allocations of jobs that have been updated to a newer version were
   not being garbage collected [[GH-4313](https://github.com/hashicorp/nomad/issues/4313)]
 * core: Fix bug when upgrading an existing server to Raft protocol 3 that
   caused servers to never change their ID in the Raft configuration. [[GH-4349](https://github.com/hashicorp/nomad/issues/4349)]
 * core: Fix bug with scheduler not creating a new deployment when job is purged
   and re-added [[GH-4377](https://github.com/hashicorp/nomad/issues/4377)]
 * api/client: Fix potentially out of order logs and streamed file contents
   [[GH-4234](https://github.com/hashicorp/nomad/issues/4234)]
 * discovery: Fix flapping services when Nomad Server and Client point to the same
   Consul agent [[GH-4365](https://github.com/hashicorp/nomad/issues/4365)]
 * driver/docker: Fix docker credential helper support [[GH-4266](https://github.com/hashicorp/nomad/issues/4266)]
 * driver/docker: Fix panic when docker client configuration options are invalid [[GH-4303](https://github.com/hashicorp/nomad/issues/4303)]
 * driver/exec: Disable exec on non-linux platforms [[GH-4366](https://github.com/hashicorp/nomad/issues/4366)]
 * rpc: Fix RPC tunneling when running both client/server on one machine [[GH-4317](https://github.com/hashicorp/nomad/issues/4317)]
 * ui: Track the method in XHR tracking to prevent errant ACL error dialogs when stopping a job [[GH-4319](https://github.com/hashicorp/nomad/issues/4319)]
 * ui: Make the tasks list on the Allocation Detail Page look and behave like other lists [[GH-4387](https://github.com/hashicorp/nomad/issues/4387)] [[GH-4393](https://github.com/hashicorp/nomad/issues/4393)]
 * ui: Use the Network IP, not the Node IP, for task addresses [[GH-4369](https://github.com/hashicorp/nomad/issues/4369)]
 * ui: Use Polling instead of Streaming for logs in Safari [[GH-4335](https://github.com/hashicorp/nomad/issues/4335)]
 * ui: Track PlaceCanaries in deployment metrics [[GH-4325](https://github.com/hashicorp/nomad/issues/4325)]

## 0.8.3 (April 27, 2018)

BUG FIXES:
 * core: Fix panic proxying node connections when the server does not have a
   connection to the node [[GH-4231](https://github.com/hashicorp/nomad/issues/4231)]
 * core: Fix bug with not updating ModifyIndex of allocations after updates to
   the `NextAllocation` field [[GH-4250](https://github.com/hashicorp/nomad/issues/4250)]

## 0.8.2 (April 26, 2018)

IMPROVEMENTS:
 * api: Add /v1/jobs/parse api endpoint for rendering HCL jobs files as JSON [[GH-2782](https://github.com/hashicorp/nomad/issues/2782)]
 * api: Include reschedule tracking events in end points that return a list of allocations [[GH-4240](https://github.com/hashicorp/nomad/issues/4240)]
 * cli: Improve help text when invalid arguments are given [[GH-4176](https://github.com/hashicorp/nomad/issues/4176)]
 * client: Create new process group on process startup. [[GH-3572](https://github.com/hashicorp/nomad/issues/3572)]
 * discovery: Periodically sync services and checks with Consul [[GH-4170](https://github.com/hashicorp/nomad/issues/4170)]
 * driver/rkt: Enable stats collection for rkt tasks [[GH-4188](https://github.com/hashicorp/nomad/pull/4188)]
 * ui: Stop job button added to job detail pages [[GH-4189](https://github.com/hashicorp/nomad/pull/4189)]

BUG FIXES:
 * core: Handle invalid cron specifications more gracefully [[GH-4224](https://github.com/hashicorp/nomad/issues/4224)]
 * core: Sort signals in implicit constraint avoiding unnecessary updates
   [[GH-4216](https://github.com/hashicorp/nomad/issues/4216)]
 * core: Improve tracking of node connections even if the address being used to
   contact the server changes [[GH-4222](https://github.com/hashicorp/nomad/issues/4222)]
 * core: Fix panic when doing a node drain effecting a job that has an
   allocation that was on a node that no longer exists
   [[GH-4215](https://github.com/hashicorp/nomad/issues/4215)]
 * api: Fix an issue in which the autopilot configuration could not be updated
   [[GH-4220](https://github.com/hashicorp/nomad/issues/4220)]
 * client: Populate access time and modify time when unarchiving tar archives
   that do not specify them explicitly [[GH-4217](https://github.com/hashicorp/nomad/issues/4217)]
 * driver/exec: Create process group for Windows process and send Ctrl-Break
   signal on Shutdown [[GH-4153](https://github.com/hashicorp/nomad/pull/4153)]
 * ui: Alloc stats will continue to poll after a request errors or returns an invalid response [[GH-4195](https://github.com/hashicorp/nomad/pull/4195)]

## 0.8.1 (April 17, 2018)

BUG FIXES:
 * client: Fix a race condition while concurrently fingerprinting and accessing
   the node that could cause a panic [[GH-4166](https://github.com/hashicorp/nomad/issues/4166)]

## 0.8.0 (April 12, 2018)

__BACKWARDS INCOMPATIBILITIES:__
 * cli: node drain now blocks until the drain completes and all allocations on
   the draining node have stopped. Use -detach for the old behavior.
 * client: Periods (`.`) are no longer replaced with underscores (`_`) in
   environment variables as many applications rely on periods in environment
   variable names. [[GH-3760](https://github.com/hashicorp/nomad/issues/3760)]
 * client/metrics: The key emitted for tracking a client's uptime has changed
   from "uptime" to "client.uptime". Users monitoring this metric will have to
   switch to the new key name [[GH-4128](https://github.com/hashicorp/nomad/issues/4128)]
 * discovery: Prevent absolute URLs in check paths. The documentation indicated
   that absolute URLs are not allowed, but it was not enforced. Absolute URLs
   in HTTP check paths will now fail to validate. [[GH-3685](https://github.com/hashicorp/nomad/issues/3685)]
 * drain: Draining a node no longer stops all allocations immediately: a new
   [migrate stanza](https://www.nomadproject.io/docs/job-specification/migrate.html)
   allows jobs to specify how quickly task groups can be drained. A `-force`
   option can be used to emulate the old drain behavior.
 * jobspec: The default values for restart policy have changed. Restart policy
   mode defaults to "fail" and the attempts/time interval values have been
   changed to enable faster server side rescheduling. See [restart
   stanza](https://www.nomadproject.io/docs/job-specification/restart.html) for
   more information.
 * jobspec: Removed compatibility code that migrated pre Nomad 0.6.0 Update
   stanza syntax. All job spec files should be using update stanza fields
   introduced in 0.7.0
   [[GH-3979](https://github.com/hashicorp/nomad/pull/3979/files)]

IMPROVEMENTS:
 * core: Servers can now service client HTTP endpoints [[GH-3892](https://github.com/hashicorp/nomad/issues/3892)]
 * core: More efficient garbage collection of large batches of jobs [[GH-3982](https://github.com/hashicorp/nomad/issues/3982)]
 * core: Allow upgrading/downgrading TLS via SIGHUP on both servers and clients [[GH-3492](https://github.com/hashicorp/nomad/issues/3492)]
 * core: Node events are emitted for events such as node registration and
   heartbeating [[GH-3945](https://github.com/hashicorp/nomad/issues/3945)]
 * core: A set of features (Autopilot) has been added to allow for automatic operator-friendly management of Nomad servers. For more information about Autopilot, see the [Autopilot Guide](https://www.nomadproject.io/guides/cluster/autopilot.html). [[GH-3670](https://github.com/hashicorp/nomad/pull/3670)]
 * core: Failed tasks are automatically rescheduled according to user specified criteria. For more information on configuration, see the [Reshedule Stanza](https://www.nomadproject.io/docs/job-specification/reschedule.html) [[GH-3981](https://github.com/hashicorp/nomad/issues/3981)]
 * core: Servers can now service client HTTP endpoints [[GH-3892](https://github.com/hashicorp/nomad/issues/3892)]
 * core: Servers can now retry connecting to Vault to verify tokens without requiring a SIGHUP to do so [[GH-3957](https://github.com/hashicorp/nomad/issues/3957)]
 * core: Updated yamux library to pick up memory and CPU performance improvements [[GH-3980](https://github.com/hashicorp/nomad/issues/3980)]
 * core: Client stanza now supports overriding total memory [[GH-4052](https://github.com/hashicorp/nomad/issues/4052)]
 * core: Node draining is now able to migrate allocations in a controlled
   manner with parameters specified by the drain command and in job files using
   the migrate stanza [[GH-4010](https://github.com/hashicorp/nomad/issues/4010)]
 * acl: Increase token name limit from 64 characters to 256 [[GH-3888](https://github.com/hashicorp/nomad/issues/3888)]
 * cli: Node status and filesystem related commands do not require direct
   network access to the Nomad client nodes [[GH-3892](https://github.com/hashicorp/nomad/issues/3892)]
 * cli: Common commands highlighed [[GH-4027](https://github.com/hashicorp/nomad/issues/4027)]
 * cli: Colored error and warning outputs [[GH-4027](https://github.com/hashicorp/nomad/issues/4027)]
 * cli: All commands are grouped by subsystem [[GH-4027](https://github.com/hashicorp/nomad/issues/4027)]
 * cli: Use ISO_8601 time format for cli output [[GH-3814](https://github.com/hashicorp/nomad/pull/3814)]
 * cli: Clearer task event descriptions in `nomad alloc-status` when there are server side failures authenticating to Vault [[GH-3968](https://github.com/hashicorp/nomad/issues/3968)]
 * client: Allow '.' in environment variable names [[GH-3760](https://github.com/hashicorp/nomad/issues/3760)]
 * client: Improved handling of failed RPCs and heartbeat retry logic [[GH-4106](https://github.com/hashicorp/nomad/issues/4106)]
 * client: Refactor client fingerprint methods to a request/response format [[GH-3781](https://github.com/hashicorp/nomad/issues/3781)]
 * client: Enable periodic health checks for drivers. Initial support only includes the Docker driver. [[GH-3856](https://github.com/hashicorp/nomad/issues/3856)]
 * discovery: Allow `check_restart` to be specified in the `service` stanza
   [[GH-3718](https://github.com/hashicorp/nomad/issues/3718)]
 * discovery: Allow configuring names of Nomad client and server health checks
   [[GH-4003](https://github.com/hashicorp/nomad/issues/4003)]
 * discovery: Only log if Consul does not support TLSSkipVerify instead of
   dropping checks which relied on it. Consul has had this feature since 0.7.2 [[GH-3983](https://github.com/hashicorp/nomad/issues/3983)]
 * driver/docker: Support hard CPU limits [[GH-3825](https://github.com/hashicorp/nomad/issues/3825)]
 * driver/docker: Support advertising IPv6 addresses [[GH-3790](https://github.com/hashicorp/nomad/issues/3790)]
 * driver/docker; Support overriding image entrypoint [[GH-3788](https://github.com/hashicorp/nomad/issues/3788)]
 * driver/docker: Support adding or dropping capabilities [[GH-3754](https://github.com/hashicorp/nomad/issues/3754)]
 * driver/docker: Support mounting root filesystem as read-only [[GH-3802](https://github.com/hashicorp/nomad/issues/3802)]
 * driver/docker: Retry on Portworx "volume is attached on another node" errors
   [[GH-3993](https://github.com/hashicorp/nomad/issues/3993)]
 * driver/lxc: Add volumes config to LXC driver [[GH-3687](https://github.com/hashicorp/nomad/issues/3687)]
 * driver/rkt: Allow overriding group [[GH-3990](https://github.com/hashicorp/nomad/issues/3990)]
 * telemetry: Support DataDog tags [[GH-3839](https://github.com/hashicorp/nomad/issues/3839)]
 * ui: Specialized job detail pages for each job type (system, service, batch, periodic, parameterized, periodic instance, parameterized instance) [[GH-3829](https://github.com/hashicorp/nomad/issues/3829)]
 * ui: Allocation stats requests are made through the server instead of directly through clients [[GH-3908](https://github.com/hashicorp/nomad/issues/3908)]
 * ui: Allocation log requests fallback to using the server when the client can't be reached [[GH-3908](https://github.com/hashicorp/nomad/issues/3908)]
 * ui: All views poll for changes using long-polling via blocking queries [[GH-3936](https://github.com/hashicorp/nomad/issues/3936)]
 * ui: Dispatch payload on the parameterized instance job detail page [[GH-3829](https://github.com/hashicorp/nomad/issues/3829)]
 * ui: Periodic force launch button on the periodic job detail page [[GH-3829](https://github.com/hashicorp/nomad/issues/3829)]
 * ui: Allocation breadcrumbs now extend job breadcrumbs [[GH-3829](https://github.com/hashicorp/nomad/issues/3974)]
 * vault: Allow Nomad to create orphaned tokens for allocations [[GH-3992](https://github.com/hashicorp/nomad/issues/3992)]

BUG FIXES:
 * core: Fix search endpoint forwarding for multi-region clusters [[GH-3680](https://github.com/hashicorp/nomad/issues/3680)]
 * core: Fix an issue in which batch jobs with queued placements and lost
   allocations could result in improper placement counts [[GH-3717](https://github.com/hashicorp/nomad/issues/3717)]
 * core: Fix an issue where an entire region leaving caused `nomad server-members` to fail with a 500 response [[GH-1515](https://github.com/hashicorp/nomad/issues/1515)]
 * core: Fix an issue in which multiple servers could be acting as a leader. A
   prominent side-effect being nodes TTLing incorrectly [[GH-3890](https://github.com/hashicorp/nomad/issues/3890)]
 * core: Fix an issue where jobs with the same name in a different namespace were not being blocked correctly [[GH-3972](https://github.com/hashicorp/nomad/issues/3972)]
 * cli: server member command handles failure to retrieve leader in remote
   regions [[GH-4087](https://github.com/hashicorp/nomad/issues/4087)]
 * client: Support IP detection of wireless interfaces on Windows [[GH-4011](https://github.com/hashicorp/nomad/issues/4011)]
 * client: Migrated ephemeral_disk's maintain directory permissions [[GH-3723](https://github.com/hashicorp/nomad/issues/3723)]
 * client: Always advertise driver IP when in driver address mode [[GH-3682](https://github.com/hashicorp/nomad/issues/3682)]
 * client: Preserve permissions on directories when expanding tarred artifacts [[GH-4129](https://github.com/hashicorp/nomad/issues/4129)]
 * client: Improve auto-detection of network interface when interface name has a
   space in it on Windows [[GH-3855](https://github.com/hashicorp/nomad/issues/3855)]
 * client/vault: Recognize renewing non-renewable Vault lease as fatal [[GH-3727](https://github.com/hashicorp/nomad/issues/3727)]
 * client/vault: Improved error handling of network errors with Vault [[GH-4100](https://github.com/hashicorp/nomad/issues/4100)]
 * config: Revert minimum CPU limit back to 20 from 100 [[GH-3706](https://github.com/hashicorp/nomad/issues/3706)]
 * config: Always add core scheduler to enabled schedulers and add invalid
   EnabledScheduler detection [[GH-3978](https://github.com/hashicorp/nomad/issues/3978)]
 * driver/exec: Properly disable swapping [[GH-3958](https://github.com/hashicorp/nomad/issues/3958)]
 * driver/lxc: Cleanup LXC containers after errors on container startup. [[GH-3773](https://github.com/hashicorp/nomad/issues/3773)]
 * ui: Always show the task name in the task recent events table on the allocation detail page. [[GH-3985](https://github.com/hashicorp/nomad/pull/3985)]
 * ui: Only show the placement failures section when there is a blocked evaluation. [[GH-3956](https://github.com/hashicorp/nomad/pull/3956)]
 * ui: Fix requests using client-side certificates in Firefox. [[GH-3728](https://github.com/hashicorp/nomad/pull/3728)]
 * ui: Fix ui on non-leaders when ACLs are enabled [[GH-3722](https://github.com/hashicorp/nomad/issues/3722)]


## 0.7.1 (December 19, 2017)

__BACKWARDS INCOMPATIBILITIES:__
 * client: The format of service IDs in Consul has changed. If you rely upon
   Nomad's service IDs (*not* service names; those are stable), you will need
   to update your code.  [[GH-3632](https://github.com/hashicorp/nomad/issues/3632)]
 * config: Nomad no longer parses Atlas configuration stanzas. Atlas has been
   deprecated since earlier this year. If you have an Atlas stanza in your
   config file it will have to be removed.
 * config: Default minimum CPU configuration has been changed to 100 from 20. Jobs
   using the old minimum value of 20 will have to be updated.
 * telemetry: Hostname is now emitted via a tag rather than within the key name.
   To maintain old behavior during an upgrade path specify
   `backwards_compatible_metrics` in the telemetry configuration.

IMPROVEMENTS:
 * core: Allow operators to reload TLS certificate and key files via SIGHUP
   [[GH-3479](https://github.com/hashicorp/nomad/issues/3479)]
 * core: Allow configurable stop signals for a task, when drivers support
   sending stop signals [[GH-1755](https://github.com/hashicorp/nomad/issues/1755)]
 * core: Allow agents to be run in `rpc_upgrade_mode` when migrating a cluster
   to TLS rather than changing `heartbeat_grace`
 * api: Allocations now track and return modify time in addition to create time
   [[GH-3446](https://github.com/hashicorp/nomad/issues/3446)]
 * api: Introduced new fields to track details and display message for task
   events, and deprecated redundant existing fields [[GH-3399](https://github.com/hashicorp/nomad/issues/3399)]
 * api: Environment variables are ignored during service name validation [[GH-3532](https://github.com/hashicorp/nomad/issues/3532)]
 * cli: Allocation create and modify times are displayed in a human readable
   relative format like `6 h ago` [[GH-3449](https://github.com/hashicorp/nomad/issues/3449)]
 * client: Support `address_mode` on checks [[GH-3619](https://github.com/hashicorp/nomad/issues/3619)]
 * client: Sticky volume migrations are now atomic. [[GH-3563](https://github.com/hashicorp/nomad/issues/3563)]
 * client: Added metrics to track state transitions of allocations [[GH-3061](https://github.com/hashicorp/nomad/issues/3061)]
 * client: When `network_interface` is unspecified use interface attached to
   default route [[GH-3546](https://github.com/hashicorp/nomad/issues/3546)]
 * client: Support numeric ports on services and checks when
   `address_mode="driver"` [[GH-3619](https://github.com/hashicorp/nomad/issues/3619)]
 * driver/docker: Detect OOM kill event [[GH-3459](https://github.com/hashicorp/nomad/issues/3459)]
 * driver/docker: Adds support for adding host device to container via
   `--device` [[GH-2938](https://github.com/hashicorp/nomad/issues/2938)]
 * driver/docker: Adds support for `ulimit` and `sysctl` options [[GH-3568](https://github.com/hashicorp/nomad/issues/3568)]
 * driver/docker: Adds support for StopTimeout (set to the same value as
   kill_timeout [[GH-3601](https://github.com/hashicorp/nomad/issues/3601)]
 * driver/rkt: Add support for passing through user [[GH-3612](https://github.com/hashicorp/nomad/issues/3612)]
 * driver/qemu: Support graceful shutdowns on unix platforms [[GH-3411](https://github.com/hashicorp/nomad/issues/3411)]
 * template: Updated to consul template 0.19.4 [[GH-3543](https://github.com/hashicorp/nomad/issues/3543)]
 * core/enterprise: Return 501 status code in Nomad Pro for Premium end points
 * ui: Added log streaming for tasks [[GH-3564](https://github.com/hashicorp/nomad/issues/3564)]
 * ui: Show the modify time for allocations [[GH-3607](https://github.com/hashicorp/nomad/issues/3607)]
 * ui: Added a dedicated Task page under allocations [[GH-3472](https://github.com/hashicorp/nomad/issues/3472)]
 * ui: Added placement failures to the Job Detail page [[GH-3603](https://github.com/hashicorp/nomad/issues/3603)]
 * ui: Warn uncaught exceptions to the developer console [[GH-3623](https://github.com/hashicorp/nomad/issues/3623)]

BUG FIXES:

 * core: Fix issue in which restoring periodic jobs could fail when a leader
   election occurs [[GH-3646](https://github.com/hashicorp/nomad/issues/3646)]
 * core: Fix race condition in which rapid reprocessing of a blocked evaluation
   may lead to the scheduler not seeing the results of the previous scheduling
   event [[GH-3669](https://github.com/hashicorp/nomad/issues/3669)]
 * core: Fixed an issue where the leader server could get into a state where it
   was no longer performing the periodic leader loop duties after a barrier
   timeout error [[GH-3402](https://github.com/hashicorp/nomad/issues/3402)]
 * core: Fixes an issue with jobs that have `auto_revert` set to true, where
   reverting to a previously stable job that fails to start up causes an
   infinite cycle of reverts [[GH-3496](https://github.com/hashicorp/nomad/issues/3496)]
 * api: Apply correct memory default when task's do not specify memory
   explicitly [[GH-3520](https://github.com/hashicorp/nomad/issues/3520)]
 * cli: Fix passing Consul address via flags [[GH-3504](https://github.com/hashicorp/nomad/issues/3504)]
 * cli: Fix panic when running `keyring` commands [[GH-3509](https://github.com/hashicorp/nomad/issues/3509)]
 * client: Fix advertising services with tags that require URL escaping
   [[GH-3632](https://github.com/hashicorp/nomad/issues/3632)]
 * client: Fix a panic when restoring an allocation with a dead leader task
   [[GH-3502](https://github.com/hashicorp/nomad/issues/3502)]
 * client: Fix crash when following logs from a Windows node [[GH-3608](https://github.com/hashicorp/nomad/issues/3608)]
 * client: Fix service/check updating when just interpolated variables change
   [[GH-3619](https://github.com/hashicorp/nomad/issues/3619)]
 * client: Fix allocation accounting in GC and trigger GCs on allocation
   updates [[GH-3445](https://github.com/hashicorp/nomad/issues/3445)]
 * driver/docker: Fix container name conflict handling [[GH-3551](https://github.com/hashicorp/nomad/issues/3551)]
 * driver/rkt: Remove pods on shutdown [[GH-3562](https://github.com/hashicorp/nomad/issues/3562)]
 * driver/rkt: Don't require port maps when using host networking [[GH-3615](https://github.com/hashicorp/nomad/issues/3615)]
 * template: Fix issue where multiple environment variable templates would be
   parsed incorrectly when contents of one have changed after the initial
   rendering [[GH-3529](https://github.com/hashicorp/nomad/issues/3529)]
 * sentinel: (Nomad Enterprise) Fix an issue that could cause an import error
   when multiple Sentinel policies are applied
 * telemetry: Do not emit metrics for non-running tasks [[GH-3559](https://github.com/hashicorp/nomad/issues/3559)]
 * telemetry: Emit hostname as a tag rather than within the key name [[GH-3616](https://github.com/hashicorp/nomad/issues/3616)]
 * ui: Remove timezone text from timestamps [[GH-3621](https://github.com/hashicorp/nomad/issues/3621)]
 * ui: Allow cross-origin requests from the UI [[GH-3530](https://github.com/hashicorp/nomad/issues/3530)]
 * ui: Consistently use Clients instead of Nodes in copy [[GH-3466](https://github.com/hashicorp/nomad/issues/3466)]
 * ui: Fully expand the job definition on the Job Definition page [[GH-3631](https://github.com/hashicorp/nomad/issues/3631)]

## 0.7.0 (November 1, 2017)

__BACKWARDS INCOMPATIBILITIES:__
 * driver/rkt: Nomad now requires at least rkt version `1.27.0` for the rkt
   driver to function. Please update your version of rkt to at least this
   version.

IMPROVEMENTS:
 * core: Capability based ACL system with authoritative region, providing
   federated ACLs.
 * core/enterprise: Sentinel integration for fine grain policy enforcement.
 * core/enterprise: Namespace support allowing jobs and their associated
   objects to be isolated from each other and other users of the cluster.
 * api: Allow force deregistration of a node [[GH-3447](https://github.com/hashicorp/nomad/issues/3447)]
 * api: New `/v1/agent/health` endpoint for health checks.
 * api: Metrics endpoint exposes Prometheus formatted metrics [[GH-3171](https://github.com/hashicorp/nomad/issues/3171)]
 * cli: Consul config option flags for nomad agent command [[GH-3327](https://github.com/hashicorp/nomad/issues/3327)]
 * discovery: Allow restarting unhealthy tasks with `check_restart` [[GH-3105](https://github.com/hashicorp/nomad/issues/3105)]
 * driver/rkt: Enable rkt driver to use address_mode = 'driver' [[GH-3256](https://github.com/hashicorp/nomad/issues/3256)]
 * telemetry: Add support for tagged metrics for Nomad clients [[GH-3147](https://github.com/hashicorp/nomad/issues/3147)]
 * telemetry: Add basic Prometheus configuration for a Nomad cluster [[GH-3186](https://github.com/hashicorp/nomad/issues/3186)]

BUG FIXES:
 * core: Fix restoration of stopped periodic jobs [[GH-3201](https://github.com/hashicorp/nomad/issues/3201)]
 * core: Run deployment garbage collector on an interval [[GH-3267](https://github.com/hashicorp/nomad/issues/3267)]
 * core: Fix parameterized jobs occasionally showing status dead incorrectly
   [[GH-3460](https://github.com/hashicorp/nomad/issues/3460)]
 * core: Fix issue in which job versions above a threshold potentially wouldn't
   be stored [[GH-3372](https://github.com/hashicorp/nomad/issues/3372)]
 * core: Fix issue where node-drain with complete batch allocation would create
   replacement [[GH-3217](https://github.com/hashicorp/nomad/issues/3217)]
 * core: Allow batch jobs that have been purged to be rerun without a job
   specification change [[GH-3375](https://github.com/hashicorp/nomad/issues/3375)]
 * core: Fix issue in which batch allocations from previous job versions may not
   have been stopped properly. [[GH-3217](https://github.com/hashicorp/nomad/issues/3217)]
 * core: Fix issue in which allocations with the same name during a scale
   down/stop event wouldn't be properly stopped [[GH-3217](https://github.com/hashicorp/nomad/issues/3217)]
 * core: Fix a race condition in which scheduling results from one invocation of
   the scheduler wouldn't be considered by the next for the same job [[GH-3206](https://github.com/hashicorp/nomad/issues/3206)]
 * api: Sort /v1/agent/servers output so that output of Consul checks does not
   change [[GH-3214](https://github.com/hashicorp/nomad/issues/3214)]
 * api: Fix search handling of jobs with more than four hyphens and case were
   length could cause lookup error [[GH-3203](https://github.com/hashicorp/nomad/issues/3203)]
 * client: Improve the speed at which clients detect garbage collection events [[GH-3452](https://github.com/hashicorp/nomad/issues/3452)]
 * client: Fix lock contention that could cause a node to miss a heartbeat and
   be marked as down [[GH-3195](https://github.com/hashicorp/nomad/issues/3195)]
 * client: Fix data race that could lead to concurrent map read/writes during
   heartbeating and fingerprinting [[GH-3461](https://github.com/hashicorp/nomad/issues/3461)]
 * driver/docker: Fix docker user specified syslogging [[GH-3184](https://github.com/hashicorp/nomad/issues/3184)]
 * driver/docker: Fix issue where CPU usage statistics were artificially high
   [[GH-3229](https://github.com/hashicorp/nomad/issues/3229)]
 * client/template: Fix issue in which secrets would be renewed too aggressively
   [[GH-3360](https://github.com/hashicorp/nomad/issues/3360)]

## 0.6.3 (September 11, 2017)

BUG FIXES:
 * api: Search handles prefix longer than allowed UUIDs [[GH-3138](https://github.com/hashicorp/nomad/issues/3138)]
 * api: Search endpoint handles even UUID prefixes with hyphens [[GH-3120](https://github.com/hashicorp/nomad/issues/3120)]
 * api: Don't merge empty update stanza from job into task groups [[GH-3139](https://github.com/hashicorp/nomad/issues/3139)]
 * cli: Sort task groups when displaying a deployment [[GH-3137](https://github.com/hashicorp/nomad/issues/3137)]
 * cli: Handle reading files that are in a symlinked directory [[GH-3164](https://github.com/hashicorp/nomad/issues/3164)]
 * cli: All status commands handle even UUID prefixes with hyphens [[GH-3122](https://github.com/hashicorp/nomad/issues/3122)]
 * cli: Fix autocompletion of paths that include directories on zsh [[GH-3129](https://github.com/hashicorp/nomad/issues/3129)]
 * cli: Fix job deployment -latest handling of jobs without deployments
   [[GH-3166](https://github.com/hashicorp/nomad/issues/3166)]
 * cli: Hide CLI commands not expected to be run by user from autocomplete
   suggestions [[GH-3177](https://github.com/hashicorp/nomad/issues/3177)]
 * cli: Status command honors exact job match even when it is the prefix of
   another job [[GH-3120](https://github.com/hashicorp/nomad/issues/3120)]
 * cli: Fix setting of TLSServerName for node API Client. This fixes an issue of
   contacting nodes that are using TLS [[GH-3127](https://github.com/hashicorp/nomad/issues/3127)]
 * client/template: Fix issue in which the template block could cause high load
   on Vault when secret lease duration was less than the Vault grace [[GH-3153](https://github.com/hashicorp/nomad/issues/3153)]
 * driver/docker: Always purge stopped containers [[GH-3148](https://github.com/hashicorp/nomad/issues/3148)]
 * driver/docker: Fix MemorySwappiness on Windows [[GH-3187](https://github.com/hashicorp/nomad/issues/3187)]
 * driver/docker: Fix issue in which mounts could parse incorrectly [[GH-3163](https://github.com/hashicorp/nomad/issues/3163)]
 * driver/docker: Fix issue where potentially incorrect syslog server address is
   used [[GH-3135](https://github.com/hashicorp/nomad/issues/3135)]
 * driver/docker: Fix server url passed to credential helpers and properly
   capture error output [[GH-3165](https://github.com/hashicorp/nomad/issues/3165)]
 * jobspec: Allow distinct_host constraint to have L/RTarget set [[GH-3136](https://github.com/hashicorp/nomad/issues/3136)]

## 0.6.2 (August 28, 2017)

BUG FIXES:
 * api/cli: Fix logs and fs api and command [[GH-3116](https://github.com/hashicorp/nomad/issues/3116)]

## 0.6.1 (August 28, 2017)

__BACKWARDS INCOMPATIBILITIES:__
 * deployment: Specifying an update stanza with a max_parallel of zero is now
   a validation error. Please update the stanza to be greater than zero or
   remove the stanza as a zero parallelism update is not valid.

IMPROVEMENTS:
 * core: Lost allocations replaced even if part of failed deployment [[GH-2961](https://github.com/hashicorp/nomad/issues/2961)]
 * core: Add autocomplete functionality for resources: allocations, evaluations,
   jobs, deployments and nodes [[GH-2964](https://github.com/hashicorp/nomad/issues/2964)]
 * core: `distinct_property` constraint can set the number of allocations that
   are allowed to share a property value [[GH-2942](https://github.com/hashicorp/nomad/issues/2942)]
 * core: Placing allocation counts towards placement limit fixing issue where
   rolling update could remove an unnecessary amount of allocations [[GH-3070](https://github.com/hashicorp/nomad/issues/3070)]
 * api: Redact Vault.Token from AgentSelf response [[GH-2988](https://github.com/hashicorp/nomad/issues/2988)]
 * cli: node-status displays node version [[GH-3002](https://github.com/hashicorp/nomad/issues/3002)]
 * cli: Disable color output when STDOUT is not a TTY [[GH-3057](https://github.com/hashicorp/nomad/issues/3057)]
 * cli: Add autocomplete functionality for flags for all CLI command [GH 3087]
 * cli: Add status command which takes any identifier and routes to the
   appropriate status command.
 * client: Unmount task directories when alloc is terminal [[GH-3006](https://github.com/hashicorp/nomad/issues/3006)]
 * client/template: Allow template to set Vault grace [[GH-2947](https://github.com/hashicorp/nomad/issues/2947)]
 * client/template: Template emits events explaining why it is blocked [[GH-3001](https://github.com/hashicorp/nomad/issues/3001)]
 * deployment: Disallow max_parallel of zero [[GH-3081](https://github.com/hashicorp/nomad/issues/3081)]
 * deployment: Emit task events explaining unhealthy allocations [[GH-3025](https://github.com/hashicorp/nomad/issues/3025)]
 * deployment: Better description when a deployment should auto-revert but there
   is no target [[GH-3024](https://github.com/hashicorp/nomad/issues/3024)]
 * discovery: Add HTTP header and method support to checks [[GH-3031](https://github.com/hashicorp/nomad/issues/3031)]
 * driver/docker: Added DNS options [[GH-2992](https://github.com/hashicorp/nomad/issues/2992)]
 * driver/docker: Add mount options for volumes [[GH-3021](https://github.com/hashicorp/nomad/issues/3021)]
 * driver/docker: Allow retry of 500 API errors to be handled by restart
   policies when starting a container [[GH-3073](https://github.com/hashicorp/nomad/issues/3073)]
 * driver/rkt: support read-only volume mounts [[GH-2883](https://github.com/hashicorp/nomad/issues/2883)]
 * jobspec: Add `shutdown_delay` so tasks can delay shutdown after deregistering
   from Consul [[GH-3043](https://github.com/hashicorp/nomad/issues/3043)]

BUG FIXES:
 * core: Fix purging of job versions [[GH-3056](https://github.com/hashicorp/nomad/issues/3056)]
 * core: Fix race creating EvalFuture [[GH-3051](https://github.com/hashicorp/nomad/issues/3051)]
 * core: Fix panic occurring from improper bitmap size [[GH-3023](https://github.com/hashicorp/nomad/issues/3023)]
 * core: Fix restoration of parameterized, periodic jobs [[GH-2959](https://github.com/hashicorp/nomad/issues/2959)]
 * core: Fix incorrect destructive update with `distinct_property` constraint
   [[GH-2939](https://github.com/hashicorp/nomad/issues/2939)]
 * cli: Fix autocompleting global flags [[GH-2928](https://github.com/hashicorp/nomad/issues/2928)]
 * cli: Fix panic when using 0.6.0 cli with an older cluster [[GH-2929](https://github.com/hashicorp/nomad/issues/2929)]
 * cli: Fix TLS handling for alloc stats API calls [[GH-3108](https://github.com/hashicorp/nomad/issues/3108)]
 * client: Fix `LC_ALL=C` being set on subprocesses [[GH-3041](https://github.com/hashicorp/nomad/issues/3041)]
 * client/networking: Handle interfaces that only have link-local addresses
   while preferring globally routable addresses [[GH-3089](https://github.com/hashicorp/nomad/issues/3089)]
 * deployment: Fix alloc health with services/checks using interpolation
   [[GH-2984](https://github.com/hashicorp/nomad/issues/2984)]
 * discovery: Fix timeout validation for script checks [[GH-3022](https://github.com/hashicorp/nomad/issues/3022)]
 * driver/docker: Fix leaking plugin file used by syslog server [[GH-2937](https://github.com/hashicorp/nomad/issues/2937)]

## 0.6.0 (July 26, 2017)

__BACKWARDS INCOMPATIBILITIES:__
 * cli: When given a prefix that does not resolve to a particular object,
   commands now return exit code 1 rather than 0.

IMPROVEMENTS:
 * core: Rolling updates based on allocation health [GH-2621, GH-2634, GH-2799]
 * core: New deployment object to track job updates [GH-2621, GH-2634, GH-2799]
 * core: Default advertise to private IP address if bind is 0.0.0.0 [[GH-2399](https://github.com/hashicorp/nomad/issues/2399)]
 * core: Track multiple job versions and add a stopped state for jobs [[GH-2566](https://github.com/hashicorp/nomad/issues/2566)]
 * core: Job updates can create canaries before beginning rolling update
   [GH-2621, GH-2634, GH-2799]
 * core: Back-pressure when evaluations are nacked and ensure scheduling
   progress on evaluation failures [[GH-2555](https://github.com/hashicorp/nomad/issues/2555)]
 * agent/config: Late binding to IP addresses using go-sockaddr/template syntax
   [[GH-2399](https://github.com/hashicorp/nomad/issues/2399)]
 * api: Add `verify_https_client` to require certificates from HTTP clients
   [[GH-2587](https://github.com/hashicorp/nomad/issues/2587)]
 * api/job: Ability to revert job to older versions [[GH-2575](https://github.com/hashicorp/nomad/issues/2575)]
 * cli: Autocomplete for CLI commands [[GH-2848](https://github.com/hashicorp/nomad/issues/2848)]
 * client: Use a random host UUID by default [[GH-2735](https://github.com/hashicorp/nomad/issues/2735)]
 * client: Add `NOMAD_GROUP_NAME` environment variable [[GH-2877](https://github.com/hashicorp/nomad/issues/2877)]
 * client: Environment variables for client DC and Region [[GH-2507](https://github.com/hashicorp/nomad/issues/2507)]
 * client: Hash host ID so its stable and well distributed [[GH-2541](https://github.com/hashicorp/nomad/issues/2541)]
 * client: GC dead allocs if total allocs > `gc_max_allocs` tunable [[GH-2636](https://github.com/hashicorp/nomad/issues/2636)]
 * client: Persist state using bolt-db and more efficient write patterns
   [[GH-2610](https://github.com/hashicorp/nomad/issues/2610)]
 * client: Fingerprint all routable addresses on an interface including IPv6
   addresses [[GH-2536](https://github.com/hashicorp/nomad/issues/2536)]
 * client/artifact: Support .xz archives [[GH-2836](https://github.com/hashicorp/nomad/issues/2836)]
 * client/artifact: Allow specifying a go-getter mode [[GH-2781](https://github.com/hashicorp/nomad/issues/2781)]
 * client/artifact: Support non-Amazon S3-compatible sources [[GH-2781](https://github.com/hashicorp/nomad/issues/2781)]
 * client/template: Support reading env vars from templates [[GH-2654](https://github.com/hashicorp/nomad/issues/2654)]
 * config: Support Unix socket addresses for Consul [[GH-2622](https://github.com/hashicorp/nomad/issues/2622)]
 * discovery: Advertise driver-specified IP address and port [[GH-2709](https://github.com/hashicorp/nomad/issues/2709)]
 * discovery: Support `tls_skip_verify` for Consul HTTPS checks [[GH-2467](https://github.com/hashicorp/nomad/issues/2467)]
 * driver/docker: Allow specifying extra hosts [[GH-2547](https://github.com/hashicorp/nomad/issues/2547)]
 * driver/docker: Allow setting seccomp profiles [[GH-2658](https://github.com/hashicorp/nomad/issues/2658)]
 * driver/docker: Support Docker credential helpers [[GH-2651](https://github.com/hashicorp/nomad/issues/2651)]
 * driver/docker: Auth failures can optionally be ignored [[GH-2786](https://github.com/hashicorp/nomad/issues/2786)]
 * driver/docker: Add `driver.docker.bridge_ip` node attribute [[GH-2797](https://github.com/hashicorp/nomad/issues/2797)]
 * driver/docker: Allow setting container IP with user defined networks
   [[GH-2535](https://github.com/hashicorp/nomad/issues/2535)]
 * driver/rkt: Support `no_overlay` [[GH-2702](https://github.com/hashicorp/nomad/issues/2702)]
 * driver/rkt: Support `insecure_options` list [[GH-2695](https://github.com/hashicorp/nomad/issues/2695)]
 * server: Allow tuning of node heartbeat TTLs [[GH-2859](https://github.com/hashicorp/nomad/issues/2859)]
 * server/networking: Shrink dynamic port range to not overlap with majority of
   operating system's ephemeral port ranges to avoid port conflicts [[GH-2856](https://github.com/hashicorp/nomad/issues/2856)]

BUG FIXES:
 * core: Protect against nil job in new allocation, avoiding panic [[GH-2592](https://github.com/hashicorp/nomad/issues/2592)]
 * core: System jobs should be running until explicitly stopped [[GH-2750](https://github.com/hashicorp/nomad/issues/2750)]
 * core: Prevent invalid job updates (eg service -> batch) [[GH-2746](https://github.com/hashicorp/nomad/issues/2746)]
 * client: Lookup `ip` utility on `$PATH` [[GH-2729](https://github.com/hashicorp/nomad/issues/2729)]
 * client: Add sticky bit to temp directory [[GH-2519](https://github.com/hashicorp/nomad/issues/2519)]
 * client: Shutdown task group leader before other tasks [[GH-2753](https://github.com/hashicorp/nomad/issues/2753)]
 * client: Include symlinks in snapshots when migrating disks [[GH-2687](https://github.com/hashicorp/nomad/issues/2687)]
 * client: Regression for allocation directory unix perms introduced in v0.5.6
   fixed [[GH-2675](https://github.com/hashicorp/nomad/issues/2675)]
 * client: Client syncs allocation state with server before waiting for
   allocation destroy fixing a corner case in which an allocation may be blocked
   till destroy [[GH-2563](https://github.com/hashicorp/nomad/issues/2563)]
 * client: Improved state file handling and reduced write volume [[GH-2878](https://github.com/hashicorp/nomad/issues/2878)]
 * client/artifact: Honor netrc [[GH-2524](https://github.com/hashicorp/nomad/issues/2524)]
 * client/artifact: Handle tars where file in directory is listed before
   directory [[GH-2524](https://github.com/hashicorp/nomad/issues/2524)]
 * client/config: Use `cpu_total_compute` whenever it is set [[GH-2745](https://github.com/hashicorp/nomad/issues/2745)]
 * client/config: Respect `vault.tls_server_name` setting in consul-template
   [[GH-2793](https://github.com/hashicorp/nomad/issues/2793)]
 * driver/exec: Properly set file/dir ownership in chroots [[GH-2552](https://github.com/hashicorp/nomad/issues/2552)]
 * driver/docker: Fix panic in Docker driver on Windows [[GH-2614](https://github.com/hashicorp/nomad/issues/2614)]
 * driver/rkt: Fix env var interpolation [[GH-2777](https://github.com/hashicorp/nomad/issues/2777)]
 * jobspec/validation: Prevent static port conflicts [[GH-2807](https://github.com/hashicorp/nomad/issues/2807)]
 * server: Reject non-TLS clients when TLS enabled [[GH-2525](https://github.com/hashicorp/nomad/issues/2525)]
 * server: Fix a panic in plan evaluation with partial failures and all_at_once
   set [[GH-2544](https://github.com/hashicorp/nomad/issues/2544)]
 * server/periodic: Restoring periodic jobs takes launch time zone into
   consideration [[GH-2808](https://github.com/hashicorp/nomad/issues/2808)]
 * server/vault: Fix Vault Client panic when given nonexistent role [[GH-2648](https://github.com/hashicorp/nomad/issues/2648)]
 * telemetry: Fix merging of use node name [[GH-2762](https://github.com/hashicorp/nomad/issues/2762)]

## 0.5.6 (March 31, 2017)

IMPROVEMENTS:
  * api: Improve log API error when task doesn't exist or hasn't started
    [[GH-2512](https://github.com/hashicorp/nomad/issues/2512)]
  * client: Improve error message when artifact downloading fails [[GH-2289](https://github.com/hashicorp/nomad/issues/2289)]
  * client: Track task start/finish time [[GH-2512](https://github.com/hashicorp/nomad/issues/2512)]
  * client/template: Access Node meta and attributes in template [[GH-2488](https://github.com/hashicorp/nomad/issues/2488)]

BUG FIXES:
  * core: Fix periodic job state switching to dead incorrectly [[GH-2486](https://github.com/hashicorp/nomad/issues/2486)]
  * core: Fix dispatch of periodic job launching allocations immediately
    [[GH-2489](https://github.com/hashicorp/nomad/issues/2489)]
  * api: Fix TLS in logs and fs commands/APIs [[GH-2290](https://github.com/hashicorp/nomad/issues/2290)]
  * cli/plan: Fix diff alignment and remove no change DC output [[GH-2465](https://github.com/hashicorp/nomad/issues/2465)]
  * client: Fix panic when restarting non-running tasks [[GH-2480](https://github.com/hashicorp/nomad/issues/2480)]
  * client: Fix env vars when multiple tasks and ports present [[GH-2491](https://github.com/hashicorp/nomad/issues/2491)]
  * client: Fix `user` attribute disregarding membership of non-main group
    [[GH-2461](https://github.com/hashicorp/nomad/issues/2461)]
  * client/vault: Stop Vault token renewal on task exit [[GH-2495](https://github.com/hashicorp/nomad/issues/2495)]
  * driver/docker: Proper reference counting through task restarts [[GH-2484](https://github.com/hashicorp/nomad/issues/2484)]

## 0.5.5 (March 14, 2017)

__BACKWARDS INCOMPATIBILITIES:__
  * api: The api package definition of a Job has changed from exposing
    primitives to pointers to primitives to allow defaulting of unset fields.
  * driver/docker: The `load` configuration took an array of paths to images
    prior to this release. A single image is expected by the driver so this
    behavior has been changed to take a single path as a string. Jobs using the
    `load` command should update the syntax to a single string.  [[GH-2361](https://github.com/hashicorp/nomad/issues/2361)]

IMPROVEMENTS:
  * core: Handle Serf Reap event [[GH-2310](https://github.com/hashicorp/nomad/issues/2310)]
  * core: Update Serf and Memberlist for more reliable gossip [[GH-2255](https://github.com/hashicorp/nomad/issues/2255)]
  * api: API defaults missing values [[GH-2300](https://github.com/hashicorp/nomad/issues/2300)]
  * api: Validate the restart policy interval [[GH-2311](https://github.com/hashicorp/nomad/issues/2311)]
  * api: New task event for task environment setup [[GH-2302](https://github.com/hashicorp/nomad/issues/2302)]
  * api/cli: Add nomad operator command and API for interacting with Raft
    configuration [[GH-2305](https://github.com/hashicorp/nomad/issues/2305)]
  * cli: node-status displays enabled drivers on the node [[GH-2349](https://github.com/hashicorp/nomad/issues/2349)]
  * client: Apply GC related configurations properly [[GH-2273](https://github.com/hashicorp/nomad/issues/2273)]
  * client: Don't force uppercase meta keys in env vars [[GH-2338](https://github.com/hashicorp/nomad/issues/2338)]
  * client: Limit parallelism during garbage collection [[GH-2427](https://github.com/hashicorp/nomad/issues/2427)]
  * client: Don't exec `uname -r` for node attribute kernel.version [[GH-2380](https://github.com/hashicorp/nomad/issues/2380)]
  * client: Artifact support for git and hg as well as netrc support [[GH-2386](https://github.com/hashicorp/nomad/issues/2386)]
  * client: Add metrics to show number of allocations on in each state [[GH-2425](https://github.com/hashicorp/nomad/issues/2425)]
  * client: Add `NOMAD_{IP,PORT}_<task>_<label>` environment variables [[GH-2426](https://github.com/hashicorp/nomad/issues/2426)]
  * client: Allow specification of `cpu_total_compute` to override fingerprinter
    [[GH-2447](https://github.com/hashicorp/nomad/issues/2447)]
  * client: Reproducible Node ID on OSes that provide system-level UUID
    [[GH-2277](https://github.com/hashicorp/nomad/issues/2277)]
  * driver/docker: Add support for volume drivers [[GH-2351](https://github.com/hashicorp/nomad/issues/2351)]
  * driver/docker: Docker image coordinator and caching [[GH-2361](https://github.com/hashicorp/nomad/issues/2361)]
  * jobspec: Add leader task to allow graceful shutdown of other tasks within
    the task group [[GH-2308](https://github.com/hashicorp/nomad/issues/2308)]
  * periodic: Allow specification of timezones in Periodic Jobs [[GH-2321](https://github.com/hashicorp/nomad/issues/2321)]
  * scheduler: New `distinct_property` constraint [[GH-2418](https://github.com/hashicorp/nomad/issues/2418)]
  * server: Allow specification of eval/job gc threshold [[GH-2370](https://github.com/hashicorp/nomad/issues/2370)]
  * server/vault: Vault Client on Server handles SIGHUP to reload configs
    [[GH-2270](https://github.com/hashicorp/nomad/issues/2270)]
  * telemetry: Clients report allocated/unallocated resources [[GH-2327](https://github.com/hashicorp/nomad/issues/2327)]
  * template: Allow specification of template delimiters [[GH-2315](https://github.com/hashicorp/nomad/issues/2315)]
  * template: Permissions can be set on template destination file [[GH-2262](https://github.com/hashicorp/nomad/issues/2262)]
  * vault: Server side Vault telemetry [[GH-2318](https://github.com/hashicorp/nomad/issues/2318)]
  * vault: Disallow root policy from being specified [[GH-2309](https://github.com/hashicorp/nomad/issues/2309)]

BUG FIXES:
  * core: Handle periodic parameterized jobs [[GH-2385](https://github.com/hashicorp/nomad/issues/2385)]
  * core: Improve garbage collection of stopped batch jobs [[GH-2432](https://github.com/hashicorp/nomad/issues/2432)]
  * api: Fix escaping of HTML characters [[GH-2322](https://github.com/hashicorp/nomad/issues/2322)]
  * cli: Display disk resources in alloc-status [[GH-2404](https://github.com/hashicorp/nomad/issues/2404)]
  * client: Drivers log during fingerprinting [[GH-2337](https://github.com/hashicorp/nomad/issues/2337)]
  * client: Fix race condition with deriving vault tokens [[GH-2275](https://github.com/hashicorp/nomad/issues/2275)]
  * client: Fix remounting alloc dirs after reboots [[GH-2391](https://github.com/hashicorp/nomad/issues/2391)] [[GH-2394](https://github.com/hashicorp/nomad/issues/2394)]
  * client: Replace `-` with `_` in environment variable names [[GH-2406](https://github.com/hashicorp/nomad/issues/2406)]
  * client: Fix panic and deadlock during client restore state when prestart
    fails [[GH-2376](https://github.com/hashicorp/nomad/issues/2376)]
  * config: Fix Consul Config Merging/Copying [[GH-2278](https://github.com/hashicorp/nomad/issues/2278)]
  * config: Fix Client reserved resource merging panic [[GH-2281](https://github.com/hashicorp/nomad/issues/2281)]
  * server: Fix panic when forwarding Vault derivation requests from non-leader
    servers [[GH-2267](https://github.com/hashicorp/nomad/issues/2267)]

## 0.5.4 (January 31, 2017)

IMPROVEMENTS:
  * client: Made the GC related tunables configurable via client configuration
    [[GH-2261](https://github.com/hashicorp/nomad/issues/2261)]

BUG FIXES:
  * client: Fix panic when upgrading to 0.5.3 [[GH-2256](https://github.com/hashicorp/nomad/issues/2256)]

## 0.5.3 (January 30, 2017)

IMPROVEMENTS:
  * core: Introduce parameterized jobs and dispatch command/API [[GH-2128](https://github.com/hashicorp/nomad/issues/2128)]
  * core: Cancel blocked evals upon successful one for job [[GH-2155](https://github.com/hashicorp/nomad/issues/2155)]
  * api: Added APIs for requesting GC of allocations [[GH-2192](https://github.com/hashicorp/nomad/issues/2192)]
  * api: Job summary endpoint includes summary status for child jobs [[GH-2128](https://github.com/hashicorp/nomad/issues/2128)]
  * api/client: Plain text log streaming suitable for viewing logs in a browser
    [[GH-2235](https://github.com/hashicorp/nomad/issues/2235)]
  * cli: Defaulting to showing allocations which belong to currently registered
    job [[GH-2032](https://github.com/hashicorp/nomad/issues/2032)]
  * client: Garbage collect Allocation Runners to free up disk resources
    [[GH-2081](https://github.com/hashicorp/nomad/issues/2081)]
  * client: Don't retrieve Driver Stats if unsupported [[GH-2173](https://github.com/hashicorp/nomad/issues/2173)]
  * client: Filter log lines in the executor based on client's log level
    [[GH-2172](https://github.com/hashicorp/nomad/issues/2172)]
  * client: Added environment variables to discover addresses of sibling tasks
    in an allocation [[GH-2223](https://github.com/hashicorp/nomad/issues/2223)]
  * discovery: Register service with duplicate names on different ports [[GH-2208](https://github.com/hashicorp/nomad/issues/2208)]
  * driver/docker: Add support for network aliases [[GH-1980](https://github.com/hashicorp/nomad/issues/1980)]
  * driver/docker: Add `force_pull` option to force downloading an image [[GH-2147](https://github.com/hashicorp/nomad/issues/2147)]
  * driver/docker: Retry when image is not found while creating a container
    [[GH-2222](https://github.com/hashicorp/nomad/issues/2222)]
  * driver/java: Support setting class_path and class name. [[GH-2199](https://github.com/hashicorp/nomad/issues/2199)]
  * telemetry: Prefix gauge values with node name instead of hostname [[GH-2098](https://github.com/hashicorp/nomad/issues/2098)]
  * template: The template block supports keyOrDefault [[GH-2209](https://github.com/hashicorp/nomad/issues/2209)]
  * template: The template block can now interpolate Nomad environment variables
    [[GH-2209](https://github.com/hashicorp/nomad/issues/2209)]
  * vault: Improve validation of the Vault token given to Nomad servers
    [[GH-2226](https://github.com/hashicorp/nomad/issues/2226)]
  * vault: Support setting the Vault role to derive tokens from with
    `create_from_role` setting [[GH-2226](https://github.com/hashicorp/nomad/issues/2226)]

BUG FIXES:
  * client: Fixed namespacing for the cpu arch attribute [[GH-2161](https://github.com/hashicorp/nomad/issues/2161)]
  * client: Fix issue where allocations weren't pulled for several minutes. This
    manifested as slow starts, delayed kills, etc [[GH-2177](https://github.com/hashicorp/nomad/issues/2177)]
  * client: Fix a panic that would occur with a racy alloc migration
    cancellation [[GH-2231](https://github.com/hashicorp/nomad/issues/2231)]
  * config: Fix merging of Consul options which caused auto_advertise to be
    ignored [[GH-2159](https://github.com/hashicorp/nomad/issues/2159)]
  * driver: Fix image based drivers (eg docker) having host env vars set [[GH-2211](https://github.com/hashicorp/nomad/issues/2211)]
  * driver/docker: Fix Docker auth/logging interpolation [GH-2063, GH-2130]
  * driver/docker: Fix parsing of Docker Auth Configurations. New parsing is
    in-line with Docker itself. Also log debug message if auth lookup failed
    [[GH-2190](https://github.com/hashicorp/nomad/issues/2190)]
  * template: Fix splay being used as a wait and instead randomize the delay
    from 0 seconds to splay duration [[GH-2227](https://github.com/hashicorp/nomad/issues/2227)]

## 0.5.2 (December 23, 2016)

BUG FIXES:
  * client: Fixed a race condition and remove panic when handling duplicate
    allocations [[GH-2096](https://github.com/hashicorp/nomad/issues/2096)]
  * client: Cancel wait for remote allocation if migration is no longer required
    [[GH-2097](https://github.com/hashicorp/nomad/issues/2097)]
  * client: Failure to stat a single mountpoint does not cause all of host
    resource usage collection to fail [[GH-2090](https://github.com/hashicorp/nomad/issues/2090)]

## 0.5.1 (December 12, 2016)

IMPROVEMENTS:
  * driver/rkt: Support rkt's `--dns=host` and `--dns=none` options [[GH-2028](https://github.com/hashicorp/nomad/issues/2028)]

BUG FIXES:
  * agent/config: Fix use of IPv6 addresses [[GH-2036](https://github.com/hashicorp/nomad/issues/2036)]
  * api: Fix file descriptor leak and high CPU usage when using the logs
    endpoint [[GH-2079](https://github.com/hashicorp/nomad/issues/2079)]
  * cli: Improve parsing error when a job without a name is specified [[GH-2030](https://github.com/hashicorp/nomad/issues/2030)]
  * client: Fixed permissions of migrated allocation directory [[GH-2061](https://github.com/hashicorp/nomad/issues/2061)]
  * client: Ensuring allocations are not blocked more than once [[GH-2040](https://github.com/hashicorp/nomad/issues/2040)]
  * client: Fix race on StreamFramer Destroy which would cause a panic [[GH-2007](https://github.com/hashicorp/nomad/issues/2007)]
  * client: Not migrating allocation directories on the same client if sticky is
    turned off [[GH-2017](https://github.com/hashicorp/nomad/issues/2017)]
  * client/vault: Fix issue in which deriving a Vault token would fail with
    allocation does not exist due to stale queries [[GH-2050](https://github.com/hashicorp/nomad/issues/2050)]
  * driver/docker: Make container exist errors non-retriable by task runner
    [[GH-2033](https://github.com/hashicorp/nomad/issues/2033)]
  * driver/docker: Fixed an issue related to purging containers with same name
    as Nomad is trying to start [[GH-2037](https://github.com/hashicorp/nomad/issues/2037)]
  * driver/rkt: Fix validation of rkt volumes [[GH-2027](https://github.com/hashicorp/nomad/issues/2027)]

## 0.5.0 (November 16, 2016)

__BACKWARDS INCOMPATIBILITIES:__
  * jobspec: Extracted the disk resources from the task to the task group. The
    new block is name `ephemeral_disk`. Nomad will automatically convert
    existing jobs but newly submitted jobs should refactor the disk resource
    [GH-1710, GH-1679]
  * agent/config: `network_speed` is now an override and not a default value. If
    the network link speed is not detected a default value is applied.

IMPROVEMENTS:
  * core: Support for gossip encryption [[GH-1791](https://github.com/hashicorp/nomad/issues/1791)]
  * core: Vault integration to handle secure introduction of tasks [GH-1583,
    GH-1713]
  * core: New `set_contains` constraint to determine if a set contains all
    specified values [[GH-1839](https://github.com/hashicorp/nomad/issues/1839)]
  * core: Scheduler version enforcement disallows different scheduler version
    from making decisions simultaneously [[GH-1872](https://github.com/hashicorp/nomad/issues/1872)]
  * core: Introduce node SecretID which can be used to minimize the available
    surface area of RPCs to malicious Nomad Clients [[GH-1597](https://github.com/hashicorp/nomad/issues/1597)]
  * core: Add `sticky` volumes which inform the scheduler to prefer placing
    updated allocations on the same node and to reuse the `local/` and
    `alloc/data` directory from previous allocation allowing semi-persistent
    data and allow those folders to be synced from a remote node [GH-1654,
    GH-1741]
  * agent: Add DataDog telemetry sync [[GH-1816](https://github.com/hashicorp/nomad/issues/1816)]
  * agent: Allow Consul health checks to use bind address rather than advertise
    [[GH-1866](https://github.com/hashicorp/nomad/issues/1866)]
  * agent/config: Advertise addresses do not need to specify a port [[GH-1902](https://github.com/hashicorp/nomad/issues/1902)]
  * agent/config: Bind address defaults to 0.0.0.0 and Advertise defaults to
    hostname [[GH-1955](https://github.com/hashicorp/nomad/issues/1955)]
  * api: Support TLS for encrypting Raft, RPC and HTTP APIs [[GH-1853](https://github.com/hashicorp/nomad/issues/1853)]
  * api: Implement blocking queries for querying a job's evaluations [[GH-1892](https://github.com/hashicorp/nomad/issues/1892)]
  * cli: `nomad alloc-status` shows allocation creation time [[GH-1623](https://github.com/hashicorp/nomad/issues/1623)]
  * cli: `nomad node-status` shows node metadata in verbose mode [[GH-1841](https://github.com/hashicorp/nomad/issues/1841)]
  * client: Failed RPCs are retried on all servers [[GH-1735](https://github.com/hashicorp/nomad/issues/1735)]
  * client: Fingerprint and driver blacklist support [[GH-1949](https://github.com/hashicorp/nomad/issues/1949)]
  * client: Introduce a `secrets/` directory to tasks where sensitive data can
    be written [[GH-1681](https://github.com/hashicorp/nomad/issues/1681)]
  * client/jobspec: Add support for templates that can render static files,
    dynamic content from Consul and secrets from Vault [[GH-1783](https://github.com/hashicorp/nomad/issues/1783)]
  * driver: Export `NOMAD_JOB_NAME` environment variable [[GH-1804](https://github.com/hashicorp/nomad/issues/1804)]
  * driver/docker: Docker For Mac support [[GH-1806](https://github.com/hashicorp/nomad/issues/1806)]
  * driver/docker: Support Docker volumes [[GH-1767](https://github.com/hashicorp/nomad/issues/1767)]
  * driver/docker: Allow Docker logging to be configured [[GH-1767](https://github.com/hashicorp/nomad/issues/1767)]
  * driver/docker: Add `userns_mode` (`--userns`) support [[GH-1940](https://github.com/hashicorp/nomad/issues/1940)]
  * driver/lxc: Support for LXC containers [[GH-1699](https://github.com/hashicorp/nomad/issues/1699)]
  * driver/rkt: Support network configurations [[GH-1862](https://github.com/hashicorp/nomad/issues/1862)]
  * driver/rkt: Support rkt volumes (rkt >= 1.0.0 required) [[GH-1812](https://github.com/hashicorp/nomad/issues/1812)]
  * server/rpc: Added an RPC endpoint for retrieving server members [[GH-1947](https://github.com/hashicorp/nomad/issues/1947)]

BUG FIXES:
  * core: Fix case where dead nodes were not properly handled by System
    scheduler [[GH-1715](https://github.com/hashicorp/nomad/issues/1715)]
  * agent: Handle the SIGPIPE signal preventing panics on journalctl restarts
    [[GH-1802](https://github.com/hashicorp/nomad/issues/1802)]
  * api: Disallow filesystem APIs to read paths that escape the allocation
    directory [[GH-1786](https://github.com/hashicorp/nomad/issues/1786)]
  * cli: `nomad run` failed to run on Windows [[GH-1690](https://github.com/hashicorp/nomad/issues/1690)]
  * cli: `alloc-status` and `node-status` work without access to task stats
    [[GH-1660](https://github.com/hashicorp/nomad/issues/1660)]
  * cli: `alloc-status` does not query for allocation statistics if node is down
    [[GH-1844](https://github.com/hashicorp/nomad/issues/1844)]
  * client: Prevent race when persisting state file [[GH-1682](https://github.com/hashicorp/nomad/issues/1682)]
  * client: Retry recoverable errors when starting a driver [[GH-1891](https://github.com/hashicorp/nomad/issues/1891)]
  * client: Do not validate the command does not contain spaces [[GH-1974](https://github.com/hashicorp/nomad/issues/1974)]
  * client: Fix old services not getting removed from consul on update [[GH-1668](https://github.com/hashicorp/nomad/issues/1668)]
  * client: Preserve permissions of nested directories while chrooting [[GH-1960](https://github.com/hashicorp/nomad/issues/1960)]
  * client: Folder permissions are dropped even when not running as root [[GH-1888](https://github.com/hashicorp/nomad/issues/1888)]
  * client: Artifact download failures will be retried before failing tasks
    [[GH-1558](https://github.com/hashicorp/nomad/issues/1558)]
  * client: Fix a memory leak in the executor that caused failed allocations
    [[GH-1762](https://github.com/hashicorp/nomad/issues/1762)]
  * client: Fix a crash related to stats publishing when driver hasn't started
    yet [[GH-1723](https://github.com/hashicorp/nomad/issues/1723)]
  * client: Chroot environment is only created once, avoid potential filesystem
    errors [[GH-1753](https://github.com/hashicorp/nomad/issues/1753)]
  * client: Failures to download an artifact are retried according to restart
    policy before failing the allocation [[GH-1653](https://github.com/hashicorp/nomad/issues/1653)]
  * client/executor: Prevent race when updating a job configuration with the
    logger [[GH-1886](https://github.com/hashicorp/nomad/issues/1886)]
  * client/fingerprint: Fix inconsistent CPU MHz fingerprinting [[GH-1366](https://github.com/hashicorp/nomad/issues/1366)]
  * env/aws: Fix an issue with reserved ports causing placement failures
    [[GH-1617](https://github.com/hashicorp/nomad/issues/1617)]
  * discovery: Interpolate all service and check fields [[GH-1966](https://github.com/hashicorp/nomad/issues/1966)]
  * discovery: Fix old services not getting removed from Consul on update
    [[GH-1668](https://github.com/hashicorp/nomad/issues/1668)]
  * discovery: Fix HTTP timeout with Server HTTP health check when there is no
    leader [[GH-1656](https://github.com/hashicorp/nomad/issues/1656)]
  * discovery: Fix client flapping when server is in a different datacenter as
    the client [[GH-1641](https://github.com/hashicorp/nomad/issues/1641)]
  * discovery/jobspec: Validate service name after interpolation [[GH-1852](https://github.com/hashicorp/nomad/issues/1852)]
  * driver/docker: Fix `local/` directory mount into container [[GH-1830](https://github.com/hashicorp/nomad/issues/1830)]
  * driver/docker: Interpolate all string configuration variables [[GH-1965](https://github.com/hashicorp/nomad/issues/1965)]
  * jobspec: Tasks without a resource block no longer fail to validate [[GH-1864](https://github.com/hashicorp/nomad/issues/1864)]
  * jobspec: Update HCL to fix panic in JSON parsing [[GH-1754](https://github.com/hashicorp/nomad/issues/1754)]

## 0.4.1 (August 18, 2016)

__BACKWARDS INCOMPATIBILITIES:__
  * telemetry: Operators will have to explicitly opt-in for Nomad client to
    publish allocation and node metrics

IMPROVEMENTS:
  * core: Allow count 0 on system jobs [[GH-1421](https://github.com/hashicorp/nomad/issues/1421)]
  * core: Summarize the current status of registered jobs. [GH-1383, GH-1517]
  * core: Gracefully handle short lived outages by holding RPC calls [[GH-1403](https://github.com/hashicorp/nomad/issues/1403)]
  * core: Introduce a lost state for allocations that were on Nodes that died
    [[GH-1516](https://github.com/hashicorp/nomad/issues/1516)]
  * api: client Logs endpoint for streaming task logs [[GH-1444](https://github.com/hashicorp/nomad/issues/1444)]
  * api/cli: Support for tailing/streaming files [GH-1404, GH-1420]
  * api/server: Support for querying job summaries [[GH-1455](https://github.com/hashicorp/nomad/issues/1455)]
  * cli: `nomad logs` command for streaming task logs [[GH-1444](https://github.com/hashicorp/nomad/issues/1444)]
  * cli: `nomad status` shows the create time of allocations [[GH-1540](https://github.com/hashicorp/nomad/issues/1540)]
  * cli: `nomad plan` exit code indicates if changes will occur [[GH-1502](https://github.com/hashicorp/nomad/issues/1502)]
  * cli: status commands support JSON output and go template formating [[GH-1503](https://github.com/hashicorp/nomad/issues/1503)]
  * cli: Validate and plan command supports reading from stdin [GH-1460,
    GH-1458]
  * cli: Allow basic authentication through address and environment variable
    [[GH-1610](https://github.com/hashicorp/nomad/issues/1610)]
  * cli: `nomad node-status` shows volume name for non-physical volumes instead
    of showing 0B used [[GH-1538](https://github.com/hashicorp/nomad/issues/1538)]
  * cli: Support retrieving job files using go-getter in the `run`, `plan` and
    `validate` command [[GH-1511](https://github.com/hashicorp/nomad/issues/1511)]
  * client: Add killing event to task state [[GH-1457](https://github.com/hashicorp/nomad/issues/1457)]
  * client: Fingerprint network speed on Windows [[GH-1443](https://github.com/hashicorp/nomad/issues/1443)]
  * discovery: Support for initial check status [[GH-1599](https://github.com/hashicorp/nomad/issues/1599)]
  * discovery: Support for query params in health check urls [[GH-1562](https://github.com/hashicorp/nomad/issues/1562)]
  * driver/docker: Allow working directory to be configured [[GH-1513](https://github.com/hashicorp/nomad/issues/1513)]
  * driver/docker: Remove docker volumes when removing container [[GH-1519](https://github.com/hashicorp/nomad/issues/1519)]
  * driver/docker: Set windows containers network mode to nat by default
    [[GH-1521](https://github.com/hashicorp/nomad/issues/1521)]
  * driver/exec: Allow chroot environment to be configurable [[GH-1518](https://github.com/hashicorp/nomad/issues/1518)]
  * driver/qemu: Allows users to pass extra args to the qemu driver [[GH-1596](https://github.com/hashicorp/nomad/issues/1596)]
  * telemetry: Circonus integration for telemetry metrics [[GH-1459](https://github.com/hashicorp/nomad/issues/1459)]
  * telemetry: Allow operators to opt-in for publishing metrics [[GH-1501](https://github.com/hashicorp/nomad/issues/1501)]

BUG FIXES:
  * agent: Reload agent configuration on SIGHUP [[GH-1566](https://github.com/hashicorp/nomad/issues/1566)]
  * core: Sanitize empty slices/maps in jobs to avoid incorrect create/destroy
    updates [[GH-1434](https://github.com/hashicorp/nomad/issues/1434)]
  * core: Fix race in which a Node registers and doesn't receive system jobs
    [[GH-1456](https://github.com/hashicorp/nomad/issues/1456)]
  * core: Fix issue in which Nodes with large amount of reserved ports would
    cause dynamic port allocations to fail [[GH-1526](https://github.com/hashicorp/nomad/issues/1526)]
  * core: Fix a condition in which old batch allocations could get updated even
    after terminal. In a rare case this could cause a server panic [[GH-1471](https://github.com/hashicorp/nomad/issues/1471)]
  * core: Do not update the Job attached to Allocations that have been marked
    terminal [[GH-1508](https://github.com/hashicorp/nomad/issues/1508)]
  * agent: Fix advertise address when using IPv6 [[GH-1465](https://github.com/hashicorp/nomad/issues/1465)]
  * cli: Fix node-status when using IPv6 advertise address [[GH-1465](https://github.com/hashicorp/nomad/issues/1465)]
  * client: Merging telemetry configuration properly [[GH-1670](https://github.com/hashicorp/nomad/issues/1670)]
  * client: Task start errors adhere to restart policy mode [[GH-1405](https://github.com/hashicorp/nomad/issues/1405)]
  * client: Reregister with servers if node is unregistered [[GH-1593](https://github.com/hashicorp/nomad/issues/1593)]
  * client: Killing an allocation doesn't cause allocation stats to block
    [[GH-1454](https://github.com/hashicorp/nomad/issues/1454)]
  * driver/docker: Disable swap on docker driver [[GH-1480](https://github.com/hashicorp/nomad/issues/1480)]
  * driver/docker: Fix improper gating on privileged mode [[GH-1506](https://github.com/hashicorp/nomad/issues/1506)]
  * driver/docker: Default network type is "nat" on Windows [[GH-1521](https://github.com/hashicorp/nomad/issues/1521)]
  * driver/docker: Cleanup created volume when destroying container [[GH-1519](https://github.com/hashicorp/nomad/issues/1519)]
  * driver/rkt: Set host environment variables [[GH-1581](https://github.com/hashicorp/nomad/issues/1581)]
  * driver/rkt: Validate the command and trust_prefix configs [[GH-1493](https://github.com/hashicorp/nomad/issues/1493)]
  * plan: Plan on system jobs discounts nodes that do not meet required
    constraints [[GH-1568](https://github.com/hashicorp/nomad/issues/1568)]

## 0.4.0 (June 28, 2016)

__BACKWARDS INCOMPATIBILITIES:__
  * api: Tasks are no longer allowed to have slashes in their name [[GH-1210](https://github.com/hashicorp/nomad/issues/1210)]
  * cli: Remove the eval-monitor command. Users should switch to `nomad
    eval-status -monitor`.
  * config: Consul configuration has been moved from client options map to
    consul block under client configuration
  * driver/docker: Enabled SSL by default for pulling images from docker
    registries. [[GH-1336](https://github.com/hashicorp/nomad/issues/1336)]

IMPROVEMENTS:
  * core: Scheduler reuses blocked evaluations to avoid unbounded creation of
    evaluations under high contention [[GH-1199](https://github.com/hashicorp/nomad/issues/1199)]
  * core: Scheduler stores placement failures in evaluations, no longer
    generating failed allocations for debug information [[GH-1188](https://github.com/hashicorp/nomad/issues/1188)]
  * api: Faster JSON response encoding [[GH-1182](https://github.com/hashicorp/nomad/issues/1182)]
  * api: Gzip compress HTTP API requests [[GH-1203](https://github.com/hashicorp/nomad/issues/1203)]
  * api: Plan api introduced for the Job endpoint [[GH-1168](https://github.com/hashicorp/nomad/issues/1168)]
  * api: Job endpoint can enforce Job Modify Index to ensure job is being
    modified from a known state [[GH-1243](https://github.com/hashicorp/nomad/issues/1243)]
  * api/client: Add resource usage APIs for retrieving tasks/allocations/host
    resource usage [[GH-1189](https://github.com/hashicorp/nomad/issues/1189)]
  * cli: Faster when displaying large amounts outputs [[GH-1362](https://github.com/hashicorp/nomad/issues/1362)]
  * cli: Deprecate `eval-monitor` and introduce `eval-status` [[GH-1206](https://github.com/hashicorp/nomad/issues/1206)]
  * cli: Unify the `fs` family of commands to be a single command [[GH-1150](https://github.com/hashicorp/nomad/issues/1150)]
  * cli: Introduce `nomad plan` to dry-run a job through the scheduler and
    determine its effects [[GH-1181](https://github.com/hashicorp/nomad/issues/1181)]
  * cli: node-status command displays host resource usage and allocation
    resources [[GH-1261](https://github.com/hashicorp/nomad/issues/1261)]
  * cli: Region flag and environment variable introduced to set region
    forwarding. Automatic region forwarding for run and plan [[GH-1237](https://github.com/hashicorp/nomad/issues/1237)]
  * client: If Consul is available, automatically bootstrap Nomad Client
    using the `_nomad` service in Consul. Nomad Servers now register
    themselves with Consul to make this possible. [[GH-1201](https://github.com/hashicorp/nomad/issues/1201)]
  * drivers: Qemu and Java can be run without an artifact being download. Useful
    if the artifact exists inside a chrooted directory [[GH-1262](https://github.com/hashicorp/nomad/issues/1262)]
  * driver/docker: Added a client options to set SELinux labels for container
    bind mounts. [[GH-788](https://github.com/hashicorp/nomad/issues/788)]
  * driver/docker: Enabled SSL by default for pulling images from docker
    registries. [[GH-1336](https://github.com/hashicorp/nomad/issues/1336)]
  * server: If Consul is available, automatically bootstrap Nomad Servers
    using the `_nomad` service in Consul.  [[GH-1276](https://github.com/hashicorp/nomad/issues/1276)]

BUG FIXES:
  * core: Improve garbage collection of allocations and nodes [[GH-1256](https://github.com/hashicorp/nomad/issues/1256)]
  * core: Fix a potential deadlock if establishing leadership fails and is
    retried [[GH-1231](https://github.com/hashicorp/nomad/issues/1231)]
  * core: Do not restart successful batch jobs when the node is removed/drained
    [[GH-1205](https://github.com/hashicorp/nomad/issues/1205)]
  * core: Fix an issue in which the scheduler could be invoked with insufficient
    state [[GH-1339](https://github.com/hashicorp/nomad/issues/1339)]
  * core: Updated User, Meta or Resources in a task cause create/destroy updates
    [GH-1128, GH-1153]
  * core: Fix blocked evaluations being run without properly accounting for
    priority [[GH-1183](https://github.com/hashicorp/nomad/issues/1183)]
  * api: Tasks are no longer allowed to have slashes in their name [[GH-1210](https://github.com/hashicorp/nomad/issues/1210)]
  * client: Delete tmp files used to communicate with executor [[GH-1241](https://github.com/hashicorp/nomad/issues/1241)]
  * client: Prevent the client from restoring with incorrect task state [[GH-1294](https://github.com/hashicorp/nomad/issues/1294)]
  * discovery: Ensure service and check names are unique [GH-1143, GH-1144]
  * driver/docker: Ensure docker client doesn't time out after a minute.
    [[GH-1184](https://github.com/hashicorp/nomad/issues/1184)]
  * driver/java: Fix issue in which Java on darwin attempted to chroot [[GH-1262](https://github.com/hashicorp/nomad/issues/1262)]
  * driver/docker: Fix issue in which logs could be spliced [[GH-1322](https://github.com/hashicorp/nomad/issues/1322)]

## 0.3.2 (April 22, 2016)

IMPROVEMENTS:
  * core: Garbage collection partitioned to avoid system delays [[GH-1012](https://github.com/hashicorp/nomad/issues/1012)]
  * core: Allow count zero task groups to enable blue/green deploys [[GH-931](https://github.com/hashicorp/nomad/issues/931)]
  * core: Validate driver configurations when submitting jobs [GH-1062, GH-1089]
  * core: Job Deregister forces an evaluation for the job even if it doesn't
    exist [[GH-981](https://github.com/hashicorp/nomad/issues/981)]
  * core: Rename successfully finished allocations to "Complete" rather than
    "Dead" for clarity [[GH-975](https://github.com/hashicorp/nomad/issues/975)]
  * cli: `alloc-status` explains restart decisions [[GH-984](https://github.com/hashicorp/nomad/issues/984)]
  * cli: `node-drain -self` drains the local node [[GH-1068](https://github.com/hashicorp/nomad/issues/1068)]
  * cli: `node-status -self` queries the local node [[GH-1004](https://github.com/hashicorp/nomad/issues/1004)]
  * cli: Destructive commands now require confirmation [[GH-983](https://github.com/hashicorp/nomad/issues/983)]
  * cli: `alloc-status` display is less verbose by default [[GH-946](https://github.com/hashicorp/nomad/issues/946)]
  * cli: `server-members` displays the current leader in each region [[GH-935](https://github.com/hashicorp/nomad/issues/935)]
  * cli: `run` has an `-output` flag to emit a JSON version of the job [[GH-990](https://github.com/hashicorp/nomad/issues/990)]
  * cli: New `inspect` command to display a submitted job's specification
    [[GH-952](https://github.com/hashicorp/nomad/issues/952)]
  * cli: `node-status` display is less verbose by default and shows a node's
    total resources [[GH-946](https://github.com/hashicorp/nomad/issues/946)]
  * client: `artifact` source can be interpreted [[GH-1070](https://github.com/hashicorp/nomad/issues/1070)]
  * client: Add IP and Port environment variables [[GH-1099](https://github.com/hashicorp/nomad/issues/1099)]
  * client: Nomad fingerprinter to detect client's version [[GH-965](https://github.com/hashicorp/nomad/issues/965)]
  * client: Tasks can interpret Meta set in the task group and job [[GH-985](https://github.com/hashicorp/nomad/issues/985)]
  * client: All tasks in a task group are killed when a task fails [[GH-962](https://github.com/hashicorp/nomad/issues/962)]
  * client: Pass environment variables from host to exec based tasks [[GH-970](https://github.com/hashicorp/nomad/issues/970)]
  * client: Allow task's to be run as particular user [GH-950, GH-978]
  * client: `artifact` block now supports downloading paths relative to the
    task's directory [[GH-944](https://github.com/hashicorp/nomad/issues/944)]
  * docker: Timeout communications with Docker Daemon to avoid deadlocks with
    misbehaving Docker Daemon [[GH-1117](https://github.com/hashicorp/nomad/issues/1117)]
  * discovery: Support script based health checks [[GH-986](https://github.com/hashicorp/nomad/issues/986)]
  * discovery: Allowing registration of services which don't expose ports
    [[GH-1092](https://github.com/hashicorp/nomad/issues/1092)]
  * driver/docker: Support for `tty` and `interactive` options [[GH-1059](https://github.com/hashicorp/nomad/issues/1059)]
  * jobspec: Improved validation of services referencing port labels [[GH-1097](https://github.com/hashicorp/nomad/issues/1097)]
  * periodic: Periodic jobs are always evaluated in UTC timezone [[GH-1074](https://github.com/hashicorp/nomad/issues/1074)]

BUG FIXES:
  * core: Prevent garbage collection of running batch jobs [[GH-989](https://github.com/hashicorp/nomad/issues/989)]
  * core: Trigger System scheduler when Node drain is disabled [[GH-1106](https://github.com/hashicorp/nomad/issues/1106)]
  * core: Fix issue where in-place updated allocation double counted resources
    [[GH-957](https://github.com/hashicorp/nomad/issues/957)]
  * core: Fix drained, batched allocations from being migrated indefinitely
    [[GH-1086](https://github.com/hashicorp/nomad/issues/1086)]
  * client: Garbage collect Docker containers on exit [[GH-1071](https://github.com/hashicorp/nomad/issues/1071)]
  * client: Fix common exec failures on CentOS and Amazon Linux [[GH-1009](https://github.com/hashicorp/nomad/issues/1009)]
  * client: Fix S3 artifact downloading with IAM credentials [[GH-1113](https://github.com/hashicorp/nomad/issues/1113)]
  * client: Fix handling of environment variables containing multiple equal
    signs [[GH-1115](https://github.com/hashicorp/nomad/issues/1115)]

## 0.3.1 (March 16, 2016)

__BACKWARDS INCOMPATIBILITIES:__
  * Service names that dont conform to RFC-1123 and RFC-2782 will fail
    validation. To fix, change service name to conform to the RFCs before
    running the job [[GH-915](https://github.com/hashicorp/nomad/issues/915)]
  * Jobs that downloaded artifacts will have to be updated to the new syntax and
    be resubmitted. The new syntax consolidates artifacts to the `task` rather
    than being duplicated inside each driver config [[GH-921](https://github.com/hashicorp/nomad/issues/921)]

IMPROVEMENTS:
  * cli: Validate job file schemas [[GH-900](https://github.com/hashicorp/nomad/issues/900)]
  * client: Add environment variables for task name, allocation ID/Name/Index
    [GH-869, GH-896]
  * client: Starting task is retried under the restart policy if the error is
    recoverable [[GH-859](https://github.com/hashicorp/nomad/issues/859)]
  * client: Allow tasks to download artifacts, which can be archives, prior to
    starting [[GH-921](https://github.com/hashicorp/nomad/issues/921)]
  * config: Validate Nomad configuration files [[GH-910](https://github.com/hashicorp/nomad/issues/910)]
  * config: Client config allows reserving resources [[GH-910](https://github.com/hashicorp/nomad/issues/910)]
  * driver/docker: Support for ECR [[GH-858](https://github.com/hashicorp/nomad/issues/858)]
  * driver/docker: Periodic Fingerprinting [[GH-893](https://github.com/hashicorp/nomad/issues/893)]
  * driver/docker: Preventing port reservation for log collection on Unix platforms [[GH-897](https://github.com/hashicorp/nomad/issues/897)]
  * driver/rkt: Pass DNS information to rkt driver [[GH-892](https://github.com/hashicorp/nomad/issues/892)]
  * jobspec: Require RFC-1123 and RFC-2782 valid service names [[GH-915](https://github.com/hashicorp/nomad/issues/915)]

BUG FIXES:
  * core: No longer cancel evaluations that are delayed in the plan queue
    [[GH-884](https://github.com/hashicorp/nomad/issues/884)]
  * api: Guard client/fs/ APIs from being accessed on a non-client node [[GH-890](https://github.com/hashicorp/nomad/issues/890)]
  * client: Allow dashes in variable names during interpolation [[GH-857](https://github.com/hashicorp/nomad/issues/857)]
  * client: Updating kill timeout adheres to operator specified maximum value [[GH-878](https://github.com/hashicorp/nomad/issues/878)]
  * client: Fix a case in which clients would pull but not run allocations
    [[GH-906](https://github.com/hashicorp/nomad/issues/906)]
  * consul: Remove concurrent map access [[GH-874](https://github.com/hashicorp/nomad/issues/874)]
  * driver/exec: Stopping tasks with more than one pid in a cgroup [[GH-855](https://github.com/hashicorp/nomad/issues/855)]
  * client/executor/linux: Add /run/resolvconf/ to chroot so DNS works [[GH-905](https://github.com/hashicorp/nomad/issues/905)]

## 0.3.0 (February 25, 2016)

__BACKWARDS INCOMPATIBILITIES:__
  * Stdout and Stderr log files of tasks have moved from task/local to
    alloc/logs [[GH-851](https://github.com/hashicorp/nomad/issues/851)]
  * Any users of the runtime environment variable `$NOMAD_PORT_` will need to
    update to the new `${NOMAD_ADDR_}` variable [[GH-704](https://github.com/hashicorp/nomad/issues/704)]
  * Service names that include periods will fail validation. To fix, remove any
    periods from the service name before running the job [[GH-770](https://github.com/hashicorp/nomad/issues/770)]
  * Task resources are now validated and enforce minimum resources. If a job
    specifies resources below the minimum they will need to be updated [[GH-739](https://github.com/hashicorp/nomad/issues/739)]
  * Node ID is no longer specifiable. For users who have set a custom Node
    ID, the node should be drained before Nomad is updated and the data_dir
    should be deleted before starting for the first time [[GH-675](https://github.com/hashicorp/nomad/issues/675)]
  * Users of custom restart policies should update to the new syntax which adds
    a `mode` field. The `mode` can be either `fail` or `delay`. The default for
    `batch` and `service` jobs is `fail` and `delay` respectively [[GH-594](https://github.com/hashicorp/nomad/issues/594)]
  * All jobs that interpret variables in constraints or driver configurations
    will need to be updated to the new syntax which wraps the interpreted
    variable in curly braces. (`$node.class` becomes `${node.class}`) [[GH-760](https://github.com/hashicorp/nomad/issues/760)]

IMPROVEMENTS:
  * core: Populate job status [[GH-663](https://github.com/hashicorp/nomad/issues/663)]
  * core: Cgroup fingerprinter [[GH-712](https://github.com/hashicorp/nomad/issues/712)]
  * core: Node class constraint [[GH-618](https://github.com/hashicorp/nomad/issues/618)]
  * core: User specifiable kill timeout [[GH-624](https://github.com/hashicorp/nomad/issues/624)]
  * core: Job queueing via blocked evaluations  [[GH-726](https://github.com/hashicorp/nomad/issues/726)]
  * core: Only reschedule failed batch allocations [[GH-746](https://github.com/hashicorp/nomad/issues/746)]
  * core: Add available nodes by DC to AllocMetrics [[GH-619](https://github.com/hashicorp/nomad/issues/619)]
  * core: Improve scheduler retry logic under contention [[GH-787](https://github.com/hashicorp/nomad/issues/787)]
  * core: Computed node class and stack optimization [GH-691, GH-708]
  * core: Improved restart policy with more user configuration [[GH-594](https://github.com/hashicorp/nomad/issues/594)]
  * core: Periodic specification for jobs [GH-540, GH-657, GH-659, GH-668]
  * core: Batch jobs are garbage collected from the Nomad Servers [[GH-586](https://github.com/hashicorp/nomad/issues/586)]
  * core: Free half the CPUs on leader node for use in plan queue and evaluation
    broker [[GH-812](https://github.com/hashicorp/nomad/issues/812)]
  * core: Seed random number generator used to randomize node traversal order
    during scheduling [[GH-808](https://github.com/hashicorp/nomad/issues/808)]
  * core: Performance improvements [GH-823, GH-825, GH-827, GH-830, GH-832,
    GH-833, GH-834, GH-839]
  * core/api: System garbage collection endpoint [[GH-828](https://github.com/hashicorp/nomad/issues/828)]
  * core/api: Allow users to set arbitrary headers via agent config [[GH-699](https://github.com/hashicorp/nomad/issues/699)]
  * core/cli: Prefix based lookups of allocs/nodes/evals/jobs [[GH-575](https://github.com/hashicorp/nomad/issues/575)]
  * core/cli: Print short identifiers and UX cleanup [GH-675, GH-693, GH-692]
  * core/client: Client pulls minimum set of required allocations [[GH-731](https://github.com/hashicorp/nomad/issues/731)]
  * cli: Output of agent-info is sorted [[GH-617](https://github.com/hashicorp/nomad/issues/617)]
  * cli: Eval monitor detects zero wait condition [[GH-776](https://github.com/hashicorp/nomad/issues/776)]
  * cli: Ability to navigate allocation directories [GH-709, GH-798]
  * client: Batch allocation updates to the server [[GH-835](https://github.com/hashicorp/nomad/issues/835)]
  * client: Log rotation for all drivers [GH-685, GH-763, GH-819]
  * client: Only download artifacts from http, https, and S3 [[GH-841](https://github.com/hashicorp/nomad/issues/841)]
  * client: Create a tmp/ directory inside each task directory [[GH-757](https://github.com/hashicorp/nomad/issues/757)]
  * client: Store when an allocation was received by the client [[GH-821](https://github.com/hashicorp/nomad/issues/821)]
  * client: Heartbeating and saving state resilient under high load [[GH-811](https://github.com/hashicorp/nomad/issues/811)]
  * client: Handle updates to tasks Restart Policy and KillTimeout [[GH-751](https://github.com/hashicorp/nomad/issues/751)]
  * client: Killing a driver handle is retried with an exponential backoff
    [[GH-809](https://github.com/hashicorp/nomad/issues/809)]
  * client: Send Node to server when periodic fingerprinters change Node
    attributes/metadata [[GH-749](https://github.com/hashicorp/nomad/issues/749)]
  * client/api: File-system access to allocation directories [[GH-669](https://github.com/hashicorp/nomad/issues/669)]
  * drivers: Validate the "command" field contains a single value [[GH-842](https://github.com/hashicorp/nomad/issues/842)]
  * drivers: Interpret Nomad variables in environment variables/args [[GH-653](https://github.com/hashicorp/nomad/issues/653)]
  * driver/rkt: Add support for CPU/Memory isolation [[GH-610](https://github.com/hashicorp/nomad/issues/610)]
  * driver/rkt: Add support for mounting alloc/task directory [[GH-645](https://github.com/hashicorp/nomad/issues/645)]
  * driver/docker: Support for .dockercfg based auth for private registries
    [[GH-773](https://github.com/hashicorp/nomad/issues/773)]

BUG FIXES:
  * core: Node drain could only be partially applied [[GH-750](https://github.com/hashicorp/nomad/issues/750)]
  * core: Fix panic when eval Ack occurs at delivery limit [[GH-790](https://github.com/hashicorp/nomad/issues/790)]
  * cli: Handle parsing of un-named ports [[GH-604](https://github.com/hashicorp/nomad/issues/604)]
  * cli: Enforce absolute paths for data directories [[GH-622](https://github.com/hashicorp/nomad/issues/622)]
  * client: Cleanup of the allocation directory [[GH-755](https://github.com/hashicorp/nomad/issues/755)]
  * client: Improved stability under high contention [[GH-789](https://github.com/hashicorp/nomad/issues/789)]
  * client: Handle non-200 codes when parsing AWS metadata [[GH-614](https://github.com/hashicorp/nomad/issues/614)]
  * client: Unmounted of shared alloc dir when client is rebooted [[GH-755](https://github.com/hashicorp/nomad/issues/755)]
  * client/consul: Service name changes handled properly [[GH-766](https://github.com/hashicorp/nomad/issues/766)]
  * driver/rkt: handle broader format of rkt version outputs [[GH-745](https://github.com/hashicorp/nomad/issues/745)]
  * driver/qemu: failed to load image and kvm accelerator fixes [[GH-656](https://github.com/hashicorp/nomad/issues/656)]

## 0.2.3 (December 17, 2015)

BUG FIXES:
  * core: Task States not being properly updated [[GH-600](https://github.com/hashicorp/nomad/issues/600)]
  * client: Fixes for user lookup to support CoreOS [[GH-591](https://github.com/hashicorp/nomad/issues/591)]
  * discovery: Using a random prefix for nomad managed services [[GH-579](https://github.com/hashicorp/nomad/issues/579)]
  * discovery: De-Registering Tasks while Nomad sleeps before failed tasks are
    restarted.
  * discovery: Fixes for service registration when multiple allocations are bin
    packed on a node [[GH-583](https://github.com/hashicorp/nomad/issues/583)]
  * configuration: Sort configuration files [[GH-588](https://github.com/hashicorp/nomad/issues/588)]
  * cli: RetryInterval was not being applied properly [[GH-601](https://github.com/hashicorp/nomad/issues/601)]

## 0.2.2 (December 11, 2015)

IMPROVEMENTS:
  * core: Enable `raw_exec` driver in dev mode [[GH-558](https://github.com/hashicorp/nomad/issues/558)]
  * cli: Server join/retry-join command line and config options [[GH-527](https://github.com/hashicorp/nomad/issues/527)]
  * cli: Nomad reports which config files are loaded at start time, or if none
    are loaded [[GH-536](https://github.com/hashicorp/nomad/issues/536)], [[GH-553](https://github.com/hashicorp/nomad/issues/553)]

BUG FIXES:
  * core: Send syslog to `LOCAL0` by default as previously documented [[GH-547](https://github.com/hashicorp/nomad/issues/547)]
  * client: remove all calls to default logger [[GH-570](https://github.com/hashicorp/nomad/issues/570)]
  * consul: Nomad is less noisy when Consul is not running [[GH-567](https://github.com/hashicorp/nomad/issues/567)]
  * consul: Nomad only deregisters services that it created [[GH-568](https://github.com/hashicorp/nomad/issues/568)]
  * driver/exec: Shutdown a task now sends the interrupt signal first to the
    process before forcefully killing it. [[GH-543](https://github.com/hashicorp/nomad/issues/543)]
  * driver/docker: Docker driver no longer leaks unix domain socket connections
    [[GH-556](https://github.com/hashicorp/nomad/issues/556)]
  * fingerprint/network: Now correctly detects interfaces on Windows [[GH-382](https://github.com/hashicorp/nomad/issues/382)]

## 0.2.1 (November 28, 2015)

IMPROVEMENTS:

  * core: Can specify a whitelist for activating drivers [[GH-467](https://github.com/hashicorp/nomad/issues/467)]
  * core: Can specify a whitelist for activating fingerprinters [[GH-488](https://github.com/hashicorp/nomad/issues/488)]
  * core/api: Can list all known regions in the cluster [[GH-495](https://github.com/hashicorp/nomad/issues/495)]
  * client/spawn: spawn package tests made portable (work on Windows) [[GH-442](https://github.com/hashicorp/nomad/issues/442)]
  * client/executor: executor package tests made portable (work on Windows) [[GH-497](https://github.com/hashicorp/nomad/issues/497)]
  * client/driver: driver package tests made portable (work on windows) [[GH-502](https://github.com/hashicorp/nomad/issues/502)]
  * client/discovery: Added more consul client api configuration options [[GH-503](https://github.com/hashicorp/nomad/issues/503)]
  * driver/docker: Added TLS client options to the config file [[GH-480](https://github.com/hashicorp/nomad/issues/480)]
  * jobspec: More flexibility in naming Services [[GH-509](https://github.com/hashicorp/nomad/issues/509)]

BUG FIXES:

  * core: Shared reference to DynamicPorts caused port conflicts when scheduling
    count > 1 [[GH-494](https://github.com/hashicorp/nomad/issues/494)]
  * client/restart policy: Not restarting Batch Jobs if the exit code is 0 [[GH-491](https://github.com/hashicorp/nomad/issues/491)]
  * client/service discovery: Make Service IDs unique [[GH-479](https://github.com/hashicorp/nomad/issues/479)]
  * client/service: Fixes update to check definitions and services which are already registered [[GH-498](https://github.com/hashicorp/nomad/issues/498)]
  * driver/docker: Expose the container port instead of the host port [[GH-466](https://github.com/hashicorp/nomad/issues/466)]
  * driver/docker: Support `port_map` for static ports [[GH-476](https://github.com/hashicorp/nomad/issues/476)]
  * driver/docker: Pass 0.2.0-style port environment variables to the docker container [[GH-476](https://github.com/hashicorp/nomad/issues/476)]
  * jobspec: distinct_hosts constraint can be specified as a boolean (previously panicked) [[GH-501](https://github.com/hashicorp/nomad/issues/501)]

## 0.2.0 (November 18, 2015)

__BACKWARDS INCOMPATIBILITIES:__

  * core: HTTP API `/v1/node/<id>/allocations` returns full Allocation and not
    stub [[GH-402](https://github.com/hashicorp/nomad/issues/402)]
  * core: Removed weight and hard/soft fields in constraints [[GH-351](https://github.com/hashicorp/nomad/issues/351)]
  * drivers: Qemu and Java driver configurations have been updated to both use
    `artifact_source` as the source for external images/jars to be ran
  * jobspec: New reserved and dynamic port specification [[GH-415](https://github.com/hashicorp/nomad/issues/415)]
  * jobspec/drivers: Driver configuration supports arbitrary struct to be
    passed in jobspec [[GH-415](https://github.com/hashicorp/nomad/issues/415)]

FEATURES:

  * core: Blocking queries supported in API [[GH-366](https://github.com/hashicorp/nomad/issues/366)]
  * core: System Scheduler that runs tasks on every node [[GH-287](https://github.com/hashicorp/nomad/issues/287)]
  * core: Regexp, version and lexical ordering constraints [[GH-271](https://github.com/hashicorp/nomad/issues/271)]
  * core: distinctHost constraint ensures Task Groups are running on distinct
    clients [[GH-321](https://github.com/hashicorp/nomad/issues/321)]
  * core: Service block definition with Consul registration [GH-463, GH-460,
    GH-458, GH-455, GH-446, GH-425]
  * client: GCE Fingerprinting [[GH-215](https://github.com/hashicorp/nomad/issues/215)]
  * client: Restart policy for task groups enforced by the client [GH-369,
    GH-393]
  * driver/rawexec: Raw Fork/Exec Driver [[GH-237](https://github.com/hashicorp/nomad/issues/237)]
  * driver/rkt: Experimental Rkt Driver [GH-165, GH-247]
  * drivers: Add support for downloading external artifacts to execute for
    Exec, Raw exec drivers [[GH-381](https://github.com/hashicorp/nomad/issues/381)]

IMPROVEMENTS:

  * core: Configurable Node GC threshold [[GH-362](https://github.com/hashicorp/nomad/issues/362)]
  * core: Overlap plan verification and plan application for increased
    throughput [[GH-272](https://github.com/hashicorp/nomad/issues/272)]
  * cli: Output of `alloc-status` also displays task state [[GH-424](https://github.com/hashicorp/nomad/issues/424)]
  * cli: Output of `server-members` is sorted [[GH-323](https://github.com/hashicorp/nomad/issues/323)]
  * cli: Show node attributes in `node-status` [[GH-313](https://github.com/hashicorp/nomad/issues/313)]
  * client/fingerprint: Network fingerprinter detects interface suitable for
    use, rather than defaulting to eth0 [GH-334, GH-356]
  * client: Client Restore State properly reattaches to tasks and recreates
    them as needed [GH-364, GH-380, GH-388, GH-392, GH-394, GH-397, GH-408]
  * client: Periodic Fingerprinting [[GH-391](https://github.com/hashicorp/nomad/issues/391)]
  * client: Precise snapshotting of TaskRunner and AllocRunner [GH-403, GH-411]
  * client: Task State is tracked by client [[GH-416](https://github.com/hashicorp/nomad/issues/416)]
  * client: Test Skip Detection [[GH-221](https://github.com/hashicorp/nomad/issues/221)]
  * driver/docker: Can now specify auth for docker pull [[GH-390](https://github.com/hashicorp/nomad/issues/390)]
  * driver/docker: Can now specify DNS and DNSSearch options [[GH-390](https://github.com/hashicorp/nomad/issues/390)]
  * driver/docker: Can now specify the container's hostname [[GH-426](https://github.com/hashicorp/nomad/issues/426)]
  * driver/docker: Containers now have names based on the task name. [[GH-389](https://github.com/hashicorp/nomad/issues/389)]
  * driver/docker: Mount task local and alloc directory to docker containers [[GH-290](https://github.com/hashicorp/nomad/issues/290)]
  * driver/docker: Now accepts any value for `network_mode` to support userspace networking plugins in docker 1.9
  * driver/java: Pass JVM options in java driver [GH-293, GH-297]
  * drivers: Use BlkioWeight rather than BlkioThrottleReadIopsDevice [[GH-222](https://github.com/hashicorp/nomad/issues/222)]
  * jobspec and drivers: Driver configuration supports arbitrary struct to be passed in jobspec [[GH-415](https://github.com/hashicorp/nomad/issues/415)]

BUG FIXES:

  * core: Nomad Client/Server RPC codec encodes strings properly [[GH-420](https://github.com/hashicorp/nomad/issues/420)]
  * core: Reset Nack timer in response to scheduler operations [[GH-325](https://github.com/hashicorp/nomad/issues/325)]
  * core: Scheduler checks for updates to environment variables [[GH-327](https://github.com/hashicorp/nomad/issues/327)]
  * cli: Fix crash when -config was given a directory or empty path [[GH-119](https://github.com/hashicorp/nomad/issues/119)]
  * client/fingerprint: Use correct local interface on OS X [GH-361, GH-365]
  * client: Nomad Client doesn't restart failed containers [[GH-198](https://github.com/hashicorp/nomad/issues/198)]
  * client: Reap spawn-daemon process, avoiding a zombie process [[GH-240](https://github.com/hashicorp/nomad/issues/240)]
  * client: Resource exhausted errors because of link-speed zero [GH-146,
    GH-205]
  * client: Restarting Nomad Client leads to orphaned containers [[GH-159](https://github.com/hashicorp/nomad/issues/159)]
  * driver/docker: Apply SELinux label for mounting directories in docker
    [[GH-377](https://github.com/hashicorp/nomad/issues/377)]
  * driver/docker: Docker driver exposes ports when creating container [GH-212,
    GH-412]
  * driver/docker: Docker driver uses docker environment variables correctly
    [[GH-407](https://github.com/hashicorp/nomad/issues/407)]
  * driver/qemu: Qemu fingerprint and tests work on both windows/linux [[GH-352](https://github.com/hashicorp/nomad/issues/352)]

## 0.1.2 (October 6, 2015)

IMPROVEMENTS:

  * client: Nomad client cleans allocations on exit when in dev mode [[GH-214](https://github.com/hashicorp/nomad/issues/214)]
  * drivers: Use go-getter for artifact retrieval, add artifact support to
    Exec, Raw Exec drivers [[GH-288](https://github.com/hashicorp/nomad/issues/288)]

## 0.1.1 (October 5, 2015)

IMPROVEMENTS:

  * cli: Nomad Client configurable from command-line [[GH-191](https://github.com/hashicorp/nomad/issues/191)]
  * client/fingerprint: Native IP detection and user specifiable network
    interface for fingerprinting [[GH-189](https://github.com/hashicorp/nomad/issues/189)]
  * driver/docker: Docker networking mode is configurable [[GH-184](https://github.com/hashicorp/nomad/issues/184)]
  * drivers: Set task environment variables [[GH-206](https://github.com/hashicorp/nomad/issues/206)]

BUG FIXES:

  * client/fingerprint: Network fingerprinting failed if default network
    interface did not exist [[GH-189](https://github.com/hashicorp/nomad/issues/189)]
  * client: Fixed issue where network resources throughput would be set to 0
    MBits if the link speed could not be determined [[GH-205](https://github.com/hashicorp/nomad/issues/205)]
  * client: Improved detection of Nomad binary [[GH-181](https://github.com/hashicorp/nomad/issues/181)]
  * driver/docker: Docker dynamic port mapping were not being set properly
    [[GH-199](https://github.com/hashicorp/nomad/issues/199)]

## 0.1.0 (September 28, 2015)

  * Initial release

