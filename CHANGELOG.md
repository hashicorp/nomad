## 1.3.16 (August 18, 2023)

BUG FIXES:

* core: fixed a bug that caused job validation to fail when a task with `kill_timeout` was placed inside a group with `update.progress_deadline` set to 0 [[GH-17342](https://github.com/hashicorp/nomad/issues/17342)]

## 1.3.15 (May 19, 2023)

BUG FIXES:

* bug: Corrected status description and modification time for canceled evaluations [[GH-17071](https://github.com/hashicorp/nomad/issues/17071)]
* client: Fixed a bug where restarting a terminal allocation turns it into a zombie where allocation and task hooks will run unexpectedly [[GH-17175](https://github.com/hashicorp/nomad/issues/17175)]
* client: clean up resources upon failure to restore task during client restart [[GH-17104](https://github.com/hashicorp/nomad/issues/17104)]
* scale: Fixed a bug where evals could be created with the wrong type [[GH-17092](https://github.com/hashicorp/nomad/issues/17092)]
* scheduler: Fixed a bug where implicit `spread` targets were treated as separate targets for scoring [[GH-17195](https://github.com/hashicorp/nomad/issues/17195)]
* scheduler: Fixed a bug where scores for spread scheduling could be -Inf [[GH-17198](https://github.com/hashicorp/nomad/issues/17198)]

## 1.3.14 (May 02, 2023)

IMPROVEMENTS:

* build: Update from Go 1.20.3 to Go 1.20.4 [[GH-17056](https://github.com/hashicorp/nomad/issues/17056)]
* core: Prevent `task.kill_timeout` being greater than `update.progress_deadline` [[GH-16761](https://github.com/hashicorp/nomad/issues/16761)]

BUG FIXES:

* api: Fixed filtering on maps with missing keys [[GH-16991](https://github.com/hashicorp/nomad/issues/16991)]
* build: Linux packages now have vendor label and set the default label to HashiCorp. This fix is implemented for any future releases, but will not be updated for historical releases [[GH-16071](https://github.com/hashicorp/nomad/issues/16071)]
* client: Fix CNI plugin version fingerprint when output includes protocol version [[GH-16776](https://github.com/hashicorp/nomad/issues/16776)]
* client: Fix address for ports in IPv6 networks [[GH-16723](https://github.com/hashicorp/nomad/issues/16723)]
* client: Fixed a bug where restarting proxy sidecar tasks failed [[GH-16815](https://github.com/hashicorp/nomad/issues/16815)]
* client: Prevent a panic when an allocation has a legacy task-level bridge network and uses a driver that does not create a network namespace [[GH-16921](https://github.com/hashicorp/nomad/issues/16921)]
* core: the deployment's list endpoint now supports look up by prefix using the wildcard for namespace [[GH-16792](https://github.com/hashicorp/nomad/issues/16792)]
* csi: gracefully recover tasks that use csi node plugins [[GH-16809](https://github.com/hashicorp/nomad/issues/16809)]
* docker: Fixed a bug where plugin config values were ignored [[GH-16713](https://github.com/hashicorp/nomad/issues/16713)]
* drain: Fixed a bug where drains would complete based on the server status and not the client status of an allocation [[GH-14348](https://github.com/hashicorp/nomad/issues/14348)]
* driver/exec: Fixed a bug where `cap_drop` and `cap_add` would not expand capabilities [[GH-16643](https://github.com/hashicorp/nomad/issues/16643)]
* scale: Do not allow scale requests for jobs of type system [[GH-16969](https://github.com/hashicorp/nomad/issues/16969)]
* scheduler: Fix reconciliation of reconnecting allocs when the replacement allocations are not running [[GH-16609](https://github.com/hashicorp/nomad/issues/16609)]
* scheduler: honor false value for distinct_hosts constraint [[GH-16907](https://github.com/hashicorp/nomad/issues/16907)]
* server: Added verification of cron jobs already running before forcing new evals right after leader change [[GH-16583](https://github.com/hashicorp/nomad/issues/16583)]
* services: Fixed a bug preventing group service deregistrations after alloc restarts [[GH-16905](https://github.com/hashicorp/nomad/issues/16905)]

## 1.3.13 (April 04, 2023)

SECURITY:

* build: update to Go 1.20.3 to prevent denial of service attack via malicious HTTP headers [CVE-2023-24534](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2023-24534) [[GH-16788](https://github.com/hashicorp/nomad/issues/16788)]

## 1.3.12 (March 21, 2023)

IMPROVEMENTS:

* build: Update to go1.20.2 [[GH-16427](https://github.com/hashicorp/nomad/issues/16427)]

BUG FIXES:

* client: Fixed a bug where clients using Consul discovery to join the cluster would get permission denied errors [[GH-16490](https://github.com/hashicorp/nomad/issues/16490)]
* client: Fixed a bug where cpuset initialization fails after Client restart [[GH-16467](https://github.com/hashicorp/nomad/issues/16467)]
* plugin: Add missing fields to `TaskConfig` so they can be accessed by external task drivers [[GH-16434](https://github.com/hashicorp/nomad/issues/16434)]
* services: Fixed a bug where a service would be deregistered twice [[GH-16289](https://github.com/hashicorp/nomad/issues/16289)]

## 1.3.11 (March 10, 2023)

IMPROVEMENTS:

* env/ec2: update cpu metadata [[GH-16417](https://github.com/hashicorp/nomad/issues/16417)]

BUG FIXES:

* client: Fixed a bug where clients used the serf advertise address to connect to servers when using Consul auto-discovery [[GH-16217](https://github.com/hashicorp/nomad/issues/16217)]
* docker: Fixed a bug where pause containers would be erroneously removed [[GH-16352](https://github.com/hashicorp/nomad/issues/16352)]
* scheduler: Fixed a bug where collisions in dynamic port offerings would result in spurious plan-for-node-rejected errors [[GH-16401](https://github.com/hashicorp/nomad/issues/16401)]
* server: Fixed a bug where deregistering a job that was already garbage collected would create a new evaluation [[GH-16287](https://github.com/hashicorp/nomad/issues/16287)]
* server: Fixed a bug where node updates that produced errors from service discovery or CSI plugin updates were not logged [[GH-16287](https://github.com/hashicorp/nomad/issues/16287)]
* server: Fixed a bug where the `system reconcile summaries` command and API would not return any scheduler-related errors [[GH-16287](https://github.com/hashicorp/nomad/issues/16287)]

## 1.3.10 (March 01, 2023)

BREAKING CHANGES:

* core: Ensure no leakage of evaluations for batch jobs. Prior to this change allocations and evaluations for batch jobs were never garbage collected until the batch job was explicitly stopped. The new `batch_eval_gc_threshold` server configuration controls how often they are collected. The default threshold is `24h`. [[GH-15097](https://github.com/hashicorp/nomad/issues/15097)]

IMPROVEMENTS:

* build: Update to go1.19.5 [[GH-15769](https://github.com/hashicorp/nomad/issues/15769)]
* build: Update to go1.20 [[GH-16029](https://github.com/hashicorp/nomad/issues/16029)]
* client: detect and cleanup leaked iptables rules [[GH-15407](https://github.com/hashicorp/nomad/issues/15407)]
* consul: add client configuration for grpc_ca_file [[GH-15701](https://github.com/hashicorp/nomad/issues/15701)]
* env/ec2: update cpu metadata [[GH-15770](https://github.com/hashicorp/nomad/issues/15770)]
* fingerprint: Detect CNI plugins and set versions as node attributes [[GH-15452](https://github.com/hashicorp/nomad/issues/15452)]

DEPRECATIONS:

* api: The connect `ConsulExposeConfig.Path` field is deprecated in favor of `ConsulExposeConfig.Paths` [[GH-15541](https://github.com/hashicorp/nomad/issues/15541)]
* api: The connect `ConsulProxy.ExposeConfig` field is deprecated in favor of `ConsulProxy.Expose` [[GH-15541](https://github.com/hashicorp/nomad/issues/15541)]

BUG FIXES:

* acl: Fixed a bug where creating/updating a policy which was invalid would return a 404 status code, not a 400 [[GH-16000](https://github.com/hashicorp/nomad/issues/16000)]
* agent: Make agent syslog log level follow log_level config [[GH-15625](https://github.com/hashicorp/nomad/issues/15625)]
* api: Added missing node states to NodeStatus constants [[GH-16166](https://github.com/hashicorp/nomad/issues/16166)]
* api: Fix stale querystring parameter value as boolean [[GH-15605](https://github.com/hashicorp/nomad/issues/15605)]
* api: Fixed a bug where exposeConfig field was not provided correctly when getting the jobs via the API [[GH-15541](https://github.com/hashicorp/nomad/issues/15541)]
* api: Fixed a nil pointer dereference when periodic jobs are missing their periodic spec [[GH-13845](https://github.com/hashicorp/nomad/issues/13845)]
* cgutil: handle panic coming from runc helper method [[GH-16180](https://github.com/hashicorp/nomad/issues/16180)]
* cli: Fixed a panic in `deployment status` when rollback deployments are slow to appear [[GH-16011](https://github.com/hashicorp/nomad/issues/16011)]
* connect: ingress http/2/grpc listeners may exclude hosts [[GH-15749](https://github.com/hashicorp/nomad/issues/15749)]
* consul: Fixed a bug where acceptable service identity on Consul token was not accepted [[GH-15928](https://github.com/hashicorp/nomad/issues/15928)]
* consul: Fixed a bug where consul token was not respected when reverting a job [[GH-15996](https://github.com/hashicorp/nomad/issues/15996)]
* consul: Fixed a bug where services would continuously re-register when using ipv6 [[GH-15411](https://github.com/hashicorp/nomad/issues/15411)]
* core: enforce strict ordering that node status updates are recorded after allocation updates for reconnecting clients [[GH-15808](https://github.com/hashicorp/nomad/issues/15808)]
* csi: Fixed a bug where a crashing plugin could panic the Nomad client [[GH-15518](https://github.com/hashicorp/nomad/issues/15518)]
* csi: Fixed a bug where secrets that include '=' were incorrectly rejected [[GH-15670](https://github.com/hashicorp/nomad/issues/15670)]
* csi: Fixed a bug where volumes in non-default namespaces could not be scheduled for system or sysbatch jobs [[GH-15372](https://github.com/hashicorp/nomad/issues/15372)]
* csi: Fixed potential state store corruption when garbage collecting CSI volume claims or checking whether it's safe to force-deregister a volume [[GH-16256](https://github.com/hashicorp/nomad/issues/16256)]
* docker: Fixed a bug where images referenced by multiple tags would not be GC'd [[GH-15962](https://github.com/hashicorp/nomad/issues/15962)]
* docker: Fixed a bug where infra_image did not get alloc_id label [[GH-15898](https://github.com/hashicorp/nomad/issues/15898)]
* docker: configure restart policy for bridge network pause container [[GH-15732](https://github.com/hashicorp/nomad/issues/15732)]
* event stream: Fixed a bug where undefined ACL policies on the request's ACL would result in incorrect authentication errors [[GH-15495](https://github.com/hashicorp/nomad/issues/15495)]
* fix: Add the missing option propagation_mode for volume_mount [[GH-15626](https://github.com/hashicorp/nomad/issues/15626)]
* parser: Fixed a panic in the job spec parser when a variable validation block was missing its condition [[GH-16018](https://github.com/hashicorp/nomad/issues/16018)]
* scheduler (Enterprise): Fixed a bug that prevented new allocations from multiregion jobs to be placed in situations where other regions are not involved, such as node updates. [[GH-15325](https://github.com/hashicorp/nomad/issues/15325)]
* template: Fixed a bug that caused the chage script to fail to run [[GH-15915](https://github.com/hashicorp/nomad/issues/15915)]
* ui: Fix allocation memory chart to display the same value as the CLI [[GH-15909](https://github.com/hashicorp/nomad/issues/15909)]
* ui: Fix navigation to pages for jobs that are not in the default namespace [[GH-15906](https://github.com/hashicorp/nomad/issues/15906)]
* volumes: Fixed a bug where `per_alloc` was allowed for volume blocks on system and sysbatch jobs, which do not have an allocation index [[GH-16030](https://github.com/hashicorp/nomad/issues/16030)]

## 1.3.9 (February 14, 2023)

SECURITY:

* artifact: Provide mitigations against unbounded artifact decompression [[GH-16126](https://github.com/hashicorp/nomad/issues/16126)]
* build: Update to go1.20.1 [[GH-16182](https://github.com/hashicorp/nomad/issues/16182)]

## 1.3.8 (November 21, 2022)

BUG FIXES:

* api: Ensure all request body decode errors return a 400 status code [[GH-15252](https://github.com/hashicorp/nomad/issues/15252)]
* cleanup: fixed missing timer.Reset for plan queue stat emitter [[GH-15134](https://github.com/hashicorp/nomad/issues/15134)]
* client: Fixed a bug where tasks would restart without waiting for interval [[GH-15215](https://github.com/hashicorp/nomad/issues/15215)]
* client: fixed a bug where non-`docker` tasks with network isolation would leak network namespaces and iptables rules if the client was restarted while they were running [[GH-15214](https://github.com/hashicorp/nomad/issues/15214)]
* client: prevent allocations from failing on client reconnect by retrying RPC requests when no servers are available yet [[GH-15140](https://github.com/hashicorp/nomad/issues/15140)]
* csi: Fixed race condition that can cause a panic when volume is garbage collected [[GH-15101](https://github.com/hashicorp/nomad/issues/15101)]
* device: Fixed a bug where device plugins would not fingerprint on startup [[GH-15125](https://github.com/hashicorp/nomad/issues/15125)]
* drivers: Fixed a bug where one goroutine was leaked per task [[GH-15180](https://github.com/hashicorp/nomad/issues/15180)]
* drivers: pass missing `propagation_mode` configuration for volume mounts to external plugins [[GH-15096](https://github.com/hashicorp/nomad/issues/15096)]
* event_stream: fixed a bug where dynamic port values would fail to serialize in the event stream [[GH-12916](https://github.com/hashicorp/nomad/issues/12916)]
* fingerprint: Ensure Nomad can correctly fingerprint Consul gRPC where the Consul agent is running v1.14.0 or greater [[GH-15309](https://github.com/hashicorp/nomad/issues/15309)]
* scheduler: Fixed a bug that prevented disconnected allocations to be updated after they reconnect. [[GH-15068](https://github.com/hashicorp/nomad/issues/15068)]
* scheduler: Prevent unnecessary placements when disconnected allocations reconnect. [[GH-15068](https://github.com/hashicorp/nomad/issues/15068)]
* template: Fixed a bug where template could cause agent panic on startup [[GH-15192](https://github.com/hashicorp/nomad/issues/15192)]

## 1.3.7 (October 26, 2022)

IMPROVEMENTS:

* deps: update go-memdb for goroutine leak fix [[GH-14983](https://github.com/hashicorp/nomad/issues/14983)]
* docker: improve memory usage for docker_logger [[GH-14875](https://github.com/hashicorp/nomad/issues/14875)]

BUG FIXES:

* acl: Fixed a bug where Nomad version checking for one-time tokens was enforced across regions [[GH-14911](https://github.com/hashicorp/nomad/issues/14911)]
* client: Fixed a bug where Nomad could not detect cores on recent RHEL systems [[GH-15027](https://github.com/hashicorp/nomad/issues/15027)]
* consul: Fixed a bug where services continuously re-registered [[GH-14917](https://github.com/hashicorp/nomad/issues/14917)]
* consul: atomically register checks on initial service registration [[GH-14944](https://github.com/hashicorp/nomad/issues/14944)]
* deps: Update hashicorp/raft to v1.3.11; fixes unstable leadership on server removal [[GH-15021](https://github.com/hashicorp/nomad/issues/15021)]
* nomad native service discovery: Ensure all local servers meet v.1.3.0 minimum before service registrations can be written [[GH-14924](https://github.com/hashicorp/nomad/issues/14924)]
* scheduler: Fixed a bug where version checking for disconnected clients handling was enforced across regions [[GH-14911](https://github.com/hashicorp/nomad/issues/14911)]

## 1.3.6 (October 04, 2022)

SECURITY:

* client: recover from panics caused by artifact download to prevent the Nomad client from crashing [[GH-14696](https://github.com/hashicorp/nomad/issues/14696)]

IMPROVEMENTS:

* api: return a more descriptive error when /v1/acl/bootstrap fails to decode request body [[GH-14629](https://github.com/hashicorp/nomad/issues/14629)]
* cli: ignore `-hcl2-strict` when -hcl1 is set. [[GH-14426](https://github.com/hashicorp/nomad/issues/14426)]
* cli: warn destructive update only when count is greater than 1 [[GH-13103](https://github.com/hashicorp/nomad/issues/13103)]
* consul: Allow interpolation of task environment values into Consul Service Mesh configuration [[GH-14445](https://github.com/hashicorp/nomad/issues/14445)]
* ui: Display different message when trying to exec into a job with no task running. [[GH-14071](https://github.com/hashicorp/nomad/issues/14071)]

BUG FIXES:

* api: Fixed a bug where the List Volume API did not include the `ControllerRequired` and `ResourceExhausted` fields. [[GH-14484](https://github.com/hashicorp/nomad/issues/14484)]
* cli: Ignore Vault token when generating job diff. [[GH-14424](https://github.com/hashicorp/nomad/issues/14424)]
* cli: fixed a bug in the `operator api` command where the HTTPS scheme was not always correctly calculated [[GH-14635](https://github.com/hashicorp/nomad/issues/14635)]
* cli: return exit code `255` when `nomad job plan` fails job validation. [[GH-14426](https://github.com/hashicorp/nomad/issues/14426)]
* cli: set content length on POST requests when using the `nomad operator api` command [[GH-14634](https://github.com/hashicorp/nomad/issues/14634)]
* client: Fixed bug where clients could attempt to connect to servers with invalid addresses retrieved from Consul. [[GH-14431](https://github.com/hashicorp/nomad/issues/14431)]
* csi: Fixed a bug where a volume that was successfully unmounted by the client but then failed controller unpublishing would not be marked free until garbage collection ran. [[GH-14675](https://github.com/hashicorp/nomad/issues/14675)]
* csi: Fixed a bug where the server would not send controller unpublish for a failed allocation. [[GH-14484](https://github.com/hashicorp/nomad/issues/14484)]
* csi: Fixed a data race in the volume unpublish endpoint that could result in claims being incorrectly marked as freed before being persisted to raft. [[GH-14484](https://github.com/hashicorp/nomad/issues/14484)]
* helpers: Fixed a bug where random stagger func did not protect against negative inputs [[GH-14497](https://github.com/hashicorp/nomad/issues/14497)]
* jobspec: Fixed a bug where an `artifact` with `headers` configuration would fail to parse when using HCLv1 [[GH-14637](https://github.com/hashicorp/nomad/issues/14637)]
* metrics: Update client `node_scheduling_eligibility` value with server heartbeats. [[GH-14483](https://github.com/hashicorp/nomad/issues/14483)]
* quotas (Enterprise): Fixed a server crashing panic when updating and checking a quota concurrently.
* rpc: check for spec changes in all regions when registering multiregion jobs [[GH-14519](https://github.com/hashicorp/nomad/issues/14519)]
* scheduler: Fixed bug where the scheduler would treat multiregion jobs as paused for job types that don't use deployments [[GH-14659](https://github.com/hashicorp/nomad/issues/14659)]
* template: Fixed a bug where the `splay` timeout was not being applied when `change_mode` was set to `script`. [[GH-14749](https://github.com/hashicorp/nomad/issues/14749)]
* ui: Remove extra space when displaying the version in the menu footer. [[GH-14457](https://github.com/hashicorp/nomad/issues/14457)]

## 1.3.5 (August 31, 2022)

IMPROVEMENTS:

* cgroups: use cgroup.kill interface file when using cgroups v2 [[GH-14371](https://github.com/hashicorp/nomad/issues/14371)]
* consul: Reduce load on Consul leader server by allowing stale results when listing namespaces. [[GH-12953](https://github.com/hashicorp/nomad/issues/12953)]

BUG FIXES:

* cli: Fixed a bug where forcing a periodic job would fail if the job ID prefix-matched other periodic jobs [[GH-14333](https://github.com/hashicorp/nomad/issues/14333)]
* template: Fixed a bug that could cause Nomad to panic when using `change_mode = "script"` [[GH-14374](https://github.com/hashicorp/nomad/issues/14374)]
* ui: Revert a change that resulted in UI errors when ACLs were not used. [[GH-14381](https://github.com/hashicorp/nomad/issues/14381)]

## 1.3.4 (August 25, 2022)

IMPROVEMENTS:

* api: HTTP server now returns a 429 error code when hitting the connection limit [[GH-13621](https://github.com/hashicorp/nomad/issues/13621)]
* build: update to go1.19 [[GH-14132](https://github.com/hashicorp/nomad/issues/14132)]
* cli: `operator debug` now outputs current leader to debug bundle [[GH-13472](https://github.com/hashicorp/nomad/issues/13472)]
* cli: `operator snapshot state` supports `-filter` expressions and avoids writing large temporary files [[GH-13658](https://github.com/hashicorp/nomad/issues/13658)]
* client: add option to restart all tasks of an allocation, regardless of lifecycle type or state. [[GH-14127](https://github.com/hashicorp/nomad/issues/14127)]
* client: only start poststop tasks after poststart tasks are done. [[GH-14127](https://github.com/hashicorp/nomad/issues/14127)]
* deps: Updated `github.com/hashicorp/go-discover` to latest to allow setting the AWS endpoint definition [[GH-13491](https://github.com/hashicorp/nomad/issues/13491)]
* driver/docker: Added config option to disable container healthcheck [[GH-14089](https://github.com/hashicorp/nomad/issues/14089)]
* qemu: Added option to configure `drive_interface` [[GH-11864](https://github.com/hashicorp/nomad/issues/11864)]
* sentinel: add the ability to reference the namespace and Nomad acl token in policies [[GH-14171](https://github.com/hashicorp/nomad/issues/14171)]
* template: add script change_mode that allows scripts to be executed on template change [[GH-13972](https://github.com/hashicorp/nomad/issues/13972)]
* ui: Add button to restart all tasks in an allocation. [[GH-14223](https://github.com/hashicorp/nomad/issues/14223)]
* ui: add general keyboard navigation to the Nomad UI [[GH-14138](https://github.com/hashicorp/nomad/issues/14138)]

BUG FIXES:

* api: cleanup whitespace from failed api response body [[GH-14145](https://github.com/hashicorp/nomad/issues/14145)]
* cli: Fixed a bug where job validation requeset was not sent to leader [[GH-14065](https://github.com/hashicorp/nomad/issues/14065)]
* cli: Fixed a bug where the memory usage reported by Allocation Resource Utilization is zero on systems using cgroups v2 [[GH-14069](https://github.com/hashicorp/nomad/issues/14069)]
* cli: Fixed a bug where vault token not respected in plan command [[GH-14088](https://github.com/hashicorp/nomad/issues/14088)]
* client/logmon: fixed a bug where logmon cannot find nomad executable [[GH-14297](https://github.com/hashicorp/nomad/issues/14297)]
* client: Fixed a bug where cpuset initialization would not work on first agent startup [[GH-14230](https://github.com/hashicorp/nomad/issues/14230)]
* client: Fixed a bug where user lookups would hang or panic [[GH-14248](https://github.com/hashicorp/nomad/issues/14248)]
* client: Fixed a problem calculating a services namespace [[GH-13493](https://github.com/hashicorp/nomad/issues/13493)]
* csi: Fixed a bug where volume claims on lost or garbage collected nodes could not be freed [[GH-13301](https://github.com/hashicorp/nomad/issues/13301)]
* template: Fixed a bug where job templates would use `uid` and `gid` 0 after upgrading to Nomad 1.3.3, causing tasks to fail with the error `failed looking up user: managing file ownership is not supported on Windows`. [[GH-14203](https://github.com/hashicorp/nomad/issues/14203)]
* ui: Fixed a bug that caused the allocation details page to display the stats bar chart even if the task was pending. [[GH-14224](https://github.com/hashicorp/nomad/issues/14224)]
* ui: Removes duplicate breadcrumb header when navigating from child job back to parent. [[GH-14115](https://github.com/hashicorp/nomad/issues/14115)]
* vault: Fixed a bug where Vault clients were recreated when the server configuration was reloaded, even if there were no changes to the Vault configuration. [[GH-14298](https://github.com/hashicorp/nomad/issues/14298)]
* vault: Fixed a bug where changing the Vault configuration `namespace` field was not detected as a change during server configuration reload. [[GH-14298](https://github.com/hashicorp/nomad/issues/14298)]

## 1.3.3 (August 05, 2022)

IMPROVEMENTS:

* csi: Add `stage_publish_base_dir` field to `csi_plugin` block to support plugins that require a specific staging/publishing directory for mounts [[GH-13919](https://github.com/hashicorp/nomad/issues/13919)]
* qemu: use shorter socket file names to reduce the chance of hitting the max path length [[GH-13971](https://github.com/hashicorp/nomad/issues/13971)]
* template: Expose consul-template configuration options at the client level for `nomad_retry`. [[GH-13907](https://github.com/hashicorp/nomad/issues/13907)]
* template: Templates support new uid/gid parameter pair [[GH-13755](https://github.com/hashicorp/nomad/issues/13755)]
* ui: Reorder and apply the same style to the Evaluations list page filters to match the Job list page. [[GH-13866](https://github.com/hashicorp/nomad/issues/13866)]

BUG FIXES:

* acl: Fixed a bug where the timestamp for expiring one-time tokens was not deterministic between servers [[GH-13737](https://github.com/hashicorp/nomad/issues/13737)]
* deployments: Fixed a bug that prevented auto-approval if canaries were marked as unhealthy during deployment [[GH-14001](https://github.com/hashicorp/nomad/issues/14001)]
* metrics: Fixed a bug where blocked evals with no class produced no dc:class scope metrics [[GH-13786](https://github.com/hashicorp/nomad/issues/13786)]
* namespaces: Fixed a bug that allowed deleting a namespace that contained a CSI volume [[GH-13880](https://github.com/hashicorp/nomad/issues/13880)]
* qemu: restore the monitor socket path when restoring a QEMU task. [[GH-14000](https://github.com/hashicorp/nomad/issues/14000)]
* servicedisco: Fixed a bug where non-unique services would escape job validation [[GH-13869](https://github.com/hashicorp/nomad/issues/13869)]
* ui: Add missing breadcrumb in the Evaluations page. [[GH-13865](https://github.com/hashicorp/nomad/issues/13865)]
* ui: Fixed a bug where task memory was reported as zero on systems using cgroups v2 [[GH-13670](https://github.com/hashicorp/nomad/issues/13670)]

## 1.3.2 (July 13, 2022)

IMPROVEMENTS:

* agent: Added delete support to the eval HTTP API [[GH-13492](https://github.com/hashicorp/nomad/issues/13492)]
* agent: emit a warning message if the agent starts with `bootstrap_expect` set to an even number. [[GH-12961](https://github.com/hashicorp/nomad/issues/12961)]
* agent: logs are no longer buffered at startup when logging in JSON format [[GH-13076](https://github.com/hashicorp/nomad/issues/13076)]
* api: enable setting `?choose` parameter when querying services [[GH-12862](https://github.com/hashicorp/nomad/issues/12862)]
* api: refactor ACL check when using the all namespaces wildcard in the job and alloc list endpoints [[GH-13608](https://github.com/hashicorp/nomad/issues/13608)]
* api: support Authorization Bearer header in lieu of X-Nomad-Token header [[GH-12534](https://github.com/hashicorp/nomad/issues/12534)]
* bootstrap: Added option to allow for an operator generated bootstrap token to be passed to the `acl bootstrap` command [[GH-12520](https://github.com/hashicorp/nomad/issues/12520)]
* cli: Added `delete` command to the eval CLI [[GH-13492](https://github.com/hashicorp/nomad/issues/13492)]
* cli: Added `scheduler get-config` and `scheduler set-config` commands to the operator CLI [[GH-13045](https://github.com/hashicorp/nomad/issues/13045)]
* cli: always display job ID and namespace in the `eval status` command [[GH-13581](https://github.com/hashicorp/nomad/issues/13581)]
* cli: display namespace and node ID in the `eval list` command and when `eval status` matches multiple evals [[GH-13581](https://github.com/hashicorp/nomad/issues/13581)]
* cli: update default redis and use nomad service discovery [[GH-13044](https://github.com/hashicorp/nomad/issues/13044)]
* client: added more fault tolerant defaults for template configuration [[GH-13041](https://github.com/hashicorp/nomad/issues/13041)]
* core: Added the ability to pause and un-pause the eval broker and blocked eval broker [[GH-13045](https://github.com/hashicorp/nomad/issues/13045)]
* core: On node updates skip creating evaluations for jobs not in the node's datacenter. [[GH-12955](https://github.com/hashicorp/nomad/issues/12955)]
* core: automatically mark clients with recurring plan rejections as ineligible [[GH-13421](https://github.com/hashicorp/nomad/issues/13421)]
* driver/docker: Eliminate excess Docker registry pulls for the `infra_image` when it already exists locally. [[GH-13265](https://github.com/hashicorp/nomad/issues/13265)]
* fingerprint: add support for detecting kernel architecture of clients. (attribute: `kernel.arch`) [[GH-13182](https://github.com/hashicorp/nomad/issues/13182)]
* hcl: added support for using the `filebase64` function in jobspecs [[GH-11791](https://github.com/hashicorp/nomad/issues/11791)]
* metrics: emit `nomad.nomad.plan.rejection_tracker.node_score` metric for the number of times a node had a plan rejection within the past time window [[GH-13421](https://github.com/hashicorp/nomad/issues/13421)]
* qemu: add support for guest agent socket [[GH-12800](https://github.com/hashicorp/nomad/issues/12800)]
* ui: Namespace filter query paramters are now isolated by route [[GH-13679](https://github.com/hashicorp/nomad/issues/13679)]

BUG FIXES:

* api: Fix listing evaluations with the wildcard namespace and an ACL token [[GH-13530](https://github.com/hashicorp/nomad/issues/13530)]
* api: Fixed a bug where Consul token was not respected for job revert API [[GH-13065](https://github.com/hashicorp/nomad/issues/13065)]
* cli: Fixed a bug in the names of the `node drain` and `node status` sub-commands [[GH-13656](https://github.com/hashicorp/nomad/issues/13656)]
* cli: Fixed a bug where job validate did not respect vault token or namespace [[GH-13070](https://github.com/hashicorp/nomad/issues/13070)]
* client: Fixed a bug where max_kill_timeout client config was ignored [[GH-13626](https://github.com/hashicorp/nomad/issues/13626)]
* client: Fixed a bug where network.dns block was not interpolated [[GH-12817](https://github.com/hashicorp/nomad/issues/12817)]
* cni: Fixed a bug where loopback address was not set for all drivers [[GH-13428](https://github.com/hashicorp/nomad/issues/13428)]
* connect: Added missing ability of setting Connect upstream destination namespace [[GH-13125](https://github.com/hashicorp/nomad/issues/13125)]
* core: Fixed a bug where an evicted batch job would not be rescheduled [[GH-13205](https://github.com/hashicorp/nomad/issues/13205)]
* core: Fixed a bug where blocked eval resources were incorrectly computed [[GH-13104](https://github.com/hashicorp/nomad/issues/13104)]
* core: Fixed a bug where reserved ports on multiple node networks would be treated as a collision. `client.reserved.reserved_ports` is now merged into each `host_network`'s reserved ports instead of being treated as a collision. [[GH-13651](https://github.com/hashicorp/nomad/issues/13651)]
* core: Fixed a bug where the plan applier could deadlock if leader's state lagged behind plan's creation index for more than 5 seconds. [[GH-13407](https://github.com/hashicorp/nomad/issues/13407)]
* csi: Fixed a regression where a timeout was introduced that prevented some plugins from running by marking them as unhealthy after 30s by introducing a configurable `health_timeout` field [[GH-13340](https://github.com/hashicorp/nomad/issues/13340)]
* csi: Fixed a scheduler bug where failed feasibility checks would return early and prevent processing additional nodes [[GH-13274](https://github.com/hashicorp/nomad/issues/13274)]
* docker: Fixed a bug where cgroups-v1 parent was being set [[GH-13058](https://github.com/hashicorp/nomad/issues/13058)]
* lifecycle: fixed a bug where sidecar tasks were not being stopped last [[GH-13055](https://github.com/hashicorp/nomad/issues/13055)]
* state: Fix listing evaluations from all namespaces [[GH-13551](https://github.com/hashicorp/nomad/issues/13551)]
* ui: Allow running jobs from a namespace-limited token [[GH-13659](https://github.com/hashicorp/nomad/issues/13659)]
* ui: Fix a bug that prevented viewing the details of an evaluation in a non-default namespace [[GH-13530](https://github.com/hashicorp/nomad/issues/13530)]
* ui: Fixed a bug that prevented the UI task exec functionality to work from behind a reverse proxy. [[GH-12925](https://github.com/hashicorp/nomad/issues/12925)]
* ui: Fixed an issue where editing or running a job with a namespace via the UI would throw a 404 on redirect. [[GH-13588](https://github.com/hashicorp/nomad/issues/13588)]
* ui: fixed a bug where links to jobs with "@" in their name would mis-identify namespace and 404 [[GH-13012](https://github.com/hashicorp/nomad/issues/13012)]
* volumes: Fixed a bug where additions, updates, or removals of host volumes or CSI volumes were not treated as destructive updates [[GH-13008](https://github.com/hashicorp/nomad/issues/13008)]

## 1.3.1 (May 19, 2022)

SECURITY:

* A vulnerability was identified in the go-getter library that Nomad uses for its artifacts such that a specially crafted Nomad jobspec can be used for privilege escalation onto client agent hosts. [CVE-2022-30324](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2022-30324) [[GH-13057](https://github.com/hashicorp/nomad/issues/13057)]

BUG FIXES:

* agent: fixed a panic on startup when the `server.protocol_version` config parameter was set [[GH-12962](https://github.com/hashicorp/nomad/issues/12962)]

## 1.3.0 (May 11, 2022)

FEATURES:

* **Edge compute improvements**: Added support for reconnecting healthy allocations when disconnected clients reconnect. [[GH-12476](https://github.com/hashicorp/nomad/issues/12476)]
* **Native service discovery**: Register and discover services using builtin simple service discovery. [[GH-12368](https://github.com/hashicorp/nomad/issues/12368)]

BREAKING CHANGES:

* agent: The state database on both clients and servers will automatically migrate its underlying database on startup. Downgrading to a previous version of an agent after upgrading it to Nomad 1.3 is not supported. [[GH-12107](https://github.com/hashicorp/nomad/issues/12107)]
* client: The client state store will be automatically migrated to a new schema version when upgrading a client. Downgrading to a previous version of the client after upgrading it to Nomad 1.3 is not supported. To downgrade safely, users should erase the Nomad client's data directory. [[GH-12078](https://github.com/hashicorp/nomad/issues/12078)]
* connect: Consul Service Identity ACL tokens automatically generated for Connect services are now
created as Local rather than Global tokens. Nomad clusters with Connect services making cross-Consul
datacenter requests will need to ensure their Consul agents are configured with anonymous ACL tokens
of sufficient node and service read permissions. [[GH-8068](https://github.com/hashicorp/nomad/issues/8068)]
* connect: The minimum Consul version supported by Nomad's Connect integration is now Consul v1.8.0. [[GH-8068](https://github.com/hashicorp/nomad/issues/8068)]
* csi: The client filesystem layout for CSI plugins has been updated to correctly handle the lifecycle of multiple allocations serving the same plugin. Running plugin tasks will not be updated after upgrading the client, but it is recommended to redeploy CSI plugin jobs after upgrading the cluster. [[GH-12078](https://github.com/hashicorp/nomad/issues/12078)]
* raft: The default raft protocol version is now 3 so you must follow the [Upgrading to Raft Protocol 3](https://www.nomadproject.io/docs/upgrade#upgrading-to-raft-protocol-3) guide when upgrading an existing cluster to Nomad 1.3.0. Downgrading the raft protocol version is not supported. [[GH-11572](https://github.com/hashicorp/nomad/issues/11572)]

SECURITY:

* server: validate mTLS certificate names on agent to agent endpoints [[GH-11956](https://github.com/hashicorp/nomad/issues/11956)]

IMPROVEMENTS:

* agent: Switch from boltdb/bolt to go.etcd.io/bbolt [[GH-12107](https://github.com/hashicorp/nomad/issues/12107)]
* api: Add `related` query parameter to the Evaluation details endpoint [[GH-12305](https://github.com/hashicorp/nomad/issues/12305)]
* api: Add support for filtering and pagination to the jobs and volumes list endpoint [[GH-12186](https://github.com/hashicorp/nomad/issues/12186)]
* api: Add support for filtering and pagination to the node list endpoint [[GH-12727](https://github.com/hashicorp/nomad/issues/12727)]
* api: Add support for filtering, sorting, and pagination to the ACL tokens and allocations list endpoint [[GH-12186](https://github.com/hashicorp/nomad/issues/12186)]
* api: Added ParseHCLOpts helper func to ease parsing HCLv1 jobspecs [[GH-12777](https://github.com/hashicorp/nomad/issues/12777)]
* api: CSI secrets for list and delete snapshots are now passed in HTTP headers [[GH-12144](https://github.com/hashicorp/nomad/issues/12144)]
* api: `AllocFS.Logs` now explicitly closes frames channel after being canceled [[GH-12248](https://github.com/hashicorp/nomad/issues/12248)]
* api: default to using `DefaultPooledTransport` client to support keep-alive by default [[GH-12492](https://github.com/hashicorp/nomad/issues/12492)]
* api: filter values of evaluation and deployment list api endpoints [[GH-12034](https://github.com/hashicorp/nomad/issues/12034)]
* api: sort return values of evaluation and deployment list api endpoints by creation index [[GH-12054](https://github.com/hashicorp/nomad/issues/12054)]
* build: make targets now respect GOBIN variable [[GH-12077](https://github.com/hashicorp/nomad/issues/12077)]
* build: upgrade and speedup circleci configuration [[GH-11889](https://github.com/hashicorp/nomad/issues/11889)]
* cli: Added -json flag to `nomad job {run,plan,validate}` to support parsing JSON formatted jobs [[GH-12591](https://github.com/hashicorp/nomad/issues/12591)]
* cli: Added -os flag to node status to display operating system name [[GH-12388](https://github.com/hashicorp/nomad/issues/12388)]
* cli: Added `nomad operator api` command to ease querying Nomad's HTTP API. [[GH-10808](https://github.com/hashicorp/nomad/issues/10808)]
* cli: CSI secrets argument for `volume snapshot list` has been made consistent with `volume snapshot delete` [[GH-12144](https://github.com/hashicorp/nomad/issues/12144)]
* cli: Return a redacted value for mount flags in the `volume status` command, instead of `<none>` [[GH-12150](https://github.com/hashicorp/nomad/issues/12150)]
* cli: `operator debug` command now skips generating pprofs to avoid a panic on Nomad 0.11.2. 0.11.1, and 0.11.0 [[GH-12807](https://github.com/hashicorp/nomad/issues/12807)]
* cli: add `nomad config validate` command to check configuration files without an agent [[GH-9198](https://github.com/hashicorp/nomad/issues/9198)]
* cli: added `-pprof-interval` to `nomad operator debug` command [[GH-11938](https://github.com/hashicorp/nomad/issues/11938)]
* cli: display the Raft version instead of the Serf protocol in the `nomad server members` command [[GH-12317](https://github.com/hashicorp/nomad/issues/12317)]
* cli: rename the `nomad server members` `-detailed` flag to `-verbose` so it matches other commands [[GH-12317](https://github.com/hashicorp/nomad/issues/12317)]
* client: Added `NOMAD_SHORT_ALLOC_ID` allocation env var [[GH-12603](https://github.com/hashicorp/nomad/issues/12603)]
* client: Allow interpolation of the network.dns block [[GH-12021](https://github.com/hashicorp/nomad/issues/12021)]
* client: Download up to 3 artifacts concurrently [[GH-11531](https://github.com/hashicorp/nomad/issues/11531)]
* client: Enable support for cgroups v2 [[GH-12274](https://github.com/hashicorp/nomad/issues/12274)]
* client: fingerprint AWS instance life cycle option [[GH-12371](https://github.com/hashicorp/nomad/issues/12371)]
* client: set NOMAD_CPU_CORES environment variable when reserving cpu cores [[GH-12496](https://github.com/hashicorp/nomad/issues/12496)]
* connect: automatically set alloc_id in envoy_stats_tags configuration [[GH-12543](https://github.com/hashicorp/nomad/issues/12543)]
* connect: bootstrap envoy sidecars using -proxy-for [[GH-12011](https://github.com/hashicorp/nomad/issues/12011)]
* consul/connect: write Envoy bootstrapping information to disk for debugging [[GH-11975](https://github.com/hashicorp/nomad/issues/11975)]
* consul: Added implicit Consul constraint for task groups utilising Consul service and check registrations [[GH-12602](https://github.com/hashicorp/nomad/issues/12602)]
* consul: add go-sockaddr templating support to nomad consul address [[GH-12084](https://github.com/hashicorp/nomad/issues/12084)]
* consul: improve service name validation message to include maximum length requirement [[GH-12012](https://github.com/hashicorp/nomad/issues/12012)]
* core: Enable configuring raft boltdb freelist sync behavior [[GH-12107](https://github.com/hashicorp/nomad/issues/12107)]
* core: The unused protocol_version agent configuration value has been removed. [[GH-11600](https://github.com/hashicorp/nomad/issues/11600)]
* csi: Add pagination parameters to `volume snapshot list` command [[GH-12193](https://github.com/hashicorp/nomad/issues/12193)]
* csi: Added `-secret` and `-parameter` flags to `volume snapshot create` command [[GH-12360](https://github.com/hashicorp/nomad/issues/12360)]
* csi: Added support for storage topology [[GH-12129](https://github.com/hashicorp/nomad/issues/12129)]
* csi: Allow for concurrent plugin allocations [[GH-12078](https://github.com/hashicorp/nomad/issues/12078)]
* csi: Allow volumes to be re-registered to be updated while not in use [[GH-12167](https://github.com/hashicorp/nomad/issues/12167)]
* csi: Display plugin capabilities in `nomad plugin status -verbose` output [[GH-12116](https://github.com/hashicorp/nomad/issues/12116)]
* csi: Respect the verbose flag in the output of `volume status` [[GH-12153](https://github.com/hashicorp/nomad/issues/12153)]
* csi: Sort allocations in `plugin status` output [[GH-12154](https://github.com/hashicorp/nomad/issues/12154)]
* csi: add flag for providing secrets as a set of key/value pairs to delete a volume [[GH-11245](https://github.com/hashicorp/nomad/issues/11245)]
* csi: allow namespace field to be passed in volume spec [[GH-12400](https://github.com/hashicorp/nomad/issues/12400)]
* deps: Update hashicorp/raft-boltdb to v2.2.0 [[GH-12107](https://github.com/hashicorp/nomad/issues/12107)]
* deps: Update serf library to v0.9.7 [[GH-12130](https://github.com/hashicorp/nomad/issues/12130)]
* deps: Updated hashicorp/consul-template to v0.29.0 [[GH-12747](https://github.com/hashicorp/nomad/issues/12747)]
* deps: Updated hashicorp/raft to v1.3.5 [[GH-12079](https://github.com/hashicorp/nomad/issues/12079)]
* deps: Upgrade kr/pty to creack/pty v1.1.5 [[GH-11855](https://github.com/hashicorp/nomad/issues/11855)]
* deps: use gorilla package for gzip http handler [[GH-11843](https://github.com/hashicorp/nomad/issues/11843)]
* drainer: defer draining CSI plugin jobs until system jobs are drained [[GH-12324](https://github.com/hashicorp/nomad/issues/12324)]
* drivers/raw_exec: Add support for cgroups v2 in raw_exec driver [[GH-12419](https://github.com/hashicorp/nomad/issues/12419)]
* drivers: removed support for restoring tasks created before Nomad 0.9 [[GH-12791](https://github.com/hashicorp/nomad/issues/12791)]
* fingerprint: add support for detecting DigitalOcean environment [[GH-12015](https://github.com/hashicorp/nomad/issues/12015)]
* metrics: Emit metrics regarding raft boltdb operations [[GH-12107](https://github.com/hashicorp/nomad/issues/12107)]
* metrics: emit `nomad.vault.token_last_renewal` and `nomad.vault.token_next_renewal` metrics for Vault token renewal information [[GH-12435](https://github.com/hashicorp/nomad/issues/12435)]
* namespaces: Allow adding custom metadata to namespaces. [[GH-12138](https://github.com/hashicorp/nomad/issues/12138)]
* namespaces: Allow enabling/disabling allowed drivers per namespace. [[GH-11807](https://github.com/hashicorp/nomad/issues/11807)]
* raft: The default raft protocol version is now 3. [[GH-11572](https://github.com/hashicorp/nomad/issues/11572)]
* scheduler: Seed node shuffling with the evaluation ID to make the order reproducible [[GH-12008](https://github.com/hashicorp/nomad/issues/12008)]
* scheduler: recover scheduler goroutines on panic [[GH-12009](https://github.com/hashicorp/nomad/issues/12009)]
* server: Transfer Raft leadership in case the Nomad server fails to establish leadership [[GH-12293](https://github.com/hashicorp/nomad/issues/12293)]
* server: store and check previous Raft protocol version to prevent downgrades [[GH-12362](https://github.com/hashicorp/nomad/issues/12362)]
* services: Enable setting arbitrary address on Nomad or Consul service registration [[GH-12720](https://github.com/hashicorp/nomad/issues/12720)]
* template: Upgraded to from consul-template v0.25.2 to v0.28.0 which includes the sprig library of functions and more. [[GH-12312](https://github.com/hashicorp/nomad/issues/12312)]
* ui: added visual indicators for disconnected allocations and client nodes [[GH-12544](https://github.com/hashicorp/nomad/issues/12544)]
* ui: break long service tags into multiple lines [[GH-11995](https://github.com/hashicorp/nomad/issues/11995)]
* ui: change sort-order of evaluations to be reverse-chronological [[GH-12847](https://github.com/hashicorp/nomad/issues/12847)]
* ui: make buttons with confirmation more descriptive of their actions [[GH-12252](https://github.com/hashicorp/nomad/issues/12252)]

DEPRECATIONS:

* Raft protocol version 2 is deprecated and will be removed in Nomad 1.4.0. [[GH-11572](https://github.com/hashicorp/nomad/issues/11572)]

BUG FIXES:

* api: Apply prefix filter when querying CSI volumes in all namespaces [[GH-12184](https://github.com/hashicorp/nomad/issues/12184)]
* cleanup: prevent leaks from time.After [[GH-11983](https://github.com/hashicorp/nomad/issues/11983)]
* client: Fixed a bug that could prevent a preempting alloc from ever starting. [[GH-12779](https://github.com/hashicorp/nomad/issues/12779)]
* client: Fixed a bug where clients that retry blocking queries would not reset the correct blocking duration [[GH-12593](https://github.com/hashicorp/nomad/issues/12593)]
* config: Fixed a bug where the `reservable_cores` setting was not respected [[GH-12044](https://github.com/hashicorp/nomad/issues/12044)]
* core: Fixed auto-promotion of canaries in jobs with at least one task group without canaries. [[GH-11878](https://github.com/hashicorp/nomad/issues/11878)]
* core: prevent malformed plans from crashing leader [[GH-11944](https://github.com/hashicorp/nomad/issues/11944)]
* csi: Fixed a bug where `plugin status` commands could choose the incorrect plugin if a plugin with a name that matched the same prefix existed. [[GH-12194](https://github.com/hashicorp/nomad/issues/12194)]
* csi: Fixed a bug where `volume snapshot list` did not correctly filter by plugin IDs. The `-plugin` parameter is required. [[GH-12197](https://github.com/hashicorp/nomad/issues/12197)]
* csi: Fixed a bug where allocations with volume claims would fail their first placement after a reschedule [[GH-12113](https://github.com/hashicorp/nomad/issues/12113)]
* csi: Fixed a bug where allocations with volume claims would fail to restore after a client restart [[GH-12113](https://github.com/hashicorp/nomad/issues/12113)]
* csi: Fixed a bug where creating snapshots required a plugin ID instead of falling back to the volume's plugin ID [[GH-12195](https://github.com/hashicorp/nomad/issues/12195)]
* csi: Fixed a bug where fields were missing from the Read Volume API response [[GH-12178](https://github.com/hashicorp/nomad/issues/12178)]
* csi: Fixed a bug where garbage collected nodes would block releasing a volume [[GH-12350](https://github.com/hashicorp/nomad/issues/12350)]
* csi: Fixed a bug where per-alloc volumes used the incorrect ID when querying for `alloc status -verbose` [[GH-12573](https://github.com/hashicorp/nomad/issues/12573)]
* csi: Fixed a bug where plugin configuration updates were not considered destructive [[GH-12774](https://github.com/hashicorp/nomad/issues/12774)]
* csi: Fixed a bug where plugins would not restart if they failed any time after a client restart [[GH-12752](https://github.com/hashicorp/nomad/issues/12752)]
* csi: Fixed a bug where plugins written in NodeJS could fail to fingerprint [[GH-12359](https://github.com/hashicorp/nomad/issues/12359)]
* csi: Fixed a bug where purging a job with a missing plugin would fail [[GH-12114](https://github.com/hashicorp/nomad/issues/12114)]
* csi: Fixed a bug where single-use access modes were not enforced during validation [[GH-12337](https://github.com/hashicorp/nomad/issues/12337)]
* csi: Fixed a bug where the maximum number of volume claims was incorrectly enforced when an allocation claims a volume [[GH-12112](https://github.com/hashicorp/nomad/issues/12112)]
* csi: Fixed a bug where the plugin instance manager would not retry the initial gRPC connection to plugins [[GH-12057](https://github.com/hashicorp/nomad/issues/12057)]
* csi: Fixed a bug where the plugin supervisor would not restart the task if it failed to connect to the plugin [[GH-12057](https://github.com/hashicorp/nomad/issues/12057)]
* csi: Fixed a bug where volume snapshot timestamps were always zero values [[GH-12352](https://github.com/hashicorp/nomad/issues/12352)]
* csi: Fixed bug where accessing plugins was subject to a data race [[GH-12553](https://github.com/hashicorp/nomad/issues/12553)]
* csi: fixed a bug where `volume detach`, `volume deregister`, and `volume status` commands did not accept an exact ID if multiple volumes matched the prefix [[GH-12051](https://github.com/hashicorp/nomad/issues/12051)]
* csi: provide `CSI_ENDPOINT` environment variable to plugin tasks [[GH-12050](https://github.com/hashicorp/nomad/issues/12050)]
* jobspec: Fixed a bug where connect sidecar resources were ignored when using HCL1 [[GH-11927](https://github.com/hashicorp/nomad/issues/11927)]
* lifecycle: Fixed a bug where successful poststart tasks were marked as unhealthy [[GH-11945](https://github.com/hashicorp/nomad/issues/11945)]
* recommendations (Enterprise): Fixed a bug where the recommendations list RPC incorrectly forwarded requests to the authoritative region [[GH-12040](https://github.com/hashicorp/nomad/issues/12040)]
* scheduler: fixed a bug where in-place updates on ineligible nodes would be ignored [[GH-12264](https://github.com/hashicorp/nomad/issues/12264)]
* server: Write peers.json file with correct permissions [[GH-12369](https://github.com/hashicorp/nomad/issues/12369)]
* template: Fixed a bug preventing allowing all consul-template functions. [[GH-12312](https://github.com/hashicorp/nomad/issues/12312)]
* template: Fixed a bug where the default `function_denylist` would be appended to a specified list [[GH-12071](https://github.com/hashicorp/nomad/issues/12071)]
* ui: Fix the link target for CSI volumes on the task detail page [[GH-11896](https://github.com/hashicorp/nomad/issues/11896)]
* ui: Fixed a bug where volumes were being incorrectly linked when per_alloc=true [[GH-12713](https://github.com/hashicorp/nomad/issues/12713)]
* ui: fix broken link to task-groups in the Recent Allocations table in the Job Detail overview page. [[GH-12765](https://github.com/hashicorp/nomad/issues/12765)]
* ui: fix the unit for the task row memory usage metric [[GH-11980](https://github.com/hashicorp/nomad/issues/11980)]

## Unsupported Versions

Versions of Nomad before 1.3.0 are no longer supported. See [CHANGELOG-unsupported.md](../CHANGELOG-unsupported.md) for their changelogs.
