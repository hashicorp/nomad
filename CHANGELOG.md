## 0.13.0 (Unreleased)

## 0.12.5 (September 17, 2020)

BUG FIXES:
 * core: Fixed a panic on job submission when the job contains a service with `expose = true` set [[GH-8882](https://github.com/hashicorp/nomad/issues/8882)]
 * core: Fixed a regression where stopping the sole job allocation result in two replacement allocations [[GH-8867](https://github.com/hashicorp/nomad/issues/8867)]
 * core: Fixed a bug where an allocation may be left running expectedly despite promoting a new job version [[GH-8886](https://github.com/hashicorp/nomad/issues/8886)]
 * cli: Fixed the whitespace in nomad monitor help output [[GH-8884](https://github.com/hashicorp/nomad/issues/8884)]
 * cli: Updated job samples to avoid using deprecated task level networks and mbit syntax [[GH-8911](https://github.com/hashicorp/nomad/issues/8911)]
 * cli: Fixed a bug where alloc signal fails if the CLI cannot contact the Nomad client directly [[GH-8897](https://github.com/hashicorp/nomad/issues/8897)]
 * cli: Fixed a bug where host volumes could cause `nomad node status` to panic when the `-verbose` flag was used. [[GH-8902](https://github.com/hashicorp/nomad/issues/8902)]
 * ui: Fixed ability to switch between tasks in alloc exec sessions [[GH-8856](https://github.com/hashicorp/nomad/issues/8856)]
 * ui: Task log streaming will no longer suddenly flip to a different task's logs. [[GH-8833](https://github.com/hashicorp/nomad/issues/8833)]

## 0.12.4 (September 9, 2020)

FEATURES:

 * **Consul Ingress Gateways**: Support for Consul Connect Ingress Gateways [[GH-8709](https://github.com/hashicorp/nomad/pull/8709)]

IMPROVEMENTS:

 * api: Added node purge SDK functionality. [[GH-8142](https://github.com/hashicorp/nomad/issues/8142)]
 * api: Added an option to stop multiregion jobs globally. [[GH-8776](https://github.com/hashicorp/nomad/issues/8776)]
 * core: Added `poststart` hook to task lifecycle [[GH-8390](https://github.com/hashicorp/nomad/pull/8390)]
 * csi: Improved the accuracy of plugin `Expected` allocation counts. [[GH-8699](https://github.com/hashicorp/nomad/pull/8699)]
 * driver/docker: Allow configurable image pull context timeout setting. [[GH-5718](https://github.com/hashicorp/nomad/issues/5718)]
 * ui: Added exec keepalive heartbeat. [[GH-8759](https://github.com/hashicorp/nomad/pull/8759)]

BUG FIXES:

 * core: Fixed a bug where unpromoted job versions are used when rescheduling failed allocations [[GH-8691](https://github.com/hashicorp/nomad/issues/8691)]
 * core: Fixed a bug where servers become unresponsive when cron jobs containing zero-padded months [[GH-8804](https://github.com/hashicorp/nomad/issues/8804)]
 * core: Fixed bugs where scaling policies could be matched against incorrect jobs with a similar prefix [[GH-8753](https://github.com/hashicorp/nomad/issues/8753)]
 * core: Fixed a bug where garbage collection evaluations that failed or spanned leader elections would be re-enqueued forever. [[GH-8682](https://github.com/hashicorp/nomad/issues/8682)]
 * core (Enterprise): Fixed a bug where enterprise servers may self-terminate as licenses are ignored after a Raft snapshot restore. [[GH-8737](https://github.com/hashicorp/nomad/issues/8737)]
 * cli (Enterprise): Fixed a panic in `nomad operator snapshot agent` if local path is not set [[GH-8809](https://github.com/hashicorp/nomad/issues/8809)]
 * client: Fixed a bug where `nomad operator debug` could cause a client agent to panic when the `-node-id` flag was used. [[GH-8795](https://github.com/hashicorp/nomad/issues/8795)]
 * csi: Fixed a bug where errors while connecting to plugins could cause a panic in the Nomad client. [[GH-8825](https://github.com/hashicorp/nomad/issues/8825)]
 * csi: Fixed a bug where querying CSI volumes would cause a panic if an allocation that claimed the volume had been garbage collected but the claim was not yet dropped. [[GH-8735](https://github.com/hashicorp/nomad/issues/8735)]
 * deployments (Enterprise): Fixed a bug where counts could not be changed in the web UI for multiregion jobs. [[GH-8685](https://github.com/hashicorp/nomad/issues/8685)]
 * deployments (Enterprise): Fixed a bug in multi-region deployments where a region that was dropped from the jobspec was not deregistered. [[GH-8763](https://github.com/hashicorp/nomad/issues/8763)]
 * exec: Fixed a bug causing escape characters to be missed in special cases [[GH-8798](https://github.com/hashicorp/nomad/issues/8798)]
 * plan: Fixed a bug where plans always included a change for the `NomadTokenID`. [[GH-8687](https://github.com/hashicorp/nomad/issues/8687)]

## 0.12.3 (August 13, 2020)

BUG FIXES:

 * csi: Fixed a panic in the API affecting both plugins and volumes. [[GH-8655](https://github.com/hashicorp/nomad/issues/8655)]

## 0.12.2 (August 12, 2020)

FEATURES:

 * **Multiple Vault Namespaces (Enterprise)**: Support for multiple Vault Namespaces [[GH-8453](https://github.com/hashicorp/nomad/issues/8453)]
 * **Scaling Observability UI**: View changes in task group scale (both manual and automatic) over time. [[GH-8551](https://github.com/hashicorp/nomad/issues/8551)]

IMPROVEMENTS:

 * cli: Move the `debug` command to `nomad operator debug` [[GH-8602](https://github.com/hashicorp/nomad/pull/8602)]
 * consul/connect: Added support for bridge networks with Connect Native tasks [[GH-8290](https://github.com/hashicorp/nomad/issues/8290)]
 * consul: Added support for setting `success_before_passing` and `failures_before_critical` on consul service checks. [[GH-6913](https://github.com/hashicorp/nomad/issues/6913)]
 * csi: Added a `nomad volume detach` command to manually detach unused volumes. [[GH-8584](https://github.com/hashicorp/nomad/issues/8584)]

BUG FIXES:

 * core: Fixed a bug where `nomad job plan` reports success and no updates if the job contains a scaling policy [[GH-8567](https://github.com/hashicorp/nomad/issues/8567)]
 * api: Added missing namespace field to scaling status GET response object [[GH-8530](https://github.com/hashicorp/nomad/issues/8530)]
 * api: Do not allow submission of jobs of type `system` that include task groups with scaling stanzas [[GH-8491](https://github.com/hashicorp/nomad/issues/8491)]
 * build: Updated to Go 1.14.7. Go 1.14.6 contained a CVE that is not believed to impact Nomad [[GH-8601](https://github.com/hashicorp/nomad/issues/8601)]
 * csi: Fixed a bug where ACL tokens were not used to call internal RPCs. [[GH-8373](https://github.com/hashicorp/nomad/issues/8373)]
 * csi: Fixed a bug where volumes could not be detached during node drains. [[GH-8580](https://github.com/hashicorp/nomad/issues/8580)]
 * csi: Fixed a bug where allocations in the API were omitted from plugins and volumes. [[GH-8362](https://github.com/hashicorp/nomad/issues/8362)]
 * csi: Fixed a bug where controller plugin RPCs would not be retried to a second controller if available. [[GH-8561](https://github.com/hashicorp/nomad/issues/8561)]
 * csi: Fixed a bug where retries of plugin RPCs would not gracefully resume from checkpoints in the workflow. [[GH-8605](https://github.com/hashicorp/nomad/issues/8605)]
 * csi: Fixed a bug causing errors during client deregistration if CSI node plugins did not fingerprint after stopping. [[GH-8619](https://github.com/hashicorp/nomad/issues/8619)]
 * csi: Fixed a bug where the `NodePublish` workflow incorrectly created target paths that should be created by the CSI plugin. [[GH-8505](https://github.com/hashicorp/nomad/issues/8505)]
 * csi: Fixed a bug in `nomad node status` where volumes attached to a node for an improperly cleaned-up allocation caused a panic in the CLI. [[GH-8525](https://github.com/hashicorp/nomad/issues/8525)]
 * deployments: Fixed a bug where Nomad Enterprise multi-region deployments would not leave "pending" status if namespaces were also in use.
 * vault: Fixed a bug where vault integration fails if Vault's /sys/init endpoint is disabled [[GH-8524](https://github.com/hashicorp/nomad/issues/8524)]
 * vault: Fixed a bug where upgrades from pre-0.11.3 that use Vault can lead to memory spikes and write large Raft messages. [[GH-8553](https://github.com/hashicorp/nomad/issues/8553)]
 * ui: Fixed various accessibility audit failures [[GH-8455](https://github.com/hashicorp/nomad/pull/8455)]
 * ui: Fixed global search navigation where job name ≠ ID [[GH-8560](https://github.com/hashicorp/nomad/pull/8560)]
 * ui: Fixed slow global search rendering by truncating results [[GH-8571](https://github.com/hashicorp/nomad/pull/8571)]

## 0.12.1 (July 23, 2020)

SECURITY:

 * build: Updated to Go 1.14.6. Go 1.14.5 contained 2 CVEs which are low severity for Nomad [[GH-8467](https://github.com/hashicorp/nomad/issues/8467)]

IMPROVEMENTS:

 * device/nvidia: Added a plugin config option to disable the plugin [[GH-8353](https://github.com/hashicorp/nomad/issues/8353)]

BUG FIXES:

 * core: Fixed an atomicity bug where a job may fail to start if leadership transition occured while processing the job [[GH-8435](https://github.com/hashicorp/nomad/issues/8435)]
 * core: Fixed a regression bug where jobs with group level networking stanza fail to be scheduled with "missing network" constraint error [[GH-8407](https://github.com/hashicorp/nomad/pull/8407)]
 * core (Enterprise): Fixed a bug where users were not given full 6 hours to apply initial license when upgrading from unlicensed versions of Nomad. [[GH-8457](https://github.com/hashicorp/nomad/issues/8457)]
 * client: Fixed a bug where `network_interface` client configuration was ignored [[GH-8486](https://github.com/hashicorp/nomad/issues/8486)]
 * jobspec: Fixed validation of multi-region datacenters to allow empty region `datacenters` to default to job-level `datacenters` [[GH-8426](https://github.com/hashicorp/nomad/issues/8426)]
 * scheduler: Fixed a bug in Nomad Enterprise where canaries were not being created during multi-region deployments [[GH-8456](https://github.com/hashicorp/nomad/pull/8456)]
 * ui: Fixed stale namespaces after changing acl tokens [[GH-8413](https://github.com/hashicorp/nomad/issues/8413)]
 * ui: Fixed inclusion of allocation when opening exec window [[GH-8460](https://github.com/hashicorp/nomad/pull/8460)]
 * ui: Fixed layout of parameterized/periodic job title elemetns [[GH-8495](https://github.com/hashicorp/nomad/pull/8495)]
 * ui: Fixed order of column headers in client allocations table [[GH-8409](https://github.com/hashicorp/nomad/pull/8409)]
 * ui: Fixed missing namespace query param after changing acl tokens [[GH-8413](https://github.com/hashicorp/nomad/issues/8413)]
 * ui: Fixed exec to derive group and task when possible from allocation [[GH-8463](https://github.com/hashicorp/nomad/pull/8463)]
 * ui: Fixed runtime error when clicking "Run Job" while a prefix filter is set [[GH-8412](https://github.com/hashicorp/nomad/issues/8412)]
 * ui: Fixed the absence of the region query parameter on various actions, such as job stop, allocation restart, node drain. [[GH-8477](https://github.com/hashicorp/nomad/issues/8477)]
 * ui: Fixed issue where an orphaned child job would make it so navigating to a job detail page would hang the UI [[GH-8319](https://github.com/hashicorp/nomad/issues/8319)]
 * ui: Fixed issue where clicking View Raw File in a non-default region would not provide the region param resulting in a 404 [[GH-8509](https://github.com/hashicorp/nomad/issues/8509)]
 * vault: Fixed a bug where vault identity policies not considered in permissions check [[GH-7732](https://github.com/hashicorp/nomad/issues/7732)]

## 0.12.0 (July 9, 2020)

FEATURES:
 * **Preemption**: Preemption is now an open source feature
 * **Licensing (Enterprise)**: Nomad Enterprise now requires a license [[GH-8076](https://github.com/hashicorp/nomad/issues/8076)]
 * **Multiregion Deployments (Enterprise)**: Nomad Enterprise now enables orchestrating deployments across multiple regions. [[GH-8184](https://github.com/hashicorp/nomad/issues/8184)]
 * **Snapshot Backup and Restore**: Nomad eases disaster recovery with new endpoints and commands for point-in-time snapshots.
 * **Debug Log Archive**: Nomad debug captures state and logs to facilitate support [[GH-8273](https://github.com/hashicorp/nomad/issues/8273)]
 * **Container Network Interface (CNI)**: Support for third-party vendors using the CNI plugin system. [[GH-7518](https://github.com/hashicorp/nomad/issues/7518)]
 * **Multi-interface Networking**: Support for scheduling on specific network interfaces. [[GH-8208](https://github.com/hashicorp/nomad/issues/8208)]
 * **Consul Connect Native**: Support for running Consul Connect Native tasks. [[GH-6083](https://github.com/hashicorp/nomad/issues/6083)]
 * **Global Search**: Access jobs and clients from anywhere in the UI using the always available global search bar. [[GH-8175](https://github.com/hashicorp/nomad/issues/8175)]
 * **Monitor UI**: Stream client and agent logs from the UI just like you would with the nomad monitor CLI command. [[GH-8177](https://github.com/hashicorp/nomad/issues/8177)]
 * **Scaling UI**: Quickly adjust the count of a task group from the UI for task groups with a scaling declaration. [[GH-8207](https://github.com/hashicorp/nomad/issues/8207)]

__BACKWARDS INCOMPATIBILITIES:__
 * driver/docker: The Docker driver no longer allows binding host volumes by default.
   Operators can set `volume` `enabled` plugin configuration to restore previous permissive behavior. [[GH-8261](https://github.com/hashicorp/nomad/issues/8261)]
 * driver/qemu: The Qemu driver requires images to reside in a operator-defined paths allowed for task access. [[GH-8261](https://github.com/hashicorp/nomad/issues/8261)]

IMPROVEMENTS:

* core: Support for persisting previous task group counts when updating a job [[GH-8168](https://github.com/hashicorp/nomad/issues/8168)]
* core: Block Job.Scale actions when the job is under active deployment [[GH-8187](https://github.com/hashicorp/nomad/issues/8187)]
* api: Better error messages around Scaling->Max [[GH-8360](https://github.com/hashicorp/nomad/issues/8360)]
* api: Persist previous count with scaling events [[GH-8167](https://github.com/hashicorp/nomad/issues/8167)]
* api: Support querying for jobs and allocations across all namespaces [[GH-8192](https://github.com/hashicorp/nomad/issues/8192)]
* api: New `/agent/host` endpoint returns diagnostic information about the host [[GH-8325](https://github.com/hashicorp/nomad/pull/8325)]
* build: Updated to Go 1.14.4 [[GH-8172](https://github.com/hashicorp/nomad/issues/9172)]
* build: Switched to Go modules for dependency management [[GH-8041](https://github.com/hashicorp/nomad/pull/8041)]
* connect: Infer service task parameter where possible [[GH-8274](https://github.com/hashicorp/nomad/issues/8274)]
* csi: Added `-force` flag to `nomad volume deregister` [[GH-8251](https://github.com/hashicorp/nomad/issues/8251)]
* server: Added `raft_multiplier` config to tweak Raft related timeouts [[GH-8082](https://github.com/hashicorp/nomad/issues/8082)]

BUG FIXES:

 * cli: Fixed malformed alloc status address list when listing more than 1 address [[GH-8161](https://github.com/hashicorp/nomad/issues/8161)]
 * client: Fixed a bug where stdout/stderr were not properly reopened for community task drivers [[GH-8155](https://github.com/hashicorp/nomad/issues/8155)]
 * client: Fixed a bug where batch job sidecars may be left running after the main task completes [[GH-8311](https://github.com/hashicorp/nomad/issues/8311)]
 * connect: Fixed a bug where custom `sidecar_task` definitions were being shared [[GH-8337](https://github.com/hashicorp/nomad/issues/8337)]
 * csi: Fixed a bug where `NodeStageVolume` and `NodePublishVolume` requests were not receiving volume context [[GH-8239](https://github.com/hashicorp/nomad/issues/8239)]
 * driver/docker: Fixed a bug to set correct value for `memory-swap` when using `memory_hard_limit` [[GH-8153](https://github.com/hashicorp/nomad/issues/8153)]
 * ui: The log streamer will now always follow logs when the current scroll position is the end of the buffer. [[GH-8177](https://github.com/hashicorp/nomad/issues/8177)]
 * ui: The task group detail page no longer makes excessive requests to the allocation and stats endpoints. [[GH-8216](https://github.com/hashicorp/nomad/issues/8216)]
 * ui: Polling endpoints that have yet to be fetched normally works as expected (regression from 0.11.3). [[GH-8207](https://github.com/hashicorp/nomad/issues/8207)]

## 0.11.4 (August 7, 2020)

SECURITY:

 * build: *Backport from v0.12.1* - Updated to Go 1.14.6. Go 1.14.5 contained 2 CVEs which are low severity for Nomad [[GH-8467](https://github.com/hashicorp/nomad/issues/8467)]

BUG FIXES:

 * vault: *Backport from v0.12.2* - Fixed a bug where upgrades from pre-0.11.3 that use Vault can lead to memory spikes and write large Raft messages. [[GH-8553](https://github.com/hashicorp/nomad/issues/8553)]

## 0.11.3 (June 5, 2020)

IMPROVEMENTS:

 * build: Updated to Go 1.14.3 [[GH-7431](https://github.com/hashicorp/nomad/issues/7970)]
 * csi: Return better error messages [[GH-7984](https://github.com/hashicorp/nomad/issues/7984)] [[GH-8030](https://github.com/hashicorp/nomad/issues/8030)]
 * csi: Move volume claim releases out of evaluation workers [[GH-8021](https://github.com/hashicorp/nomad/issues/8021)]
 * csi: Added support for `VolumeContext` and `VolumeParameters` [[GH-7957](https://github.com/hashicorp/nomad/issues/7957)]
 * driver/docker: Added support for `memory_hard_limit` configuration in docker task driver [[GH-2093](https://github.com/hashicorp/nomad/issues/2093)]
 * logging: Remove spurious error log on task shutdown [[GH-8028](https://github.com/hashicorp/nomad/issues/8028)]
 * ui: Added filesystem browsing for allocations [[GH-5871](https://github.com/hashicorp/nomad/pull/7951)]

BUG FIXES:

 * core: Fixed a critical bug causing agent to become unresponsive [[GH-7431](https://github.com/hashicorp/nomad/issues/7970)], [[GH-8163](https://github.com/hashicorp/nomad/issues/8163)]
 * core: Fixed a bug impacting performance of scheduler on a server after it steps down [[GH-8089](https://github.com/hashicorp/nomad/issues/8089)]
 * core: Fixed a bug where new leader may take a long time until it can process requests [[GH-8036](https://github.com/hashicorp/nomad/issues/8036)]
 * core: Fixed a bug where stop_after_client_disconnect could cause the server to become unresponsive [[GH-8098](https://github.com/hashicorp/nomad/issues/8098)
 * core: Fixed a bug where an internal metadata, ClusterID, may not be initialized properly upon a slow server upgrade [[GH-8078](https://github.com/hashicorp/nomad/issues/8078)]
 * api: Fixed a bug where setting connect sidecar task resources could fail [[GH-7993](https://github.com/hashicorp/nomad/issues/7993)]
 * client: Fixed a bug where artifact downloads failed on redirects [[GH-7854](https://github.com/hashicorp/nomad/issues/7854)]
 * csi: Validate empty volume arguments in API. [[GH-8027](https://github.com/hashicorp/nomad/issues/8027)]

## 0.11.2 (May 14, 2020)

FEATURES:
 * **Task dependencies UI**: task lifecycle charts and details

IMPROVEMENTS:

 * core: Added support for a per-group policy to stop tasks when a client is disconnected [[GH-2185](https://github.com/hashicorp/nomad/issues/2185)]
 * core: Allow spreading allocations as an alternative to binpacking [[GH-7810](https://github.com/hashicorp/nomad/issues/7810)]
 * client: Improve AWS CPU performance fingerprinting [[GH-7681](https://github.com/hashicorp/nomad/issues/7681)]
 * csi: Added support for volume secrets [[GH-7923](https://github.com/hashicorp/nomad/issues/7923)]
 * csi: Added periodic garbage collection of plugins and volume claims [[GH-7825](https://github.com/hashicorp/nomad/issues/7825)]
 * csi: Improved performance of volume claim releases by moving work out of scheduler [[GH-7794](https://github.com/hashicorp/nomad/issues/7794)]
 * driver/docker: Added support for custom runtimes [[GH-7932](https://github.com/hashicorp/nomad/pull/7932)]
 * ui: Added ACL-checking to conditionally turn off exec button [[GH-7919](https://github.com/hashicorp/nomad/pull/7919)]
 * ui: CSI searchable volumes and plugins pages [[GH-7895](https://github.com/hashicorp/nomad/issues/7895)]
 * ui: CSI plugins list and etail pages [[GH-7872](https://github.com/hashicorp/nomad/issues/7872)] [[GH-7911](https://github.com/hashicorp/nomad/issues/7911)]
 * ui: CSI volume constraints table [[GH-7872](https://github.com/hashicorp/nomad/issues/7872)]

BUG FIXES:

 * core: job scale status endpoint was returning incorrect counts [[GH-7789](https://github.com/hashicorp/nomad/issues/7789)]
 * core: Fixed bugs related to periodic jobs scheduled during daylight saving transition periods [[GH-7894](https://github.com/hashicorp/nomad/issues/7894)]
 * core: Fixed a bug where scores for allocations were biased toward nodes with resource reservations [[GH-7730](https://github.com/hashicorp/nomad/issues/7730)]
 * agent: Fine-tuned the severity level of http request failures [[GH-7785](https://github.com/hashicorp/nomad/pull/7785)]
 * api: api.ScalingEvent struct was missing .Count [[GH-7915](https://github.com/hashicorp/nomad/pulls/7915)]
 * api: validate scale count value is not negative [[GH-7902](https://github.com/hashicorp/nomad/issues/7902)]
 * api: autoscaling policies should not be returned for stopped jobs [[GH-7768](https://github.com/hashicorp/nomad/issues/7768)]
 * client: Fixed a bug where an multi-task allocation maybe considered unhealthy if some tasks are slow to start [[GH-7944](https://github.com/hashicorp/nomad/issues/7944)]
 * csi: Fixed checking of volume validation responses from plugins [[GH-7831](https://github.com/hashicorp/nomad/issues/7831)]
 * csi: Fixed counting of healthy and expected plugins after plugin job updates or stops [[GH-7844](https://github.com/hashicorp/nomad/issues/7844)]
 * csi: Added checkpointing to volume claim release to avoid unreleased claims on plugin errors [[GH-7782](https://github.com/hashicorp/nomad/issues/7782)]
 * driver/docker: Fixed a bug preventing garbage collecting unused docker images [[GH-7947](https://github.com/hashicorp/nomad/issues/7947)]
 * jobspec: autoscaling policy block should return a parsing error multiple `policy` blocks are provided [[GH-7716](https://github.com/hashicorp/nomad/issues/7716)]
 * ui: Fixed a bug where exec popup had incorrect URL for jobs where name ≠ id [[GH-7814](https://github.com/hashicorp/nomad/issues/7814)]
 * ui: Fixed a timeout issue where if the log stream request to a client eventually returns but only after the timeout it never gets closed [[GH-7820](https://github.com/hashicorp/nomad/issues/7820)]
 * ui: Setting a namespace on Volumes or Jobs persists that namespace choice when switching to another namespace-away page [[GH-7896](https://github.com/hashicorp/nomad/issues/7896)]
 * ui: Fixed a bug where clicking stdout or stderr when already on that clicked view would pause log streaming [[GH-7820](https://github.com/hashicorp/nomad/issues/7820)]
 * ui: Fixed a race condition that made swithing from stdout to stderr too quickly show an error [[GH-7820](https://github.com/hashicorp/nomad/issues/7820)]
 * ui: Switching namespaces now redirects to Volumes instead of Jobs when on a Storage page [[GH-7896](https://github.com/hashicorp/nomad/issues/7896)]
 * ui: Allocations always report resource reservations based on thier own job version [[GH-7855](https://github.com/hashicorp/nomad/issues/7855)]
 * vault: Fixed a bug where nomad retries revoking tokens indefinitely [[GH-7959](https://github.com/hashicorp/nomad/issues/7959)]

## 0.11.1 (April 22, 2020)

BUG FIXES:

 * core: Fixed a bug that only ran a task `shutdown_delay` if the task had a registered service [[GH-7663](https://github.com/hashicorp/nomad/issues/7663)]
 * core: Fixed a panic when garbage collecting a job with allocations spanning multiple versions [[GH-7758](https://github.com/hashicorp/nomad/issues/7758)]
 * agent: Fixed a bug where http server logs did not honor json log formatting, and reduced http server logging level to Trace [[GH-7748](https://github.com/hashicorp/nomad/issues/7748)]
 * connect: Fixed bugs where some connect parameters would be ignored [[GH-7690](https://github.com/hashicorp/nomad/pull/7690)] [[GH-7684](https://github.com/hashicorp/nomad/pull/7684)]
 * connect: Fixed a bug where an absent connect sidecar_service stanza would trigger panic [[GH-7683](https://github.com/hashicorp/nomad/pull/7683)]
 * connect: Fixed a bug where some connect proxy fields would be dropped from 'job inspect' output [[GH-7397](https://github.com/hashicorp/nomad/issues/7397)]
 * csi: Fixed a panic when claiming a volume for an allocation that was already garbage collected [[GH-7760](https://github.com/hashicorp/nomad/issues/7760)]
 * csi: Fixed a bug where CSI plugins with `NODE_STAGE_VOLUME` capabilities were receiving an incorrect volume ID [[GH-7754](https://github.com/hashicorp/nomad/issues/7754)]
 * driver/docker: Fixed a bug where retrying failed docker creation may in rare cases trigger a panic [[GH-7749](https://github.com/hashicorp/nomad/issues/7749)]
 * scheduler: Fixed a bug in managing allocated devices for a job allocation in in-place update scenarios [[GH-7762](https://github.com/hashicorp/nomad/issues/7762)]
 * vault: Upgrade http2 library to fix Vault API calls that fail with `http2: no cached connection was available` [[GH-7673](https://github.com/hashicorp/nomad/issues/7673)]

## 0.11.0 (April 8, 2020)

FEATURES:
 * **Container Storage Interface [beta]**: Nomad has expanded support
   of stateful workloads through support for CSI plugins.
 * **Exec UI**: an in-browser terminal for connecting to running allocations.
 * **Audit Logging (Enterprise)**: Audit logging support for Nomad
   Enterprise.
 * **Scaling APIs**: new scaling policy API and job scaling APIs to support external autoscalers
 * **Task Dependencies**: introduces `lifecycle` stanza with prestart and sidecar hooks for tasks within a task group


__BACKWARDS INCOMPATIBILITIES:__
 * driver/rkt: The Rkt driver is no longer packaged with Nomad and is instead
   distributed separately as a driver plugin. Further, the Rkt driver codebase
   is now in a separate
   [repository](https://github.com/hashicorp/nomad-driver-rkt).

IMPROVEMENTS:

 * core: Optimized streaming RPCs made between Nomad agents [[GH-7044](https://github.com/hashicorp/nomad/issues/7044)]
 * build: Updated to Go 1.14.1 [[GH-7431](https://github.com/hashicorp/nomad/issues/7431)]
 * consul: Added support for configuring `enable_tag_override` on service stanzas. [[GH-2057](https://github.com/hashicorp/nomad/issues/2057)]
 * client: Updated consul-template library to v0.24.1 - added support for working with consul connect. [Deprecated vault_grace](https://nomadproject.io/guides/upgrade/upgrade-specific/#nomad-0110) [[GH-7170](https://github.com/hashicorp/nomad/pull/7170)]
 * driver/exec: Added `no_pivot_root` option for ramdisk use [[GH-7149](https://github.com/hashicorp/nomad/issues/7149)]
 * jobspec: Added task environment interpolation to `volume_mount` [[GH-7364](https://github.com/hashicorp/nomad/issues/7364)]
 * jobspec: Added support for a per-task restart policy [[GH-7288](https://github.com/hashicorp/nomad/pull/7288)]
 * server: Added minimum quorum check to Autopilot with minQuorum option [[GH-7171](https://github.com/hashicorp/nomad/issues/7171)]
 * connect: Added support for specifying Envoy expose path configurations [[GH-7323](https://github.com/hashicorp/nomad/pull/7323)] [[GH-7396](https://github.com/hashicorp/nomad/pull/7515)]
 * connect: Added support for using Connect with TLS enabled Consul agents [[GH-7602](https://github.com/hashicorp/nomad/pull/7602)]

BUG FIXES:

 * core: Fixed a bug where group network mode changes were not honored [[GH-7414](https://github.com/hashicorp/nomad/issues/7414)]
 * core: Optimized and fixed few bugs in underlying RPC handling [[GH-7044](https://github.com/hashicorp/nomad/issues/7044)] [[GH-7045](https://github.com/hashicorp/nomad/issues/7045)]
 * api: Fixed a panic when canonicalizing a jobspec with an incorrect job type [[GH-7207](https://github.com/hashicorp/nomad/pull/7207)]
 * api: Fixed a bug where calling the node GC or GcAlloc endpoints resulted in an error EOF return on successful requests [[GH-5970](https://github.com/hashicorp/nomad/issues/5970)]
 * api: Fixed a bug where `/client/allocations/...` (e.g. allocation stats) requests may hang in special cases after a leader election [[GH-7370](https://github.com/hashicorp/nomad/issues/7370)]
 * cli: Fixed a bug where `nomad agent -dev` fails on Windows [[GH-7534](https://github.com/hashicorp/nomad/pull/7534)]
 * cli: Fixed a panic when displaying device plugins without stats [[GH-7231](https://github.com/hashicorp/nomad/issues/7231)]
 * cli: Fixed a bug where `alloc exec` command in TLS environments may fail [[GH-7274](https://github.com/hashicorp/nomad/issues/7274)]
 * client: Fixed a panic when running in Debian with `/etc/debian_version` is empty [[GH-7350](https://github.com/hashicorp/nomad/issues/7350)]
 * client: Fixed a bug affecting network detection in environments that mimic the EC2 Metadata API [[GH-7509](https://github.com/hashicorp/nomad/issues/7509)]
 * client: Fixed a bug where a multi-task allocation maybe considered healthy despite a task restarting [[GH-7383](https://github.com/hashicorp/nomad/issues/7383)]
 * consul: Fixed a bug where modified Consul service definitions would not be updated [[GH-6459](https://github.com/hashicorp/nomad/issues/6459)]
 * connect: Fixed a bug where Connect enabled allocation would not stop after promotion [[GH-7540](https://github.com/hashicorp/nomad/issues/7540)]
 * connect: Fixed a bug where restarting a client would prevent Connect enabled allocations from cleaning up properly [[GH-7643](https://github.com/hashicorp/nomad/issues/7643)]
 * driver/docker: Fixed handling of seccomp `security_opts` option [[GH-7554](https://github.com/hashicorp/nomad/issues/7554)]
 * driver/docker: Fixed a bug causing docker containers to use swap memory unexpectedly [[GH-7550](https://github.com/hashicorp/nomad/issues/7550)]
 * scheduler: Fixed a bug where changes to task group `shutdown_delay` were not persisted or displayed in plan output [[GH-7618](https://github.com/hashicorp/nomad/issues/7618)]
 * ui: Fixed handling of multi-byte unicode characters in allocation log view [[GH-7470](https://github.com/hashicorp/nomad/issues/7470)] [[GH-7551](https://github.com/hashicorp/nomad/pull/7551)]

## 0.10.5 (March 24, 2020)

SECURITY:

 * server: Override content-type headers for unsafe content. CVE-2020-10944 [[GH-7468](https://github.com/hashicorp/nomad/issues/7468)]

## 0.10.4 (February 19, 2020)

FEATURES:

 * api: Nomad now supports ability to remotely request /debug/pprof endpoints from a remote agent. [[GH-6841](https://github.com/hashicorp/nomad/issues/6841)]
 * consul/connect: Nomad may now register Consul Connect services when Consul is configured with ACLs enabled [[GH-6701](https://github.com/hashicorp/nomad/issues/6701)]
 * jobspec: Add `shutdown_delay` to task groups so task groups can delay shutdown after deregistering from Consul [[GH-6746](https://github.com/hashicorp/nomad/issues/6746)]

IMPROVEMENTS:

 * Our Windows 32-bit and 64-bit executables for this version and up will be signed with a HashiCorp cert. Windows users will no longer see a warning about an "unknown publisher" when running our software.
 * build: Updated to Go 1.12.16 [[GH-7009](https://github.com/hashicorp/nomad/issues/7009)]
 * cli: Included namespace in output when querying job status [[GH-6912](https://github.com/hashicorp/nomad/issues/6912)]
 * cli: Added option to change the name of the file created by the `nomad init` command [[GH-6520]](https://github.com/hashicorp/nomad/pull/6520)
 * client: Supported AWS EC2 Instance Metadata Service Version 2 (IMDSv2) [[GH-6779](https://github.com/hashicorp/nomad/issues/6779)]
 * consul: Add support for service `canary_meta` [[GH-6690](https://github.com/hashicorp/nomad/pull/6690)]
 * driver/docker: Added a `disable_log_collection` parameter to disable nomad log collection [[GH-6820](https://github.com/hashicorp/nomad/issues/6820)]
 * server: Introduced a `default_scheduler_config` config parameter to seed initial preemption configuration. [[GH-6935](https://github.com/hashicorp/nomad/issues/6935)]
 * scheduler: Removed penalty for allocation's previous node if the allocation did not fail. [[GH-6781](https://github.com/hashicorp/nomad/issues/6781)]
 * scheduler: Reduced logging verbosity during preemption [[GH-6849](https://github.com/hashicorp/nomad/issues/6849)]
 * ui: Updated Run Job button to be conditionally enabled according to ACLs [[GH-5944](https://github.com/hashicorp/nomad/pull/5944)]

BUG FIXES:

 * agent: Fixed a panic when using `nomad monitor` on a client node [[GH-7053](https://github.com/hashicorp/nomad/issues/7053)]
 * agent: Fixed race condition in logging when using `nomad monitor` command [[GH-6872](https://github.com/hashicorp/nomad/issues/6872)]
 * agent: Fixed a bug where `nomad monitor -server-id` only work for a server's name instead of uuid or name [[GH-7015](https://github.com/hashicorp/nomad/issues/7015)]
 * core: Addressed an inconsistency where allocations created prior to 0.9 had missing fields [[GH-6922](https://github.com/hashicorp/nomad/issues/6922)]
 * cli: Fixed a bug where error messages appeared interleaved with help text inconsistently [[GH-6865](https://github.com/hashicorp/nomad/issues/6865)]
 * cli: Fixed a bug where `nomad monitor -node-id` would cause a cli panic when no nodes where found [[GH-6828](https://github.com/hashicorp/nomad/issues/6828)]
 * config: Fixed a bug where agent startup would fail if the `consul.timeout` configuration was set [[GH-6907](https://github.com/hashicorp/nomad/issues/6907)]
 * consul: Fixed a bug where script-based health checks would fail if the service configuration included interpolation [[GH-6916](https://github.com/hashicorp/nomad/issues/6916)]
 * consul/connect: Fixed a bug where Connect-enabled jobs failed to validate when service names used interpolation [[GH-6855](https://github.com/hashicorp/nomad/issues/6855)]
 * drivers: Fixed a bug where exec, java, and raw_exec drivers collected and emited stats every second regardless of the telemetry config [[GH-7043](https://github.com/hashicorp/nomad/issues/7043)]
 * driver/exec: Fixed a bug where systemd cgroup wasn't removed upon a task completion [[GH-6839](https://github.com/hashicorp/nomad/issues/6839)]
 * server: Fixed a deadlock that may occur when server leadership flaps very quickly [[GH-6977](https://github.com/hashicorp/nomad/issues/6977)]
 * scheduler: Fixed a bug that caused evicted allocs on a lost node to be stuck in running [[GH-6902](https://github.com/hashicorp/nomad/issues/6902)]
 * scheduler: Fixed a bug where `nomad job plan/apply` returned errors instead of ignoring system job updates for ineligible nodes. [[GH-6996](https://github.com/hashicorp/nomad/issues/6996)]
 * scheduler: Fixed a bug where canary allocations where not properly stored across servers during deployments [[GH-6975](https://github.com/hashicorp/nomad/pull/6975)]

SECURITY:

 * client: Nomad will no longer pass through the `CONSUL_HTTP_TOKEN` environment variable when launching a task. [[GH-7131](https://github.com/hashicorp/nomad/issues/7131)]

## 0.10.3 (January 29, 2020)

SECURITY:

 * agent: Added unauthenticated connection timeouts and limits to prevent resource exhaustion. CVE-2020-7218 [[GH-7002](https://github.com/hashicorp/nomad/issues/7002)]
 * server: Fixed insufficient validation for role and region for RPC connections when TLS enabled. CVE-2020-7956 [[GH-7003](https://github.com/hashicorp/nomad/issues/7003)]

IMPROVEMENTS:

 * build: Updated to Go 1.12.16

## 0.10.2 (December 4, 2019)

NOTES:

* cli: Our [nomad_0.10.2_darwin_amd64_notarized](https://releases.hashicorp.com/nomad/0.10.2/nomad_0.10.2_darwin_amd64_notarized.zip) release has been signed and notarized according to Apple's requirements. In the future, darwin releases will be signed and notarized with our standard naming convention.

    Prior to this release, MacOS 10.15+ users attempting to run our software may see the error: "'nomad' cannot be opened because the developer cannot be verified." This error affected all MacOS 10.15+ users who downloaded our software directly via web browsers, and was caused by [changes to Apple's third-party software requirements](https://developer.apple.com/news/?id=04102019a).

    MacOS 10.15+ users should plan to upgrade to 0.10.2+.

FEATURES:

 * **Nomad Monitor**: New `nomad monitor` command allows remotely following
   the logs of any Nomad Agent (clients or servers). See
   https://nomadproject.io/docs/commands/monitor.html
 * **Docker Container Cleanup**: Nomad will now automatically remove Docker
   containers for tasks leaked due to Nomad or Docker crashes or bugs.

IMPROVEMENTS:

 * agent: Added support for running under Windows Service Manager [[GH-6220](https://github.com/hashicorp/nomad/issues/6220)]
 * api: Added `StartedAt` field to `Node.DrainStrategy` [[GH-6698](https://github.com/hashicorp/nomad/issues/6698)]
 * api: Added JSON representation of rules to policy endpoint response [[GH-6017](https://github.com/hashicorp/nomad/pull/6017)]
 * api: Update policy endpoint to permit anonymous access [[GH-6021](https://github.com/hashicorp/nomad/issues/6021)]
 * build: Updated to Go 1.12.13 [[GH-6606](https://github.com/hashicorp/nomad/issues/6606)]
 * cli: Show full ID in node and alloc individual status views [[GH-6425](https://github.com/hashicorp/nomad/issues/6425)]
 * client: Enable setting tags on Consul Connect sidecar service [[GH-6448](https://github.com/hashicorp/nomad/issues/6448)]
 * client: Added support for downloading artifacts from Google Cloud Storage [[GH-6692](https://github.com/hashicorp/nomad/pull/6692)]
 * command: Added -tls-server-name flag [[GH-6370](https://github.com/hashicorp/nomad/issues/6370)]
 * command: Added `nomad monitor` command to stream logs at a specified level for debugging [[GH-6499](https://github.com/hashicorp/nomad/issues/6499)]
 * quota: Added support for network bandwidth quota limits in Nomad enterprise

BUG FIXES:

 * core: Ignore `server` config values if `server` is disabled [[GH-6047](https://github.com/hashicorp/nomad/issues/6047)]
 * core: Added `semver` constraint for strict Semver 2.0 version comparisons [[GH-6699](https://github.com/hashicorp/nomad/issues/6699)]
 * core: Fixed server panic caused by a plan evicting and preempting allocs on a node [[GH-6792](https://github.com/hashicorp/nomad/issues/6792)]
 * api: Return a 404 if endpoint not found instead of redirecting to /ui/ [[GH-6658](https://github.com/hashicorp/nomad/issues/6658)]
 * api: Decompress web socket response body if gzipped on error responses [[GH-6650](https://github.com/hashicorp/nomad/issues/6650)]
 * api: Fixed a bug where some FS/Allocation API endpoints didn't return error messages [[GH-6427](https://github.com/hashicorp/nomad/issues/6427)]
 * api: Return 40X status code for failing ACL requests, rather than 500 [[GH-6421](https://github.com/hashicorp/nomad/issues/6421)]
 * cli: Made scoring column orders consistent `nomad alloc status` [[GH-6609](https://github.com/hashicorp/nomad/issues/6609)]
 * cli: Fixed a bug where `nomad alloc exec` fails if stdout is being redirected and not a TTY [[GH-6684](https://github.com/hashicorp/nomad/issues/6684)]
 * cli: Fixed a bug where a cli user may fail to query FS/Allocation API endpoints if they lack `node:read` capability [[GH-6423](https://github.com/hashicorp/nomad/issues/6423)]
 * client: client: Return empty values when host stats fail [[GH-6349](https://github.com/hashicorp/nomad/issues/6349)]
 * client: Fixed a bug where a client may not restart dead internal processes upon client's restart on Windows [[GH-6426](https://github.com/hashicorp/nomad/issues/6426)]
 * consul/connect: Fixed registering multiple Connect-enabled services in the same task group [[GH-6646](https://github.com/hashicorp/nomad/issues/6646)]
 * drivers: Fixed a bug where client may panic if a restored task failed to shutdown cleanly [[GH-6763](https://github.com/hashicorp/nomad/issues/6763)]
 * driver/exec: Fixed a bug where exec tasks can spawn processes that live beyond task lifecycle [[GH-6722](https://github.com/hashicorp/nomad/issues/6722)]
 * driver/docker: Added mechanism for detecting running unexpectedly running docker containers [[GH-6325](https://github.com/hashicorp/nomad/issues/6325)]
 * scheduler: Changes to devices in resource stanza should cause rescheduling [[GH-6644](https://github.com/hashicorp/nomad/issues/6644)]
 * scheduler: Fixed a bug that allowed inplace updates after affinity or spread were changed [[GH-6703](https://github.com/hashicorp/nomad/issues/6703)]
 * ui: Fixed client sorting [[GH-6817](https://github.com/hashicorp/nomad/issues/6817)]
 * vault: Allow overriding implicit Vault version constraint [[GH-6687](https://github.com/hashicorp/nomad/issues/6687)]
 * vault: Supported Vault auth role's new fields, `token_period` and `token_explicit_max_ttl` [[GH-6574](https://github.com/hashicorp/nomad/issues/6574)], [[GH-6580](https://github.com/hashicorp/nomad/issues/6580)]

## 0.10.1 (November 4, 2019)

BUG FIXES:

 * core: Fixed server panic when upgrading from 0.8 -> 0.10 and performing an
   inplace update of an allocation. [[GH-6541](https://github.com/hashicorp/nomad/issues/6541)]
 * api: Fixed panic when submitting Connect-enabled job without using a bridge
   network [[GH-6575](https://github.com/hashicorp/nomad/issues/6575)]
 * client: Fixed client panic when upgrading from 0.8 -> 0.10 and performing an
   inplace update of an allocation. [[GH-6605](https://github.com/hashicorp/nomad/issues/6605)]

## 0.10.0 (October 22, 2019)

FEATURES:
 * **Consul Connect**: Nomad may now register Consul Connect services and
   manages an Envoy proxy sidecar to provide secured service-to-service
   communication.
 * **Network Namespaces**: Task Groups may now define a shared network
   namespace. Each allocation will receive its own network namespace and
   loopback interface. Ports may be forwarded from the host into the network
   namespace.
 * **Host Volumes**: Nomad expanded support of stateful workloads through locally mounted storage volumes.
 * **UI Allocation File Explorer**: Nomad UI enhanced operability with a visual file system explorer for allocations.

IMPROVEMENTS:
 * core: Added rolling deployments for service jobs by default and max_parallel=0 disables deployments [[GH-6191](https://github.com/hashicorp/nomad/pull/6100)]
 * agent: Allowed the job GC interval to be configured [[GH-5978](https://github.com/hashicorp/nomad/issues/5978)]
 * agent: Added `log_level` to be reloaded on SIGHUP [[GH-5996](https://github.com/hashicorp/nomad/pull/5996)]
 * api: Added follow parameter to file streaming endpoint to support older browsers [[GH-6049](https://github.com/hashicorp/nomad/issues/6049)]
 * client: Upgraded `go-getter` to support GCP links [[GH-6215](https://github.com/hashicorp/nomad/pull/6215)]
 * client: Remove consul service stanza from `job init --short` jobspec [[GH-6179](https://github.com/hashicorp/nomad/issues/6179)]
 * drivers: Exposed namespace as `NOMAD_NAMESPACE` environment variable in running tasks [[GH-6192](https://github.com/hashicorp/nomad/pull/6192)]
 * metrics: Added job status (pending, running, dead) metrics [[GH-6003](https://github.com/hashicorp/nomad/issues/6003)]
 * metrics: Added status and scheduling ability to client metrics [[GH-6130](https://github.com/hashicorp/nomad/issues/6130)]
 * server: Added an option to configure job GC interval [[GH-5978](https://github.com/hashicorp/nomad/issues/5978)]
 * ui: Added allocation filesystem explorer [[GH-5871](https://github.com/hashicorp/nomad/pull/5871)]
 * ui: Added creation time to evaluations table [[GH-6050](https://github.com/hashicorp/nomad/pull/6050)]

BUG FIXES:

 * cli: Fixed `nomad run ...` on Windows so it works with unprivileged accounts [[GH-6009](https://github.com/hashicorp/nomad/issues/6009)]
 * client: Fixed a bug in client fingerprinting on 32-bit nodes [[GH-6239](https://github.com/hashicorp/nomad/issues/6239)]
 * client: Fixed a bug where completed allocations may re-run after client restart [[GH-6216](https://github.com/hashicorp/nomad/issues/6216)]
 * client: Fixed failure to start if another client is already running with the same data directory [[GH-6348](https://github.com/hashicorp/nomad/pull/6348)]
 * client: Fixed a panic that may occur when an `nomad alloc exec` is initiated while process is terminating [[GH-6065](https://github.com/hashicorp/nomad/issues/6065)]
 * devices: Fixed a bug causing CPU usage spike when a device is detected [[GH-6201](https://github.com/hashicorp/nomad/issues/6201)]
 * drivers: Allowd user-defined environment variable keys to contain dashes [[GH-6080](https://github.com/hashicorp/nomad/issues/6080)]
 * driver/docker: Set gc image_delay default to 3 minutes [[GH-6078](https://github.com/hashicorp/nomad/pull/6078)]
 * driver/docker: Improved docker driver handling of container creation or starting failures [[GH-6326](https://github.com/hashicorp/nomad/issues/6326)], [[GH-6346](https://github.com/hashicorp/nomad/issues/6346)]
 * ui: Fixed a bug where the allocation log viewer would render HTML or hide content that matched XML syntax [[GH-6048](https://github.com/hashicorp/nomad/issues/6048)]
 * ui: Fixed a bug where allocation log viewer doesn't show all content in Firefox [[GH-6466](https://github.com/hashicorp/nomad/issues/6466)]
 * ui: Fixed navigation via clicking recent allocation row [[GH-6087](https://github.com/hashicorp/nomad/pull/6087)]
 * ui: Fixed a bug where the allocation log viewer would render HTML or hide content that matched XML syntax [[GH-6048](https://github.com/hashicorp/nomad/issues/6048)]
 * ui: Fixed a bug where allocation log viewer doesn't show all content in Firefox [[GH-6466](https://github.com/hashicorp/nomad/issues/6466)]

## 0.9.7 (December 4, 2019)

BUG FIXES:

 * core: Fixed server panic caused by a plan evicting and preempting allocs on a node [[GH-6792](https://github.com/hashicorp/nomad/issues/6792)]

## 0.9.6 (October 7, 2019)

SECURITY:

 * core: Redacted replication token in agent/self API endpoint.  The replication token is a management token that can be used for further privilege escalation. CVE-2019-12741 [[GH-6430](https://github.com/hashicorp/nomad/issues/6430)]
 * core: Fixed a bug where a user may start raw_exec task on clients despite driver being disabled. CVE-2019-15928 [[GH-6227](https://github.com/hashicorp/nomad/issues/6227)] [[GH-6431](https://github.com/hashicorp/nomad/issues/6431)]
 * enterprise/acl: Fix ACL access checks in Nomad Enterprise where users may query allocation information and perform lifecycle actions in namespaces they are not authorized to. CVE-2019-16742 [[GH-6432](https://github.com/hashicorp/nomad/issues/6432)]

IMPROVEMENTS:

 * client: Reduced memory footprint of nomad logging and executor processes [[GH-6341](https://github.com/hashicorp/nomad/issues/6341)]

BUG FIXES:

 * core: Fixed a bug where scheduler may schedule an allocation on a node without required drivers [[GH-6227](https://github.com/hashicorp/nomad/issues/6227)]
 * client: Fixed a bug where completed allocations may re-run after client restart [[GH-6216](https://github.com/hashicorp/nomad/issues/6216)] [[GH-6207](https://github.com/hashicorp/nomad/issues/6207)]
 * devices: Fixed a bug causing CPU usage spike when a device is detected [[GH-6201](https://github.com/hashicorp/nomad/issues/6201)]
 * drivers: Fixed port mapping for docker and qemu drivers [[GH-6251](https://github.com/hashicorp/nomad/pull/6251)]
 * drivers/docker: Fixed a case where a `nomad alloc exec` would never time out [[GH-6144](https://github.com/hashicorp/nomad/pull/6144)]
 * ui: Fixed a bug where allocation log viewer doesn't show all content. [[GH-6048](https://github.com/hashicorp/nomad/issues/6048)]

## 0.9.5 (21 August 2019)

SECURITY:

 * client/template: Fix security vulnerabilities associated with task template rendering (CVE-2019-14802), introduced in Nomad 0.5.0 [[GH-6055](https://github.com/hashicorp/nomad/issues/6055)] [[GH-6075](https://github.com/hashicorp/nomad/issues/6075)]
 * client/artifact: Fix a privilege escalation in the `exec` driver exploitable by artifacts with setuid permissions (CVE-2019-14803) [[GH-6176](https://github.com/hashicorp/nomad/issues/6176)]

__BACKWARDS INCOMPATIBILITIES:__

 * client/template: When rendering a task template, only task environment variables are included by default. [[GH-6055](https://github.com/hashicorp/nomad/issues/6055)]
 * client/template: When rendering a task template, the `plugin` function is no longer permitted by default and will raise an error. [[GH-6075](https://github.com/hashicorp/nomad/issues/6075)]
 * client/template: When rendering a task template, path parameters for the `file` function will be restricted to the task directory by default. Relative paths or symlinks that point outside the task directory will raise an error. [[GH-6075](https://github.com/hashicorp/nomad/issues/6075)]

IMPROVEMENTS:
 * core: Added create and modify timestamps to evaluations [[GH-5881](https://github.com/hashicorp/nomad/pull/5881)]

BUG FIXES:
 * api: Fixed job region to default to client node region if none provided [[GH-6064](https://github.com/hashicorp/nomad/pull/6064)]
 * ui: Fixed links containing IPv6 addresses to include required square brackets [[GH-6007](https://github.com/hashicorp/nomad/pull/6007)]
 * vault: Fix deadlock when reloading server Vault configuration [[GH-6082](https://github.com/hashicorp/nomad/issues/6082)]

## 0.9.4 (July 30, 2019)

IMPROVEMENTS:
 * api: Inferred content type of file in alloc filesystem stat endpoint [[GH-5907](https://github.com/hashicorp/nomad/issues/5907)]
 * api: Used region from job hcl when not provided as query parameter in job registration and plan endpoints [[GH-5664](https://github.com/hashicorp/nomad/pull/5664)]
 * core: Deregister nodes in batches rather than one at a time [[GH-5784](https://github.com/hashicorp/nomad/pull/5784)]
 * core: Removed deprecated upgrade path code pertaining to older versions of Nomad [[GH-5894](https://github.com/hashicorp/nomad/issues/5894)]
 * core: System jobs that fail because of resource availability are retried when resources are freed [[GH-5900](https://github.com/hashicorp/nomad/pull/5900)]
 * core: Support reloading log level in agent via SIGHUP [[GH-5996](https://github.com/hashicorp/nomad/issues/5996)]
 * client: Improved task event display message to include kill time out [[GH-5943](https://github.com/hashicorp/nomad/issues/5943)]
 * client: Removed extraneous information to improve formatting for hcl parsing error messages [[GH-5972](https://github.com/hashicorp/nomad/pull/5972)]
 * driver/docker: Added logging defaults to use json-file log driver with log rotation [[GH-5846](https://github.com/hashicorp/nomad/pull/5846)]
 * metrics: Added namespace label as appropriate to metrics [[GH-5847](https://github.com/hashicorp/nomad/issues/5847)]
 * ui: Added page titles [[GH-5924](https://github.com/hashicorp/nomad/pull/5924)]
 * ui: Added buttons to copy client and allocation UUIDs [[GH-5926](https://github.com/hashicorp/nomad/pull/5926)]
 * ui: Moved client status, draining, and eligibility fields into single state column [[GH-5789](https://github.com/hashicorp/nomad/pull/5789)]

BUG FIXES:

 * core: Ensure plans are evaluated against a new enough snapshot index [[GH-5791](https://github.com/hashicorp/nomad/issues/5791)]
 * core: Handle error case when attempting to stop a non-existent allocation [[GH-5865](https://github.com/hashicorp/nomad/issues/5865)]
 * core: Improved job spec parsing error messages for variable interpolation failures [[GH-5844](https://github.com/hashicorp/nomad/issues/5844)]
 * core: Fixed a bug where nomad log and exec requests may time out or fail in tls enabled clusters [[GH-5954](https://github.com/hashicorp/nomad/issues/5954)].
 * client: Fixed a bug where consul service health checks may flap on client restart [[GH-5837](https://github.com/hashicorp/nomad/issues/5837)]
 * client: Fixed a bug where too many check-based restarts would deadlock the client [[GH-5975](https://github.com/hashicorp/nomad/issues/5975)]
 * client: Fixed a bug where successfully completed tasks may restart on client restart [[GH-5890](https://github.com/hashicorp/nomad/issues/5890)]
 * client: Fixed a bug where stats of external driver plugins aren't collected on plugin restart [[GH-5948](https://github.com/hashicorp/nomad/issues/5948)]
 * client: Fixed an issue where an alloc remains in pending state if nomad fails to create alloc directory [[GH-5905](https://github.com/hashicorp/nomad/issues/5905)]
 * client: Fixed an issue where client may kill running allocs if the client and the leader are restarting simultaneously [[GH-5906](//github.com/hashicorp/nomad/issues/5906)]
 * client: Fixed regression that prevented registering multiple services with the same name but different ports in Consul correctly [[GH-5829](https://github.com/hashicorp/nomad/issues/5829)]
 * client: Fixed a race condition when performing local task restarts that would result in incorrect task not found errors on Windows [[GH-5899](https://github.com/hashicorp/nomad/pull/5889)]
 * client: Reduce CPU usage on clients running many tasks on Linux [[GH-5951](https://github.com/hashicorp/nomad/pull/5951)]
 * client: Updated consul-template dependency to address issue with anonymous requests [[GH-5976](https://github.com/hashicorp/nomad/issues/5976)]
 * driver: Fixed an issue preventing local task restarts on Windows [[GH-5864](https://github.com/hashicorp/nomad/pull/5864)]
 * driver: Fixed an issue preventing external driver plugins from launching executor process [[GH-5726](https://github.com/hashicorp/nomad/issues/5726)]
 * driver/docker: Fixed a bug mounting relative paths on Windows [[GH-5811](https://github.com/hashicorp/nomad/issues/5811)]
 * driver/exec: Upgraded libcontainer dependency to avoid zombie `runc:[1:CHILD]]` processes [[GH-5851](https://github.com/hashicorp/nomad/issues/5851)]
 * metrics: Added metrics for raft and state store indexes. [[GH-5841](https://github.com/hashicorp/nomad/issues/5841)]
 * metrics: Upgrade prometheus client to avoid label conflicts [[GH-5850](https://github.com/hashicorp/nomad/issues/5850)]
 * ui: Fixed ability to click sort arrow to change sort direction [[GH-5833](https://github.com/hashicorp/nomad/pull/5833)]

## 0.9.3 (June 12, 2019)

BUG FIXES:

 * core: Fixed a panic that occurs if a job is updated with new task groups [[GH-5805](https://github.com/hashicorp/nomad/issues/5805)]
 * core: Update node's `StatusUpdatedAt` when node drain or eligibility changes [[GH-5746](https://github.com/hashicorp/nomad/issues/5746)]
 * core: Fixed a panic that may occur when preempting jobs for network resources [[GH-5794](https://github.com/hashicorp/nomad/issues/5794)]
 * core: Fixed a config parsing issue when client metadata contains a boolean value [[GH-5802](https://github.com/hashicorp/nomad/issues/5802)]
 * core: Fixed a config parsing issue where consul, vault, and autopilot stanzas break when using a config directory [[GH-5817](https://github.com/hashicorp/nomad/issues/5817)]
 * api: Allow sumitting alloc restart requests with an empty body [[GH-5823](https://github.com/hashicorp/nomad/pull/5823)]
 * client: Fixed an issue where task restart attempts is not honored properly [[GH-5737](https://github.com/hashicorp/nomad/issues/5737)]
 * client: Fixed a panic that occurs when a 0.9.2 client is running with 0.8 nomad servers [[GH-5812](https://github.com/hashicorp/nomad/issues/5812)]
 * client: Fixed an issue with cleaning up consul service registration entries when tasks fail to start. [[GH-5821](https://github.com/hashicorp/nomad/pull/5821)]

## 0.9.2 (June 5, 2019)

SECURITY:

 * driver/exec: Fix privilege escalation issue introduced in Nomad 0.9.0.  In
   Nomad 0.9.0 and 0.9.1, exec tasks by default run as `nobody` but with
   elevated capabilities, allowing tasks to perform privileged linux operations
   and potentially escalate permissions. (CVE-2019-12618)
   [[GH-5728](https://github.com/hashicorp/nomad/pull/5728)]

__BACKWARDS INCOMPATIBILITIES:__

 * api: The `api` package removed `Config.SetTimeout` and `Config.ConfigureTLS` functions, intended
   to be used internally only. [[GH-5275](https://github.com/hashicorp/nomad/pull/5275)]
 * api: The [job deployments](https://www.nomadproject.io/api/jobs.html#list-job-deployments) endpoint
   now filters out deployments associated with older instances of the job. This can happen if jobs are
   purged and recreated with the same id. To get all deployments irrespective of creation time, add
   `all=true`. The `nomad job deployment`CLI also defaults to doing this filtering. [[GH-5702](https://github.com/hashicorp/nomad/issues/5702)]
 * client: The format of service IDs in Consul has changed. If you rely upon
   Nomad's service IDs (*not* service names; those are stable), you will need
   to update your code.  [[GH-5536](https://github.com/hashicorp/nomad/pull/5536)]
 * client: The format of check IDs in Consul has changed. If you rely upon
   Nomad's check IDs you will need to update your code.  [[GH-5536](https://github.com/hashicorp/nomad/pull/5536)]
 * client: On startup a client will reattach to running tasks as before but
   will not restart exited tasks. Exited tasks will be restarted only after the
   client has reestablished communication with servers. System jobs will always
   be restarted. [[GH-5669](https://github.com/hashicorp/nomad/pull/5669)]

FEATURES:

 * core: Add `nomad alloc stop` command to reschedule allocs [[GH-5512](https://github.com/hashicorp/nomad/pull/5512)]
 * core: Add `nomad alloc signal` command to signal allocs and tasks [[GH-5515](https://github.com/hashicorp/nomad/pull/5515)]
 * core: Add `nomad alloc restart` command to restart allocs and tasks [[GH-5502](https://github.com/hashicorp/nomad/pull/5502)]
 * code: Add `nomad alloc exec` command for debugging and running commands in a alloc [[GH-5632](https://github.com/hashicorp/nomad/pull/5632)]
 * core/enterprise: Preemption capabilities for batch and service jobs
 * ui: Preemption reporting everywhere where allocations are shown and as part of the plan step of job submit [[GH-5594](https://github.com/hashicorp/nomad/issues/5594)]
 * ui: Ability to search clients list by class, status, datacenter, or eligibility flags [[GH-5318](https://github.com/hashicorp/nomad/issues/5318)]
 * ui: Ability to search jobs list by type, status, datacenter, or prefix [[GH-5236](https://github.com/hashicorp/nomad/issues/5236)]
 * ui: Ability to stop and restart allocations [[GH-5734](https://github.com/hashicorp/nomad/issues/5734)]
 * ui: Ability to restart tasks [[GH-5734](https://github.com/hashicorp/nomad/issues/5734)]
 * vault: Add initial support for Vault namespaces [[GH-5520](https://github.com/hashicorp/nomad/pull/5520)]

IMPROVEMENTS:

 * core: Add `-verbose` flag to `nomad status` wrapper command [[GH-5516](https://github.com/hashicorp/nomad/pull/5516)]
 * core: Add ability to filter job deployments by most recent version of job [[GH-5702](https://github.com/hashicorp/nomad/issues/5702)]
 * core: Add node name to output of `nomad node status` command in verbose mode [[GH-5224](https://github.com/hashicorp/nomad/pull/5224)]
 * core: Reduce the size of the raft transaction for plans by only sending fields updated by the plan applier [[GH-5602](https://github.com/hashicorp/nomad/pull/5602)]
 * core: Add job update `auto_promote` flag, which causes deployments to promote themselves when all canaries become healthy [[GH-5719](https://github.com/hashicorp/nomad/pull/5719)]
 * api: Support configuring `http.Client` used by golang `api` package [[GH-5275](https://github.com/hashicorp/nomad/pull/5275)]
 * api: Add preemption related fields to API results that return an allocation list. [[GH-5580](https://github.com/hashicorp/nomad/pull/5580)]
 * api: Add additional config options to scheduler configuration endpoint to disable preemption [[GH-5628](https://github.com/hashicorp/nomad/issues/5628)]
 * cli: Add acl token list command [[GH-5557](https://github.com/hashicorp/nomad/issues/5557)]
 * client: Reduce unnecessary lost nodes on server failure [[GH-5654](https://github.com/hashicorp/nomad/issues/5654)]
 * client: Canary Promotion no longer causes services registered in Consul to become unhealthy [[GH-4566](https://github.com/hashicorp/nomad/issues/4566)]
 * client: Allow use of maintenance mode and externally registered checks against Nomad-registered consul services [[GH-4537](https://github.com/hashicorp/nomad/issues/4537)]
 * driver/exec: Fixed an issue causing large memory consumption for light processes [[GH-5437](https://github.com/hashicorp/nomad/pull/5437)]
 * telemetry: Add `client.allocs.memory.allocated` metric to expose allocated task memory in bytes. [[GH-5492](https://github.com/hashicorp/nomad/issues/5492)]
 * ui: Colored log support [[GH-5620](https://github.com/hashicorp/nomad/issues/5620)]
 * ui: Upgraded from Ember 2.18 to 3.4 [[GH-5544](https://github.com/hashicorp/nomad/issues/5544)]
 * ui: Replace XHR cancellation by URL with XHR cancellation by token [[GH-5721](https://github.com/hashicorp/nomad/issues/5721)]

BUG FIXES:

 * core: Fixed accounting of allocated resources in metrics. [[GH-5637](https://github.com/hashicorp/nomad/issues/5637)]
 * core: Fixed disaster recovering with raft 3 protocol peers.json [[GH-5629](https://github.com/hashicorp/nomad/issues/5629)], [[GH-5651](https://github.com/hashicorp/nomad/issues/5651)]
 * core: Fixed a panic that may occur when preempting service jobs [[GH-5545](https://github.com/hashicorp/nomad/issues/5545)]
 * core: Fixed an edge case that caused division by zero when computing spread score [[GH-5713](https://github.com/hashicorp/nomad/issues/5713)]
 * core: Change configuration parsing to use the HCL library's decode, improving JSON support [[GH-1290](https://github.com/hashicorp/nomad/issues/1290)]
 * core: Fix a case where non-leader servers would have an ever growing number of waiting evaluations [[GH-5699](https://github.com/hashicorp/nomad/pull/5699)]
 * cli: Fix output and exit status for system jobs with constraints [[GH-2381](https://github.com/hashicorp/nomad/issues/2381)] and [[GH-5169](https://github.com/hashicorp/nomad/issues/5169)]
 * client: Fix network fingerprinting to honor manual configuration [[GH-2619](https://github.com/hashicorp/nomad/issues/2619)]
 * client: Job validation now checks that the datacenter field does not contain empty strings [[GH-5665](https://github.com/hashicorp/nomad/pull/5665)]
 * client: Fix network port mapping  related environment variables when running with Nomad 0.8 servers [[GH-5587](https://github.com/hashicorp/nomad/issues/5587)]
 * client: Fix issue with terminal state deployments being modified when allocation subsequently fails [[GH-5645](https://github.com/hashicorp/nomad/issues/5645)]
 * driver/docker: Fix regression around image GC [[GH-5768](https://github.com/hashicorp/nomad/issues/5768)]
 * metrics: Fixed stale metrics [[GH-5540](https://github.com/hashicorp/nomad/issues/5540)]
 * vault: Fix renewal time to be 1/2 lease duration with jitter [[GH-5479](https://github.com/hashicorp/nomad/issues/5479)]

## 0.9.1 (April 29, 2019)

BUG FIXES:

* core: Fix bug with incorrect metrics on pending allocations [[GH-5541](https://github.com/hashicorp/nomad/pull/5541)]
* client: Fix issue with recovering from logmon failures [[GH-5577](https://github.com/hashicorp/nomad/pull/5577)], [[GH-5616](https://github.com/hashicorp/nomad/pull/5616)]
* client: Fix deadlock on client startup after reboot [[GH-5568](https://github.com/hashicorp/nomad/pull/5568)]
* client: Fix issue with node registration where newly registered nodes would not run system jobs [[GH-5585](https://github.com/hashicorp/nomad/pull/5585)]
* driver/docker: Fix regression around volume handling [[GH-5572](https://github.com/hashicorp/nomad/pull/5572)]
* driver/docker: Fix regression in which logs aren't collected for docker container with `tty` set to true [[GH-5609](https://github.com/hashicorp/nomad/pull/5609)]
* driver/exec: Fix an issue where raw_exec and exec processes are leaked when nomad agent is restarted [[GH-5598](https://github.com/hashicorp/nomad/pull/5598)]

## 0.9.0 (April 9, 2019)

__BACKWARDS INCOMPATIBILITIES:__

 * core: Drop support for CentOS/RHEL 6. glibc >= 2.14 is required.
 * core: Switch to structured logging using
   [go-hclog](https://github.com/hashicorp/go-hclog). If you have tooling that
   parses Nomad's logs, the format of logs has changed and your tools may need
   updating.
 * core: IOPS as a resource is now deprecated
   [[GH-4970](https://github.com/hashicorp/nomad/issues/4970)]. Nomad continues
   to parse IOPS in jobs to allow job authors time to remove iops from their
   jobs.
 * core: Allow the != constraint to match against keys that do not exist [[GH-4875](https://github.com/hashicorp/nomad/pull/4875)]
 * client: Task config validation is more strict in 0.9. For example unknown
   parameters in stanzas under the task config were ignored in previous
   versions but in 0.9 this will cause a task failure.
 * client: Task config interpolation requires names to be valid identifiers
   (`node.region` or `NOMAD_DC`). Interpolating other variables requires a new
   indexing syntax: `env[".invalid.identifier."]`. [[GH-4843](https://github.com/hashicorp/nomad/issues/4843)]
 * client: Node metadata variables must have valid identifiers, whether
   specified in the config file (`client.meta` stanza) or on the command line
   (`-meta`). [[GH-5158](https://github.com/hashicorp/nomad/pull/5158)]
 * driver/lxc: The LXC driver is no longer packaged with Nomad and is instead
   distributed separately as a driver plugin. Further, the LXC driver codebase
   is now in a separate
   [repository](https://github.com/hashicorp/nomad-driver-lxc). If you are using
   LXC, please follow the 0.9.0 upgrade guide as you will have to install the
   LXC driver before conducting an in-place upgrade to Nomad 0.9.0 [[GH-5162](https://github.com/hashicorp/nomad/issues/5162)]

FEATURES:

 * **Affinities and Spread**: Jobs may now specify affinities towards certain
   node attributes. Affinities act as soft constraints, and inform the
   scheduler that the job has a preference for certain node properties. The new
   spread stanza informs the scheduler that allocations should be spread across a
   specific property such as datacenter or availability zone. This is useful to
   increase failure tolerance of critical applications.
 * **System Job Preemption**: System jobs may now preempt lower priority
   allocations. The ability to place system jobs on all targeted nodes is
   critical since system jobs often run applications that provide services for
   all allocations on the node.
 * **Driver Plugins**: Nomad now supports task drivers as plugins. Driver
   plugins operate the same as built-in drivers and can be developed and
   distributed independently from Nomad.
 * **Device Plugins**: Nomad now supports scheduling and mounting devices via
   device plugins. Device plugins expose hardware devices such as GPUs to Nomad
   and instruct the client on how to make them available to tasks. Device
   plugins can expose the health of devices, the devices attributes, and device
   usage statistics. Device plugins can be developed and distributed
   independently from Nomad.
 * **Nvidia GPU Device Plugin**: Nomad builds-in a Nvidia GPU device plugin to
   add out-of-the-box support for scheduling Nvidia GPUs.
 * **Client Refactor**: Major focus has been put in this release to refactor the
   Nomad Client codebase. The goal of the refactor has been to make the
   codebase more modular to increase developer velocity and testability.
 * **Mobile UI Views:** The side-bar navigation, breadcrumbs, and various other page
   elements are now responsively resized and repositioned based on your browser size.
 * **Job Authoring from the UI:** It is now possible to plan and submit new jobs, edit
   existing jobs, stop and start jobs, and promote canaries all from the UI.
 * **Improved Stat Tracking in UI:** The client detail, allocation detail, and task
   detail pages now have line charts that plot CPU and Memory usage changes over time.
 * **Structured Logging**: Nomad now uses structured logging with the ability to
   output logs in a JSON format.

IMPROVEMENTS:

 * core: Added advertise address to client node meta data [[GH-4390](https://github.com/hashicorp/nomad/issues/4390)]
 * core: Added support for specifying node affinities. Affinities allow job operators to specify weighted placement preferences according to different node attributes [[GH-4512](https://github.com/hashicorp/nomad/issues/4512)]
 * core: Added support for spreading allocations across a specific attribute. Operators can specify spread target percentages across failure domains such as datacenter or rack [[GH-4512](https://github.com/hashicorp/nomad/issues/4512)]
 * core: Added preemption support for system jobs. System jobs can now preempt other jobs of lower priority. See [preemption](https://www.nomadproject.io/docs/internals/scheduling/preemption.html) for more details. [[GH-4794](https://github.com/hashicorp/nomad/pull/4794)]
 * acls: Allow support for using globs in namespace definitions [[GH-4982](https://github.com/hashicorp/nomad/pull/4982)]
 * agent: Support JSON log output [[GH-5173](https://github.com/hashicorp/nomad/issues/5173)]
 * api: Reduced api package dependencies [[GH-5213](https://github.com/hashicorp/nomad/pull/5213)]
 * client: Extend timeout to 60 seconds for Windows CPU fingerprinting [[GH-4441](https://github.com/hashicorp/nomad/pull/4441)]
 * client: Refactor client to support plugins and improve state handling [[GH-4792](https://github.com/hashicorp/nomad/pull/4792)]
 * client: Updated consul-template library to pick up recent fixes and improvements[[GH-4885](https://github.com/hashicorp/nomad/pull/4885)]
 * client: When retrying a failed artifact, do not download any successfully downloaded artifacts again [[GH-5322](https://github.com/hashicorp/nomad/issues/5322)]
 * client: Added service metadata tag that enables the Consul UI to show a Nomad icon for services registered by Nomad [[GH-4889](https://github.com/hashicorp/nomad/issues/4889)]
 * cli: Added support for coloured output on Windows [[GH-5342](https://github.com/hashicorp/nomad/pull/5342)]
 * driver/docker: Rename Logging `type` to `driver` [[GH-5372](https://github.com/hashicorp/nomad/pull/5372)]
 * driver/docker: Support logs when using Docker for Mac [[GH-4758](https://github.com/hashicorp/nomad/issues/4758)]
 * driver/docker: Added support for specifying `storage_opt` in the Docker driver [[GH-4908](https://github.com/hashicorp/nomad/pull/4908)]
 * driver/docker: Added support for specifying `cpu_cfs_period` in the Docker driver [[GH-4462](https://github.com/hashicorp/nomad/pull/4462)]
 * driver/docker: Added support for setting bind and tmpfs mounts in the Docker driver [[GH-4924](https://github.com/hashicorp/nomad/pull/4924)]
 * driver/docker: Report container images with user friendly name rather than underlying image ID [[GH-4926](https://github.com/hashicorp/nomad/pull/4926)]
 * driver/docker: Add support for collecting stats on Windows [[GH-5355](https://github.com/hashicorp/nomad/pull/5355)]
   drivers/docker: Report docker driver as undetected before first connecting to the docker daemon [[GH-5362](https://github.com/hashicorp/nomad/pull/5362)]
 * drivers: Added total memory usage to task resource metrics [[GH-5190](https://github.com/hashicorp/nomad/pull/5190)]
 * server/rpc: Reduce logging when undergoing temporary network errors such as hitting file descriptor limits [[GH-4974](https://github.com/hashicorp/nomad/issues/4974)]
 * server/vault: Tweaked logs to better identify vault connection errors [[GH-5228](https://github.com/hashicorp/nomad/pull/5228)]
 * server/vault: Added Vault token expiry info in `nomad status` CLI, and some improvements to token refresh process [[GH-4817](https://github.com/hashicorp/nomad/pull/4817)]
 * telemetry: All client metrics include a new `node_class` tag [[GH-3882](https://github.com/hashicorp/nomad/issues/3882)]
 * telemetry: Added new tags with value of child job id and parent job id for parameterized and periodic jobs [[GH-4392](https://github.com/hashicorp/nomad/issues/4392)]
 * ui: Improved JSON editor [[GH-4541](https://github.com/hashicorp/nomad/issues/4541)]
 * ui: Mobile friendly views [[GH-4536](https://github.com/hashicorp/nomad/issues/4536)]
 * ui: Filled out the styleguide [[GH-4468](https://github.com/hashicorp/nomad/issues/4468)]
 * ui: Support switching regions [[GH-4572](https://github.com/hashicorp/nomad/issues/4572)]
 * ui: Canaries can now be promoted from the UI [[GH-4616](https://github.com/hashicorp/nomad/issues/4616)]
 * ui: Stopped jobs can be restarted from the UI [[GH-4615](https://github.com/hashicorp/nomad/issues/4615)]
 * ui: Support widescreen format in alloc logs view [[GH-5400](https://github.com/hashicorp/nomad/pull/5400)]
 * ui: Gracefully handle errors from the stats end points [[GH-4833](https://github.com/hashicorp/nomad/issues/4833)]
 * ui: Added links to Jobs and Clients from the error page template [[GH-4850](https://github.com/hashicorp/nomad/issues/4850)]
 * ui: Jobs can be authored, planned, submitted, and edited from the UI [[GH-4600](https://github.com/hashicorp/nomad/issues/4600)]
 * ui: Display recent allocations on job page and introduce allocation tab [[GH-4529](https://github.com/hashicorp/nomad/issues/4529)]
 * ui: Refactored breadcrumbs and adjusted the breadcrumb paths on each page [[GH-4458](https://github.com/hashicorp/nomad/issues/4458)]
 * ui: Switching namespaces in the UI will now always "reset" back to the jobs list page [[GH-4533](https://github.com/hashicorp/nomad/issues/4533)]
 * ui: CPU and Memory metrics are plotted over time during a session in line charts on node detail, allocation detail, and task detail pages [[GH-4661](https://github.com/hashicorp/nomad/issues/4661)], [[GH-4718](https://github.com/hashicorp/nomad/issues/4718)], [[GH-4727](https://github.com/hashicorp/nomad/issues/4727)]

BUG FIXES:

 * core: Removed some GPL code inadvertently added for macOS support [[GH-5202](https://github.com/hashicorp/nomad/pull/5202)]
 * core: Fix an issue where artifact checksums containing interpolated variables failed validation [[GH-4810](https://github.com/hashicorp/nomad/pull/4819)]
 * core: Fix an issue where job summaries for parent dispatch/periodic jobs were not being computed correctly [[GH-5205](https://github.com/hashicorp/nomad/pull/5205)]
 * core: Fix an issue where a canary allocation with a deployment no longer in the state store caused a panic [[GH-5259](https://github.com/hashicorp/nomad/pull/5259)
 * client: Fix an issue reloading the client config [[GH-4730](https://github.com/hashicorp/nomad/issues/4730)]
 * client: Fix an issue where driver attributes are not updated in node API responses if they change after after startup [[GH-4984](https://github.com/hashicorp/nomad/pull/4984)]
 * driver/docker: Fix a path traversal issue where mounting paths outside alloc dir might be possible despite `docker.volumes.enabled` set to false [[GH-4983](https://github.com/hashicorp/nomad/pull/4983)]
 * driver/raw_exec: Fix an issue where tasks that used an interpolated command in driver configuration would not start [[GH-4813](https://github.com/hashicorp/nomad/pull/4813)]
 * drivers: Fix a bug where exec and java drivers get reported as detected and healthy when nomad is not running as root and without cgroup support
 * quota: Fixed a bug in Nomad enterprise where quota specifications were not being replicated to non authoritative regions correctly.
 * scheduler: When dequeueing evals ensure workers wait to the proper Raft index [[GH-5381](https://github.com/hashicorp/nomad/issues/5381)]
 * scheduler: Allow schedulers to handle evaluations that are created due to previous evaluation failures [[GH-4712](https://github.com/hashicorp/nomad/issues/4712)]
 * server/api: Fixed bug when trying to route to a down node [[GH-5261](https://github.com/hashicorp/nomad/pull/5261)]
 * server/vault: Fixed bug in Vault token renewal that could panic on a malformed Vault response [[GH-4904](https://github.com/hashicorp/nomad/issues/4904)], [[GH-4937](https://github.com/hashicorp/nomad/pull/4937)]
 * template: Fix parsing of environment templates when destination path is interpolated [[GH-5253](https://github.com/hashicorp/nomad/issues/5253)]
 * ui: Fixes for viewing objects that contain dots in their names [[GH-4994](https://github.com/hashicorp/nomad/issues/4994)]
 * ui: Correctly labeled certain classes of unknown errors as 404 errors [[GH-4841](https://github.com/hashicorp/nomad/issues/4841)]
 * ui: Fixed an issue where searching while viewing a paginated table could display no results [[GH-4822](https://github.com/hashicorp/nomad/issues/4822)]
 * ui: Fixed an issue where the task group breadcrumb didn't always include the namesapce query param [[GH-4801](https://github.com/hashicorp/nomad/issues/4801)]
 * ui: Added an empty state for the tasks list on the allocation detail page, for when an alloc has no tasks [[GH-4860](https://github.com/hashicorp/nomad/issues/4860)]
 * ui: Fixed an issue where dispatched jobs would get the wrong template type which could cause runtime errors [[GH-4852](https://github.com/hashicorp/nomad/issues/4852)]
 * ui: Fixed an issue where distribution bar corners weren't rounded when there was only one or two slices in the chart [[GH-4507](https://github.com/hashicorp/nomad/issues/4507)]

## 0.8.7 (January 14, 2019)

IMPROVEMENTS:
* core: Added `filter_default`, `prefix_filter` and `disable_dispatched_job_summary_metrics`
  client options to improve metric filtering [[GH-4878](https://github.com/hashicorp/nomad/issues/4878)]
* driver/docker: Support `bind` mount type in order to allow Windows users to mount absolute paths [[GH-4958](https://github.com/hashicorp/nomad/issues/4958)]

BUG FIXES:
* core: Fixed panic when Vault secret response is nil [[GH-4904](https://github.com/hashicorp/nomad/pull/4904)] [[GH-4937](https://github.com/hashicorp/nomad/pull/4937)]
* core: Fixed issue with negative counts in job summary [[GH-4949](https://github.com/hashicorp/nomad/issues/4949)]
* core: Fixed issue with handling duplicated blocked evaluations [[GH-4867](https://github.com/hashicorp/nomad/pull/4867)]
* core: Fixed bug where some successfully completed jobs get re-run after job
  garbage collection [[GH-4861](https://github.com/hashicorp/nomad/pull/4861)]
* core: Fixed bug in reconciler where allocs already stopped were being
  unnecessarily updated [[GH-4764](https://github.com/hashicorp/nomad/issues/4764)]
* core: Fixed bug that affects garbage collection of batch jobs that are purged
  and resubmitted with the same id [[GH-4839](https://github.com/hashicorp/nomad/pull/4839)]
* core: Fixed an issue with garbage collection where terminal but still running
  allocations could be garbage collected server side [[GH-4965](https://github.com/hashicorp/nomad/issues/4965)]
* deployments: Fix an issue where a deployment with multiple task groups could
  be marked as failed when the first progress deadline was hit regardless of if
  that group was done deploying [[GH-4842](https://github.com/hashicorp/nomad/issues/4842)]

## 0.8.6 (September 26, 2018)

IMPROVEMENTS:
* core: Increased scheduling performance when annotating existing allocations
  [[GH-4713](https://github.com/hashicorp/nomad/issues/4713)]
* core: Unique TriggerBy for evaluations that are created to place queued
  allocations [[GH-4716](https://github.com/hashicorp/nomad/issues/4716)]

BUG FIXES:
* core: Fix a bug in Nomad Enterprise where non-voting servers could get
  bootstrapped as voting servers [[GH-4702](https://github.com/hashicorp/nomad/issues/4702)]
* core: Fix an issue where an evaluation could fail if an allocation was being
  rescheduled and the node containing it was at capacity [[GH-4713](https://github.com/hashicorp/nomad/issues/4713)]
* core: Fix an issue in which schedulers would reject evaluations created when
  prior scheduling for a job failed [[GH-4712](https://github.com/hashicorp/nomad/issues/4712)]
* cli: Fix a bug where enabling custom upgrade versions for autopilot was not
  being honored [[GH-4723](https://github.com/hashicorp/nomad/issues/4723)]
* deployments: Fix an issue where the deployment watcher could create a high
  volume of evaluations [[GH-4709](https://github.com/hashicorp/nomad/issues/4709)]
* vault: Fix a regression in which Nomad was only compatible with Vault versions
  greater than 0.10.0 [[GH-4698](https://github.com/hashicorp/nomad/issues/4698)]

## 0.8.5 (September 13, 2018)

IMPROVEMENTS:

* core: Failed deployments no longer block migrations [[GH-4659](https://github.com/hashicorp/nomad/issues/4659)]
* client: Added option to prevent Nomad from removing containers when the task exits [[GH-4535](https://github.com/hashicorp/nomad/issues/4535)]

BUG FIXES:

* core: Reset queued allocation summary to zero when job stopped [[GH-4414](https://github.com/hashicorp/nomad/issues/4414)]
* core: Fix inverted logic bug where if `disable_update_check` was enabled, update checks would be performed [[GH-4570](https://github.com/hashicorp/nomad/issues/4570)]
* core: Fix panic due to missing synchronization in delayed evaluations heap [[GH-4632](https://github.com/hashicorp/nomad/issues/4632)]
* core: Fix treating valid PEM files as invalid [[GH-4613](https://github.com/hashicorp/nomad/issues/4613)]
* core: Fix panic in nomad job history when invoked with a job version that doesn't exist [[GH-4577](https://github.com/hashicorp/nomad/issues/4577)]
* core: Fix issue with not properly closing connection multiplexer when its context is cancelled [[GH-4573](https://github.com/hashicorp/nomad/issues/4573)]
* core: Upgrade vendored Vault client library to fix API incompatibility issue [[GH-4658](https://github.com/hashicorp/nomad/issues/4658)]
* driver/docker: Fix kill timeout not being respected when timeout is over five minutes [[GH-4599](https://github.com/hashicorp/nomad/issues/4599)]
* scheduler: Fix nil pointer dereference [[GH-4474](https://github.com/hashicorp/nomad/issues/4474)]
* scheduler: Fix panic when allocation's reschedule policy doesn't exist [[GH-4647](https://github.com/hashicorp/nomad/issues/4647)]
* client: Fix migrating ephemeral disks when TLS is enabled [[GH-4648](https://github.com/hashicorp/nomad/issues/4648)]

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
