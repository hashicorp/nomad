# Change log for unsupported versions of Nomad

The versions of Nomad listed here are no longer supported by HashiCorp.

## 1.6.15 Enterprise (September 17, 2024)

BREAKING CHANGES:

* docker: The default infra_image for pause containers is now registry.k8s.io/pause [[GH-23927](https://github.com/hashicorp/nomad/issues/23927)]

IMPROVEMENTS:

* build: update to go1.22.6 [[GH-23805](https://github.com/hashicorp/nomad/issues/23805)]
* cli: Increase default log level and duration when capturing logs with `operator debug` [[GH-23850](https://github.com/hashicorp/nomad/issues/23850)]

BUG FIXES:

* node: Fixed bug where sysbatch allocations were started prematurely [[GH-23858](https://github.com/hashicorp/nomad/issues/23858)]

## 1.6.14 Enterprise (August 13, 2024)

SECURITY:

* security: Fix symlink escape during unarchiving by removing existing paths within the same allocdir. Compromising the Nomad client agent at the source allocation first is a prerequisite for leveraging this issue. [[GH-23738](https://github.com/hashicorp/nomad/issues/23738)]

IMPROVEMENTS:

* keyring: Added support for prepublishing keys [[GH-23577](https://github.com/hashicorp/nomad/issues/23577)]

BUG FIXES:

* cni: .conf and .json config files are now parsed properly [[GH-23629](https://github.com/hashicorp/nomad/issues/23629)]
* docker: Fixed a bug where plugin SELinux labels would conflict with read-only `volume` options [[GH-23750](https://github.com/hashicorp/nomad/issues/23750)]
* keyring: Fixed a bug where keys could be garbage collected before workload identities expire [[GH-23577](https://github.com/hashicorp/nomad/issues/23577)]
* keyring: Fixed a bug where keys would never exit the "rekeying" state after a rotation with the `-full` flag [[GH-23577](https://github.com/hashicorp/nomad/issues/23577)]
* keyring: Fixed a bug where periodic key rotation would not occur [[GH-23577](https://github.com/hashicorp/nomad/issues/23577)]
* networking: The same static port can now be used more than once on host networks with multiple IPs [[GH-23693](https://github.com/hashicorp/nomad/issues/23693)]
* scaling: Fixed a bug where state store corruption could occur when writing scaling events [[GH-23673](https://github.com/hashicorp/nomad/issues/23673)]
* template: Fixed a bug where change_mode = "script" would not execute after a client restart [[GH-23663](https://github.com/hashicorp/nomad/issues/23663)]
* windows: Fix bug with containers capabilities on Docker CE [[GH-23599](https://github.com/hashicorp/nomad/issues/23599)]

## 1.6.13 Enterprise (July 16, 2024)

BREAKING CHANGES:

* docker: default to hyper-v isolation mode on Windows [[GH-23452](https://github.com/hashicorp/nomad/issues/23452)]

SECURITY:

* build: Updated Go to 1.22.5 to address CVE-2024-24791 [[GH-23498](https://github.com/hashicorp/nomad/issues/23498)]
* migration: Added a check for relative paths escaping the allocation directory when unpacking archive during migration, to harden clients against compromised peer clients sending malicious archives [[GH-23319](https://github.com/hashicorp/nomad/issues/23319)]
* security: Removed insecure TLS cipher suites: `TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256`, `TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA25` and `TLS_RSA_WITH_AES_128_CBC_SHA256`. [[GH-23551](https://github.com/hashicorp/nomad/issues/23551)]

IMPROVEMENTS:

* deps: Updated Consul API to 1.29.1. [[GH-23436](https://github.com/hashicorp/nomad/issues/23436)]
* deps: Updated consul-template to 0.39 to allow admin partition and sameness groups queries. [[GH-23436](https://github.com/hashicorp/nomad/issues/23436)]
* docker: Validate that unprivileged containers aren't running as ContainerAdmin on Windows [[GH-23443](https://github.com/hashicorp/nomad/issues/23443)]

BUG FIXES:

* api: Fixed bug where newlines in JobSubmission vars weren't encoded correctly [[GH-23560](https://github.com/hashicorp/nomad/issues/23560)]
* cli: Fixed bug where the `plugin status` command would fail if the plugin ID was a prefix of another plugin ID [[GH-23502](https://github.com/hashicorp/nomad/issues/23502)]
* cli: Fixed bug where the `quota status` and `quota inspect` commands would fail if the quota name was a prefix of another quota name [[GH-23502](https://github.com/hashicorp/nomad/issues/23502)]
* cli: Fixed bug where the `scaling policy info` command would fail if the policy ID was a prefix of another policy ID [[GH-23502](https://github.com/hashicorp/nomad/issues/23502)]
* cli: Fixed bug where the `service info` command would fail if the service name was a prefix of another service name in the same namespace [[GH-23502](https://github.com/hashicorp/nomad/issues/23502)]
* cli: Fixed bug where the `volume deregister`, `volume detach`, and `volume status` commands would fail if the volume ID was a prefix of another volume ID in the same namespace [[GH-23502](https://github.com/hashicorp/nomad/issues/23502)]
* quota (Enterprise): Fixed a bug where a task's resource core count was not translated to CPU MHz and checked against its quota when performing a job plan [[GH-18876](https://github.com/hashicorp/nomad/issues/18876)]
* scheduler: Fix a bug where reserved resources are not calculated correctly [[GH-23386](https://github.com/hashicorp/nomad/issues/23386)]
* server: Fixed a bug where expiring heartbeats for garbage collected nodes could panic the server [[GH-23383](https://github.com/hashicorp/nomad/issues/23383)]
* template: Fix template rendering on Windows [[GH-23432](https://github.com/hashicorp/nomad/issues/23432)]

## 1.6.12 Enterprise (June 19, 2024)

SECURITY:

* build: Updated Go to 1.22.4 to address Go stdlib vulnerabilities CVE-2024-24789 and CVE-2024-24790 [[GH-23172](https://github.com/hashicorp/nomad/issues/23172)]

IMPROVEMENTS:

* cli: `operator snapshot inspect` now includes details of data in snapshot [[GH-18372](https://github.com/hashicorp/nomad/issues/18372)]
* docker: Added container_exists_attempts plugin configuration variable [[GH-22419](https://github.com/hashicorp/nomad/issues/22419)]
* exec: Fixed a bug where `exec` driver tasks would fail on older versions of glibc [[GH-23331](https://github.com/hashicorp/nomad/issues/23331)]

BUG FIXES:

* acl: Fix plugin policy validation when checking write permissions [[GH-23274](https://github.com/hashicorp/nomad/issues/23274)]
* connect: fix validation with multiple socket paths [[GH-22312](https://github.com/hashicorp/nomad/issues/22312)]
* driver: Fixed a bug where the exec, java, and raw_exec drivers would not configure cgroups to allow access to devices provided by device plugins [[GH-22518](https://github.com/hashicorp/nomad/issues/22518)]
* scheduler: Fixed a bug where rescheduled allocations that could not be placed would later ignore their reschedule policy limits [[GH-12319](https://github.com/hashicorp/nomad/issues/12319)]

## 1.6.11 Enterprise (May 28, 2024)

SECURITY:

* deps: Updated `docker` dependency to 25.0.5 [[GH-20171](https://github.com/hashicorp/nomad/issues/20171)]

BUG FIXES:

* cli: Fix handling of scaling jobs which don't generate evals [[GH-20479](https://github.com/hashicorp/nomad/issues/20479)]
* client: terminate old exec task processes before starting new ones, to avoid accidentally leaving running processes in case of an error [[GH-20500](https://github.com/hashicorp/nomad/issues/20500)]
* core: Fix multiple incorrect type conversion for potential overflows [[GH-20553](https://github.com/hashicorp/nomad/issues/20553)]
* csi: Fixed a bug where concurrent mount and unmount operations could unstage volumes needed by another allocation [[GH-20550](https://github.com/hashicorp/nomad/issues/20550)]
* csi: Fixed a bug where plugins would not be deleted on GC if their job updated the plugin ID [[GH-20555](https://github.com/hashicorp/nomad/issues/20555)]
* csi: Fixed a bug where volumes in different namespaces but the same ID would fail to stage on the same client [[GH-20532](https://github.com/hashicorp/nomad/issues/20532)]
* quota (Enterprise): Fixed a bug where quota usage would not be freed if a job was purged
* services: Added retry to Nomad service deregistration RPCs during alloc stop [[GH-20596](https://github.com/hashicorp/nomad/issues/20596)]
* services: Fixed bug where Nomad services might not be deregistered when nodes are marked down or allocations are terminal [[GH-20590](https://github.com/hashicorp/nomad/issues/20590)]
* structs: Fix job canonicalization for array type fields [[GH-20522](https://github.com/hashicorp/nomad/issues/20522)]
* ui: Show the namespace in the web UI exec command hint [[GH-20218](https://github.com/hashicorp/nomad/issues/20218)]

## 1.6.10 (April 16, 2024)

SECURITY:

* artifact: Updated `go-getter` dependency to v1.7.4 to address CVE-2024-3817 [[GH-20391](https://github.com/hashicorp/nomad/issues/20391)]

BUG FIXES:

* api: Fixed a bug where `AllocDirStats` field was missing from Read Stats client API [[GH-20261](https://github.com/hashicorp/nomad/issues/20261)]
* cli: Fixed a bug where `operator debug` did not respect the `-pprof-interval` flag and would take only one profile [[GH-20206](https://github.com/hashicorp/nomad/issues/20206)]
* cni: Fixed a regression where default DNS set by `dockerd` or other task drivers was not respected [[GH-20189](https://github.com/hashicorp/nomad/issues/20189)]
* config: Fixed a bug where IPv6 addresses were not accepted without ports for `client.servers` blocks [[GH-20324](https://github.com/hashicorp/nomad/issues/20324)]
* deployments: Fixed a goroutine leak when jobs are purged [[GH-20348](https://github.com/hashicorp/nomad/issues/20348)]
* deps: Updated consul-template dependency to 0.37.4 to fix a resource leak [[GH-20234](https://github.com/hashicorp/nomad/issues/20234)]
* drain: Fixed a bug where Workload Identity tokens could not be used to drain a node [[GH-20317](https://github.com/hashicorp/nomad/issues/20317)]
* namespace/node pool: Fixed a bug where the `-region` flag would not be respected for namespace and node pool updates if ACLs were disabled [[GH-20220](https://github.com/hashicorp/nomad/issues/20220)]
* state: Fixed a bug where restarting a server could fail if the Raft logs include a drain update that used a now-expired token [[GH-20317](https://github.com/hashicorp/nomad/issues/20317)]
* template: Fixed a bug where a partial `client.template` block would cause defaults for unspecified fields to be ignored [[GH-20165](https://github.com/hashicorp/nomad/issues/20165)]
* ui: Fix an issue where the job status box would error if an allocation had no task events [[GH-20383](https://github.com/hashicorp/nomad/issues/20383)]

## 1.6.9 (March 12, 2024)

SECURITY:

* build: Update to go1.22 to address Go standard library vulnerabilities CVE-2024-24783, CVE-2023-45290, and CVE-2024-24785. [[GH-20066](https://github.com/hashicorp/nomad/issues/20066)]
* deps: Upgrade protobuf library to 1.33.0 to avoid scan alerts for CVE-2024-24786, which Nomad is not vulnerable to [[GH-20100](https://github.com/hashicorp/nomad/issues/20100)]

BUG FIXES:

* cli: Fixed a bug where the `nomad job restart` command could crash if the job type was not present in a response from the server [[GH-20049](https://github.com/hashicorp/nomad/issues/20049)]
* client: Fixed a bug where corrupt client state could panic the client [[GH-19972](https://github.com/hashicorp/nomad/issues/19972)]
* cni: Fixed a bug where DNS set by CNI plugins was not provided to task drivers [[GH-20007](https://github.com/hashicorp/nomad/issues/20007)]
* connect: Fixed a bug where `expose` blocks would not appear in `job plan` diff output [[GH-19990](https://github.com/hashicorp/nomad/issues/19990)]

## 1.6.8 (February 13, 2024)

SECURITY:

* windows: Remove `LazyDLL` calls for system modules to harden Nomad against attacks from the host [[GH-19925](https://github.com/hashicorp/nomad/issues/19925)]

BUG FIXES:

* cli: Fix return code when `nomad job run` succeeds after a blocked eval [[GH-19876](https://github.com/hashicorp/nomad/issues/19876)]
* cli: Fixed a bug where the `nomad tls ca create` command failed when the `-domain` was used without other values [[GH-19892](https://github.com/hashicorp/nomad/issues/19892)]
* connect: Fixed envoy sidecars being unable to restart after node reboots [[GH-19787](https://github.com/hashicorp/nomad/issues/19787)]
* exec: Fixed a bug in `alloc exec` where closing websocket streams could cause a panic [[GH-19932](https://github.com/hashicorp/nomad/issues/19932)]
* scheduler: Fixed a bug that caused blocked evaluations due to port conflict to not have a reason explaining why the evaluation was blocked [[GH-19933](https://github.com/hashicorp/nomad/issues/19933)]
* ui: Fix an issue where a same-named task from a different group could be selected when the user clicks Exec from a task group page where multiple allocations would be valid [[GH-19878](https://github.com/hashicorp/nomad/issues/19878)]

## 1.6.7 (February 08, 2024)

SECURITY:

* deps: Updated runc to 1.1.12 to address CVE-2024-21626 [[GH-19851](https://github.com/hashicorp/nomad/issues/19851)]
* migration: Fixed a bug where archives used for migration were not checked for symlinks that escaped the allocation directory [[GH-19887](https://github.com/hashicorp/nomad/issues/19887)]
* template: Fixed a bug where symlinks could force templates to read and write to arbitrary locations (CVE-2024-1329) [[GH-19888](https://github.com/hashicorp/nomad/issues/19888)]

## 1.6.6 (January 15, 2024)

IMPROVEMENTS:

* build: update to go 1.21.6 [[GH-19709](https://github.com/hashicorp/nomad/issues/19709)]

BUG FIXES:

* acl: Fixed auth method hashing which meant changing some fields would be silently ignored [[GH-19677](https://github.com/hashicorp/nomad/issues/19677)]
* auth: Added new optional OIDCDisableUserInfo setting for OIDC auth provider [[GH-19566](https://github.com/hashicorp/nomad/issues/19566)]
* core: Ensure job HCL submission data is persisted and restored during the FSM snapshot process [[GH-19605](https://github.com/hashicorp/nomad/issues/19605)]
* namespaces: Failed delete calls no longer return success codes [[GH-19483](https://github.com/hashicorp/nomad/issues/19483)]
* server: Fix server not waiting for workers to submit nacks for dequeued evaluations before shutting down [[GH-19560](https://github.com/hashicorp/nomad/issues/19560)]
* state: Fixed a bug where purged jobs would not get new deployments [[GH-19609](https://github.com/hashicorp/nomad/issues/19609)]

## 1.6.5 (December 13, 2023)

BUG FIXES:

* cli: Fix a bug in the `var put` command which prevented combining items as CLI arguments and other parameters as flags [[GH-19423](https://github.com/hashicorp/nomad/issues/19423)]
* client: remove incomplete allocation entries from client state database during client restarts [[GH-16638](https://github.com/hashicorp/nomad/issues/16638)]
* connect: Fixed a bug where deployments would not wait for Connect sidecar task health checks to pass [[GH-19334](https://github.com/hashicorp/nomad/issues/19334)]
* consul: uses token namespace to fetch policies for verification [[GH-18516](https://github.com/hashicorp/nomad/issues/18516)]
* csi: Added validation to `csi_plugin` blocks to prevent `stage_publish_base_dir` from being a subdirectory of `mount_dir` [[GH-19441](https://github.com/hashicorp/nomad/issues/19441)]
* metrics: Revert upgrade of `go-metrics` to fix an issue where metrics from dependencies, such as raft, were no longer emitted [[GH-19375](https://github.com/hashicorp/nomad/issues/19375)]

## 1.6.4 (December 07, 2023)

BREAKING CHANGES:

* core: Honor job's namespace when checking `distinct_hosts` feasibility [[GH-19004](https://github.com/hashicorp/nomad/issues/19004)]

SECURITY:

* build: Update to go1.21.4 to resolve Windows path validation CVE in Go [[GH-19013](https://github.com/hashicorp/nomad/issues/19013)]
* build: Update to go1.21.5 to resolve Windows path validation CVE in Go [[GH-19320](https://github.com/hashicorp/nomad/issues/19320)]

IMPROVEMENTS:

* cli: Add file prediction for operator raft/snapshot commands [[GH-18901](https://github.com/hashicorp/nomad/issues/18901)]
* ui: color-code node and server status cells [[GH-18318](https://github.com/hashicorp/nomad/issues/18318)]
* ui: show plan output warnings alongside placement failures and dry-run info when running a job through the web ui [[GH-19225](https://github.com/hashicorp/nomad/issues/19225)]

BUG FIXES:

* agent: Correct websocket status code handling [[GH-19172](https://github.com/hashicorp/nomad/issues/19172)]
* api: Fix panic in `Allocation.Stub` method when `Job` is unset [[GH-19115](https://github.com/hashicorp/nomad/issues/19115)]
* cli: Fixed a bug that caused the `nomad job restart` command to miscount the allocations to restart [[GH-19155](https://github.com/hashicorp/nomad/issues/19155)]
* cli: Fixed a panic when the `nomad job restart` command received an interrupt signal while waiting for an answer [[GH-19154](https://github.com/hashicorp/nomad/issues/19154)]
* cli: Fixed the `nomad job restart` command to create replacements for batch and system jobs and to prevent sysbatch jobs from being rescheduled since they never create replacements [[GH-19147](https://github.com/hashicorp/nomad/issues/19147)]
* client: Fixed a bug where client API calls would fail incorrectly with permission denied errors when using ACL tokens with dangling policies [[GH-18972](https://github.com/hashicorp/nomad/issues/18972)]
* core: Fix incorrect submit time for stopped jobs [[GH-18967](https://github.com/hashicorp/nomad/issues/18967)]
* ui: Fixed an issue where purging a job with a namespace did not process correctly [[GH-19139](https://github.com/hashicorp/nomad/issues/19139)]
* ui: fix an issue where starting a stopped job with default-less variables would not retain those variables when done via the job page start button in the web ui [[GH-19220](https://github.com/hashicorp/nomad/issues/19220)]
* ui: fix the job auto-linked variable path name when user lacks variable write permissions [[GH-18598](https://github.com/hashicorp/nomad/issues/18598)]
* variables: Fixed a bug where poststop tasks were not allowed access to Variables [[GH-19270](https://github.com/hashicorp/nomad/issues/19270)]
* vault: Fixed a bug where poststop tasks would not get a Vault token [[GH-19268](https://github.com/hashicorp/nomad/issues/19268)]
* vault: Fixed an issue that could cause Nomad to attempt to renew a Vault token that is already expired [[GH-18985](https://github.com/hashicorp/nomad/issues/18985)]

## 1.6.3 (October 30, 2023)

SECURITY:

* build: Update to Go 1.21.3 [[GH-18717](https://github.com/hashicorp/nomad/issues/18717)]

IMPROVEMENTS:

* agent: Added config option to enable file and line log detail [[GH-18768](https://github.com/hashicorp/nomad/issues/18768)]
* api: Added support for the `log_include_location` query parameter within the
  `/v1/agent/monitor` HTTP endpoint [[GH-18795](https://github.com/hashicorp/nomad/issues/18795)]
* cli: Add `-prune` flag to `nomad operator force-leave` command [[GH-18463](https://github.com/hashicorp/nomad/issues/18463)]
* cli: Added `log-include-location` flag to the `monitor` command [[GH-18795](https://github.com/hashicorp/nomad/issues/18795)]
* cli: Added `log-include-location` flag to the `operator debug` command [[GH-18795](https://github.com/hashicorp/nomad/issues/18795)]
* csi: add ability to expand the size of volumes for plugins that support it [[GH-18359](https://github.com/hashicorp/nomad/issues/18359)]
* template: reduce memory usage associated with communicating with the Nomad API [[GH-18524](https://github.com/hashicorp/nomad/issues/18524)]
* ui: observe a token's roles' rules in the UI and add an interface for managing tokens, roles, and policies [[GH-17770](https://github.com/hashicorp/nomad/issues/17770)]

BUG FIXES:

* build: Add `timetzdata` Go build tag on Windows binaries to embed time zone data so periodic jobs are able to specify a time zone value on Windows environments [[GH-18676](https://github.com/hashicorp/nomad/issues/18676)]
* cli: Fixed an unexpected behavior of the `nomad acl token update` command that could cause a management token to be downgraded to client on update [[GH-18689](https://github.com/hashicorp/nomad/issues/18689)]
* cli: Use same offset when following single or multiple alloc logs [[GH-18604](https://github.com/hashicorp/nomad/issues/18604)]
* cli: ensure HCL env vars are added to the job submission object in the `job run` command [[GH-18832](https://github.com/hashicorp/nomad/issues/18832)]
* client: ensure null dynamic node metadata values are removed from memory [[GH-18664](https://github.com/hashicorp/nomad/issues/18664)]
* client: prevent tasks from starting without the prestart hooks running [[GH-18662](https://github.com/hashicorp/nomad/issues/18662)]
* metrics: Fixed a bug where CPU counters could report errors for negative values [[GH-18835](https://github.com/hashicorp/nomad/issues/18835)]
* scaling: Unblock blocking queries to /v1/job/{job-id}/scale if the job goes away [[GH-18637](https://github.com/hashicorp/nomad/issues/18637)]
* scheduler (Enterprise): auto-unblock evals with associated quotas when node resources are freed up [[GH-18838](https://github.com/hashicorp/nomad/issues/18838)]
* scheduler: Ensure duplicate allocation indexes are tracked and fixed when performing job updates [[GH-18873](https://github.com/hashicorp/nomad/issues/18873)]
* server: Fixed a bug where Raft server configuration parameters were not correctly merged [[GH-18494](https://github.com/hashicorp/nomad/issues/18494)]
* services: use interpolated address when performing nomad service health checks [[GH-18584](https://github.com/hashicorp/nomad/issues/18584)]
* ui: using start/stop from the job page in the UI will no longer fail when the job lacks HCL submission data [[GH-18621](https://github.com/hashicorp/nomad/issues/18621)]

## 1.6.2 (September 13, 2023)

IMPROVEMENTS:

* build: Update to Go 1.21.0 [[GH-18184](https://github.com/hashicorp/nomad/issues/18184)]
* cli: support wildcard namespaces in alloc subcommands when the `-job` flag is used [[GH-18095](https://github.com/hashicorp/nomad/issues/18095)]
* config: Added an option to configure how many historic versions of jobs are retained in the state store [[GH-17939](https://github.com/hashicorp/nomad/issues/17939)]
* consul/connect: Added support for `DestinationPeer`, `DestinationType`, `LocalBindSocketPath`, and `LocalBindSocketMode` in upstream block [[GH-16745](https://github.com/hashicorp/nomad/issues/16745)]
* jobspec: Add 'crons' field for multiple `cron` expressions [[GH-17858](https://github.com/hashicorp/nomad/issues/17858)]
* jobspec: Add new parameter `render_templates` for `restart` block to allow explicit re-render of templates on task restart. The default value is `false` and is fully backward compatible [[GH-18054](https://github.com/hashicorp/nomad/issues/18054)]
* jobspec: add `node_pool` as a valid field [[GH-18366](https://github.com/hashicorp/nomad/issues/18366)]
* raft: remove use of deprecated Leader func [[GH-18352](https://github.com/hashicorp/nomad/issues/18352)]
* status: go-getter failure reason now shown in `alloc status` [[GH-18444](https://github.com/hashicorp/nomad/issues/18444)]
* ui: Added configurable content security policy header [[GH-18085](https://github.com/hashicorp/nomad/issues/18085)]
* ui: adds a new Variables page to all job pages [[GH-17964](https://github.com/hashicorp/nomad/issues/17964)]
* ui: adds keyboard commands for pagination on lists using [[ and ]] [[GH-18210](https://github.com/hashicorp/nomad/issues/18210)]
* ui: sort variable key/values alphabetically by key when editing [[GH-18051](https://github.com/hashicorp/nomad/issues/18051)]
* ui: trim variable path names before saving [[GH-18198](https://github.com/hashicorp/nomad/issues/18198)]

BUG FIXES:

* acl: Fixed a bug where ACL tokens linked to ACL roles containing duplicate policies would cause erronous permission denined responses [[GH-18419](https://github.com/hashicorp/nomad/issues/18419)]
* cli: Add missing help message for the `-consul-namespace` flag in the `nomad job run` command [[GH-18081](https://github.com/hashicorp/nomad/issues/18081)]
* cli: Fix panic in `alloc logs` command when receiving empty stdout or stderr log frames [[GH-17815](https://github.com/hashicorp/nomad/issues/17815)]
* cli: Fixed a bug that prevented CSI volumes in namespaces other than `default` from being displayed in the `nomad node status -verbose` output [[GH-17925](https://github.com/hashicorp/nomad/issues/17925)]
* cli: Snapshot name is required in `volume snapshot create` command [[GH-17958](https://github.com/hashicorp/nomad/issues/17958)]
* client: Fixed a bug where the state of poststop tasks could be corrupted by client gc [[GH-17971](https://github.com/hashicorp/nomad/issues/17971)]
* client: Ignore stale server updates to prevent GCing allocations that should be running [[GH-18269](https://github.com/hashicorp/nomad/issues/18269)]
* client: return 404 instead of 500 when trying to access logs and files from allocations that have been garbage collected [[GH-18232](https://github.com/hashicorp/nomad/issues/18232)]
* core: Fixed a bug where exponential backoff could result in excessive CPU usage [[GH-18200](https://github.com/hashicorp/nomad/issues/18200)]
* csi: fixed a bug that could case a panic when deleting volumes [[GH-18234](https://github.com/hashicorp/nomad/issues/18234)]
* fingerprint: fix 'default' alias not being added to interface specified by network_interface [[GH-18096](https://github.com/hashicorp/nomad/issues/18096)]
* jobspec: Add diff for Task Group scaling block [[GH-18332](https://github.com/hashicorp/nomad/issues/18332)]
* migration: Fixed a bug where previous alloc logs were destroyed when migrating ephemeral_disk on the same client [[GH-18108](https://github.com/hashicorp/nomad/issues/18108)]
* scheduler: Fixed a bug where device IDs were not correctly filtered in constraints [[GH-18141](https://github.com/hashicorp/nomad/issues/18141)]
* services: Add validation message when `tls_skip_verify` is set to `true` on a Nomad service [[GH-18333](https://github.com/hashicorp/nomad/issues/18333)]
* ui: maintain HCL2 jobspec when using Start Job in the web ui [[GH-18120](https://github.com/hashicorp/nomad/issues/18120)]
* ui: search results are no longer overridden by sorting preferences on the jobs index page [[GH-18053](https://github.com/hashicorp/nomad/issues/18053)]

## 1.6.1 (July 21, 2023)

IMPROVEMENTS:

* cli: Display volume namespace on `nomad volume status` and `nomad node status` output [[GH-17911](https://github.com/hashicorp/nomad/issues/17911)]
* cpustats: Use config "cpu_total_compute" (if set) for all CPU statistics [[GH-17628](https://github.com/hashicorp/nomad/issues/17628)]
* metrics: Add `allocs.memory.max_allocated` to report the value of tasks' `memory_max` resource value [[GH-17938](https://github.com/hashicorp/nomad/issues/17938)]
* ui: added a button to copy variable path to clipboard [[GH-17935](https://github.com/hashicorp/nomad/issues/17935)]
* ui: adds a keyboard shortcut for Create Variable [[GH-17932](https://github.com/hashicorp/nomad/issues/17932)]
* ui: if a job is remotely purged while you're actively on it, it will let you know and re-route you to the index page [[GH-17915](https://github.com/hashicorp/nomad/issues/17915)]
* ui: indicate that nomad/jobs as a variable path is auto-accessible by all nomad jobs [[GH-17933](https://github.com/hashicorp/nomad/issues/17933)]

BUG FIXES:

* core: Fixed a bug where namespaces were not canonicalized on snapshot restore, resulting in potential nil access panic [[GH-18017](https://github.com/hashicorp/nomad/issues/18017)]
* csi: Fixed a bug in sending concurrent requests to CSI controller plugins by serializing them per plugin [[GH-17996](https://github.com/hashicorp/nomad/issues/17996)]
* csi: Fixed a bug where CSI controller requests could be sent to unhealthy plugins [[GH-17996](https://github.com/hashicorp/nomad/issues/17996)]
* csi: Fixed a bug where CSI controller requests could not be sent to controllers on nodes ineligible for scheduling [[GH-17996](https://github.com/hashicorp/nomad/issues/17996)]
* services: Fixed a bug that prevented passing query parameters in Nomad native service discovery HTTP health check paths [[GH-17936](https://github.com/hashicorp/nomad/issues/17936)]
* ui: Fixed a bug that could cause an error when accessing a region running versions of Nomad prior to 1.6.0 [[GH-18021](https://github.com/hashicorp/nomad/issues/18021)]
* ui: Fixed a bug that prevented nodes from being filtered by the "Ineligible" and "Draining" state filters [[GH-17940](https://github.com/hashicorp/nomad/issues/17940)]
* ui: Fixed error handling for cross-region requests when the receiving region does not implement the endpoint being requested [[GH-18020](https://github.com/hashicorp/nomad/issues/18020)]

## 1.6.0 (July 18, 2023)

FEATURES:

* **Node Pools**: Allow cluster operators to partition Nomad clients and control which jobs are allowed to run in each pool. [[GH-11041](https://github.com/hashicorp/nomad/issues/11041)]

BREAKING CHANGES:

* acl: Job evaluate endpoint now requires `submit-job` instead of `read-job` capability [[GH-16463](https://github.com/hashicorp/nomad/issues/16463)]

SECURITY:

* acl: Fixed a bug where a namespace ACL policy without label was applied to an unexpected namespace. [CVE-2023-3072](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2023-3072) [[GH-17908](https://github.com/hashicorp/nomad/issues/17908)]
* search: Fixed a bug where ACL did not filter plugin and variable names in search endpoint. [CVE-2023-3300](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2023-3300) [[GH-17906](https://github.com/hashicorp/nomad/issues/17906)]
* sentinel (Enterprise): Fixed a bug where ACL tokens could be exfiltrated via Sentinel logs [CVE-2023-3299](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2023-3299) [[GH-17907](https://github.com/hashicorp/nomad/issues/17907)]

IMPROVEMENTS:

* agent: Display server node ID in agent configuration at startup [[GH-17084](https://github.com/hashicorp/nomad/issues/17084)]
* api: enable support for storing original job source [[GH-16763](https://github.com/hashicorp/nomad/issues/16763)]
* api: return a structured error for unexpected responses [[GH-16743](https://github.com/hashicorp/nomad/issues/16743)]
* build: Publish official Docker images with the Nomad CLI [[GH-17017](https://github.com/hashicorp/nomad/issues/17017)]
* checks: Added support for Consul check field tls_server_name [[GH-17334](https://github.com/hashicorp/nomad/issues/17334)]
* cli: Add `-quiet` flag to `nomad var init` command [[GH-17526](https://github.com/hashicorp/nomad/issues/17526)]
* cli: Add check for missing host volume `path` in `nomad config validate` command [[GH-17393](https://github.com/hashicorp/nomad/issues/17393)]
* cli: Add leader status to output of `nomad server members -json` [[GH-17138](https://github.com/hashicorp/nomad/issues/17138)]
* cli: Add the ability to customize the details of the CA when running `nomad tls ca create` [[GH-17309](https://github.com/hashicorp/nomad/issues/17309)]
* cli: Sort output by Node name of the command `nomad operator raft list-peers` [[GH-16221](https://github.com/hashicorp/nomad/issues/16221)]
* cli: `job plan` help text for running the plan now includes the `-namespace` flag [[GH-16243](https://github.com/hashicorp/nomad/issues/16243)]
* client: check kernel module in `/sys/module` to help with WSL2 bridge networking [[GH-17306](https://github.com/hashicorp/nomad/issues/17306)]
* client: de-duplicate allocation client status updates and prevent allocation client status updates from being sent until clients have first synchronized with the server [[GH-17074](https://github.com/hashicorp/nomad/issues/17074)]
* client: prioritize allocation updates to reduce Raft and RPC load [[GH-17354](https://github.com/hashicorp/nomad/issues/17354)]
* cni: Ensure to setup CNI addresses in deterministic order [[GH-17766](https://github.com/hashicorp/nomad/issues/17766)]
* connect: Auto detect when to use podman for connect sidecar proxies [[GH-17065](https://github.com/hashicorp/nomad/issues/17065)]
* connect: do not restrict automatic envoy versioning to docker driver [[GH-17041](https://github.com/hashicorp/nomad/issues/17041)]
* connect: use full docker.io prefixed name for envoy image references [[GH-17045](https://github.com/hashicorp/nomad/issues/17045)]
* deploymentwatcher: Allow deployments to fail early when running out of reschedule attempts [[GH-17341](https://github.com/hashicorp/nomad/issues/17341)]
* deps: Updated Vault SDK to 0.9.0 [[GH-17281](https://github.com/hashicorp/nomad/issues/17281)]
* deps: Updated consul-template to v0.31.0 [[GH-16908](https://github.com/hashicorp/nomad/issues/16908)]
* deps: update docker to 23.0.3 [[GH-16862](https://github.com/hashicorp/nomad/issues/16862)]
* deps: update github.com/hashicorp/raft from 1.3.11 to 1.5.0 [[GH-17421](https://github.com/hashicorp/nomad/issues/17421)]
* deps: update go.etcd.io/bbolt from 1.3.6 to 1.3.7 [[GH-16228](https://github.com/hashicorp/nomad/issues/16228)]
* docker: Add `group_add` configuration [[GH-17313](https://github.com/hashicorp/nomad/issues/17313)]
* docker: Added option for labeling container with parent job ID of periodic/dispatch jobs [[GH-17843](https://github.com/hashicorp/nomad/issues/17843)]
* drivers: Add `DisableLogCollection` to task driver capabilities interface [[GH-17196](https://github.com/hashicorp/nomad/issues/17196)]
* metrics: add "total_ticks_count" counter for allocs/host CPU usage [[GH-17579](https://github.com/hashicorp/nomad/issues/17579)]
* runtime: Added 'os.build' attribute to node fingerprint on windows os [[GH-17576](https://github.com/hashicorp/nomad/issues/17576)]
* ui: Added a new Job Status Panel that helps show allocation status throughout a deployment and in steady state [[GH-16134](https://github.com/hashicorp/nomad/issues/16134)]
* ui: Adds a Download as .nomad.hcl button to jobspec editing in the UI [[GH-17752](https://github.com/hashicorp/nomad/issues/17752)]
* ui: Job status and deployment redesign [[GH-16932](https://github.com/hashicorp/nomad/issues/16932)]
* ui: Restyles "toast" notifications in the web UI with the Helios Design System [[GH-16099](https://github.com/hashicorp/nomad/issues/16099)]
* ui: add tooltips to the node and datacenter labels in the Topology page [[GH-17647](https://github.com/hashicorp/nomad/issues/17647)]
* ui: adds a toggle and localStorage property to Word Wrap logs and job definitions [[GH-17754](https://github.com/hashicorp/nomad/issues/17754)]
* ui: adds keyboard nav for switching between regions by pressing "r 1", "r 2", etc. [[GH-17169](https://github.com/hashicorp/nomad/issues/17169)]
* ui: affix page header to the top of the browser window to handle browser extension push-down gracefully [[GH-17783](https://github.com/hashicorp/nomad/issues/17783)]
* ui: change token input type from text to password [[GH-17345](https://github.com/hashicorp/nomad/issues/17345)]
* ui: remove namespace, type, and priority columns from child job table [[GH-17645](https://github.com/hashicorp/nomad/issues/17645)]
* vault: Add new configuration `disable_file` to prevent access to the Vault token by tasks that use `image` filesystem isolation [[GH-13343](https://github.com/hashicorp/nomad/issues/13343)]

DEPRECATIONS:

* envoy: remove support for envoy fallback image [[GH-17044](https://github.com/hashicorp/nomad/issues/17044)]

BUG FIXES:

* api: Fixed a bug that caused a panic when calling the `Jobs().Plan()` function with a job missing an ID [[GH-17689](https://github.com/hashicorp/nomad/issues/17689)]
* api: add missing constant for unknown allocation status [[GH-17726](https://github.com/hashicorp/nomad/issues/17726)]
* api: add missing field NetworkStatus for Allocation [[GH-17280](https://github.com/hashicorp/nomad/issues/17280)]
* cgroups: Fixed a bug removing all DevicesSets when alloc is created/removed [[GH-17535](https://github.com/hashicorp/nomad/issues/17535)]
* cli: Fix a panic in the `nomad job restart` command when monitoring replacement allocations [[GH-17346](https://github.com/hashicorp/nomad/issues/17346)]
* cli: Output error messages during deployment monitoring [[GH-17348](https://github.com/hashicorp/nomad/issues/17348)]
* client: Fixed a bug where Nomad incorrectly wrote to memory swappiness cgroup on old kernels [[GH-17625](https://github.com/hashicorp/nomad/issues/17625)]
* client: Fixed a bug where agent would panic during drain incurred by shutdown [[GH-17450](https://github.com/hashicorp/nomad/issues/17450)]
* client: fixed a bug that prevented Nomad from fingerprinting Consul 1.13.8 correctly [[GH-17349](https://github.com/hashicorp/nomad/issues/17349)]
* consul: Fixed a bug where Nomad would repeatedly try to revoke successfully revoked SI tokens [[GH-17847](https://github.com/hashicorp/nomad/issues/17847)]
* core: Fix panic around client deregistration and pending heartbeats [[GH-17316](https://github.com/hashicorp/nomad/issues/17316)]
* core: fixed a bug that caused job validation to fail when a task with `kill_timeout` was placed inside a group with `update.progress_deadline` set to 0 [[GH-17342](https://github.com/hashicorp/nomad/issues/17342)]
* csi: Fixed a bug where CSI volumes would fail to restore during client restarts [[GH-17840](https://github.com/hashicorp/nomad/issues/17840)]
* docker: Fixed a bug where network pause container would not be removed after node restart [[GH-17455](https://github.com/hashicorp/nomad/issues/17455)]
* drivers/docker: Fixed a bug where long-running docker operations would incorrectly timeout [[GH-17731](https://github.com/hashicorp/nomad/issues/17731)]
* identity: Fixed a bug where workload identities for periodic and dispatch jobs would not have access to their parent job's ACL policy [[GH-17018](https://github.com/hashicorp/nomad/issues/17018)]
* replication: Fix a potential panic when a non-authoritative region is upgraded and a server with the new version becomes the leader. [[GH-17476](https://github.com/hashicorp/nomad/issues/17476)]
* scheduler: Fixed a panic when a node has only one configured dynamic port [[GH-17619](https://github.com/hashicorp/nomad/issues/17619)]
* tls: Fixed a bug where the `nomad tls cert` command did not create certificates with the correct SANs for them to work with non default domain and region names. [[GH-16959](https://github.com/hashicorp/nomad/issues/16959)]
* ui: dont show a service as healthy when its parent allocation stops running [[GH-17465](https://github.com/hashicorp/nomad/issues/17465)]
* ui: fix a mirage-only issue where our mock token logs repeated unnecessarily [[GH-17010](https://github.com/hashicorp/nomad/issues/17010)]
* ui: fixed a handful of UX-related bugs during variable editing [[GH-17319](https://github.com/hashicorp/nomad/issues/17319)]
* ui: fixes an issue where the allocations table on child (periodic, parameterized) job pages wouldn't update when accessed via their parent [[GH-17214](https://github.com/hashicorp/nomad/issues/17214)]
* ui: preserve newlines when displaying shown variables in non-json mode [[GH-17343](https://github.com/hashicorp/nomad/issues/17343)]

## 1.5.17 (April 16, 2024)
SECURITY:

artifact: Updated go-getter dependency to v1.7.4 to address CVE-2024-3817 [GH-20391]
BUG FIXES:

* api: Fixed a bug where AllocDirStats field was missing from Read Stats client API [GH-20261]
* cli: Fixed a bug where operator debug did not respect the -pprof-interval flag and would take only one profile [GH-20206]
* cni: Fixed a regression where default DNS set by dockerd or other task drivers was not respected [GH-20189]
* config: Fixed a bug where IPv6 addresses were not accepted without ports for client.servers blocks [GH-20324]
* deployments: Fixed a goroutine leak when jobs are purged [GH-20348]
* deps: Updated consul-template dependency to 0.37.4 to fix a resource leak [GH-20234]
* drain: Fixed a bug where Workload Identity tokens could not be used to drain a node [GH-20317]
* state: Fixed a bug where restarting a server could fail if the Raft logs include a drain update that used a now-expired token [GH-20317]
* template: Fixed a bug where a partial client.template block would cause defaults for unspecified fields to be ignored [GH-20165]

## 1.5.16 (March 12, 2024)

SECURITY:

* build: Update to go1.22 to address Go standard library vulnerabilities CVE-2024-24783, CVE-2023-45290, and CVE-2024-24785. [[GH-20066](https://github.com/hashicorp/nomad/issues/20066)]
* deps: Upgrade protobuf library to 1.33.0 to avoid scan alerts for CVE-2024-24786, which Nomad is not vulnerable to [[GH-20100](https://github.com/hashicorp/nomad/issues/20100)]

BUG FIXES:

* cli: Fixed a bug where the `nomad job restart` command could crash if the job type was not present in a response from the server [[GH-20049](https://github.com/hashicorp/nomad/issues/20049)]
* client: Fixed a bug where corrupt client state could panic the client [[GH-19972](https://github.com/hashicorp/nomad/issues/19972)]
* cni: Fixed a bug where DNS set by CNI plugins was not provided to task drivers [[GH-20007](https://github.com/hashicorp/nomad/issues/20007)]
* connect: Fixed a bug where `expose` blocks would not appear in `job plan` diff output [[GH-19990](https://github.com/hashicorp/nomad/issues/19990)]

## 1.5.15 (February 13, 2024)

SECURITY:

* windows: Remove `LazyDLL` calls for system modules to harden Nomad against attacks from the host [[GH-19925](https://github.com/hashicorp/nomad/issues/19925)]

BUG FIXES:

* cli: Fix return code when `nomad job run` succeeds after a blocked eval [[GH-19876](https://github.com/hashicorp/nomad/issues/19876)]
* connect: Fixed envoy sidecars being unable to restart after node reboots [[GH-19787](https://github.com/hashicorp/nomad/issues/19787)]
* exec: Fixed a bug in `alloc exec` where closing websocket streams could cause a panic [[GH-19932](https://github.com/hashicorp/nomad/issues/19932)]
* scheduler: Fixed a bug that caused blocked evaluations due to port conflict to not have a reason explaining why the evaluation was blocked [[GH-19933](https://github.com/hashicorp/nomad/issues/19933)]
* ui: Fix an issue where a same-named task from a different group could be selected when the user clicks Exec from a task group page where multiple allocations would be valid [[GH-19878](https://github.com/hashicorp/nomad/issues/19878)]

## 1.5.14 (February 08, 2024)

SECURITY:

* deps: Updated runc to 1.1.12 to address CVE-2024-21626 [[GH-19851](https://github.com/hashicorp/nomad/issues/19851)]
* migration: Fixed a bug where archives used for migration were not checked for symlinks that escaped the allocation directory [[GH-19887](https://github.com/hashicorp/nomad/issues/19887)]
* template: Fixed a bug where symlinks could force templates to read and write to arbitrary locations (CVE-2024-1329) [[GH-19888](https://github.com/hashicorp/nomad/issues/19888)]

## 1.5.13 (January 15, 2024)

IMPROVEMENTS:

* build: update to go 1.21.6 [[GH-19709](https://github.com/hashicorp/nomad/issues/19709)]

BUG FIXES:

* acl: Fixed auth method hashing which meant changing some fields would be silently ignored [[GH-19677](https://github.com/hashicorp/nomad/issues/19677)]
* auth: Added new optional OIDCDisableUserInfo setting for OIDC auth provider [[GH-19566](https://github.com/hashicorp/nomad/issues/19566)]
* namespaces: Failed delete calls no longer return success codes [[GH-19483](https://github.com/hashicorp/nomad/issues/19483)]
* server: Fix server not waiting for workers to submit nacks for dequeued evaluations before shutting down [[GH-19560](https://github.com/hashicorp/nomad/issues/19560)]
* state: Fixed a bug where purged jobs would not get new deployments [[GH-19609](https://github.com/hashicorp/nomad/issues/19609)]

## 1.5.12 (December 13, 2023)

BUG FIXES:

* cli: Fix a bug in the `var put` command which prevented combining items as CLI arguments and other parameters as flags [[GH-19423](https://github.com/hashicorp/nomad/issues/19423)]
* client: remove incomplete allocation entries from client state database during client restarts [[GH-16638](https://github.com/hashicorp/nomad/issues/16638)]
* connect: Fixed a bug where deployments would not wait for Connect sidecar task health checks to pass [[GH-19334](https://github.com/hashicorp/nomad/issues/19334)]
* consul: uses token namespace to fetch policies for verification [[GH-18516](https://github.com/hashicorp/nomad/issues/18516)]
* csi: Added validation to `csi_plugin` blocks to prevent `stage_publish_base_dir` from being a subdirectory of `mount_dir` [[GH-19441](https://github.com/hashicorp/nomad/issues/19441)]
* metrics: Revert upgrade of `go-metrics` to fix an issue where metrics from dependencies, such as raft, were no longer emitted [[GH-19376](https://github.com/hashicorp/nomad/issues/19376)]

## 1.5.11 (December 07, 2023)

BREAKING CHANGES:

* core: Honor job's namespace when checking `distinct_hosts` feasibility [[GH-19004](https://github.com/hashicorp/nomad/issues/19004)]

SECURITY:

* build: Update to go1.21.5 to resolve Windows path validation CVE in Go [[GH-19320](https://github.com/hashicorp/nomad/issues/19320)]

BUG FIXES:

* agent: Correct websocket status code handling [[GH-19172](https://github.com/hashicorp/nomad/issues/19172)]
* api: Fix panic in `Allocation.Stub` method when `Job` is unset [[GH-19115](https://github.com/hashicorp/nomad/issues/19115)]
* cli: Fixed a panic when the `nomad job restart` command received an interrupt signal while waiting for an answer [[GH-19154](https://github.com/hashicorp/nomad/issues/19154)]
* cli: Fixed the `nomad job restart` command to create replacements for batch and system jobs and to prevent sysbatch jobs from being rescheduled since they never create replacements [[GH-19147](https://github.com/hashicorp/nomad/issues/19147)]
* client: Fixed a bug where client API calls would fail incorrectly with permission denied errors when using ACL tokens with dangling policies [[GH-18972](https://github.com/hashicorp/nomad/issues/18972)]
* core: Fix incorrect submit time for stopped jobs [[GH-18967](https://github.com/hashicorp/nomad/issues/18967)]
* ui: Fixed an issue where purging a job with a namespace did not process correctly [[GH-19139](https://github.com/hashicorp/nomad/issues/19139)]
* variables: Fixed a bug where poststop tasks were not allowed access to Variables [[GH-19270](https://github.com/hashicorp/nomad/issues/19270)]
* vault: Fixed a bug where poststop tasks would not get a Vault token [[GH-19268](https://github.com/hashicorp/nomad/issues/19268)]
* vault: Fixed an issue that could cause Nomad to attempt to renew a Vault token that is already expired [[GH-18985](https://github.com/hashicorp/nomad/issues/18985)]

## 1.5.10 (October 30, 2023)

SECURITY:

* build: Update to Go 1.21.3 [[GH-18717](https://github.com/hashicorp/nomad/issues/18717)]

BUG FIXES:

* build: Add `timetzdata` Go build tag on Windows binaries to embed time zone data so periodic jobs are able to specify a time zone value on Windows environments [[GH-18676](https://github.com/hashicorp/nomad/issues/18676)]
* cli: Fixed an unexpected behavior of the `nomad acl token update` command that could cause a management token to be downgraded to client on update [[GH-18689](https://github.com/hashicorp/nomad/issues/18689)]
* client: ensure null dynamic node metadata values are removed from memory [[GH-18664](https://github.com/hashicorp/nomad/issues/18664)]
* client: prevent tasks from starting without the prestart hooks running [[GH-18662](https://github.com/hashicorp/nomad/issues/18662)]
* csi: check controller plugin health early during volume register/create [[GH-18570](https://github.com/hashicorp/nomad/issues/18570)]
* metrics: Fixed a bug where CPU counters could report errors for negative values [[GH-18835](https://github.com/hashicorp/nomad/issues/18835)]
* scaling: Unblock blocking queries to /v1/job/{job-id}/scale if the job goes away [[GH-18637](https://github.com/hashicorp/nomad/issues/18637)]
* scheduler (Enterprise): auto-unblock evals with associated quotas when node resources are freed up [[GH-18838](https://github.com/hashicorp/nomad/issues/18838)]
* scheduler: Ensure duplicate allocation indexes are tracked and fixed when performing job updates [[GH-18873](https://github.com/hashicorp/nomad/issues/18873)]
* services: use interpolated address when performing nomad service health checks [[GH-18584](https://github.com/hashicorp/nomad/issues/18584)]

## 1.5.9 (September 13, 2023)

IMPROVEMENTS:

* build: Update to Go 1.21.0 [[GH-18184](https://github.com/hashicorp/nomad/issues/18184)]
* raft: remove use of deprecated Leader func [[GH-18352](https://github.com/hashicorp/nomad/issues/18352)]

BUG FIXES:

* acl: Fixed a bug where ACL tokens linked to ACL roles containing duplicate policies would cause erronous permission denined responses [[GH-18419](https://github.com/hashicorp/nomad/issues/18419)]
* cli: Add missing help message for the `-consul-namespace` flag in the `nomad job run` command [[GH-18081](https://github.com/hashicorp/nomad/issues/18081)]
* cli: Fix panic in `alloc logs` command when receiving empty stdout or stderr log frames [[GH-17815](https://github.com/hashicorp/nomad/issues/17815)]
* cli: Fixed a bug that prevented CSI volumes in namespaces other than `default` from being displayed in the `nomad node status -verbose` output [[GH-17925](https://github.com/hashicorp/nomad/issues/17925)]
* cli: Snapshot name is required in `volume snapshot create` command [[GH-17958](https://github.com/hashicorp/nomad/issues/17958)]
* client: Fixed a bug where the state of poststop tasks could be corrupted by client gc [[GH-17971](https://github.com/hashicorp/nomad/issues/17971)]
* client: Ignore stale server updates to prevent GCing allocations that should be running [[GH-18269](https://github.com/hashicorp/nomad/issues/18269)]
* client: return 404 instead of 500 when trying to access logs and files from allocations that have been garbage collected [[GH-18232](https://github.com/hashicorp/nomad/issues/18232)]
* core: Fixed a bug where exponential backoff could result in excessive CPU usage [[GH-18200](https://github.com/hashicorp/nomad/issues/18200)]
* csi: fixed a bug that could case a panic when deleting volumes [[GH-18234](https://github.com/hashicorp/nomad/issues/18234)]
* fingerprint: fix 'default' alias not being added to interface specified by network_interface [[GH-18096](https://github.com/hashicorp/nomad/issues/18096)]
* jobspec: Add diff for Task Group scaling block [[GH-18332](https://github.com/hashicorp/nomad/issues/18332)]
* migration: Fixed a bug where previous alloc logs were destroyed when migrating ephemeral_disk on the same client [[GH-18108](https://github.com/hashicorp/nomad/issues/18108)]
* scheduler: Fixed a bug where device IDs were not correctly filtered in constraints [[GH-18141](https://github.com/hashicorp/nomad/issues/18141)]
* services: Add validation message when `tls_skip_verify` is set to `true` on a Nomad service [[GH-18333](https://github.com/hashicorp/nomad/issues/18333)]

## 1.5.8 (July 21, 2023)

IMPROVEMENTS:

* cpustats: Use config "cpu_total_compute" (if set) for all CPU statistics [[GH-17628](https://github.com/hashicorp/nomad/issues/17628)]

BUG FIXES:

* csi: Fixed a bug in sending concurrent requests to CSI controller plugins by serializing them per plugin [[GH-17996](https://github.com/hashicorp/nomad/issues/17996)]
* csi: Fixed a bug where CSI controller requests could be sent to unhealthy plugins [[GH-17996](https://github.com/hashicorp/nomad/issues/17996)]
* csi: Fixed a bug where CSI controller requests could not be sent to controllers on nodes ineligible for scheduling [[GH-17996](https://github.com/hashicorp/nomad/issues/17996)]
* services: Fixed a bug that prevented passing query parameters in Nomad native service discovery HTTP health check paths [[GH-17936](https://github.com/hashicorp/nomad/issues/17936)]
* ui: Fixed a bug that prevented nodes from being filtered by the "Ineligible" and "Draining" state filters [[GH-17940](https://github.com/hashicorp/nomad/issues/17940)]
* ui: Fixed error handling for cross-region requests when the receiving region does not implement the endpoint being requested [[GH-18020](https://github.com/hashicorp/nomad/issues/18020)]

## 1.5.7 (July 18, 2023)

SECURITY:

* acl: Fixed a bug where a namespace ACL policy without label was applied to an unexpected namespace. [CVE-2023-3072](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2023-3072) [[GH-17908](https://github.com/hashicorp/nomad/issues/17908)]
* search: Fixed a bug where ACL did not filter plugin and variable names in search endpoint. [CVE-2023-3300](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2023-3300) [[GH-17906](https://github.com/hashicorp/nomad/issues/17906)]
* sentinel (Enterprise): Fixed a bug where ACL tokens could be exfiltrated via Sentinel logs [CVE-2023-3299](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2023-3299) [[GH-17907](https://github.com/hashicorp/nomad/issues/17907)]

IMPROVEMENTS:

* cli: Add `-quiet` flag to `nomad var init` command [[GH-17526](https://github.com/hashicorp/nomad/issues/17526)]
* cli: Add check for missing host volume `path` in `nomad config validate` command [[GH-17393](https://github.com/hashicorp/nomad/issues/17393)]
* client: check kernel module in `/sys/module` to help with WSL2 bridge networking [[GH-17306](https://github.com/hashicorp/nomad/issues/17306)]
* cni: Ensure to setup CNI addresses in deterministic order [[GH-17766](https://github.com/hashicorp/nomad/issues/17766)]
* deps: Updated Vault SDK to 0.9.0 [[GH-17281](https://github.com/hashicorp/nomad/issues/17281)]
* deps: update docker to 23.0.3 [[GH-16862](https://github.com/hashicorp/nomad/issues/16862)]
* docker: Add `group_add` configuration [[GH-17313](https://github.com/hashicorp/nomad/issues/17313)]
* ui: adds keyboard nav for switching between regions by pressing "r 1", "r 2", etc. [[GH-17169](https://github.com/hashicorp/nomad/issues/17169)]

BUG FIXES:

* api: Fixed a bug that caused a panic when calling the `Jobs().Plan()` function with a job missing an ID [[GH-17689](https://github.com/hashicorp/nomad/issues/17689)]
* api: add missing constant for unknown allocation status [[GH-17726](https://github.com/hashicorp/nomad/issues/17726)]
* api: add missing field NetworkStatus for Allocation [[GH-17280](https://github.com/hashicorp/nomad/issues/17280)]
* cgroups: Fixed a bug removing all DevicesSets when alloc is created/removed [[GH-17535](https://github.com/hashicorp/nomad/issues/17535)]
* cli: Fix a panic in the `nomad job restart` command when monitoring replacement allocations [[GH-17346](https://github.com/hashicorp/nomad/issues/17346)]
* cli: Output error messages during deployment monitoring [[GH-17348](https://github.com/hashicorp/nomad/issues/17348)]
* client: Fixed a bug where Nomad incorrectly wrote to memory swappiness cgroup on old kernels [[GH-17625](https://github.com/hashicorp/nomad/issues/17625)]
* client: Fixed a bug where agent would panic during drain incurred by shutdown [[GH-17450](https://github.com/hashicorp/nomad/issues/17450)]
* client: fixed a bug that prevented Nomad from fingerprinting Consul 1.13.8 correctly [[GH-17349](https://github.com/hashicorp/nomad/issues/17349)]
* consul: Fixed a bug where Nomad would repeatedly try to revoke successfully revoked SI tokens [[GH-17847](https://github.com/hashicorp/nomad/issues/17847)]
* core: Fix panic around client deregistration and pending heartbeats [[GH-17316](https://github.com/hashicorp/nomad/issues/17316)]
* core: fixed a bug that caused job validation to fail when a task with `kill_timeout` was placed inside a group with `update.progress_deadline` set to 0 [[GH-17342](https://github.com/hashicorp/nomad/issues/17342)]
* csi: Fixed a bug where CSI volumes would fail to restore during client restarts [[GH-17840](https://github.com/hashicorp/nomad/issues/17840)]
* docker: Fixed a bug where network pause container would not be removed after node restart [[GH-17455](https://github.com/hashicorp/nomad/issues/17455)]
* drivers/docker: Fixed a bug where long-running docker operations would incorrectly timeout [[GH-17731](https://github.com/hashicorp/nomad/issues/17731)]
* identity: Fixed a bug where workload identities for periodic and dispatch jobs would not have access to their parent job's ACL policy [[GH-17018](https://github.com/hashicorp/nomad/issues/17018)]
* replication: Fix a potential panic when a non-authoritative region is upgraded and a server with the new version becomes the leader. [[GH-17476](https://github.com/hashicorp/nomad/issues/17476)]
* scheduler: Fixed a bug that could cause replacements for failed allocations to be placed in the wrong datacenter during a canary deployment [[GH-17652](https://github.com/hashicorp/nomad/issues/17652)]
* scheduler: Fixed a panic when a node has only one configured dynamic port [[GH-17619](https://github.com/hashicorp/nomad/issues/17619)]
* tls: Fixed a bug where the `nomad tls cert` command did not create certificates with the correct SANs for them to work with non default domain and region names. [[GH-16959](https://github.com/hashicorp/nomad/issues/16959)]
* ui: dont show a service as healthy when its parent allocation stops running [[GH-17465](https://github.com/hashicorp/nomad/issues/17465)]
* ui: fixed a handful of UX-related bugs during variable editing [[GH-17319](https://github.com/hashicorp/nomad/issues/17319)]

## 1.5.6 (May 19, 2023)

IMPROVEMENTS:

* core: Prevent `task.kill_timeout` being greater than `update.progress_deadline` [[GH-16761](https://github.com/hashicorp/nomad/issues/16761)]

BUG FIXES:

* bug: Corrected status description and modification time for canceled evaluations [[GH-17071](https://github.com/hashicorp/nomad/issues/17071)]
* build: Linux packages now have vendor label and set the default label to HashiCorp. This fix is implemented for any future releases, but will not be updated for historical releases [[GH-16071](https://github.com/hashicorp/nomad/issues/16071)]
* client: Fixed a bug where restarting a terminal allocation turns it into a zombie where allocation and task hooks will run unexpectedly [[GH-17175](https://github.com/hashicorp/nomad/issues/17175)]
* client: clean up resources upon failure to restore task during client restart [[GH-17104](https://github.com/hashicorp/nomad/issues/17104)]
* logs: Fixed a bug where disabling log collection would prevent Windows tasks from starting [[GH-17199](https://github.com/hashicorp/nomad/issues/17199)]
* scale: Fixed a bug where evals could be created with the wrong type [[GH-17092](https://github.com/hashicorp/nomad/issues/17092)]
* scheduler: Fixed a bug where implicit `spread` targets were treated as separate targets for scoring [[GH-17195](https://github.com/hashicorp/nomad/issues/17195)]
* scheduler: Fixed a bug where scores for spread scheduling could be -Inf [[GH-17198](https://github.com/hashicorp/nomad/issues/17198)]
* services: Fixed a bug preventing group service deregistrations after alloc restarts [[GH-16905](https://github.com/hashicorp/nomad/issues/16905)]

## 1.5.5 (May 05, 2023)

BUG FIXES:

* logging: Fixed a bug where alloc logs would not be collected after an upgrade to 1.5.4 [[GH-17087](https://github.com/hashicorp/nomad/issues/17087)]

## 1.5.4 (May 02, 2023)

BREAKING CHANGES:

* artifact: environment variables no longer inherited by default from Nomad client [[GH-15514](https://github.com/hashicorp/nomad/issues/15514)]

IMPROVEMENTS:

* acl: New auth-method type: JWT [[GH-15897](https://github.com/hashicorp/nomad/issues/15897)]
* build: Update from Go 1.20.3 to Go 1.20.4 [[GH-17056](https://github.com/hashicorp/nomad/issues/17056)]
* cli: Added new `nomad job restart` command to restart all allocations for a job [[GH-16278](https://github.com/hashicorp/nomad/issues/16278)]
* cli: stream both stdout and stderr logs by default when following an allocation [[GH-16556](https://github.com/hashicorp/nomad/issues/16556)]
* client/fingerprint: detect fastest cpu core during cpu performance fallback [[GH-16740](https://github.com/hashicorp/nomad/issues/16740)]
* client: Added `drain_on_shutdown` configuration [[GH-16827](https://github.com/hashicorp/nomad/issues/16827)]
* connect: Added support for meta field on sidecar service block [[GH-16705](https://github.com/hashicorp/nomad/issues/16705)]
* dependency: update runc to 1.1.5 [[GH-16712](https://github.com/hashicorp/nomad/issues/16712)]
* driver/docker: Default `devices.container_path` to `devices.host_path` like Docker's CLI [[GH-16811](https://github.com/hashicorp/nomad/issues/16811)]
* ephemeral disk: migrate=true now implies sticky=true [[GH-16826](https://github.com/hashicorp/nomad/issues/16826)]
* fingerprint/cpu: correctly fingerprint P/E cores of Apple Silicon chips [[GH-16672](https://github.com/hashicorp/nomad/issues/16672)]
* jobspec: Added option for disabling task log collection in the `logs` block [[GH-16962](https://github.com/hashicorp/nomad/issues/16962)]
* license: show Terminated field in `license get` command [[GH-16892](https://github.com/hashicorp/nomad/issues/16892)]
* ui: Added copy-to-clipboard buttons to server and client pages [[GH-16548](https://github.com/hashicorp/nomad/issues/16548)]
* ui: added new keyboard commands for job start, stop, exec, and client metadata [[GH-16378](https://github.com/hashicorp/nomad/issues/16378)]

BUG FIXES:

* api: Fixed filtering on maps with missing keys [[GH-16991](https://github.com/hashicorp/nomad/issues/16991)]
* cli: Fix panic on job plan when -diff=false [[GH-16944](https://github.com/hashicorp/nomad/issues/16944)]
* client: Fix CNI plugin version fingerprint when output includes protocol version [[GH-16776](https://github.com/hashicorp/nomad/issues/16776)]
* client: Fix address for ports in IPv6 networks [[GH-16723](https://github.com/hashicorp/nomad/issues/16723)]
* client: Fixed a bug where restarting proxy sidecar tasks failed [[GH-16815](https://github.com/hashicorp/nomad/issues/16815)]
* client: Prevent a panic when an allocation has a legacy task-level bridge network and uses a driver that does not create a network namespace [[GH-16921](https://github.com/hashicorp/nomad/issues/16921)]
* client: Remove setting attributes when spawning the getter child [[GH-16791](https://github.com/hashicorp/nomad/issues/16791)]
* core: the deployment's list endpoint now supports look up by prefix using the wildcard for namespace [[GH-16792](https://github.com/hashicorp/nomad/issues/16792)]
* csi: gracefully recover tasks that use csi node plugins [[GH-16809](https://github.com/hashicorp/nomad/issues/16809)]
* docker: Fixed a bug where plugin config values were ignored [[GH-16713](https://github.com/hashicorp/nomad/issues/16713)]
* drain: Fixed a bug where drains would complete based on the server status and not the client status of an allocation [[GH-14348](https://github.com/hashicorp/nomad/issues/14348)]
* driver/exec: Fixed a bug where `cap_drop` and `cap_add` would not expand capabilities [[GH-16643](https://github.com/hashicorp/nomad/issues/16643)]
* fix: Added "/usr/libexec" to the landlocked directories the getter has access to [[GH-16900](https://github.com/hashicorp/nomad/issues/16900)]
* scale: Do not allow scale requests for jobs of type system [[GH-16969](https://github.com/hashicorp/nomad/issues/16969)]
* scheduler: Fix reconciliation of reconnecting allocs when the replacement allocations are not running [[GH-16609](https://github.com/hashicorp/nomad/issues/16609)]
* scheduler: honor false value for distinct_hosts constraint [[GH-16907](https://github.com/hashicorp/nomad/issues/16907)]
* server: Added verification of cron jobs already running before forcing new evals right after leader change [[GH-16583](https://github.com/hashicorp/nomad/issues/16583)]
* ui: Fix a visual bug where evaluation response wasn't scrollable in the Web UI. [[GH-16960](https://github.com/hashicorp/nomad/issues/16960)]

## 1.5.3 (April 04, 2023)

SECURITY:

* acl: Fixed a bug where unauthenticated HTTP API requests through the client could bypass ACL policy checking [CVE-2023-1782](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2023-1782) [[GH-16775](https://github.com/hashicorp/nomad/issues/16775)] [[GH-16775](https://github.com/hashicorp/nomad/issues/16775)]
* build: update to Go 1.20.3 to prevent denial of service attack via malicious HTTP headers [CVE-2023-24534](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2023-24534) [[GH-16788](https://github.com/hashicorp/nomad/issues/16788)]

## 1.5.2 (March 21, 2023)

BREAKING CHANGES:

* cli: nomad login no longer requires -type flag, since auth method names are globally unique. [[GH-16504](https://github.com/hashicorp/nomad/issues/16504)]

IMPROVEMENTS:

* agent: trim leading and trailing spaces when parsing `X-Nomad-Token` header [[GH-16469](https://github.com/hashicorp/nomad/issues/16469)]
* build: Update to go1.20.2 [[GH-16427](https://github.com/hashicorp/nomad/issues/16427)]
* cli: Added `-json` and `-t` flag to `namespace status` command [[GH-16442](https://github.com/hashicorp/nomad/issues/16442)]
* cli: Added `-json` and `-t` flag to `quota status` command [[GH-16485](https://github.com/hashicorp/nomad/issues/16485)]
* cli: Added `-json` and `-t` flag to `server members` command [[GH-16444](https://github.com/hashicorp/nomad/issues/16444)]
* cli: Added `-json` flag to `quota inspect` command [[GH-16478](https://github.com/hashicorp/nomad/issues/16478)]
* scheduler: remove most uses of reflection for task comparisons [[GH-16421](https://github.com/hashicorp/nomad/issues/16421)]

BUG FIXES:

* artifact: Fixed a bug where artifact downloading failed when using git-ssh [[GH-16495](https://github.com/hashicorp/nomad/issues/16495)]
* cli: nomad login no longer ignores default auth method if they are present. [[GH-16504](https://github.com/hashicorp/nomad/issues/16504)]
* client: Fixed a bug where artifact downloading failed on hardened nodes [[GH-16375](https://github.com/hashicorp/nomad/issues/16375)]
* client: Fixed a bug where clients using Consul discovery to join the cluster would get permission denied errors [[GH-16490](https://github.com/hashicorp/nomad/issues/16490)]
* client: Fixed a bug where cpuset initialization fails after Client restart [[GH-16467](https://github.com/hashicorp/nomad/issues/16467)]
* core: Fixed a bug where Dynamic Node Metadata requests could crash servers [[GH-16549](https://github.com/hashicorp/nomad/issues/16549)]
* plugin: Add missing fields to `TaskConfig` so they can be accessed by external task drivers [[GH-16434](https://github.com/hashicorp/nomad/issues/16434)]
* services: Fixed a bug where a service would be deregistered twice [[GH-16289](https://github.com/hashicorp/nomad/issues/16289)]

## 1.5.1 (March 10, 2023)

BREAKING CHANGES:

* api: job register and register requests from API clients older than version 0.12.1 will not longer emit an evaluation [[GH-16305](https://github.com/hashicorp/nomad/issues/16305)]

SECURITY:

* variables: Fixed a bug where a workload identity without any workload-associated policies was treated as a management token [CVE-2023-1299](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2023-1299) [[GH-16419](https://github.com/hashicorp/nomad/issues/16419)]
* variables: Fixed a bug where a workload-associated policy with a deny capability was ignored for the workload's own variables [CVE-2023-1296](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2023-1296) [[GH-16349](https://github.com/hashicorp/nomad/issues/16349)]

IMPROVEMENTS:

* cli: Add job prefix match to the `nomad job dispatch`, `nomad job eval`, `nomad job scale`, and `nomad job scaling-events` commands [[GH-16306](https://github.com/hashicorp/nomad/issues/16306)]
* cli: Add support for the wildcard namespace `*` to the `nomad job dispatch`, `nomad job eval`, `nomad job scale`, and `nomad job scaling-events` commands [[GH-16306](https://github.com/hashicorp/nomad/issues/16306)]
* cli: Added `-json` and `-t` flag to `alloc checks` command [[GH-16405](https://github.com/hashicorp/nomad/issues/16405)]
* env/ec2: update cpu metadata [[GH-16417](https://github.com/hashicorp/nomad/issues/16417)]

DEPRECATIONS:

* api: The `Restart()`, `Stop()`, and `Signal()` methods in the `Allocations` struct will have their signatures modified in Nomad 1.6.0 [[GH-16319](https://github.com/hashicorp/nomad/issues/16319)]
* api: The `RestartAllTasks()` method in the `Allocations` struct will be removed in Nomad 1.6.0 [[GH-16319](https://github.com/hashicorp/nomad/issues/16319)]

BUG FIXES:

* api: Fix `Allocations().Stop()` method to properly set the request `LastIndex` and `RequestTime` in the response [[GH-16319](https://github.com/hashicorp/nomad/issues/16319)]
* cli: Fixed a bug where the `-json` and `-t` flags were not respected on the `acl binding-rule info` command [[GH-16357](https://github.com/hashicorp/nomad/issues/16357)]
* client: Don't emit shutdown delay task event when the shutdown operation is configured to skip the delay [[GH-16281](https://github.com/hashicorp/nomad/issues/16281)]
* client: Fixed a bug that prevented allocations with interpolated values in Consul services from being marked as healthy [[GH-16402](https://github.com/hashicorp/nomad/issues/16402)]
* client: Fixed a bug where clients used the serf advertise address to connect to servers when using Consul auto-discovery [[GH-16217](https://github.com/hashicorp/nomad/issues/16217)]
* docker: Fixed a bug where pause containers would be erroneously removed [[GH-16352](https://github.com/hashicorp/nomad/issues/16352)]
* scheduler: Fixed a bug where allocs of system jobs with wildcard datacenters would be destructively updated [[GH-16362](https://github.com/hashicorp/nomad/issues/16362)]
* scheduler: Fixed a bug where collisions in dynamic port offerings would result in spurious plan-for-node-rejected errors [[GH-16401](https://github.com/hashicorp/nomad/issues/16401)]
* server: Fixed a bug where deregistering a job that was already garbage collected would create a new evaluation [[GH-16287](https://github.com/hashicorp/nomad/issues/16287)]
* server: Fixed a bug where node updates that produced errors from service discovery or CSI plugin updates were not logged [[GH-16287](https://github.com/hashicorp/nomad/issues/16287)]
* server: Fixed a bug where the `system reconcile summaries` command and API would not return any scheduler-related errors [[GH-16287](https://github.com/hashicorp/nomad/issues/16287)]
* service: Fixed a bug where attaching a policy to a job would prevent workload identities for the job from reading the service registration API [[GH-16316](https://github.com/hashicorp/nomad/issues/16316)]
* ui: fixed an issue where system/sysbatch jobs with wildcard datacenters (like ["dc*"]) were not showing client status charts [[GH-16274](https://github.com/hashicorp/nomad/issues/16274)]
* ui: fixed outbound link to outage recovery on error page [[GH-16365](https://github.com/hashicorp/nomad/issues/16365)]

## 1.5.0 (March 01, 2023)

FEATURES:

* **Dynamic Node Metadata**: Allow users and tasks to update Node metadata via an API [[GH-15844](https://github.com/hashicorp/nomad/issues/15844)]
* **SSO via OIDC**: Allow users to authenticate with Nomad via OIDC providers [[GH-15816](https://github.com/hashicorp/nomad/issues/15816)]

BREAKING CHANGES:

* cli: The deprecated gossip keyring commands `nomad operator keyring`, `nomad keyring`, `nomad operator keygen`, and `nomad keygen` have been removed. Use the `nomad operator gossip keyring` commands to manage the gossip keyring [[GH-16068](https://github.com/hashicorp/nomad/issues/16068)]
* config: the `datacenter` field for agent configuration no longer accepts the `*` character as part of the datacenter name [[GH-11170](https://github.com/hashicorp/nomad/issues/11170)]
* core: Ensure no leakage of evaluations for batch jobs. Prior to this change allocations and evaluations for batch jobs were never garbage collected until the batch job was explicitly stopped. The new `batch_eval_gc_threshold` server configuration controls how often they are collected. The default threshold is `24h`. [[GH-15097](https://github.com/hashicorp/nomad/issues/15097)]
* metrics: The metric `nomad.nomad.broker.total_blocked` has been renamed to `nomad.nomad.broker.total_pending` to reduce confusion with the `nomad.blocked_eval.total_blocked` metric. [[GH-15835](https://github.com/hashicorp/nomad/issues/15835)]
* artifact: Environment variables are no longer inherited by default from the Nomad client [[GH-15514](https://github.com/hashicorp/nomad/issues/15514)]
* artifact: File size and count limits are now applied by default to artifact downloads [[GH-16151](https://github.com/hashicorp/nomad/issues/16151)]

SECURITY:

* build: Update to go1.20.1 [[GH-16182](https://github.com/hashicorp/nomad/issues/16182)]

IMPROVEMENTS:

* acl: refactor ACL cache based on golang-lru/v2 [[GH-16085](https://github.com/hashicorp/nomad/issues/16085)]
* agent: Allow configurable range of Job priorities [[GH-16084](https://github.com/hashicorp/nomad/issues/16084)]
* api: improved error returned from AllocFS.Logs when response is not JSON [[GH-15558](https://github.com/hashicorp/nomad/issues/15558)]
* artifact: Provide mitigations against unbounded artifact decompression [[GH-16151](https://github.com/hashicorp/nomad/issues/16151)]
* build: Added hyper-v isolation mode for docker on Windows [[GH-15819](https://github.com/hashicorp/nomad/issues/15819)]
* build: Update to go1.20 [[GH-16029](https://github.com/hashicorp/nomad/issues/16029)]
* cli: Add `-json` and `-t` flag to `nomad acl token create` command [[GH-16055](https://github.com/hashicorp/nomad/issues/16055)]
* cli: Added `-wait` flag to `deployment status` for use with `-monitor` mode [[GH-15262](https://github.com/hashicorp/nomad/issues/15262)]
* cli: Added sprig function support for `-t` templates [[GH-9053](https://github.com/hashicorp/nomad/issues/9053)]
* cli: Added tls command to enable creating Certificate Authority and Self signed TLS certificates.
There are two sub commands `tls ca` and `tls cert` that are helpers when creating certificates. [[GH-14296](https://github.com/hashicorp/nomad/issues/14296)]
* cli: Warn when variable key includes characters that require the use of the `index` function in templates [[GH-15933](https://github.com/hashicorp/nomad/issues/15933)]
* cli: `nomad job stop` can be used to stop multiple jobs concurrently. [[GH-12582](https://github.com/hashicorp/nomad/issues/12582)]
* cli: add a nomad operator client state command [[GH-15469](https://github.com/hashicorp/nomad/issues/15469)]
* cli: multi-line `nomad version` output, add BuildDate [[GH-16216](https://github.com/hashicorp/nomad/issues/16216)]
* cli: we now recommend .nomad.hcl extension for job files, so `job init` creates example.nomad.hcl [[GH-15997](https://github.com/hashicorp/nomad/issues/15997)]
* client/fingerprint/storage: Added config options disk_total_mb and disk_free_mb to override detected disk space [[GH-15852](https://github.com/hashicorp/nomad/issues/15852)]
* client: Add option to enable hairpinMode on Nomad bridge [[GH-15961](https://github.com/hashicorp/nomad/issues/15961)]
* client: Added a TaskEvent when task shutdown is waiting on shutdown_delay [[GH-14775](https://github.com/hashicorp/nomad/issues/14775)]
* client: Log task events at INFO log level [[GH-15842](https://github.com/hashicorp/nomad/issues/15842)]
* client: added http api access for tasks via unix socket [[GH-15864](https://github.com/hashicorp/nomad/issues/15864)]
* client: detect and cleanup leaked iptables rules [[GH-15407](https://github.com/hashicorp/nomad/issues/15407)]
* client: execute artifact downloads in sandbox process [[GH-15328](https://github.com/hashicorp/nomad/issues/15328)]
* consul/connect: Adds support for proxy upstream opaque config [[GH-15761](https://github.com/hashicorp/nomad/issues/15761)]
* consul: add client configuration for grpc_ca_file [[GH-15701](https://github.com/hashicorp/nomad/issues/15701)]
* core: Eliminate deprecated practice of seeding rand package [[GH-16074](https://github.com/hashicorp/nomad/issues/16074)]
* core: Non-client nodes will now skip loading plugins [[GH-16111](https://github.com/hashicorp/nomad/issues/16111)]
* csi: Added server configuration for `csi_volume_claim_gc_interval` [[GH-16195](https://github.com/hashicorp/nomad/issues/16195)]
* deps: Update github.com/containerd/containerd from 1.6.6 to 1.6.12 [[GH-15726](https://github.com/hashicorp/nomad/issues/15726)]
* deps: Update github.com/docker/docker from 20.10.21+incompatible to 20.10.23+incompatible [[GH-15848](https://github.com/hashicorp/nomad/issues/15848)]
* deps: Update github.com/fsouza/go-dockerclient from 1.8.2 to 1.9.0 [[GH-14898](https://github.com/hashicorp/nomad/issues/14898)]
* deps: Update google.golang.org/grpc from 1.48.0 to 1.50.1 [[GH-14897](https://github.com/hashicorp/nomad/issues/14897)]
* deps: Update google.golang.org/grpc to v1.51.0 [[GH-15402](https://github.com/hashicorp/nomad/issues/15402)]
* docs: link to an envoy troubleshooting doc when envoy bootstrap fails [[GH-15908](https://github.com/hashicorp/nomad/issues/15908)]
* env/ec2: update cpu metadata [[GH-15770](https://github.com/hashicorp/nomad/issues/15770)]
* fingerprint: Detect CNI plugins and set versions as node attributes [[GH-15452](https://github.com/hashicorp/nomad/issues/15452)]
* identity: Add identity jobspec block for exposing workload identity to tasks [[GH-15755](https://github.com/hashicorp/nomad/issues/15755)]
* identity: Allow workloads to use RPCs associated with HTTP API [[GH-15870](https://github.com/hashicorp/nomad/issues/15870)]
* jobspec: the `datacenters` field now accepts wildcards [[GH-11170](https://github.com/hashicorp/nomad/issues/11170)]
* metrics: Added metrics for rate of RPC requests [[GH-15876](https://github.com/hashicorp/nomad/issues/15876)]
* scheduler: allow using device IDs in `affinity` and `constraint` [[GH-15455](https://github.com/hashicorp/nomad/issues/15455)]
* server: Added raft snapshot arguments to server config [[GH-15522](https://github.com/hashicorp/nomad/issues/15522)]
* server: Certain raft configuration elements can now be reloaded without restarting the server [[GH-15522](https://github.com/hashicorp/nomad/issues/15522)]
* services: Set Nomad's User-Agent by default on HTTP checks in Nomad services [[GH-16248](https://github.com/hashicorp/nomad/issues/16248)]
* ui, cli: Adds Job Templates to the "Run Job" Web UI and makes them accessible via new flags on nomad job init [[GH-15746](https://github.com/hashicorp/nomad/issues/15746)]
* ui: Add a button for expanding the Task sidebar to full width [[GH-15735](https://github.com/hashicorp/nomad/issues/15735)]
* ui: Added a Policy Editor interface for management tokens [[GH-13976](https://github.com/hashicorp/nomad/issues/13976)]
* ui: Added a ui.label block to agent config, letting operators set a visual label and color for their Nomad instance [[GH-16006](https://github.com/hashicorp/nomad/issues/16006)]
* ui: Made task rows in Allocation tables look more aligned with their parent [[GH-15363](https://github.com/hashicorp/nomad/issues/15363)]
* ui: Show events alongside logs in the Task sidebar [[GH-15733](https://github.com/hashicorp/nomad/issues/15733)]
* ui: The web UI now provides a Token Management interface for management users on policy pages [[GH-15435](https://github.com/hashicorp/nomad/issues/15435)]
* ui: The web UI will now show canary_tags of services anyplace we would normally show tags. [[GH-15458](https://github.com/hashicorp/nomad/issues/15458)]
* ui: Warn when variable key includes characters that require the use of the `index` function in templates [[GH-15933](https://github.com/hashicorp/nomad/issues/15933)]
* ui: give users a notification if their token is going to expire within the next 10 minutes [[GH-15091](https://github.com/hashicorp/nomad/issues/15091)]
* ui: redirect users to Sign In should their tokens ever come back expired or not-found [[GH-15073](https://github.com/hashicorp/nomad/issues/15073)]
* users: Added a cache for OS user lookups [[GH-16100](https://github.com/hashicorp/nomad/issues/16100)]
* variables: Increased maximum size to 64KiB [[GH-15983](https://github.com/hashicorp/nomad/issues/15983)]
* vault: configure Nomad User-Agent on vault clients [[GH-15745](https://github.com/hashicorp/nomad/issues/15745)]
* volumes: Allow `per_alloc` to be used with host_volumes [[GH-15780](https://github.com/hashicorp/nomad/issues/15780)]

DEPRECATIONS:

* api: Deprecated ErrVariableNotFound in favor of ErrVariablePathNotFound to correctly represent an error type [[GH-16237](https://github.com/hashicorp/nomad/issues/16237)]
* api: Deprecated Variables.GetItems in favor of Variables.GetVariableItems to avoid returning a pointer to a map [[GH-16237](https://github.com/hashicorp/nomad/issues/16237)]
* api: The connect `ConsulExposeConfig.Path` field is deprecated in favor of `ConsulExposeConfig.Paths` [[GH-15541](https://github.com/hashicorp/nomad/issues/15541)]
* api: The connect `ConsulProxy.ExposeConfig` field is deprecated in favor of `ConsulProxy.Expose` [[GH-15541](https://github.com/hashicorp/nomad/issues/15541)]

BUG FIXES:

* acl: Fixed a bug in token creation which failed to parse expiration TTLs correctly [[GH-15999](https://github.com/hashicorp/nomad/issues/15999)]
* acl: Fixed a bug where creating/updating a policy which was invalid would return a 404 status code, not a 400 [[GH-16000](https://github.com/hashicorp/nomad/issues/16000)]
* agent: Make agent syslog log level follow log_level config [[GH-15625](https://github.com/hashicorp/nomad/issues/15625)]
* api: Added missing node states to NodeStatus constants [[GH-16166](https://github.com/hashicorp/nomad/issues/16166)]
* api: Fix stale querystring parameter value as boolean [[GH-15605](https://github.com/hashicorp/nomad/issues/15605)]
* api: Fixed a bug where Variables.GetItems would panic if variable did not exist [[GH-16237](https://github.com/hashicorp/nomad/issues/16237)]
* api: Fixed a bug where exposeConfig field was not provided correctly when getting the jobs via the API [[GH-15541](https://github.com/hashicorp/nomad/issues/15541)]
* api: Fixed a nil pointer dereference when periodic jobs are missing their periodic spec [[GH-13845](https://github.com/hashicorp/nomad/issues/13845)]
* cgutil: handle panic coming from runc helper method [[GH-16180](https://github.com/hashicorp/nomad/issues/16180)]
* check: Add support for sending custom host header [[GH-15337](https://github.com/hashicorp/nomad/issues/15337)]
* cli: Fix unbolded header `Device Group Attributes` [[GH-16138](https://github.com/hashicorp/nomad/issues/16138)]
* cli: Fixed a bug where `nomad fmt -check` would overwrite the file being checked [[GH-16174](https://github.com/hashicorp/nomad/issues/16174)]
* cli: Fixed a bug where plans for periodic jobs would return exit code 1 when the job was already register [[GH-14492](https://github.com/hashicorp/nomad/issues/14492)]
* cli: Fixed a panic in `deployment status` when rollback deployments are slow to appear [[GH-16011](https://github.com/hashicorp/nomad/issues/16011)]
* cli: `var put`: when second arg is an @-reference, check extension for format [[GH-16181](https://github.com/hashicorp/nomad/issues/16181)]
* cli: corrected typos in ACL role create/delete CLI commands [[GH-15382](https://github.com/hashicorp/nomad/issues/15382)]
* cli: fix nomad fmt -check flag not returning error code [[GH-15797](https://github.com/hashicorp/nomad/issues/15797)]
* client: Fixed a bug where allocation cleanup hooks would not run [[GH-15477](https://github.com/hashicorp/nomad/issues/15477)]
* connect: ingress http/2/grpc listeners may exclude hosts [[GH-15749](https://github.com/hashicorp/nomad/issues/15749)]
* consul: Fixed a bug where acceptable service identity on Consul token was not accepted [[GH-15928](https://github.com/hashicorp/nomad/issues/15928)]
* consul: Fixed a bug where consul token was not respected when reverting a job [[GH-15996](https://github.com/hashicorp/nomad/issues/15996)]
* consul: Fixed a bug where services would continuously re-register when using ipv6 [[GH-15411](https://github.com/hashicorp/nomad/issues/15411)]
* consul: correctly interpret missing consul checks as unhealthy [[GH-15822](https://github.com/hashicorp/nomad/issues/15822)]
* core: enforce strict ordering that node status updates are recorded after allocation updates for reconnecting clients [[GH-15808](https://github.com/hashicorp/nomad/issues/15808)]
* csi: Fixed a bug where a crashing plugin could panic the Nomad client [[GH-15518](https://github.com/hashicorp/nomad/issues/15518)]
* csi: Fixed a bug where secrets that include '=' were incorrectly rejected [[GH-15670](https://github.com/hashicorp/nomad/issues/15670)]
* csi: Fixed a bug where volumes in non-default namespaces could not be scheduled for system or sysbatch jobs [[GH-15372](https://github.com/hashicorp/nomad/issues/15372)]
* csi: Fixed potential state store corruption when garbage collecting CSI volume claims or checking whether it's safe to force-deregister a volume [[GH-16256](https://github.com/hashicorp/nomad/issues/16256)]
* docker: Fixed a bug where images referenced by multiple tags would not be GC'd [[GH-15962](https://github.com/hashicorp/nomad/issues/15962)]
* docker: Fixed a bug where infra_image did not get alloc_id label [[GH-15898](https://github.com/hashicorp/nomad/issues/15898)]
* docker: configure restart policy for bridge network pause container [[GH-15732](https://github.com/hashicorp/nomad/issues/15732)]
* docker: disable driver when running as non-root on cgv2 hosts [[GH-7794](https://github.com/hashicorp/nomad/issues/7794)]
* eval broker: Fixed a bug where the cancelable eval reaper used an incorrect lock when getting the set of cancelable evals from the broker [[GH-16112](https://github.com/hashicorp/nomad/issues/16112)]
* event stream: Fixed a bug where undefined ACL policies on the request's ACL would result in incorrect authentication errors [[GH-15495](https://github.com/hashicorp/nomad/issues/15495)]
* fix: Add the missing option propagation_mode for volume_mount [[GH-15626](https://github.com/hashicorp/nomad/issues/15626)]
* parser: Fixed a panic in the job spec parser when a variable validation block was missing its condition [[GH-16018](https://github.com/hashicorp/nomad/issues/16018)]
* scheduler (Enterprise): Fixed a bug that prevented new allocations from multiregion jobs to be placed in situations where other regions are not involved, such as node updates. [[GH-15325](https://github.com/hashicorp/nomad/issues/15325)]
* server: Fixed a bug where rejoin_after_leave config was not being respected [[GH-15552](https://github.com/hashicorp/nomad/issues/15552)]
* services: Fixed a bug where check_restart on nomad services on tasks failed with incorrect CheckIDs [[GH-16240](https://github.com/hashicorp/nomad/issues/16240)]
* services: Fixed a bug where services would fail to register if task initially fails [[GH-15862](https://github.com/hashicorp/nomad/issues/15862)]
* template: Fixed a bug that caused the chage script to fail to run [[GH-15915](https://github.com/hashicorp/nomad/issues/15915)]
* template: Fixed a bug where the template runner's Nomad token would be erased by in-place updates to a task [[GH-16266](https://github.com/hashicorp/nomad/issues/16266)]
* ui: Fix allocation memory chart to display the same value as the CLI [[GH-15909](https://github.com/hashicorp/nomad/issues/15909)]
* ui: Fix navigation to pages for jobs that are not in the default namespace [[GH-15906](https://github.com/hashicorp/nomad/issues/15906)]
* ui: Fixed a bug where the exec window would not maintain namespace upon refresh [[GH-15454](https://github.com/hashicorp/nomad/issues/15454)]
* ui: Scale down logger height in the UI when the sidebar container also has task events [[GH-15759](https://github.com/hashicorp/nomad/issues/15759)]
* volumes: Fixed a bug where `per_alloc` was allowed for volume blocks on system and sysbatch jobs, which do not have an allocation index [[GH-16030](https://github.com/hashicorp/nomad/issues/16030)]

## 1.4.14 (October 30, 2023)

SECURITY:

* build: Update to Go 1.21.3 [[GH-18717](https://github.com/hashicorp/nomad/issues/18717)]

BUG FIXES:

* build: Add `timetzdata` Go build tag on Windows binaries to embed time zone data so periodic jobs are able to specify a time zone value on Windows environments [[GH-18676](https://github.com/hashicorp/nomad/issues/18676)]
* cli: Fixed an unexpected behavior of the `nomad acl token update` command that could cause a management token to be downgraded to client on update [[GH-18689](https://github.com/hashicorp/nomad/issues/18689)]
* client: prevent tasks from starting without the prestart hooks running [[GH-18662](https://github.com/hashicorp/nomad/issues/18662)]
* csi: check controller plugin health early during volume register/create [[GH-18570](https://github.com/hashicorp/nomad/issues/18570)]
* metrics: Fixed a bug where CPU counters could report errors for negative values [[GH-18835](https://github.com/hashicorp/nomad/issues/18835)]
* scaling: Unblock blocking queries to /v1/job/{job-id}/scale if the job goes away [[GH-18637](https://github.com/hashicorp/nomad/issues/18637)]
* scheduler (Enterprise): auto-unblock evals with associated quotas when node resources are freed up [[GH-18838](https://github.com/hashicorp/nomad/issues/18838)]
* scheduler: Ensure duplicate allocation indexes are tracked and fixed when performing job updates [[GH-18873](https://github.com/hashicorp/nomad/issues/18873)]
* services: use interpolated address when performing nomad service health checks [[GH-18584](https://github.com/hashicorp/nomad/issues/18584)]

## 1.4.13 (September 13, 2023)

IMPROVEMENTS:

* build: Update to Go 1.21.0 [[GH-18184](https://github.com/hashicorp/nomad/issues/18184)]
* raft: remove use of deprecated Leader func [[GH-18352](https://github.com/hashicorp/nomad/issues/18352)]

BUG FIXES:

* acl: Fixed a bug where ACL tokens linked to ACL roles containing duplicate policies would cause erronous permission denined responses [[GH-18419](https://github.com/hashicorp/nomad/issues/18419)]
* cli: Add missing help message for the `-consul-namespace` flag in the `nomad job run` command [[GH-18081](https://github.com/hashicorp/nomad/issues/18081)]
* cli: Fixed a bug that prevented CSI volumes in namespaces other than `default` from being displayed in the `nomad node status -verbose` output [[GH-17925](https://github.com/hashicorp/nomad/issues/17925)]
* cli: Snapshot name is required in `volume snapshot create` command [[GH-17958](https://github.com/hashicorp/nomad/issues/17958)]
* client: Fixed a bug where the state of poststop tasks could be corrupted by client gc [[GH-17971](https://github.com/hashicorp/nomad/issues/17971)]
* client: Ignore stale server updates to prevent GCing allocations that should be running [[GH-18269](https://github.com/hashicorp/nomad/issues/18269)]
* client: return 404 instead of 500 when trying to access logs and files from allocations that have been garbage collected [[GH-18232](https://github.com/hashicorp/nomad/issues/18232)]
* core: Fixed a bug where exponential backoff could result in excessive CPU usage [[GH-18200](https://github.com/hashicorp/nomad/issues/18200)]
* csi: fixed a bug that could case a panic when deleting volumes [[GH-18234](https://github.com/hashicorp/nomad/issues/18234)]
* fingerprint: fix 'default' alias not being added to interface specified by network_interface [[GH-18096](https://github.com/hashicorp/nomad/issues/18096)]
* jobspec: Add diff for Task Group scaling block [[GH-18332](https://github.com/hashicorp/nomad/issues/18332)]
* migration: Fixed a bug where previous alloc logs were destroyed when migrating ephemeral_disk on the same client [[GH-18108](https://github.com/hashicorp/nomad/issues/18108)]
* scheduler: Fixed a bug where device IDs were not correctly filtered in constraints [[GH-18141](https://github.com/hashicorp/nomad/issues/18141)]
* services: Add validation message when `tls_skip_verify` is set to `true` on a Nomad service [[GH-18333](https://github.com/hashicorp/nomad/issues/18333)]

## 1.4.12 (July 21, 2023)

BUG FIXES:

* csi: Fixed a bug in sending concurrent requests to CSI controller plugins by serializing them per plugin [[GH-17996](https://github.com/hashicorp/nomad/issues/17996)]
* csi: Fixed a bug where CSI controller requests could be sent to unhealthy plugins [[GH-17996](https://github.com/hashicorp/nomad/issues/17996)]
* csi: Fixed a bug where CSI controller requests could not be sent to controllers on nodes ineligible for scheduling [[GH-17996](https://github.com/hashicorp/nomad/issues/17996)]
* services: Fixed a bug that prevented passing query parameters in Nomad native service discovery HTTP health check paths [[GH-17936](https://github.com/hashicorp/nomad/issues/17936)]
* ui: Fixed a bug that prevented nodes from being filtered by the "Ineligible" and "Draining" state filters [[GH-17940](https://github.com/hashicorp/nomad/issues/17940)]
* ui: Fixed error handling for cross-region requests when the receiving region does not implement the endpoint being requested [[GH-18020](https://github.com/hashicorp/nomad/issues/18020)]

## 1.4.11 (July 18, 2023)

SECURITY:

* acl: Fixed a bug where a namespace ACL policy without label was applied to an unexpected namespace. [CVE-2023-3072](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2023-3072) [[GH-17908](https://github.com/hashicorp/nomad/issues/17908)]
* search: Fixed a bug where ACL did not filter plugin and variable names in search endpoint. [CVE-2023-3300](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2023-3300) [[GH-17906](https://github.com/hashicorp/nomad/issues/17906)]
* sentinel (Enterprise): Fixed a bug where ACL tokens could be exfiltrated via Sentinel logs [CVE-2023-3299](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2023-3299) [[GH-17907](https://github.com/hashicorp/nomad/issues/17907)]

IMPROVEMENTS:

* cli: Add `-quiet` flag to `nomad var init` command [[GH-17526](https://github.com/hashicorp/nomad/issues/17526)]
* cni: Ensure to setup CNI addresses in deterministic order [[GH-17766](https://github.com/hashicorp/nomad/issues/17766)]
* deps: Updated Vault SDK to 0.9.0 [[GH-17281](https://github.com/hashicorp/nomad/issues/17281)]
* deps: update docker to 23.0.3 [[GH-16862](https://github.com/hashicorp/nomad/issues/16862)]

BUG FIXES:

* api: Fixed a bug that caused a panic when calling the `Jobs().Plan()` function with a job missing an ID [[GH-17689](https://github.com/hashicorp/nomad/issues/17689)]
* api: add missing constant for unknown allocation status [[GH-17726](https://github.com/hashicorp/nomad/issues/17726)]
* api: add missing field NetworkStatus for Allocation [[GH-17280](https://github.com/hashicorp/nomad/issues/17280)]
* cgroups: Fixed a bug removing all DevicesSets when alloc is created/removed [[GH-17535](https://github.com/hashicorp/nomad/issues/17535)]
* cli: Output error messages during deployment monitoring [[GH-17348](https://github.com/hashicorp/nomad/issues/17348)]
* client: Fixed a bug where Nomad incorrectly wrote to memory swappiness cgroup on old kernels [[GH-17625](https://github.com/hashicorp/nomad/issues/17625)]
* client: fixed a bug that prevented Nomad from fingerprinting Consul 1.13.8 correctly [[GH-17349](https://github.com/hashicorp/nomad/issues/17349)]
* consul: Fixed a bug where Nomad would repeatedly try to revoke successfully revoked SI tokens [[GH-17847](https://github.com/hashicorp/nomad/issues/17847)]
* core: Fix panic around client deregistration and pending heartbeats [[GH-17316](https://github.com/hashicorp/nomad/issues/17316)]
* core: fixed a bug that caused job validation to fail when a task with `kill_timeout` was placed inside a group with `update.progress_deadline` set to 0 [[GH-17342](https://github.com/hashicorp/nomad/issues/17342)]
* csi: Fixed a bug where CSI volumes would fail to restore during client restarts [[GH-17840](https://github.com/hashicorp/nomad/issues/17840)]
* drivers/docker: Fixed a bug where long-running docker operations would incorrectly timeout [[GH-17731](https://github.com/hashicorp/nomad/issues/17731)]
* identity: Fixed a bug where workload identities for periodic and dispatch jobs would not have access to their parent job's ACL policy [[GH-17018](https://github.com/hashicorp/nomad/issues/17018)]
* replication: Fix a potential panic when a non-authoritative region is upgraded and a server with the new version becomes the leader. [[GH-17476](https://github.com/hashicorp/nomad/issues/17476)]
* scheduler: Fixed a bug that could cause replacements for failed allocations to be placed in the wrong datacenter during a canary deployment [[GH-17653](https://github.com/hashicorp/nomad/issues/17653)]
* scheduler: Fixed a panic when a node has only one configured dynamic port [[GH-17619](https://github.com/hashicorp/nomad/issues/17619)]
* ui: dont show a service as healthy when its parent allocation stops running [[GH-17465](https://github.com/hashicorp/nomad/issues/17465)]

## 1.4.10 (May 19, 2023)

IMPROVEMENTS:

* core: Prevent `task.kill_timeout` being greater than `update.progress_deadline` [[GH-16761](https://github.com/hashicorp/nomad/issues/16761)]

BUG FIXES:

* bug: Corrected status description and modification time for canceled evaluations [[GH-17071](https://github.com/hashicorp/nomad/issues/17071)]
* client: Fixed a bug where restarting a terminal allocation turns it into a zombie where allocation and task hooks will run unexpectedly [[GH-17175](https://github.com/hashicorp/nomad/issues/17175)]
* client: clean up resources upon failure to restore task during client restart [[GH-17104](https://github.com/hashicorp/nomad/issues/17104)]
* scale: Fixed a bug where evals could be created with the wrong type [[GH-17092](https://github.com/hashicorp/nomad/issues/17092)]
* scheduler: Fixed a bug where implicit `spread` targets were treated as separate targets for scoring [[GH-17195](https://github.com/hashicorp/nomad/issues/17195)]
* scheduler: Fixed a bug where scores for spread scheduling could be -Inf [[GH-17198](https://github.com/hashicorp/nomad/issues/17198)]

## 1.4.9 (May 02, 2023)

IMPROVEMENTS:

* build: Update from Go 1.20.3 to Go 1.20.4 [[GH-17056](https://github.com/hashicorp/nomad/issues/17056)]
* dependency: update runc to 1.1.5 [[GH-16712](https://github.com/hashicorp/nomad/issues/16712)]

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

## 1.4.8 (April 04, 2023)

SECURITY:

* build: update to Go 1.20.3 to prevent denial of service attack via malicious HTTP headers [CVE-2023-24534](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2023-24534) [[GH-16788](https://github.com/hashicorp/nomad/issues/16788)]

## 1.4.7 (March 21, 2023)

IMPROVEMENTS:

* build: Update to go1.20.2 [[GH-16427](https://github.com/hashicorp/nomad/issues/16427)]

BUG FIXES:

* client: Fixed a bug where clients using Consul discovery to join the cluster would get permission denied errors [[GH-16490](https://github.com/hashicorp/nomad/issues/16490)]
* client: Fixed a bug where cpuset initialization fails after Client restart [[GH-16467](https://github.com/hashicorp/nomad/issues/16467)]
* plugin: Add missing fields to `TaskConfig` so they can be accessed by external task drivers [[GH-16434](https://github.com/hashicorp/nomad/issues/16434)]
* services: Fixed a bug where a service would be deregistered twice [[GH-16289](https://github.com/hashicorp/nomad/issues/16289)]

## 1.4.6 (March 10, 2023)

SECURITY:

* variables: Fixed a bug where a workload-associated policy with a deny capability was ignored for the workload's own variables [CVE-2023-1296](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2023-1296) [[GH-16349](https://github.com/hashicorp/nomad/issues/16349)]

IMPROVEMENTS:

* env/ec2: update cpu metadata [[GH-16417](https://github.com/hashicorp/nomad/issues/16417)]

BUG FIXES:

* client: Fixed a bug that prevented allocations with interpolated values in Consul services from being marked as healthy [[GH-16402](https://github.com/hashicorp/nomad/issues/16402)]
* client: Fixed a bug where clients used the serf advertise address to connect to servers when using Consul auto-discovery [[GH-16217](https://github.com/hashicorp/nomad/issues/16217)]
* docker: Fixed a bug where pause containers would be erroneously removed [[GH-16352](https://github.com/hashicorp/nomad/issues/16352)]
* scheduler: Fixed a bug where collisions in dynamic port offerings would result in spurious plan-for-node-rejected errors [[GH-16401](https://github.com/hashicorp/nomad/issues/16401)]
* server: Fixed a bug where deregistering a job that was already garbage collected would create a new evaluation [[GH-16287](https://github.com/hashicorp/nomad/issues/16287)]
* server: Fixed a bug where node updates that produced errors from service discovery or CSI plugin updates were not logged [[GH-16287](https://github.com/hashicorp/nomad/issues/16287)]
* server: Fixed a bug where the `system reconcile summaries` command and API would not return any scheduler-related errors [[GH-16287](https://github.com/hashicorp/nomad/issues/16287)]

## 1.4.5 (March 01, 2023)

BREAKING CHANGES:

* core: Ensure no leakage of evaluations for batch jobs. Prior to this change allocations and evaluations for batch jobs were never garbage collected until the batch job was explicitly stopped. The new `batch_eval_gc_threshold` server configuration controls how often they are collected. The default threshold is `24h`. [[GH-15097](https://github.com/hashicorp/nomad/issues/15097)]

IMPROVEMENTS:

* api: improved error returned from AllocFS.Logs when response is not JSON [[GH-15558](https://github.com/hashicorp/nomad/issues/15558)]
* cli: Added `-wait` flag to `deployment status` for use with `-monitor` mode [[GH-15262](https://github.com/hashicorp/nomad/issues/15262)]
* cli: Added tls command to enable creating Certificate Authority and Self signed TLS certificates.
  There are two sub commands `tls ca` and `tls cert` that are helpers when creating certificates. [[GH-14296](https://github.com/hashicorp/nomad/issues/14296)]
* client: detect and cleanup leaked iptables rules [[GH-15407](https://github.com/hashicorp/nomad/issues/15407)]
* consul: add client configuration for grpc_ca_file [[GH-15701](https://github.com/hashicorp/nomad/issues/15701)]
* deps: Update google.golang.org/grpc to v1.51.0 [[GH-15402](https://github.com/hashicorp/nomad/issues/15402)]
* docs: link to an envoy troubleshooting doc when envoy bootstrap fails [[GH-15908](https://github.com/hashicorp/nomad/issues/15908)]
* env/ec2: update cpu metadata [[GH-15770](https://github.com/hashicorp/nomad/issues/15770)]
* fingerprint: Detect CNI plugins and set versions as node attributes [[GH-15452](https://github.com/hashicorp/nomad/issues/15452)]
* scheduler: allow using device IDs in `affinity` and `constraint` [[GH-15455](https://github.com/hashicorp/nomad/issues/15455)]
* ui: Add a button for expanding the Task sidebar to full width [[GH-15735](https://github.com/hashicorp/nomad/issues/15735)]
* ui: Made task rows in Allocation tables look more aligned with their parent [[GH-15363](https://github.com/hashicorp/nomad/issues/15363)]
* ui: Show events alongside logs in the Task sidebar [[GH-15733](https://github.com/hashicorp/nomad/issues/15733)]
* ui: The web UI will now show canary_tags of services anyplace we would normally show tags. [[GH-15458](https://github.com/hashicorp/nomad/issues/15458)]

DEPRECATIONS:

* api: The connect `ConsulExposeConfig.Path` field is deprecated in favor of `ConsulExposeConfig.Paths` [[GH-15541](https://github.com/hashicorp/nomad/issues/15541)]
* api: The connect `ConsulProxy.ExposeConfig` field is deprecated in favor of `ConsulProxy.Expose` [[GH-15541](https://github.com/hashicorp/nomad/issues/15541)]

BUG FIXES:

* acl: Fixed a bug in token creation which failed to parse expiration TTLs correctly [[GH-15999](https://github.com/hashicorp/nomad/issues/15999)]
* acl: Fixed a bug where creating/updating a policy which was invalid would return a 404 status code, not a 400 [[GH-16000](https://github.com/hashicorp/nomad/issues/16000)]
* agent: Make agent syslog log level follow log_level config [[GH-15625](https://github.com/hashicorp/nomad/issues/15625)]
* api: Added missing node states to NodeStatus constants [[GH-16166](https://github.com/hashicorp/nomad/issues/16166)]
* api: Fix stale querystring parameter value as boolean [[GH-15605](https://github.com/hashicorp/nomad/issues/15605)]
* api: Fixed a bug where exposeConfig field was not provided correctly when getting the jobs via the API [[GH-15541](https://github.com/hashicorp/nomad/issues/15541)]
* api: Fixed a nil pointer dereference when periodic jobs are missing their periodic spec [[GH-13845](https://github.com/hashicorp/nomad/issues/13845)]
* cgutil: handle panic coming from runc helper method [[GH-16180](https://github.com/hashicorp/nomad/issues/16180)]
* check: Add support for sending custom host header [[GH-15337](https://github.com/hashicorp/nomad/issues/15337)]
* cli: Fixed a bug where `nomad fmt -check` would overwrite the file being checked [[GH-16174](https://github.com/hashicorp/nomad/issues/16174)]
* cli: Fixed a panic in `deployment status` when rollback deployments are slow to appear [[GH-16011](https://github.com/hashicorp/nomad/issues/16011)]
* cli: corrected typos in ACL role create/delete CLI commands [[GH-15382](https://github.com/hashicorp/nomad/issues/15382)]
* cli: fix nomad fmt -check flag not returning error code [[GH-15797](https://github.com/hashicorp/nomad/issues/15797)]
* client: Fixed a bug where allocation cleanup hooks would not run [[GH-15477](https://github.com/hashicorp/nomad/issues/15477)]
* connect: ingress http/2/grpc listeners may exclude hosts [[GH-15749](https://github.com/hashicorp/nomad/issues/15749)]
* consul: Fixed a bug where acceptable service identity on Consul token was not accepted [[GH-15928](https://github.com/hashicorp/nomad/issues/15928)]
* consul: Fixed a bug where consul token was not respected when reverting a job [[GH-15996](https://github.com/hashicorp/nomad/issues/15996)]
* consul: Fixed a bug where services would continuously re-register when using ipv6 [[GH-15411](https://github.com/hashicorp/nomad/issues/15411)]
* consul: correctly interpret missing consul checks as unhealthy [[GH-15822](https://github.com/hashicorp/nomad/issues/15822)]
* core: enforce strict ordering that node status updates are recorded after allocation updates for reconnecting clients [[GH-15808](https://github.com/hashicorp/nomad/issues/15808)]
* csi: Fixed a bug where a crashing plugin could panic the Nomad client [[GH-15518](https://github.com/hashicorp/nomad/issues/15518)]
* csi: Fixed a bug where secrets that include '=' were incorrectly rejected [[GH-15670](https://github.com/hashicorp/nomad/issues/15670)]
* csi: Fixed a bug where volumes in non-default namespaces could not be scheduled for system or sysbatch jobs [[GH-15372](https://github.com/hashicorp/nomad/issues/15372)]
* csi: Fixed potential state store corruption when garbage collecting CSI volume claims or checking whether it's safe to force-deregister a volume [[GH-16256](https://github.com/hashicorp/nomad/issues/16256)]
* docker: Fixed a bug where images referenced by multiple tags would not be GC'd [[GH-15962](https://github.com/hashicorp/nomad/issues/15962)]
* docker: Fixed a bug where infra_image did not get alloc_id label [[GH-15898](https://github.com/hashicorp/nomad/issues/15898)]
* docker: configure restart policy for bridge network pause container [[GH-15732](https://github.com/hashicorp/nomad/issues/15732)]
* eval broker: Fixed a bug where the cancelable eval reaper used an incorrect lock when getting the set of cancelable evals from the broker [[GH-16112](https://github.com/hashicorp/nomad/issues/16112)]
* event stream: Fixed a bug where undefined ACL policies on the request's ACL would result in incorrect authentication errors [[GH-15495](https://github.com/hashicorp/nomad/issues/15495)]
* fix: Add the missing option propagation_mode for volume_mount [[GH-15626](https://github.com/hashicorp/nomad/issues/15626)]
* parser: Fixed a panic in the job spec parser when a variable validation block was missing its condition [[GH-16018](https://github.com/hashicorp/nomad/issues/16018)]
* scheduler (Enterprise): Fixed a bug that prevented new allocations from multiregion jobs to be placed in situations where other regions are not involved, such as node updates. [[GH-15325](https://github.com/hashicorp/nomad/issues/15325)]
* services: Fixed a bug where check_restart on nomad services on tasks failed with incorrect CheckIDs [[GH-16240](https://github.com/hashicorp/nomad/issues/16240)]
* template: Fixed a bug that caused the chage script to fail to run [[GH-15915](https://github.com/hashicorp/nomad/issues/15915)]
* template: Fixed a bug where the template runner's Nomad token would be erased by in-place updates to a task [[GH-16266](https://github.com/hashicorp/nomad/issues/16266)]
* ui: Fix allocation memory chart to display the same value as the CLI [[GH-15909](https://github.com/hashicorp/nomad/issues/15909)]
* ui: Fix navigation to pages for jobs that are not in the default namespace [[GH-15906](https://github.com/hashicorp/nomad/issues/15906)]
* ui: Fixed a bug where the exec window would not maintain namespace upon refresh [[GH-15454](https://github.com/hashicorp/nomad/issues/15454)]
* ui: Scale down logger height in the UI when the sidebar container also has task events [[GH-15759](https://github.com/hashicorp/nomad/issues/15759)]
* volumes: Fixed a bug where `per_alloc` was allowed for volume blocks on system and sysbatch jobs, which do not have an allocation index [[GH-16030](https://github.com/hashicorp/nomad/issues/16030)]

## 1.4.4 (February 14, 2023)

SECURITY:

* artifact: Provide mitigations against unbounded artifact decompression [[GH-16126](https://github.com/hashicorp/nomad/issues/16126)]
* build: Update to go1.20.1 [[GH-16182](https://github.com/hashicorp/nomad/issues/16182)]

## 1.4.3 (November 21, 2022)

IMPROVEMENTS:

* api: Added an API for counting evaluations that match a filter [[GH-15147](https://github.com/hashicorp/nomad/issues/15147)]
* cli: Improved performance of eval delete with large filter sets [[GH-15117](https://github.com/hashicorp/nomad/issues/15117)]
* consul: add trace logging around service registrations [[GH-6115](https://github.com/hashicorp/nomad/issues/6115)]
* deps: Updated github.com/aws/aws-sdk-go from 1.44.84 to 1.44.126 [[GH-15081](https://github.com/hashicorp/nomad/issues/15081)]
* deps: Updated github.com/docker/cli from 20.10.18+incompatible to 20.10.21+incompatible [[GH-15078](https://github.com/hashicorp/nomad/issues/15078)]
* exec: Allow running commands from mounted host volumes [[GH-14851](https://github.com/hashicorp/nomad/issues/14851)]
* scheduler: when multiple evaluations are pending for the same job, evaluate the latest and cancel the intermediaries on success [[GH-14621](https://github.com/hashicorp/nomad/issues/14621)]
* server: Add a git `revision` tag to the serf tags gossiped between servers. [[GH-9159](https://github.com/hashicorp/nomad/issues/9159)]
* template: Expose per-template configuration for `error_on_missing_key`. This allows jobspec authors to specify that a
template should fail if it references a struct or map key that does not exist. The default value is false and should be
fully backward compatible. [[GH-14002](https://github.com/hashicorp/nomad/issues/14002)]
* ui: Adds a "Pack" tag and logo on the jobs list index when appropriate [[GH-14833](https://github.com/hashicorp/nomad/issues/14833)]
* ui: add consul connect service upstream and on-update info to the service sidebar [[GH-15324](https://github.com/hashicorp/nomad/issues/15324)]
* ui: allow users to upload files by click or drag in the web ui [[GH-14747](https://github.com/hashicorp/nomad/issues/14747)]

BUG FIXES:

* api: Ensure all request body decode errors return a 400 status code [[GH-15252](https://github.com/hashicorp/nomad/issues/15252)]
* autopilot: Fixed a bug where autopilot would try to fetch raft stats from other regions [[GH-15290](https://github.com/hashicorp/nomad/issues/15290)]
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
* keyring: Fixed a bug where a missing key would prevent any further replication. [[GH-15092](https://github.com/hashicorp/nomad/issues/15092)]
* keyring: Fixed a bug where replication would stop after snapshot restores [[GH-15227](https://github.com/hashicorp/nomad/issues/15227)]
* keyring: Re-enabled keyring garbage collection after fixing a bug where keys would be garbage collected even if they were used to sign a live allocation's workload identity. [[GH-15092](https://github.com/hashicorp/nomad/issues/15092)]
* scheduler: Fixed a bug that prevented disconnected allocations to be updated after they reconnect. [[GH-15068](https://github.com/hashicorp/nomad/issues/15068)]
* scheduler: Prevent unnecessary placements when disconnected allocations reconnect. [[GH-15068](https://github.com/hashicorp/nomad/issues/15068)]
* template: Fixed a bug where template could cause agent panic on startup [[GH-15192](https://github.com/hashicorp/nomad/issues/15192)]
* ui: Fixed a bug where the task log sidebar would close and re-open if the parent job state changed [[GH-15146](https://github.com/hashicorp/nomad/issues/15146)]
* variables: Fixed a bug where a long-running rekey could hit the nack timeout [[GH-15102](https://github.com/hashicorp/nomad/issues/15102)]
* wi: Fixed a bug where clients running pre-1.4.0 allocations would erase the token used to query service registrations after upgrade [[GH-15121](https://github.com/hashicorp/nomad/issues/15121)]

## 1.4.2 (October 26, 2022)

SECURITY:

* event stream: Fixed a bug where ACL token expiration was not checked when emitting events [[GH-15013](https://github.com/hashicorp/nomad/issues/15013)]
* variables: Fixed a bug where non-sensitive variable metadata (paths and raft indexes) was exposed via the template `nomadVarList` function to other jobs in the same namespace. [[GH-15012](https://github.com/hashicorp/nomad/issues/15012)]

IMPROVEMENTS:

* cli: Added `-id-prefix-template` option to `nomad job dispatch` [[GH-14631](https://github.com/hashicorp/nomad/issues/14631)]
* cli: add nomad fmt to the CLI [[GH-14779](https://github.com/hashicorp/nomad/issues/14779)]
* deps: update go-memdb for goroutine leak fix [[GH-14983](https://github.com/hashicorp/nomad/issues/14983)]
* docker: improve memory usage for docker_logger [[GH-14875](https://github.com/hashicorp/nomad/issues/14875)]
* event stream: Added ACL role topic with create and delete types [[GH-14923](https://github.com/hashicorp/nomad/issues/14923)]
* scheduler: Allow jobs not requiring network resources even when no network is fingerprinted [[GH-14300](https://github.com/hashicorp/nomad/issues/14300)]
* ui: adds searching and filtering to the topology page [[GH-14913](https://github.com/hashicorp/nomad/issues/14913)]

BUG FIXES:

* acl: Callers should be able to read policies linked via roles to the token used [[GH-14982](https://github.com/hashicorp/nomad/issues/14982)]
* acl: Ensure all federated servers meet v.1.4.0 minimum before ACL roles can be written [[GH-14908](https://github.com/hashicorp/nomad/issues/14908)]
* acl: Fixed a bug where Nomad version checking for one-time tokens was enforced across regions [[GH-14912](https://github.com/hashicorp/nomad/issues/14912)]
* cli: prevent a panic when the Nomad API returns an error while collecting a debug bundle [[GH-14992](https://github.com/hashicorp/nomad/issues/14992)]
* client: Check ACL token expiry when resolving token within ACL cache [[GH-14922](https://github.com/hashicorp/nomad/issues/14922)]
* client: Fixed a bug where Nomad could not detect cores on recent RHEL systems [[GH-15027](https://github.com/hashicorp/nomad/issues/15027)]
* client: Fixed a bug where network fingerprinters were not reloaded when the client configuration was reloaded with SIGHUP [[GH-14615](https://github.com/hashicorp/nomad/issues/14615)]
* client: Resolve ACL roles within client ACL cache [[GH-14922](https://github.com/hashicorp/nomad/issues/14922)]
* consul: Fixed a bug where services continuously re-registered [[GH-14917](https://github.com/hashicorp/nomad/issues/14917)]
* consul: atomically register checks on initial service registration [[GH-14944](https://github.com/hashicorp/nomad/issues/14944)]
* deps: Update hashicorp/consul-template to 90370e07bf621811826b803fb633dadbfb4cf287; fixes template rerendering issues when only user or group set [[GH-15045](https://github.com/hashicorp/nomad/issues/15045)]
* deps: Update hashicorp/raft to v1.3.11; fixes unstable leadership on server removal [[GH-15021](https://github.com/hashicorp/nomad/issues/15021)]
* event stream: Check ACL token expiry when resolving tokens [[GH-14923](https://github.com/hashicorp/nomad/issues/14923)]
* event stream: Resolve ACL roles within ACL tokens [[GH-14923](https://github.com/hashicorp/nomad/issues/14923)]
* keyring: Fixed a bug where `nomad system gc` forced a root keyring rotation. [[GH-15009](https://github.com/hashicorp/nomad/issues/15009)]
* keyring: Fixed a bug where if a key is rotated immediately following a leader election, plans that are in-flight may get signed before the new leader has the key. Allow for a short timeout-and-retry to avoid rejecting plans. [[GH-14987](https://github.com/hashicorp/nomad/issues/14987)]
* keyring: Fixed a bug where keyring initialization is blocked by un-upgraded federated regions [[GH-14901](https://github.com/hashicorp/nomad/issues/14901)]
* keyring: Fixed a bug where root keyring garbage collection configuration values were not respected. [[GH-15009](https://github.com/hashicorp/nomad/issues/15009)]
* keyring: Fixed a bug where root keyring initialization could occur before the raft FSM on the leader was verified to be up-to-date. [[GH-14987](https://github.com/hashicorp/nomad/issues/14987)]
* keyring: Fixed a bug where root keyring replication could make incorrectly stale queries and exit early if those queries did not return the expected key. [[GH-14987](https://github.com/hashicorp/nomad/issues/14987)]
* keyring: Fixed a bug where the root keyring replicator's rate limiting would be skipped if the keyring replication exceeded the burst rate. [[GH-14987](https://github.com/hashicorp/nomad/issues/14987)]
* keyring: Removed root key garbage collection to avoid orphaned workload identities [[GH-15034](https://github.com/hashicorp/nomad/issues/15034)]
* nomad native service discovery: Ensure all local servers meet v.1.3.0 minimum before service registrations can be written [[GH-14924](https://github.com/hashicorp/nomad/issues/14924)]
* scheduler: Fixed a bug where version checking for disconnected clients handling was enforced across regions [[GH-14912](https://github.com/hashicorp/nomad/issues/14912)]
* servicedisco: Fixed a bug where job using checks could land on incompatible client [[GH-14868](https://github.com/hashicorp/nomad/issues/14868)]
* services: Fixed a regression where check task validation stopped allowing some configurations [[GH-14864](https://github.com/hashicorp/nomad/issues/14864)]
* ui: Fixed line charts to update x-axis (time) where relevant [[GH-14814](https://github.com/hashicorp/nomad/issues/14814)]
* ui: Fixes an issue where service tags would bleed past the edge of the screen [[GH-14832](https://github.com/hashicorp/nomad/issues/14832)]
* variables: Fixed a bug where Nomad version checking was not enforced for writing to variables [[GH-14912](https://github.com/hashicorp/nomad/issues/14912)]
* variables: Fixed a bug where getting empty results from listing variables resulted in a permission denied error. [[GH-15012](https://github.com/hashicorp/nomad/issues/15012)]

## 1.4.1 (October 06, 2022)

BUG FIXES:

* keyring: Fixed a panic that can occur during upgrades to 1.4.0 when initializing the keyring [[GH-14821](https://github.com/hashicorp/nomad/issues/14821)]

## 1.4.0 (October 04, 2022)

FEATURES:

* **ACL Roles:** Added support for ACL Roles. [[GH-14320](https://github.com/hashicorp/nomad/issues/14320)]
* **Nomad Native Service Discovery**: Add built-in support for checks on Nomad services [[GH-13715](https://github.com/hashicorp/nomad/issues/13715)]
* **Variables:** Added support for storing encrypted configuration values. [[GH-13000](https://github.com/hashicorp/nomad/issues/13000)]
* **UI Services table:** Display task-level services in addition to group-level services. [[GH-14199](https://github.com/hashicorp/nomad/issues/14199)]

BREAKING CHANGES:

* audit (Enterprise): fixed inconsistency in event filter logic [[GH-14212](https://github.com/hashicorp/nomad/issues/14212)]
* cli: `eval status -json` no longer supports listing all evals in JSON. Use `eval list -json`. [[GH-14651](https://github.com/hashicorp/nomad/issues/14651)]
* core: remove support for raft protocol version 2 [[GH-13467](https://github.com/hashicorp/nomad/issues/13467)]

SECURITY:

* client: recover from panics caused by artifact download to prevent the Nomad client from crashing [[GH-14696](https://github.com/hashicorp/nomad/issues/14696)]

IMPROVEMENTS:

* acl: ACL tokens can now be created with an expiration TTL. [[GH-14320](https://github.com/hashicorp/nomad/issues/14320)]
* api: return a more descriptive error when /v1/acl/bootstrap fails to decode request body [[GH-14629](https://github.com/hashicorp/nomad/issues/14629)]
* autopilot: upgrade to raft-autopilot library [[GH-14441](https://github.com/hashicorp/nomad/issues/14441)]
* cli: Removed deprecated network quota fields from `quota status` output [[GH-14468](https://github.com/hashicorp/nomad/issues/14468)]
* cli: `acl policy info` output format has changed to improve readability with large policy documents [[GH-14140](https://github.com/hashicorp/nomad/issues/14140)]
* cli: `operator debug` now writes newline-delimited JSON files for large collections [[GH-14610](https://github.com/hashicorp/nomad/issues/14610)]
* cli: ignore `-hcl2-strict` when -hcl1 is set. [[GH-14426](https://github.com/hashicorp/nomad/issues/14426)]
* cli: warn destructive update only when count is greater than 1 [[GH-13103](https://github.com/hashicorp/nomad/issues/13103)]
* client: Add built-in support for checks on nomad services [[GH-13715](https://github.com/hashicorp/nomad/issues/13715)]
* client: re-enable nss-based user lookups [[GH-14742](https://github.com/hashicorp/nomad/issues/14742)]
* connect: add namespace, job, and group to Envoy stats [[GH-14311](https://github.com/hashicorp/nomad/issues/14311)]
* connect: add nomad environment variables to envoy bootstrap [[GH-12959](https://github.com/hashicorp/nomad/issues/12959)]
* consul: Allow interpolation of task environment values into Consul Service Mesh configuration [[GH-14445](https://github.com/hashicorp/nomad/issues/14445)]
* consul: Enable setting custom tagged_addresses field [[GH-12951](https://github.com/hashicorp/nomad/issues/12951)]
* core: constraint operands are now compared numerically if operands are numbers [[GH-14722](https://github.com/hashicorp/nomad/issues/14722)]
* deps: Update fsouza/go-dockerclient to v1.8.2 [[GH-14112](https://github.com/hashicorp/nomad/issues/14112)]
* deps: Update go.etcd.io/bbolt to v1.3.6 [[GH-14025](https://github.com/hashicorp/nomad/issues/14025)]
* deps: Update google.golang.org/grpc to v1.48.0 [[GH-14103](https://github.com/hashicorp/nomad/issues/14103)]
* deps: Update gopsutil for improvements in fingerprinting on non-Linux platforms [[GH-14209](https://github.com/hashicorp/nomad/issues/14209)]
* deps: Updated `github.com/armon/go-metrics` to `v0.4.1` which includes a performance improvement for Prometheus sink [[GH-14493](https://github.com/hashicorp/nomad/issues/14493)]
* deps: Updated `github.com/hashicorp/go-version` to `v1.6.0` [[GH-14364](https://github.com/hashicorp/nomad/issues/14364)]
* deps: remove unused darwin C library [[GH-13894](https://github.com/hashicorp/nomad/issues/13894)]
* fingerprint: Add node attribute for number of reservable cores: `cpu.num_reservable_cores` [[GH-14694](https://github.com/hashicorp/nomad/issues/14694)]
* fingerprint: Consul and Vault attributes are no longer cleared on fingerprinting failure [[GH-14673](https://github.com/hashicorp/nomad/issues/14673)]
* jobspec: Added `strlen` HCL2 function to determine the length of a string [[GH-14463](https://github.com/hashicorp/nomad/issues/14463)]
* server: Log when a node's eligibility changes [[GH-14125](https://github.com/hashicorp/nomad/issues/14125)]
* ui: Display different message when trying to exec into a job with no task running. [[GH-14071](https://github.com/hashicorp/nomad/issues/14071)]
* ui: add service discovery, along with health checks, to job and allocation routes [[GH-14408](https://github.com/hashicorp/nomad/issues/14408)]
* ui: adds a sidebar to show in-page logs for a given task, accessible via job, client, or task group routes [[GH-14612](https://github.com/hashicorp/nomad/issues/14612)]
* ui: allow deep-dive clicks to tasks from client, job, and task group routes. [[GH-14592](https://github.com/hashicorp/nomad/issues/14592)]
* ui: attach timestamps and a visual indicator on failure to health checks in the Web UI [[GH-14677](https://github.com/hashicorp/nomad/issues/14677)]

BUG FIXES:

* api: Fixed a bug where the List Volume API did not include the `ControllerRequired` and `ResourceExhausted` fields. [[GH-14484](https://github.com/hashicorp/nomad/issues/14484)]
* cli: Ignore Vault token when generating job diff. [[GH-14424](https://github.com/hashicorp/nomad/issues/14424)]
* cli: fixed a bug in the `operator api` command where the HTTPS scheme was not always correctly calculated [[GH-14635](https://github.com/hashicorp/nomad/issues/14635)]
* cli: return exit code `255` when `nomad job plan` fails job validation. [[GH-14426](https://github.com/hashicorp/nomad/issues/14426)]
* cli: set content length on POST requests when using the `nomad operator api` command [[GH-14634](https://github.com/hashicorp/nomad/issues/14634)]
* client: Fixed bug where clients could attempt to connect to servers with invalid addresses retrieved from Consul. [[GH-14431](https://github.com/hashicorp/nomad/issues/14431)]
* core: prevent new allocations from overlapping execution with stopping allocations [[GH-10446](https://github.com/hashicorp/nomad/issues/10446)]
* csi: Fixed a bug where a volume that was successfully unmounted by the client but then failed controller unpublishing would not be marked free until garbage collection ran. [[GH-14675](https://github.com/hashicorp/nomad/issues/14675)]
* csi: Fixed a bug where the server would not send controller unpublish for a failed allocation. [[GH-14484](https://github.com/hashicorp/nomad/issues/14484)]
* csi: Fixed a data race in the volume unpublish endpoint that could result in claims being incorrectly marked as freed before being persisted to raft. [[GH-14484](https://github.com/hashicorp/nomad/issues/14484)]
* helpers: Fixed a bug where random stagger func did not protect against negative inputs [[GH-14497](https://github.com/hashicorp/nomad/issues/14497)]
* jobspec: Fixed a bug where an `artifact` with `headers` configuration would fail to parse when using HCLv1 [[GH-14637](https://github.com/hashicorp/nomad/issues/14637)]
* metrics: Update client `node_scheduling_eligibility` value with server heartbeats. [[GH-14483](https://github.com/hashicorp/nomad/issues/14483)]
* quotas (Enterprise): Fixed a server crashing panic when updating and checking a quota concurrently.
* rpc (Enterprise): check for spec changes in all regions when registering multiregion jobs [[GH-14519](https://github.com/hashicorp/nomad/issues/14519)]
* scheduler (Enterprise): Fixed bug where the scheduler would treat multiregion jobs as paused for job types that don't use deployments [[GH-14659](https://github.com/hashicorp/nomad/issues/14659)]
* template: Fixed a bug where the `splay` timeout was not being applied when `change_mode` was set to `script`. [[GH-14749](https://github.com/hashicorp/nomad/issues/14749)]
* ui: Remove extra space when displaying the version in the menu footer. [[GH-14457](https://github.com/hashicorp/nomad/issues/14457)]

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

* build: Update go toolchain to 1.18.5 [[GH-13956](https://github.com/hashicorp/nomad/pull/13956)]
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
* raft: The default raft protocol version is now 3 so you must follow the [Upgrading to Raft Protocol 3](https://developer.hashicorp.com/nomad/docs/upgrade#upgrading-to-raft-protocol-3) guide when upgrading an existing cluster to Nomad 1.3.0. Downgrading the raft protocol version is not supported. [[GH-11572](https://github.com/hashicorp/nomad/issues/11572)]

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

## 1.2.16 (February 14, 2023)

SECURITY:

* artifact: Provide mitigations against unbounded artifact decompression [[GH-16126](https://github.com/hashicorp/nomad/issues/16126)]
* build: Update to go1.20.1 [[GH-16182](https://github.com/hashicorp/nomad/issues/16182)]

## 1.2.15 (November 21, 2022)

BUG FIXES:

* api: Ensure all request body decode errors return a 400 status code [[GH-15252](https://github.com/hashicorp/nomad/issues/15252)]
* cleanup: fixed missing timer.Reset for plan queue stat emitter [[GH-15134](https://github.com/hashicorp/nomad/issues/15134)]
* client: Fixed a bug where tasks would restart without waiting for interval [[GH-15215](https://github.com/hashicorp/nomad/issues/15215)]
* client: fixed a bug where non-`docker` tasks with network isolation would leak network namespaces and iptables rules if the client was restarted while they were running [[GH-15214](https://github.com/hashicorp/nomad/issues/15214)]
* csi: Fixed race condition that can cause a panic when volume is garbage collected [[GH-15101](https://github.com/hashicorp/nomad/issues/15101)]
* device: Fixed a bug where device plugins would not fingerprint on startup [[GH-15125](https://github.com/hashicorp/nomad/issues/15125)]
* drivers: Fixed a bug where one goroutine was leaked per task [[GH-15180](https://github.com/hashicorp/nomad/issues/15180)]
* drivers: pass missing `propagation_mode` configuration for volume mounts to external plugins [[GH-15096](https://github.com/hashicorp/nomad/issues/15096)]
* event_stream: fixed a bug where dynamic port values would fail to serialize in the event stream [[GH-12916](https://github.com/hashicorp/nomad/issues/12916)]
* fingerprint: Ensure Nomad can correctly fingerprint Consul gRPC where the Consul agent is running v1.14.0 or greater [[GH-15309](https://github.com/hashicorp/nomad/issues/15309)]

## 1.2.14 (October 26, 2022)

IMPROVEMENTS:

* deps: update go-memdb for goroutine leak fix [[GH-14983](https://github.com/hashicorp/nomad/issues/14983)]

BUG FIXES:

* acl: Fixed a bug where Nomad version checking for one-time tokens was enforced across regions [[GH-14910](https://github.com/hashicorp/nomad/issues/14910)]
* deps: Update hashicorp/raft to v1.3.11; fixes unstable leadership on server removal [[GH-15021](https://github.com/hashicorp/nomad/issues/15021)]

## 1.2.13 (October 04, 2022)

SECURITY:

* client: recover from panics caused by artifact download to prevent the Nomad client from crashing [[GH-14696](https://github.com/hashicorp/nomad/issues/14696)]

BUG FIXES:

* api: Fixed a bug where the List Volume API did not include the `ControllerRequired` and `ResourceExhausted` fields. [[GH-14484](https://github.com/hashicorp/nomad/issues/14484)]
* client: Fixed bug where clients could attempt to connect to servers with invalid addresses retrieved from Consul. [[GH-14431](https://github.com/hashicorp/nomad/issues/14431)]
* csi: Fixed a bug where a volume that was successfully unmounted by the client but then failed controller unpublishing would not be marked free until garbage collection ran. [[GH-14675](https://github.com/hashicorp/nomad/issues/14675)]
* csi: Fixed a bug where the server would not send controller unpublish for a failed allocation. [[GH-14484](https://github.com/hashicorp/nomad/issues/14484)]
* csi: Fixed a bug where volume claims on lost or garbage collected nodes could not be freed [[GH-14720](https://github.com/hashicorp/nomad/issues/14720)]
* csi: Fixed a data race in the volume unpublish endpoint that could result in claims being incorrectly marked as freed before being persisted to raft. [[GH-14484](https://github.com/hashicorp/nomad/issues/14484)]
* jobspec: Fixed a bug where an `artifact` with `headers` configuration would fail to parse when using HCLv1 [[GH-14637](https://github.com/hashicorp/nomad/issues/14637)]
* metrics: Update client `node_scheduling_eligibility` value with server heartbeats. [[GH-14483](https://github.com/hashicorp/nomad/issues/14483)]
* quotas (Enterprise): Fixed a server crashing panic when updating and checking a quota concurrently.
* rpc: check for spec changes in all regions when registering multiregion jobs [[GH-14519](https://github.com/hashicorp/nomad/issues/14519)]

## 1.2.12 (August 31, 2022)

IMPROVEMENTS:

* consul: Reduce load on Consul leader server by allowing stale results when listing namespaces. [[GH-12953](https://github.com/hashicorp/nomad/issues/12953)]

BUG FIXES:

* cli: Fixed a bug where forcing a periodic job would fail if the job ID prefix-matched other periodic jobs [[GH-14333](https://github.com/hashicorp/nomad/issues/14333)]

## 1.2.11 (August 25, 2022)

IMPROVEMENTS:

* build: update to go1.19 [[GH-14132](https://github.com/hashicorp/nomad/issues/14132)]

BUG FIXES:

* api: cleanup whitespace from failed api response body [[GH-14145](https://github.com/hashicorp/nomad/issues/14145)]
* client/logmon: fixed a bug where logmon cannot find nomad executable [[GH-14297](https://github.com/hashicorp/nomad/issues/14297)]
* client: Fixed a bug where user lookups would hang or panic [[GH-14248](https://github.com/hashicorp/nomad/issues/14248)]
* ui: Fixed a bug that caused the allocation details page to display the stats bar chart even if the task was pending. [[GH-14224](https://github.com/hashicorp/nomad/issues/14224)]
* vault: Fixed a bug where Vault clients were recreated when the server configuration was reloaded, even if there were no changes to the Vault configuration. [[GH-14298](https://github.com/hashicorp/nomad/issues/14298)]
* vault: Fixed a bug where changing the Vault configuration `namespace` field was not detected as a change during server configuration reload. [[GH-14298](https://github.com/hashicorp/nomad/issues/14298)]

## 1.2.10 (August 05, 2022)

BUG FIXES:

* acl: Fixed a bug where the timestamp for expiring one-time tokens was not deterministic between servers [[GH-13737](https://github.com/hashicorp/nomad/issues/13737)]
* build: Update go toolchain to 1.18.5 [[GH-13956](https://github.com/hashicorp/nomad/pull/13956)]
* deployments: Fixed a bug that prevented auto-approval if canaries were marked as unhealthy during deployment [[GH-14001](https://github.com/hashicorp/nomad/issues/14001)]
* metrics: Fixed a bug where blocked evals with no class produced no dc:class scope metrics [[GH-13786](https://github.com/hashicorp/nomad/issues/13786)]
* namespaces: Fixed a bug that allowed deleting a namespace that contained a CSI volume [[GH-13880](https://github.com/hashicorp/nomad/issues/13880)]
* qemu: restore the monitor socket path when restoring a QEMU task. [[GH-14000](https://github.com/hashicorp/nomad/issues/14000)]

## 1.2.9 (July 13, 2022)

BUG FIXES:

* api: Fix listing evaluations with the wildcard namespace and an ACL token [[GH-13552](https://github.com/hashicorp/nomad/issues/13552)]
* api: Fixed a bug where Consul token was not respected for job revert API [[GH-13065](https://github.com/hashicorp/nomad/issues/13065)]
* cli: Fixed a bug in the names of the `node drain` and `node status` sub-commands [[GH-13656](https://github.com/hashicorp/nomad/issues/13656)]
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
* lifecycle: fixed a bug where sidecar tasks were not being stopped last [[GH-13055](https://github.com/hashicorp/nomad/issues/13055)]
* state: Fix listing evaluations from all namespaces [[GH-13551](https://github.com/hashicorp/nomad/issues/13551)]
* ui: Allow running jobs from a namespace-limited token [[GH-13659](https://github.com/hashicorp/nomad/issues/13659)]
* ui: Fixed a bug that prevented the UI task exec functionality to work from behind a reverse proxy. [[GH-12925](https://github.com/hashicorp/nomad/issues/12925)]
* volumes: Fixed a bug where additions, updates, or removals of host volumes or CSI volumes were not treated as destructive updates [[GH-13008](https://github.com/hashicorp/nomad/issues/13008)]

## 1.2.8 (May 19, 2022)

SECURITY:

* A vulnerability was identified in the go-getter library that Nomad uses for its artifacts such that a specially crafted Nomad jobspec can be used for privilege escalation onto client agent hosts. [CVE-2022-30324](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2022-30324) [[GH-13057](https://github.com/hashicorp/nomad/issues/13057)]

## 1.2.7 (May 10, 2022)

SECURITY:

* server: validate mTLS certificate names on agent to agent endpoints [[GH-11956](https://github.com/hashicorp/nomad/issues/11956)]

IMPROVEMENTS:

* build: upgrade and speedup circleci configuration [[GH-11889](https://github.com/hashicorp/nomad/issues/11889)]

BUG FIXES:

* Fixed a bug where successful poststart tasks were marked as unhealthy [[GH-11945](https://github.com/hashicorp/nomad/issues/11945)]
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
* scheduler: fixed a bug where in-place updates on ineligible nodes would be ignored [[GH-12264](https://github.com/hashicorp/nomad/issues/12264)]
* ui: Fix the link target for CSI volumes on the task detail page [[GH-11896](https://github.com/hashicorp/nomad/issues/11896)]
* ui: fix the unit for the task row memory usage metric [[GH-11980](https://github.com/hashicorp/nomad/issues/11980)]

## 1.2.6 (February 9, 2022)

__BACKWARDS INCOMPATIBILITIES:__

* ACL authentication is now required for the Nomad API job parse endpoint to address a potential security vulnerability

SECURITY:

* Add ACL requirement and HCL validation to the job parse API endpoint to prevent excessive CPU usage. [CVE-2022-24685](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2022-24685) [[GH-12038](https://github.com/hashicorp/nomad/issues/12038)]
* Fix race condition in use of go-getter that could cause a client agent to download the wrong artifact into the wrong destination. [CVE-2022-24686](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2022-24686) [[GH-12036](https://github.com/hashicorp/nomad/issues/12036)]
* Prevent panic in spread iterator during allocation stop. [CVE-2022-24684](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2022-24684) [[GH-12039](https://github.com/hashicorp/nomad/issues/12039)]
* Resolve symlinks to prevent unauthorized access to files outside the allocation directory. [CVE-2022-24683](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2022-24683) [[GH-12037](https://github.com/hashicorp/nomad/issues/12037)]

## 1.2.5 (February 1, 2022)

BUG FIXES:

* csi: Fixed a bug where garbage collected allocations could block new claims on a volume [[GH-11890](https://github.com/hashicorp/nomad/issues/11890)]
* csi: Fixed a bug where releasing volume claims would fail with ACL errors after leadership transitions. [[GH-11891](https://github.com/hashicorp/nomad/issues/11891)]
* csi: Unmount volumes from the client before sending unpublish RPC [[GH-11892](https://github.com/hashicorp/nomad/issues/11892)]
* template: Fixed a bug where client template configuration that did not include any of the new 1.2.4 configuration options could result in none of the configuration getting set. [[GH-11902](https://github.com/hashicorp/nomad/issues/11902)]

## 1.2.4 (January 18, 2022)

FEATURES:

* ui: Add filters to allocations table in jobs/job/allocation view [[GH-11544](https://github.com/hashicorp/nomad/issues/11544)]

IMPROVEMENTS:

* agent/config: Allow binding the HTTP server to multiple addresses. [[GH-11582](https://github.com/hashicorp/nomad/issues/11582)]
* agent: Added `ui` configuration block [[GH-11555](https://github.com/hashicorp/nomad/issues/11555)]
* api: Add pagination and filtering to Evaluations List API [[GH-11648](https://github.com/hashicorp/nomad/issues/11648)]
* api: Added pagination to deployments list API [[GH-11743](https://github.com/hashicorp/nomad/issues/11743)]
* api: Improve error message returned by `Operator.LicenseGet` [[GH-11644](https://github.com/hashicorp/nomad/issues/11644)]
* api: Return a HTTP 404 instead of a HTTP 500 from the Stat File and List Files API endpoints when a file or directory is not found. [[GH-11482](https://github.com/hashicorp/nomad/issues/11482)]
* api: Updated the CSI volumes list API to respect wildcard namespaces [[GH-11724](https://github.com/hashicorp/nomad/issues/11724)]
* api: Updated the deployments list API to respect wildcard namespaces [[GH-11743](https://github.com/hashicorp/nomad/issues/11743)]
* api: Updated the evaluations list API to respect wildcard namespaces [[GH-11710](https://github.com/hashicorp/nomad/issues/11710)]
* api: return HTTP204 on CORS pre-flight checks and allow dot in CORS header keys. [[GH-11323](https://github.com/hashicorp/nomad/issues/11323)]
* cli: Add `-var` and `-var-file` to the command line printed by `job plan` [[GH-11631](https://github.com/hashicorp/nomad/issues/11631)]
* cli: Add event stream capture to `nomad operator debug` [[GH-11865](https://github.com/hashicorp/nomad/issues/11865)]
* cli: Added a `nomad eval list` command. [[GH-11675](https://github.com/hashicorp/nomad/issues/11675)]
* cli: Made the `operator raft info`, `operator raft logs`, `operator raft state`, and `operator snapshot state` commands visible to command line help. [[GH-11682](https://github.com/hashicorp/nomad/issues/11682)]
* cli: Return non-zero exit code from monitor if deployment fails [[GH-11550](https://github.com/hashicorp/nomad/issues/11550)]
* cli: provide `-no-shutdown-delay` option to `job stop` and `alloc stop` commands to ignore `shutdown_delay` [[GH-11596](https://github.com/hashicorp/nomad/issues/11596)]
* core: allow setting and propagation of eval priority on job de/registration [[GH-11532](https://github.com/hashicorp/nomad/issues/11532)]
* deps: Update `armon/go-metrics` to `v0.3.10` [[GH-11504](https://github.com/hashicorp/nomad/issues/11504)]
* driver/docker: Added support for client-wide `pids_limit` configuration [[GH-11526](https://github.com/hashicorp/nomad/issues/11526)]
* hcl: tolerate empty strings for zero integer values in quota and job specification. [[GH-11325](https://github.com/hashicorp/nomad/issues/11325)]
* metrics (Enterprise): Emit `nomad.license.expiration_time_epoch` metric to show the expiration time of the Nomad Enterprise license.
* metrics: Added metric for `client.allocated.max_memory` [[GH-11490](https://github.com/hashicorp/nomad/issues/11490)]
* metrics: added nomad.client.allocs.memory.mapped_file metric [[GH-11500](https://github.com/hashicorp/nomad/issues/11500)]
* scaling: Don't emit scaling action with error in case of active deployment [[GH-11556](https://github.com/hashicorp/nomad/issues/11556)]
* scheduler: Added a `RejectJobRegistration` field to the scheduler configuration API that enabled a setting to reject job register, dispatch, and scale requests without a management ACL token [[GH-11610](https://github.com/hashicorp/nomad/issues/11610)]
* server: Make num_schedulers and enabled_schedulers hot reloadable; add agent API endpoint to enable dynamic modifications of these values. [[GH-11593](https://github.com/hashicorp/nomad/issues/11593)]
* template: Expose consul-template configuration options at the client level for `consul_retry`,
`vault_retry`, `max_stale`, `block_query_wait` and `wait`. Expose per-template configuration
for wait that will override the client level configuration. Add `wait_bounds` to
allow operators to constrain per-template overrides at the client level. [[GH-11606](https://github.com/hashicorp/nomad/issues/11606)]
* ui: Add filters to the allocation list in the client and task group details pages [[GH-11545](https://github.com/hashicorp/nomad/issues/11545)]
* ui: Add titles to breadcrumb labels in app navigation bar [[GH-11590](https://github.com/hashicorp/nomad/issues/11590)]
* ui: Display section title in the navigation breadcrumbs [[GH-11687](https://github.com/hashicorp/nomad/issues/11687)]
* ui: Display the Consul and Vault links configured in the agent [[GH-11557](https://github.com/hashicorp/nomad/issues/11557)]
* ui: add links to legend items in allocation-summary [[GH-11820](https://github.com/hashicorp/nomad/issues/11820)]

BUG FIXES:

* agent: Fixed an issue that caused Consul values to be logged during template rendering [[GH-11838](https://github.com/hashicorp/nomad/issues/11838)]
* agent: Validate reserved_ports are valid to prevent unschedulable nodes. [[GH-11830](https://github.com/hashicorp/nomad/issues/11830)]
* api: Fixed a bug where API or CLI clients could become unresponsive when cron expressions contained zero-padded months [[GH-11132](https://github.com/hashicorp/nomad/issues/11132)]
* artifact: Fixed a bug where uncompressed `.tar` archives were not unpacked after download. [[GH-11481](https://github.com/hashicorp/nomad/issues/11481)]
* cli: Fixed a bug where the `-stale` flag was not respected by `nomad operator debug` [[GH-11678](https://github.com/hashicorp/nomad/issues/11678)]
* cli: Rework meta commands cli flag logic to handle TLS options individually. [[GH-11592](https://github.com/hashicorp/nomad/issues/11592)]
* client: Fixed a bug where clients would ignore the `client_auto_join` setting after losing connection with the servers, causing them to incorrectly fallback to Consul discovery if it was set to `false`. [[GH-11585](https://github.com/hashicorp/nomad/issues/11585)]
* client: Fixed a bug where the allocation log streaming API was missing log frames that spanned log file rotation [[GH-11721](https://github.com/hashicorp/nomad/issues/11721)]
* client: Fixed a memory and goroutine leak for batch tasks and any task that exits without being shut down from the server [[GH-11741](https://github.com/hashicorp/nomad/issues/11741)]
* client: Fixed host network reserved port fingerprinting [[GH-11728](https://github.com/hashicorp/nomad/issues/11728)]
* core: Fix missing fields in Node.Copy() [[GH-11744](https://github.com/hashicorp/nomad/issues/11744)]
* csi: Fixed a bug where deregistering volumes would attempt to deregister the wrong volume if the ID was a prefix of the intended volume [[GH-11852](https://github.com/hashicorp/nomad/issues/11852)]
* csi: Fixed a bug where volume claim releases that were not fully processed before a leadership transition would be ignored [[GH-11776](https://github.com/hashicorp/nomad/issues/11776)]
* drivers: Fixed a bug where the `resolv.conf` copied from the system was not readable to unprivileged processes within the task [[GH-11856](https://github.com/hashicorp/nomad/issues/11856)]
* quotas (Enterprise): Fixed a bug quotas can be incorrectly calculated when nodes fail ranking. [[GH-11848](https://github.com/hashicorp/nomad/issues/11848)]
* rpc: Fixed scaling policy get index response when the policy is found [[GH-11579](https://github.com/hashicorp/nomad/issues/11579)]
* scheduler: detect, log, and emit `nomad.nomad.plan.node_rejected` metric when an unexpected port collision is detected [[GH-11793](https://github.com/hashicorp/nomad/issues/11793)]
* scheduler: Fixed a performance bug where `spread` and node affinity can cause a job to take longer than the nack timeout to be evaluated. [[GH-11712](https://github.com/hashicorp/nomad/issues/11712)]
* template: Fixed a bug where templates did not receive an updated vault token if `change_mode = "noop"` was set in the job definition's `vault` stanza. [[GH-11783](https://github.com/hashicorp/nomad/issues/11783)]
* ui: Fix the ACL requirements for displaying the job details page [[GH-11672](https://github.com/hashicorp/nomad/issues/11672)]

## 1.2.3 (December 13, 2021)

SECURITY:

* Updated to Go 1.17.5. Go 1.17.3 contained 2 CVEs. [CVE-2021-44717](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-44717) could allow a task on a Unix system with exhausted file handles to misdirect I/O. [CVE-2021-44716](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-44716) could create unbounded memory growth in HTTP2 servers. Nomad servers do not use HTTP2. [[GH-11662](https://github.com/hashicorp/nomad/issues/11662)]

## 1.2.2 (November 24, 2021)

BUG FIXES:

* scheduler: Fix panic when system jobs are filtered by node class [[GH-11565](https://github.com/hashicorp/nomad/issues/11565)]

## 1.2.1 (November 19, 2021)

SECURITY:

* Allow limiting QEMU arguments to reduce access to host resources. [CVE-2021-43415](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-43415) [[GH-11542](https://github.com/hashicorp/nomad/issues/11542)]

## 1.2.0 (November 15, 2021)

FEATURES:

* **System Batch scheduler**: Run batch jobs cluster-wide with the new 'sysbatch' scheduler. [[GH-9160](https://github.com/hashicorp/nomad/issues/9160)]

BREAKING CHANGES:

* cli: Renamed folders in `nomad operator debug` bundle for clarity [[GH-11307](https://github.com/hashicorp/nomad/issues/11307)]
* device/nvidia: The Nvidia device plugin is no longer packaged with Nomad and is instead distributed separately. Further, the Nvidia device plugin codebase is now in a separate [repository](https://github.com/hashicorp/nomad-device-nvidia). If you are using Nvidia devices, please follow the 1.2.0 upgrade guide as you will have to install the Nvidia device plugin before conducting an in-place upgrade to Nomad 1.2.0 [[GH-10796](https://github.com/hashicorp/nomad/issues/10796)]

IMPROVEMENTS:

* agent: Added `tls -> rpc_upgrade_mode` to be reloaded on SIGHUP [[GH-11144](https://github.com/hashicorp/nomad/issues/11144)]
* agent: Log the cause of failure if agent failed to start [[GH-11353](https://github.com/hashicorp/nomad/issues/11353)]
* build: Updated to Go 1.17.1 [[GH-11251](https://github.com/hashicorp/nomad/issues/11251)]
* cli: Add `-idempotency-token` option for the `nomad job dispatch` command [[GH-10930](https://github.com/hashicorp/nomad/issues/10930)]
* cli: Add `-show-url` option for the `nomad ui` command. [[GH-11213](https://github.com/hashicorp/nomad/issues/11213)]
* cli: Add `nomad job allocs` command [[GH-11242](https://github.com/hashicorp/nomad/issues/11242)]
* cli: Added support for `-force-color` to the CLI to force colored output. [[GH-10975](https://github.com/hashicorp/nomad/issues/10975)]
* cli: Allow specifying namesapce and region in the `nomad ui` command [[GH-11364](https://github.com/hashicorp/nomad/issues/11364)]
* cli: Improve `nomad job plan` output for `artifact` and `template` changes [[GH-11400](https://github.com/hashicorp/nomad/issues/11400)]
* cli: Improve debug capture for Consul/Vault [[GH-11466](https://github.com/hashicorp/nomad/issues/11466)]
* cli: Improve debug namespace and region support [[GH-11269](https://github.com/hashicorp/nomad/issues/11269)]
* cli: Improved autocomplete support for job dispatch and operator debug [[GH-11270](https://github.com/hashicorp/nomad/issues/11270)]
* cli: Update `nomad operator debug` bundle to include sample of clients by default [[GH-11398](https://github.com/hashicorp/nomad/issues/11398)]
* cli: added `hcl2-strict` flag to control HCL2 parsing errors where variable passed without root [[GH-11284](https://github.com/hashicorp/nomad/issues/11284)]
* cli: added json and template flag opts to the acl bootstrap command [[GH-11411](https://github.com/hashicorp/nomad/issues/11411)]
* cli: the command `node status` now returns `host_network` information as well [[GH-11432](https://github.com/hashicorp/nomad/issues/11432)]
* client/plugins/drivermanager: log if there is an error in a driver event [[GH-11280](https://github.com/hashicorp/nomad/issues/11280)]
* client: Add network interface name to log output during fingerprint [[GH-11184](https://github.com/hashicorp/nomad/issues/11184)]
* client: Allow configuring minimum and maximum host ports used for dynamic ports [[GH-11167](https://github.com/hashicorp/nomad/issues/11167)]
* client: Never embed client.alloc_dir in chroots to prevent infinite recursion from misconfiguration. [[GH-11334](https://github.com/hashicorp/nomad/issues/11334)]
* consul/connect: Allow `http2` and `grpc` protocols in ingress gateways [[GH-11187](https://github.com/hashicorp/nomad/issues/11187)]
* core: Elevated rejected node plan log lines to help diagnose #9506 [[GH-11416](https://github.com/hashicorp/nomad/issues/11416)]
* deps: Update `hashicorp/go-discover` to `20210818145131-c573d69da192` [[GH-11249](https://github.com/hashicorp/nomad/issues/11249)]
* deps: Update `hashicorp/go-hclog` to `v1.0.0` [[GH-11283](https://github.com/hashicorp/nomad/issues/11283)]
* driver/docker: Added support for Docker's `--init` parameter [[GH-11331](https://github.com/hashicorp/nomad/issues/11331)]
* scheduler: Warn users when system and sysbatch evaluations fail to place an allocation [[GH-11111](https://github.com/hashicorp/nomad/issues/11111)]
* server: Allow tuning of node failover heartbeat TTL [[GH-11127](https://github.com/hashicorp/nomad/issues/11127)]
* ui: Add new chart for `system` and `sysbatch` job status per client [[GH-11078](https://github.com/hashicorp/nomad/issues/11078)]
* ui: Display client name as a tooltip where the client ID is used [[GH-11358](https://github.com/hashicorp/nomad/issues/11358)]
* ui: Display jobs from all namespaces by default [[GH-11357](https://github.com/hashicorp/nomad/issues/11357)]
* ui: Display the Nomad version in the Servers and Clients tables and allow filtering and sorting [[GH-11366](https://github.com/hashicorp/nomad/issues/11366)]
* ui: Persist node drain settings in the browser [[GH-11368](https://github.com/hashicorp/nomad/issues/11368)]
* ui: Update Nomad UI favicon [[GH-11371](https://github.com/hashicorp/nomad/issues/11371)]
* vault: Add JobID and TaskGroup to Vault Token metadata [[GH-11397](https://github.com/hashicorp/nomad/issues/11397)]

BUG FIXES:

* agent: Fixed an issue that caused some non-JSON log output when `log_json` was enabled [[GH-11291](https://github.com/hashicorp/nomad/issues/11291)]
* agent: Fixed an issue that could cause previous log lines to be overwritten [[GH-11386](https://github.com/hashicorp/nomad/issues/11386)]
* build: Update go toolchain to 1.17.3 [[GH-11461](https://github.com/hashicorp/nomad/issues/11461)]
* cli: Fix support for `group.consul` field in the HCLv1 parser [[GH-11423](https://github.com/hashicorp/nomad/issues/11423)]
* client: Added `NOMAD_LICENSE` to default environment variable deny list. [[GH-11215](https://github.com/hashicorp/nomad/issues/11215)]
* client: Fixed a bug where network speed fingerprint could fail on Windows [[GH-11183](https://github.com/hashicorp/nomad/issues/11183)]
* client: Removed spurious error log messages when tasks complete [[GH-11273](https://github.com/hashicorp/nomad/issues/11273)]
* core: Fix a bug to stop running system job allocations once their datacenters are removed from the job [[GH-11391](https://github.com/hashicorp/nomad/issues/11391)]
* core: Fixed an issue that created incorrect plan output for jobs with services with the same name. [[GH-10965](https://github.com/hashicorp/nomad/issues/10965)]
* csi: Fixed a bug where the client would incorrectly set an empty capacity range for CSI volume creation requests. [[GH-11238](https://github.com/hashicorp/nomad/issues/11238)]
* deps: Updated `hashicorp/go-plugin` to v1.4.3 to fix handles leakage on Windows platforms [[GH-11143](https://github.com/hashicorp/nomad/issues/11143)]
* driver/exec: Set CPU resource limits when cgroup-v2 is enabled [[GH-11287](https://github.com/hashicorp/nomad/issues/11287)]
* jobspec: ensure consistent error handling between var-file & cli vars [[GH-11165](https://github.com/hashicorp/nomad/issues/11165)]
* rpc: Set the job deregistration eval priority to the job priority [[GH-11426](https://github.com/hashicorp/nomad/issues/11426)]
* rpc: Set the job scale eval priority to the job priority [[GH-11429](https://github.com/hashicorp/nomad/issues/11429)]
* server: Fixed a panic on arm64 platform when dispatching a job with a payload [[GH-11396](https://github.com/hashicorp/nomad/issues/11396)]
* server: Fixed a panic that may occur when preempting multiple allocations on the same node [[GH-11346](https://github.com/hashicorp/nomad/issues/11346)]

## 1.1.18 (August 31, 2022)

BUG FIXES:

* cli: Fixed a bug where forcing a periodic job would fail if the job ID prefix-matched other periodic jobs [[GH-14333](https://github.com/hashicorp/nomad/issues/14333)]

## 1.1.17 (August 25, 2022)

BUG FIXES:

* client/logmon: fixed a bug where logmon cannot find nomad executable [[GH-14297](https://github.com/hashicorp/nomad/issues/14297)]
* ui: Fixed a bug that caused the allocation details page to display the stats bar chart even if the task was pending. [[GH-14224](https://github.com/hashicorp/nomad/issues/14224)]
* vault: Fixed a bug where Vault clients were recreated when the server configuration was reloaded, even if there were no changes to the Vault configuration. [[GH-14298](https://github.com/hashicorp/nomad/issues/14298)]
* vault: Fixed a bug where changing the Vault configuration `namespace` field was not detected as a change during server configuration reload. [[GH-14298](https://github.com/hashicorp/nomad/issues/14298)]

## 1.1.16 (August 05, 2022)

BUG FIXES:

* acl: Fixed a bug where the timestamp for expiring one-time tokens was not deterministic between servers [[GH-13737](https://github.com/hashicorp/nomad/issues/13737)]
* deployments: Fixed a bug that prevented auto-approval if canaries were marked as unhealthy during deployment [[GH-14001](https://github.com/hashicorp/nomad/issues/14001)]
* namespaces: Fixed a bug that allowed deleting a namespace that contained a CSI volume [[GH-13880](https://github.com/hashicorp/nomad/issues/13880)]
* qemu: restore the monitor socket path when restoring a QEMU task. [[GH-14000](https://github.com/hashicorp/nomad/issues/14000)]

## 1.1.15 (July 13, 2022)

BUG FIXES:

* api: Fixed a bug where Consul token was not respected for job revert API [[GH-13065](https://github.com/hashicorp/nomad/issues/13065)]
* cli: Fixed a bug in the names of the `node drain` and `node status` sub-commands [[GH-13656](https://github.com/hashicorp/nomad/issues/13656)]
* client: Fixed a bug where max_kill_timeout client config was ignored [[GH-13626](https://github.com/hashicorp/nomad/issues/13626)]
* cni: Fixed a bug where loopback address was not set for all drivers [[GH-13428](https://github.com/hashicorp/nomad/issues/13428)]
* core: Fixed a bug where an evicted batch job would not be rescheduled [[GH-13205](https://github.com/hashicorp/nomad/issues/13205)]
* core: Fixed a bug where reserved ports on multiple node networks would be treated as a collision. `client.reserved.reserved_ports` is now merged into each `host_network`'s reserved ports instead of being treated as a collision. [[GH-13651](https://github.com/hashicorp/nomad/issues/13651)]
* core: Fixed a bug where the plan applier could deadlock if leader's state lagged behind plan's creation index for more than 5 seconds. [[GH-13407](https://github.com/hashicorp/nomad/issues/13407)]
* csi: Fixed a regression where a timeout was introduced that prevented some plugins from running by marking them as unhealthy after 30s by introducing a configurable `health_timeout` field [[GH-13340](https://github.com/hashicorp/nomad/issues/13340)]
* csi: Fixed a scheduler bug where failed feasibility checks would return early and prevent processing additional nodes [[GH-13274](https://github.com/hashicorp/nomad/issues/13274)]
* lifecycle: fixed a bug where sidecar tasks were not being stopped last [[GH-13055](https://github.com/hashicorp/nomad/issues/13055)]
* ui: Allow running jobs from a namespace-limited token [[GH-13659](https://github.com/hashicorp/nomad/issues/13659)]
* ui: Fixed a bug that prevented the UI task exec functionality to work from behind a reverse proxy. [[GH-12925](https://github.com/hashicorp/nomad/issues/12925)]
* volumes: Fixed a bug where additions, updates, or removals of host volumes or CSI volumes were not treated as destructive updates [[GH-13008](https://github.com/hashicorp/nomad/issues/13008)]

## 1.1.14 (May 19, 2022)

SECURITY:

* A vulnerability was identified in the go-getter library that Nomad uses for its artifacts such that a specially crafted Nomad jobspec can be used for privilege escalation onto client agent hosts. [CVE-2022-30324](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2022-30324) [[GH-13057](https://github.com/hashicorp/nomad/issues/13057)]

## 1.1.13 (May 10, 2022)

SECURITY:

* server: validate mTLS certificate names on agent to agent endpoints [[GH-11956](https://github.com/hashicorp/nomad/issues/11956)]

IMPROVEMENTS:

* api: Updated the CSI volumes list API to respect wildcard namespaces [[GH-11724](https://github.com/hashicorp/nomad/issues/11724)]
* build: upgrade and speedup circleci configuration [[GH-11889](https://github.com/hashicorp/nomad/issues/11889)]

BUG FIXES:

* Fixed a bug where successful poststart tasks were marked as unhealthy [[GH-11945](https://github.com/hashicorp/nomad/issues/11945)]
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
* scheduler: fixed a bug where in-place updates on ineligible nodes would be ignored [[GH-12264](https://github.com/hashicorp/nomad/issues/12264)]
* ui: Fix the link target for CSI volumes on the task detail page [[GH-11896](https://github.com/hashicorp/nomad/issues/11896)]
* ui: fix the unit for the task row memory usage metric [[GH-11980](https://github.com/hashicorp/nomad/issues/11980)]

## 1.1.12 (February 9, 2022)

__BACKWARDS INCOMPATIBILITIES:__

* ACL authentication is now required for the Nomad API job parse endpoint to address a potential security vulnerability

SECURITY:

* Add ACL requirement and HCL validation to the job parse API endpoint to prevent excessive CPU usage. [CVE-2022-24685](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2022-24685) [[GH-12038](https://github.com/hashicorp/nomad/issues/12038)]
* Fix race condition in use of go-getter that could cause a client agent to download the wrong artifact into the wrong destination. [CVE-2022-24686](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2022-24686) [[GH-12036](https://github.com/hashicorp/nomad/issues/12036)]
* Prevent panic in spread iterator during allocation stop. [CVE-2022-24684](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2022-24684) [[GH-12039](https://github.com/hashicorp/nomad/issues/12039)]
* Resolve symlinks to prevent unauthorized access to files outside the allocation directory. [CVE-2022-24683](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2022-24683) [[GH-12037](https://github.com/hashicorp/nomad/issues/12037)]

## 1.1.11 (February 1, 2022)

BUG FIXES:

* csi: Fixed a bug where garbage collected allocations could block new claims on a volume [[GH-11890](https://github.com/hashicorp/nomad/issues/11890)]
* csi: Fixed a bug where releasing volume claims would fail with ACL errors after leadership transitions. [[GH-11891](https://github.com/hashicorp/nomad/issues/11891)]
* csi: Fixed a bug where volume claim releases that were not fully processed before a leadership transition would be ignored [[GH-11776](https://github.com/hashicorp/nomad/issues/11776)]
* csi: Unmount volumes from the client before sending unpublish RPC [[GH-11892](https://github.com/hashicorp/nomad/issues/11892)]

## 1.1.10 (January 18, 2022)

BUG FIXES:

* agent: Validate reserved_ports are valid to prevent unschedulable nodes. [[GH-11830](https://github.com/hashicorp/nomad/issues/11830)]
* cli: Fixed a bug where the `-stale` flag was not respected by `nomad operator debug` [[GH-11678](https://github.com/hashicorp/nomad/issues/11678)]
* client: Fixed a bug where clients would ignore the `client_auto_join` setting after losing connection with the servers, causing them to incorrectly fallback to Consul discovery if it was set to `false`. [[GH-11585](https://github.com/hashicorp/nomad/issues/11585)]
* client: Fixed a memory and goroutine leak for batch tasks and any task that exits without being shut down from the server [[GH-11741](https://github.com/hashicorp/nomad/issues/11741)]
* client: Fixed host network reserved port fingerprinting [[GH-11728](https://github.com/hashicorp/nomad/issues/11728)]
* core: Fix missing fields in Node.Copy() [[GH-11744](https://github.com/hashicorp/nomad/issues/11744)]
* csi: Fixed a bug where deregistering volumes would attempt to deregister the wrong volume if the ID was a prefix of the intended volume [[GH-11852](https://github.com/hashicorp/nomad/issues/11852)]
* drivers: Fixed a bug where the `resolv.conf` copied from the system was not readable to unprivileged processes within the task [[GH-11856](https://github.com/hashicorp/nomad/issues/11856)]
* quotas (Enterprise): Fixed a bug quotas can be incorrectly calculated when nodes fail ranking. [[GH-11848](https://github.com/hashicorp/nomad/issues/11848)]
* rpc: Fixed scaling policy get index response when the policy is found [[GH-11579](https://github.com/hashicorp/nomad/issues/11579)]
* scheduler: detect, log, and emit `nomad.nomad.plan.node_rejected` metric when an unexpected port collision is detected [[GH-11793](https://github.com/hashicorp/nomad/issues/11793)]
* scheduler: Fixed a performance bug where `spread` and node affinity can cause a job to take longer than the nack timeout to be evaluated. [[GH-11712](https://github.com/hashicorp/nomad/issues/11712)]
* template: Fixed a bug where templates did not receive an updated vault token if `change_mode = "noop"` was set in the job definition's `vault` stanza. [[GH-11783](https://github.com/hashicorp/nomad/issues/11783)]

## 1.1.9 (December 13, 2021)

SECURITY:

* Updated to Go 1.16.12. Earlier versions of Go contained 2 CVEs. [CVE-2021-44717](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-44717) could allow a task on a Unix system with exhausted file handles to misdirect I/O. [CVE-2021-44716](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-44716) could create unbounded memory growth in HTTP2 servers. Nomad servers do not use HTTP2. [[GH-11662](https://github.com/hashicorp/nomad/issues/11662)]

## 1.1.8 (November 19, 2021)

SECURITY:

* Allow limiting QEMU arguments to reduce access to host resources. [CVE-2021-43415](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-43415) [[GH-11542](https://github.com/hashicorp/nomad/issues/11542)]

## 1.1.7 (November 15, 2021)

IMPROVEMENTS:

* cli: Improve debug namespace and region support [[GH-11269](https://github.com/hashicorp/nomad/issues/11269)]
* client/plugins/drivermanager: log if there is an error in a driver event [[GH-11280](https://github.com/hashicorp/nomad/issues/11280)]
* core: Elevated rejected node plan log lines to help diagnose #9506 [[GH-11416](https://github.com/hashicorp/nomad/issues/11416)]

BUG FIXES:

* agent: Fixed an issue that caused some non-JSON log output when `log_json` was enabled [[GH-11291](https://github.com/hashicorp/nomad/issues/11291)]
* agent: Fixed an issue that could cause previous log lines to be overwritten [[GH-11386](https://github.com/hashicorp/nomad/issues/11386)]
* cli: Fix support for `group.consul` field in the HCLv1 parser [[GH-11423](https://github.com/hashicorp/nomad/issues/11423)]
* client: Added `NOMAD_LICENSE` to default environment variable deny list. [[GH-11215](https://github.com/hashicorp/nomad/issues/11215)]
* client: Fixed a bug where network speed fingerprint could fail on Windows [[GH-11183](https://github.com/hashicorp/nomad/issues/11183)]
* client: Removed spurious error log messages when tasks complete [[GH-11273](https://github.com/hashicorp/nomad/issues/11273)]
* csi: Fixed a bug where the client would incorrectly set an empty capacity range for CSI volume creation requests. [[GH-11238](https://github.com/hashicorp/nomad/issues/11238)]
* driver/exec: Set CPU resource limits when cgroup-v2 is enabled [[GH-11287](https://github.com/hashicorp/nomad/issues/11287)]
* rpc: Set the job deregistration eval priority to the job priority [[GH-11426](https://github.com/hashicorp/nomad/issues/11426)]
* rpc: Set the job scale eval priority to the job priority [[GH-11429](https://github.com/hashicorp/nomad/issues/11429)]
* server: Fixed a panic on arm64 platform when dispatching a job with a payload [[GH-11396](https://github.com/hashicorp/nomad/issues/11396)]
* server: Fixed a panic that may occur when preempting multiple allocations on the same node [[GH-11346](https://github.com/hashicorp/nomad/issues/11346)]

## 1.1.6 (October 5, 2021)

SECURITY:

* consul/connect: Fixed a bug causing the Nomad agent to panic if a mesh gateway was registered without a `proxy` block. [[GH-11257](https://github.com/hashicorp/nomad/issues/11257)]

IMPROVEMENTS:

* build: Updated to Go 1.16.8 [[GH-11253](https://github.com/hashicorp/nomad/issues/11253)]

BUG FIXES:

* client: Fixed a memory leak in log collector when tasks restart [[GH-11261](https://github.com/hashicorp/nomad/issues/11261)]
* events: Fixed wildcard namespace handling [[GH-10935](https://github.com/hashicorp/nomad/issues/10935)]

## 1.1.5 (September 20, 2021)

IMPROVEMENTS:

* client: Allow Docker hostnames to be configured and interpolated in bridged networking mode [[GH-11173](https://github.com/hashicorp/nomad/issues/11173)]
* deps: Updated `go-memdb` to `v1.3.2` [[GH-11185](https://github.com/hashicorp/nomad/issues/11185)]

BUG FIXES:

* audit (Enterprise): Don't timestamp active audit log file. [[GH-11198](https://github.com/hashicorp/nomad/issues/11198)]
* cli: Display all possible scores in the allocation status table [[GH-11128](https://github.com/hashicorp/nomad/issues/11128)]
* cli: Fixed a bug where the NOMAD_CLI_NO_COLOR environment variable was not always applied [[GH-11168](https://github.com/hashicorp/nomad/issues/11168)]
* client: Task vars should take precedence over host vars when performing interpolation. [[GH-11206](https://github.com/hashicorp/nomad/issues/11206)]
* ui: Fixed an issue that prevented periodic and dispatch jobs in a non-default namespace to be properly rendered [[GH-11110](https://github.com/hashicorp/nomad/issues/11110)]
* ui: Fixed an issue when dispatching jobs from a non-default namespace [[GH-11141](https://github.com/hashicorp/nomad/issues/11141)]

## 1.1.4 (August 26, 2021)

SECURITY:

* Restricted access to the Raft RPC layer, so only servers within the region can issue Raft RPC requests. Previously, local clients and federated servers can issue Raft RPC requests directly. CVE-2021-37218 [[GH-11084](https://github.com/hashicorp/nomad/issues/11084)]

IMPROVEMENTS:

* build: Updated to Go 1.16.7 [[GH-11083](https://github.com/hashicorp/nomad/issues/11083)]
* client: Speed up client startup time [[GH-11005](https://github.com/hashicorp/nomad/issues/11005)]
* consul/connect: Reduced the noise of log messages emitted for connect native tasks [[GH-10951](https://github.com/hashicorp/nomad/issues/10951)]
* csi: add flag for providing secrets as a set of key/value pairs to list snapshots [[GH-10848](https://github.com/hashicorp/nomad/issues/10848)]
* deps: Updated `x/sys` to `20210818153620-00dd8d7831e7` [[GH-11065](https://github.com/hashicorp/nomad/issues/11065)]
* scheduler: Re-evaluate nodes for system jobs after attributes changes [[GH-11007](https://github.com/hashicorp/nomad/issues/11007)]
* ui: Add header separator between a child job priority and its parent [[GH-11020](https://github.com/hashicorp/nomad/issues/11020)]

BUG FIXES:

* core: Fixed a bug where system jobs with non-unique IDs may not be placed on new nodes [[GH-11054](https://github.com/hashicorp/nomad/issues/11054)]
* agent: Don't timestamp active log file. [[GH-11070](https://github.com/hashicorp/nomad/issues/11070)]
* deployments: Fixed a bug where multi-group deployments don't get auto-promoted when one group has no canaries. [[GH-11013](https://github.com/hashicorp/nomad/issues/11013)]
* driver/docker: Fixed a bug in the authentication config where not all fields were set [[GH-10929](https://github.com/hashicorp/nomad/issues/10929)]
* server: Fixed a bug where planning job update reports spurious in-place updates even if the update includes no changes [[GH-10990](https://github.com/hashicorp/nomad/issues/10990)]
* ui: Add ability to search across all namespaces [[GH-10666](https://github.com/hashicorp/nomad/issues/10666)]
* ui: Fixed a bug where the "Dispatch Job" button was displayed for non-parameterized jobs [[GH-11019](https://github.com/hashicorp/nomad/issues/11019)]
* ui: Fixed a bug where the job dispatch form is not displayed when the job doesn't have meta fields [[GH-10934](https://github.com/hashicorp/nomad/issues/10934)]

## 1.1.3 (July 29, 2021)

IMPROVEMENTS:

* api: Added `NewSystemJob` helper function to create base system job object. [[GH-10861](https://github.com/hashicorp/nomad/issues/10861)]
* audit (Enterprise): allow configuring file mode for audit logs [[GH-10916](https://github.com/hashicorp/nomad/issues/10916)]
* build: no longer use vendor directory [[GH-10898](https://github.com/hashicorp/nomad/issues/10898)]
* cli: Added a `-task` flag to `alloc restart` and `alloc signal` for consistent UX with `alloc exec` and `alloc logs` [[GH-10859](https://github.com/hashicorp/nomad/issues/10859)]
* cli: Support recent job spec construct in the HCLv1 parser [[GH-10931](https://github.com/hashicorp/nomad/issues/10931)]
* consul/connect: automatically set CONSUL_TLS_SERVER_NAME for connect native tasks [[GH-10804](https://github.com/hashicorp/nomad/issues/10804)]
* dispatch jobs: Added optional idempotency token to `WriteOptions` which prevents Nomad from creating new dispatched jobs for retried requests. [[GH-10806](https://github.com/hashicorp/nomad/issues/10806)]
* ui: Added new screen to dispatch a parameterized batch job [[GH-10675](https://github.com/hashicorp/nomad/issues/10675)]
* ui: Handle ACL token when running behind a reverse proxy [[GH-10563](https://github.com/hashicorp/nomad/issues/10563)]

BUG FIXES:

* api: Reverted to using http/1 to fix a 1.1.2 regression in `alloc exec` sessions [[GH-10958](https://github.com/hashicorp/nomad/issues/10958)]
* cli: Fixed a bug where `-namespace` flag was not respected for `job run` and `job plan` commands. [[GH-10875](https://github.com/hashicorp/nomad/issues/10875)]
* cli: Fixed a panic when deployment monitor is invoked in some CI environments [[GH-10926](https://github.com/hashicorp/nomad/issues/10926)]
* cli: Fixed system commands, so they correctly use passed flags [[GH-10822](https://github.com/hashicorp/nomad/issues/10822)]
* cli: Fixed the help message for the `nomad alloc signal` command [[GH-10917](https://github.com/hashicorp/nomad/issues/10917)]
* client: Fixed a bug where a restarted client may start an already completed tasks in rare conditions [[GH-10907](https://github.com/hashicorp/nomad/issues/10907)]
* client: Fixed bug where meta blocks were not interpolated with task environment [[GH-10876](https://github.com/hashicorp/nomad/issues/10876)]
* cni: Fixed a bug where fingerprinting of CNI configuration failed with default `cni_config_dir` and `cni_path` [[GH-10870](https://github.com/hashicorp/nomad/issues/10870)]
* consul/connect: Avoid assumption of parent service when syncing connect proxies [[GH-10872](https://github.com/hashicorp/nomad/issues/10872)]
* consul/connect: Fixed a bug causing high CPU with multiple connect sidecars in one group [[GH-10883](https://github.com/hashicorp/nomad/issues/10883)]
* consul/connect: Fixed a bug where service deregistered before connect sidecar [[GH-10873](https://github.com/hashicorp/nomad/issues/10873)]
* consul: Fixed a bug where services may incorrectly fail conflicting name validation [[GH-10868](https://github.com/hashicorp/nomad/issues/10868)]
* consul: avoid extra sync operations when no action required [[GH-10865](https://github.com/hashicorp/nomad/issues/10865)]
* consul: remove ineffective edge case handling on service deregistration [[GH-10842](https://github.com/hashicorp/nomad/issues/10842)]
* core: Fixed a bug where affinity memoization may cause planning problems [[GH-10897](https://github.com/hashicorp/nomad/issues/10897)]
* core: Fixed a bug where internalized constraint strings broke job plan [[GH-10896](https://github.com/hashicorp/nomad/issues/10896)]
* core: Fixed a panic that may arise when upgrading pre-1.1.0 cluster to 1.1.x and may cause cluster outage [[GH-10952](https://github.com/hashicorp/nomad/issues/10952)]
* csi: Fixed a bug where volume secrets were not used for creating snapshots. [[GH-10840](https://github.com/hashicorp/nomad/issues/10840)]
* csi: fixed a CLI panic when formatting `volume status` with `-verbose` flag [[GH-10818](https://github.com/hashicorp/nomad/issues/10818)]
* deps: Update `hashicorp/consul-template` to v0.25.2 to fix panic reading Vault secrets [[GH-10892](https://github.com/hashicorp/nomad/issues/10892)]
* driver/docker: Moved the generated `/etc/hosts` file's mount source to the allocation directory so that it can be shared between tasks of an allocation. [[GH-10823](https://github.com/hashicorp/nomad/issues/10823)]
* drivers: Fixed bug where Nomad incorrectly reported tasks as recovered successfully even when they were not. [[GH-10849](https://github.com/hashicorp/nomad/issues/10849)]
* scheduler: Fixed a bug where updates to the `datacenters` field were not destructive. [[GH-10864](https://github.com/hashicorp/nomad/issues/10864)]
* ui: Fixes bug where UI was not detecting namespace-specific capabilities. [[GH-10893](https://github.com/hashicorp/nomad/issues/10893)]
* volumes: Fix a bug where the HTTP server would crash if a `volume_mount` block was empty [[GH-10855](https://github.com/hashicorp/nomad/issues/10855)]

## 1.1.2 (June 22, 2021)

IMPROVEMENTS:
* cli: Added `-monitor` flag to `deployment status` command and automatically monitor deployments from `job run` command. [[GH-10661](https://github.com/hashicorp/nomad/pull/10661)]
* cli: Added remainder of available pprof profiles to `nomad operator debug` capture. [[GH-10748](https://github.com/hashicorp/nomad/issues/10748)]
* consul/connect: Validate Connect service upstream address uniqueness within task group [[GH-7833](https://github.com/hashicorp/nomad/issues/7833)]
* deps: Update gopsutil for multisocket cpuinfo detection performance fix [[GH-10761](https://github.com/hashicorp/nomad/pull/10790)]
* docker: Tasks using `network.mode = "bridge"` that don't set their `network_mode` will receive a `/etc/hosts` file that includes the pause container's hostname and any `extra_hosts`. [[GH-10766](https://github.com/hashicorp/nomad/issues/10766)]

BUG FIXES:
* artifact: Fixed support for 5 part vhosted-style AWS S3 buckets. [[GH-10778](https://github.com/hashicorp/nomad/issues/10778)]
* artifact: HTTP requests made for artifacts will default to trying HTTP2 first. [[GH-10778](https://github.com/hashicorp/nomad/issues/10778)]
* client/fingerprint/java: Fixed a bug where java fingerprinter would not detect some Java distributions [[GH-10765](https://github.com/hashicorp/nomad/pull/10765)]
* consul: Fixed a bug where consul check parameters missing in group services [[GH-10764](https://github.com/hashicorp/nomad/pull/10764)]
* consul/connect: Fixed an overly restrictive connect constraint [[GH-10754](https://github.com/hashicorp/nomad/pull/10754)]
* consul/connect: Fixed a bug where Connect upstreams would not be updated in-place [[GH-10776](https://github.com/hashicorp/nomad/pull/10776)]
* deployments: Fixed a bug where unnecessary goroutines were spawned whenever deployments were updated. [[GH-10756](https://github.com/hashicorp/nomad/issues/10756)]
* quotas (Enterprise): Fixed a bug where quotas were evaluated before constraints, resulting in quota capacity being used up by filtered nodes. [[GH-10753](https://github.com/hashicorp/nomad/issues/10753)]

## 1.1.1 (June 9, 2021)

FEATURES:
 * **Connect Mesh Gateways**: Adds built-in support for running Consul Connect Mesh Gateways [[GH-10658](https://github.com/hashicorp/nomad/pull/10658)]

IMPROVEMENTS:
* build: Updated to Go 1.16.5 [[GH-10733](https://github.com/hashicorp/nomad/issues/10733)]
* cli: Added success confirmation message for `nomad volume delete` and `nomad volume deregister`. [[GH-10591](https://github.com/hashicorp/nomad/issues/10591)]
* cli: Cross-namespace `nomad job` commands will now select exact matches if the selection is unambiguous. [[GH-10648](https://github.com/hashicorp/nomad/issues/10648)]
* client/fingerprint: Consul fingerprinter probes for additional enterprise and connect related attributes [[GH-10699](https://github.com/hashicorp/nomad/pull/10699)]
* consul/connect: Only schedule connect tasks on nodes where connect is enabled in Consul [[GH-10702](https://github.com/hashicorp/nomad/pull/10702)]
* csi: Validate that `volume` blocks for CSI volumes include the required `attachment_mode` and `access_mode` fields. [[GH-10651](https://github.com/hashicorp/nomad/issues/10651)]
* server: Make deployment rate limiting configurable for high volume loads [[GH-10706](https://github.com/hashicorp/nomad/pull/10706)]

BUG FIXES:
* api: Fixed event stream connection initialization when there are no events to send [[GH-10637](https://github.com/hashicorp/nomad/issues/10637)]
* cli: Fixed a bug where `plugin status` did not validate the passed `type` flag correctly [[GH-10712](https://github.com/hashicorp/nomad/pull/10712)]
* cli: Fixed a bug where `quota status` and `namespace status` commands may panic if the CLI targets a pre-1.1.0 cluster [[GH-10620](https://github.com/hashicorp/nomad/pull/10620)]
* cli: Fixed a bug where `alloc exec` may fail with "unexpected EOF" without returning the exit code after a command [[GH-10657](https://github.com/hashicorp/nomad/issues/10657)]
* consul: Fixed a bug where consul namespace API would be queried even when consul namespaces were not enabled [[GH-10715](https://github.com/hashicorp/nomad/pull/10715)]
* consul: Fixed a bug where connect jobs would always fail job submission when allow_unauthenticated was set to false [[GH-10718](https://github.com/hashicorp/nomad/issues/10718)]
* csi: Fixed a bug where `mount_options` were not passed to CSI controller plugins for validation during volume creation and mounting. [[GH-10643](https://github.com/hashicorp/nomad/issues/10643)]
* csi: Fixed a bug where `capability` blocks were not passed to CSI controller plugins for validation for `nomad volume register` commands. [[GH-10703](https://github.com/hashicorp/nomad/issues/10703)]
* client: Fixed a bug where `alloc exec` sessions may terminate abruptly after a few minutes [[GH-10710](https://github.com/hashicorp/nomad/issues/10710)]
* drivers/exec: Fixed a bug where `exec` and `java` tasks inherit the Nomad agent's `oom_score_adj` value [[GH-10698](https://github.com/hashicorp/nomad/issues/10698)]
* drivers/docker: Fixed a bug where short lived docker tasks may fail with obscure cpuset cgroup errors [[GH-10416](https://github.com/hashicorp/nomad/issues/10416)]
* quotas (Enterprise): Fixed a bug where stopped allocations for a failed deployment can be double-credited to quota limits, resulting in a quota limit bypass. [[GH-10694](https://github.com/hashicorp/nomad/issues/10694)]
* ui: Fixed a bug where exec would not work across regions. [[GH-10539](https://github.com/hashicorp/nomad/issues/10539)]
* ui: Fixed global-search shortcut for non-english keyboards. [[GH-10714](https://github.com/hashicorp/nomad/issues/10714)]

## 1.1.0 (May 18, 2021)

FEATURES:
 * **Memory oversubscription**: Improve cluster efficiency by allowing applications,  whether containerized or non-containerized, to use memory in excess of their scheduled amount.
 * **Reserved CPU cores**: Improve the performance of your applications by ensuring tasks have exclusive use of client CPUs.
 * **UI improvements**: Enjoy a streamlined operator experience with fuzzy search, resource monitoring, and authentication improvements.
 * **CSI enhancements**: Run stateful applications with improved volume management and support for Container Storage Interface (CSI) plugins such as Ceph.
 * **Readiness checks**: Differentiate between application liveness and readiness with new options for task health checks.
 * **Remote task drivers (technical preview)**: Use Nomad to manage your workloads on more platforms, such as AWS Lambda or Amazon ECS.
 * **Consul namespace support (Enterprise)**: Run Nomad-defined services in their HashiCorp Consul namespaces more easily using Nomad Enterprise.
 * **License autoloading (Enterprise)**: Automatically load Nomad licenses when a Nomad server agent starts using Nomad Enterprise.
 * **Autoscaling improvements**: Scale your applications more precisely with new strategies.

__BACKWARDS INCOMPATIBILITIES:__
 * csi: The `attachment_mode` and `access_mode` field are required for `volume` blocks in job specifications. Registering a volume requires at least one `capability` block with the `attachment_mode` and `access_mode` fields set. [[GH-10330](https://github.com/hashicorp/nomad/issues/10330)]
 * drivers/exec+java: Reduce set of linux capabilities enabled by default [[GH-10600](https://github.com/hashicorp/nomad/pull/10600)]
 * licensing: Enterprise licenses are no longer stored in raft or synced between servers. Loading the Enterprise license from disk or environment is required. The `nomad license put` command has been removed. [[GH-10458](https://github.com/hashicorp/nomad/issues/10458)]

SECURITY:
 * drivers/docker+exec+java: Disable `CAP_NET_RAW` linux capability by default to prevent ARP spoofing. CVE-2021-32575 [[GH-10568](https://github.com/hashicorp/nomad/issues/10568)](https://github.com/hashicorp/nomad/issues/10568)

IMPROVEMENTS:
 * api: Added an API endpoint for fuzzy search queries [[GH-10184](https://github.com/hashicorp/nomad/pull/10184)]
 * api: Removed unimplemented `CSIVolumes.PluginList` API. [[GH-10158](https://github.com/hashicorp/nomad/issues/10158)]
 * api: Added `namespace` field for the jobs list endpoint response [[GH-10434](https://github.com/hashicorp/nomad/issues/10434)]
 * build: Updated to Go 1.16.3 [[GH-10483](https://github.com/hashicorp/nomad/issues/10483)]
 * cli: Update defaults for `nomad operator debug` flags `-interval` and `-server-id` to match common usage. [[GH-10121](https://github.com/hashicorp/nomad/issues/10121)]
 * cli: Support an optional file argument for `volume init` and `quota init` commands [[GH-10397](https://github.com/hashicorp/nomad/issues/10397)]
 * client/config: Enable sockaddr templating for `network-interface` attribute. [[GH-10404](https://github.com/hashicorp/nomad/issues/10404)]
 * client/fingerprint: Added support multiple host network aliases for the same interface. [[GH-10104](https://github.com/hashicorp/nomad/issues/10104)]
 * consul: Allow setting `body` field on service/check Consul health checks. [[GH-10186](https://github.com/hashicorp/nomad/issues/10186)]
 * consul/connect: Use exponential backoff for consul envoy bootstrap process [[GH-10453](https://github.com/hashicorp/nomad/pull/10453)]
 * consul/connect: Enable setting `local_bind_address` field on connect upstreams [[GH-6248](https://github.com/hashicorp/nomad/issues/6248)]
 * consul/connect: Added job-submission validation for Connect sidecar service and group names [[GH-10455](https://github.com/hashicorp/nomad/pull/10455)]
 * consul/connect: Automatically populate `CONSUL_HTTP_ADDR` for connect native tasks in host networking mode. [[GH-10239](https://github.com/hashicorp/nomad/issues/10239)]
 * consul/connect: Added `disable_default_tcp_check` field to `connect.sidecar_service` blocks to disable the default TCP listener check for Connect sidecar tasks. [[GH-10531](https://github.com/hashicorp/nomad/pull/10531)]
 * core: Persist metadata about most recent drain in Node.LastDrain [[GH-10250](https://github.com/hashicorp/nomad/issues/10250)]
 * csi: Added support for jobs to request a unique volume ID per allocation. [[GH-10136](https://github.com/hashicorp/nomad/issues/10136)]
 * driver/docker: Added support for optional extra container labels. [[GH-9885](https://github.com/hashicorp/nomad/issues/9885)]
 * driver/docker: Added support for configuring default logger behavior in the client configuration. [[GH-10156](https://github.com/hashicorp/nomad/issues/10156)]
 * metrics: Added blocked evaluation resources metrics [[GH-10454](https://github.com/hashicorp/nomad/pull/10454)]
 * networking: Added support for user-defined iptables rules on the NOMAD-ADMIN chain. [[GH-10181](https://github.com/hashicorp/nomad/issues/10181)]
 * networking: Added support for interpolating host network names with node attributes. [[GH-10196](https://github.com/hashicorp/nomad/issues/10196)]
 * nomad/structs: Removed deprecated Node.Drain field, added API extensions to restore it [[GH-10202](https://github.com/hashicorp/nomad/issues/10202)]
 * ui: Added a job reversion button [[GH-10336](https://github.com/hashicorp/nomad/pull/10336)]
 * ui: Added memory maximum to task group ribbon [[GH-10459](https://github.com/hashicorp/nomad/pull/10459)]
 * ui: Updated global search to use fuzzy search API [[GH-10412](https://github.com/hashicorp/nomad/pull/10412)]
 * ui: Changed displays of aggregate units to use larger suffixes when appropriate [[GH-10257](https://github.com/hashicorp/nomad/pull/10257)]
 * ui: Added resource reservation indicators on client charts and task breakdowns on allocation charts [[GH-10208](https://github.com/hashicorp/nomad/pull/10208)]

BUG FIXES:
 * core (Enterprise): Update licensing library to v0.0.11 to include race condition fix. [[GH-10253](https://github.com/hashicorp/nomad/issues/10253)]
 * agent: Only allow querying Prometheus formatted metrics if Prometheus is enabled within the config [[GH-10140](https://github.com/hashicorp/nomad/pull/10140)]
 * api: Ensured that `api.LicenseGet` returned response meta data [[GH-10276](https://github.com/hashicorp/nomad/issues/10276)]
 * api: Added missing devices block to AllocatedTaskResources [[GH-10064](https://github.com/hashicorp/nomad/pull/10064)]
 * api: Fixed a panic that may occur on concurrent access to an SDK client [[GH-10302](https://github.com/hashicorp/nomad/issues/10302)]
 * cli: Fixed a bug where non-int proxy port would panic CLI [[GH-10072](https://github.com/hashicorp/nomad/issues/10072)]
 * cli: Fixed a bug where `snapshot agent` command panics on launch [[GH-10276](https://github.com/hashicorp/nomad/issues/10276)]
 * cli: Remove extra linefeeds in monitor.log files written by `nomad operator debug`. [[GH-10252](https://github.com/hashicorp/nomad/issues/10252)]
 * cli: Fixed a bug where parsing HCLv2 may panic on some variable interpolation syntax [[GH-10326](https://github.com/hashicorp/nomad/issues/10326)] [[GH-10419](https://github.com/hashicorp/nomad/issues/10419)]
 * cli: Fixed a bug where `nomad operator debug` incorrectly parsed https Consul API URLs. [[GH-10082](https://github.com/hashicorp/nomad/pull/10082)]
 * cli: Fixed a panic where `nomad job run` or `plan` would crash when supplied with non-existent `-var-file` files. [[GH-10569](https://github.com/hashicorp/nomad/issues/10569)]
 * client: Fixed log formatting when killing tasks. [[GH-10135](https://github.com/hashicorp/nomad/issues/10135)]
 * client: Added handling for cgroup-v2 memory metrics [[GH-10286](https://github.com/hashicorp/nomad/issues/10286)]
 * client: Only publish measured allocation memory metrics [[GH-10376](https://github.com/hashicorp/nomad/issues/10376)]
 * client: Fixed a bug where small files would be assigned the wrong content type. [[GH-10348](https://github.com/hashicorp/nomad/pull/10348)]
 * consul/connect: Fixed a bug where job plan always different when using expose checks. [[GH-10492](https://github.com/hashicorp/nomad/pull/10492)]
 * consul/connect: Fixed a bug where HTTP ingress gateways could not use wildcard names. [[GH-10457](https://github.com/hashicorp/nomad/pull/10457)]
 * cni: Fallback to an interface with an IP address if sandbox interface lacks one. [[GH-9895](https://github.com/hashicorp/nomad/issues/9895)]
 * csi: Fixed a bug where volume with IDs that are a substring prefix of another volume could use the wrong volume for feasibility checking. [[GH-10158](https://github.com/hashicorp/nomad/issues/10158)]
 * drivers/docker: Fixed a bug where Dockerfile `STOPSIGNAL` was not honored. [[GH-10441](https://github.com/hashicorp/nomad/issues/10441)]
 * drivers/raw_exec: Fixed a bug where exit codes could be dropped and return a spurious error. [[GH-10494](https://github.com/hashicorp/nomad/issues/10494)]
 * scheduler: Fixed a bug where Nomad reports negative or incorrect running children counts for periodic jobs. [[GH-10145](https://github.com/hashicorp/nomad/issues/10145)]
 * scheduler: Fixed a bug where jobs requesting multiple CSI volumes could be incorrectly scheduled if only one of the volumes passed feasibility checking. [[GH-10143](https://github.com/hashicorp/nomad/issues/10143)]
 * service: Fixed a bug where new script checks would not be added on job updates. [[GH-10403](https://github.com/hashicorp/nomad/issues/10403)]
 * server: Fixed a bug affecting periodic job summary counts [[GH-10145](https://github.com/hashicorp/nomad/issues/10145)]
 * server: Fixed a bug where draining a node may fail to migrate its allocations [[GH-10411](https://github.com/hashicorp/nomad/issues/10411)]
 * server: Fixed a bug where jobs may not run if submitted with ParentID field set [[GH-10424](https://github.com/hashicorp/nomad/issues/10424)]
 * server: Fixed a panic that may arise on submission of jobs containing invalid service checks [[GH-10154](https://github.com/hashicorp/nomad/issues/10154)]
 * ui: Fixed the rendering of interstitial components shown after processing a dynamic application sizing recommendation. [[GH-10094](https://github.com/hashicorp/nomad/pull/10094)]

## 1.0.18 (February 9, 2022)

__BACKWARDS INCOMPATIBILITIES:__

* ACL authentication is now required for the Nomad API job parse endpoint to address a potential security vulnerability

SECURITY:

* Add ACL requirement and HCL validation to the job parse API endpoint to prevent excessive CPU usage. [CVE-2022-24685](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2022-24685) [[GH-12038](https://github.com/hashicorp/nomad/issues/12038)]
* Fix race condition in use of go-getter that could cause a client agent to download the wrong artifact into the wrong destination. [CVE-2022-24686](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2022-24686) [[GH-12036](https://github.com/hashicorp/nomad/issues/12036)]
* Prevent panic in spread iterator during allocation stop. [CVE-2022-24684](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2022-24684) [[GH-12039](https://github.com/hashicorp/nomad/issues/12039)]
* Resolve symlinks to prevent unauthorized access to files outside the allocation directory. [CVE-2022-24683](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2022-24683) [[GH-12037](https://github.com/hashicorp/nomad/issues/12037)]

## 1.0.17 (February 1, 2022)

BUG FIXES:

* csi: Fixed a bug where garbage collected allocations could block new claims on a volume [[GH-11890](https://github.com/hashicorp/nomad/issues/11890)]
* csi: Fixed a bug where releasing volume claims would fail with ACL errors after leadership transitions. [[GH-11891](https://github.com/hashicorp/nomad/issues/11891)]
* csi: Fixed a bug where volume claim releases that were not fully processed before a leadership transition would be ignored [[GH-11776](https://github.com/hashicorp/nomad/issues/11776)]
* csi: Unmount volumes from the client before sending unpublish RPC [[GH-11892](https://github.com/hashicorp/nomad/issues/11892)]

## 1.0.16 (January 18, 2022)

BUG FIXES:

* agent: Validate reserved_ports are valid to prevent unschedulable nodes. [[GH-11830](https://github.com/hashicorp/nomad/issues/11830)]
* cli: Fixed a bug where the `-stale` flag was not respected by `nomad operator debug` [[GH-11678](https://github.com/hashicorp/nomad/issues/11678)]
* client: Fixed a bug where clients would ignore the `client_auto_join` setting after losing connection with the servers, causing them to incorrectly fallback to Consul discovery if it was set to `false`. [[GH-11585](https://github.com/hashicorp/nomad/issues/11585)]
* client: Fixed a memory and goroutine leak for batch tasks and any task that exits without being shut down from the server [[GH-11741](https://github.com/hashicorp/nomad/issues/11741)]
* client: Fixed host network reserved port fingerprinting [[GH-11728](https://github.com/hashicorp/nomad/issues/11728)]
* core: Fix missing fields in Node.Copy() [[GH-11744](https://github.com/hashicorp/nomad/issues/11744)]
* csi: Fixed a bug where deregistering volumes would attempt to deregister the wrong volume if the ID was a prefix of the intended volume [[GH-11852](https://github.com/hashicorp/nomad/issues/11852)]
* drivers: Fixed a bug where the `resolv.conf` copied from the system was not readable to unprivileged processes within the task [[GH-11856](https://github.com/hashicorp/nomad/issues/11856)]
* quotas (Enterprise): Fixed a bug quotas can be incorrectly calculated when nodes fail ranking. [[GH-11848](https://github.com/hashicorp/nomad/issues/11848)]
* rpc: Fixed scaling policy get index response when the policy is found [[GH-11579](https://github.com/hashicorp/nomad/issues/11579)]
* scheduler: detect, log, and emit `nomad.nomad.plan.node_rejected` metric when an unexpected port collision is detected [[GH-11793](https://github.com/hashicorp/nomad/issues/11793)]
* scheduler: Fixed a performance bug where `spread` and node affinity can cause a job to take longer than the nack timeout to be evaluated. [[GH-11712](https://github.com/hashicorp/nomad/issues/11712)]
* template: Fixed a bug where templates did not receive an updated vault token if `change_mode = "noop"` was set in the job definition's `vault` stanza. [[GH-11783](https://github.com/hashicorp/nomad/issues/11783)]

## 1.0.15 (December 13, 2021)

SECURITY:

* Updated to Go 1.16.12. Earlier versions of Go contained 2 CVEs. [CVE-2021-44717](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-44717) could allow a task on a Unix system with exhausted file handles to misdirect I/O. [CVE-2021-44716](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-44716) could create unbounded memory growth in HTTP2 servers. Nomad servers do not use HTTP2. [[GH-11662](https://github.com/hashicorp/nomad/issues/11662)]

## 1.0.14 (November 19, 2021)

SECURITY:

* Allow limiting QEMU arguments to reduce access to host resources. [CVE-2021-43415](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-43415) [[GH-11542](https://github.com/hashicorp/nomad/issues/11542)]

## 1.0.13 (November 15, 2021)

IMPROVEMENTS:

* cli: Improve debug namespace and region support [[GH-11269](https://github.com/hashicorp/nomad/issues/11269)]
* cli: Update defaults for `nomad operator debug` flags `-interval` and `-server-id` to match common usage [[GH-10121](https://github.com/hashicorp/nomad/issues/10121)]
* client/plugins/drivermanager: log if there is an error in a driver event [[GH-11280](https://github.com/hashicorp/nomad/issues/11280)]
* core: Elevated rejected node plan log lines to help diagnose #9506 [[GH-11416](https://github.com/hashicorp/nomad/issues/11416)]

BUG FIXES:

* agent: Fixed an issue that caused some non-JSON log output when `log_json` was enabled [[GH-11291](https://github.com/hashicorp/nomad/issues/11291)]
* agent: Fixed an issue that could cause previous log lines to be overwritten [[GH-11386](https://github.com/hashicorp/nomad/issues/11386)]
* client: Fixed a bug where network speed fingerprint could fail on Windows [[GH-11183](https://github.com/hashicorp/nomad/issues/11183)]
* client: Removed spurious error log messages when tasks complete [[GH-11273](https://github.com/hashicorp/nomad/issues/11273)]
* driver/exec: Set CPU resource limits when cgroup-v2 is enabled [[GH-11287](https://github.com/hashicorp/nomad/issues/11287)]
* rpc: Set the job deregistration eval priority to the job priority [[GH-11426](https://github.com/hashicorp/nomad/issues/11426)]
* rpc: Set the job scale eval priority to the job priority [[GH-11429](https://github.com/hashicorp/nomad/issues/11429)]
* server: Fixed a panic that may occur when preempting multiple allocations on the same node [[GH-11346](https://github.com/hashicorp/nomad/issues/11346)]

## 1.0.12 (October 5, 2021)

IMPROVEMENTS:

* build: Updated to Go 1.15.15 [[GH-11252](https://github.com/hashicorp/nomad/issues/11252)]

BUG FIXES:

* client: Fixed a memory leak in log collector when tasks restart [[GH-11261](https://github.com/hashicorp/nomad/issues/11261)]
* events: Fixed wildcard namespace handling [[GH-10935](https://github.com/hashicorp/nomad/issues/10935)]

## 1.0.11 (September 20, 2021)

IMPROVEMENTS:

* deps: Updated `go-memdb` to `v1.3.2` [[GH-11185](https://github.com/hashicorp/nomad/issues/11185)]

BUG FIXES:

* audit (Enterprise): Don't timestamp active audit log file. [[GH-11198](https://github.com/hashicorp/nomad/issues/11198)]
* cli: Display all possible scores in the allocation status table [[GH-11128](https://github.com/hashicorp/nomad/issues/11128)]
* cli: Fixed a bug where the NOMAD_CLI_NO_COLOR environment variable was not always applied [[GH-11168](https://github.com/hashicorp/nomad/issues/11168)]
* client: Task vars should take precedence over host vars when performing interpolation. [[GH-11206](https://github.com/hashicorp/nomad/issues/11206)]

## 1.0.10 (August 26, 2021)

SECURITY:

* Restricted access to the Raft RPC layer, so only servers within the region can issue Raft RPC requests. Previously, local clients and federated servers can issue Raft RPC requests directly. CVE-2021-37218 [[GH-11084](https://github.com/hashicorp/nomad/issues/11084)]

BUG FIXES:

* core: Fixed a bug where system jobs with non-unique IDs may not be placed on new nodes [[GH-11054](https://github.com/hashicorp/nomad/issues/11054)]
* agent: Don't timestamp active log file. [[GH-11070](https://github.com/hashicorp/nomad/issues/11070)]
* deployments: Fixed a bug where multi-group deployments don't get auto-promoted when one group has no canaries. [[GH-11013](https://github.com/hashicorp/nomad/issues/11013)]
* driver/docker: Fixed a bug in the authentication config where not all fields were set [[GH-10929](https://github.com/hashicorp/nomad/issues/10929)]
* server: Fixed a bug where planning job update reports spurious in-place updates even if the update includes no changes [[GH-10990](https://github.com/hashicorp/nomad/issues/10990)]

## 1.0.9 (July 29, 2021)

BUG FIXES:

* core: Fixed a bug where internalized constraint strings broke job plan [[GH-10896](https://github.com/hashicorp/nomad/issues/10896)]
* core: Fixed a bug where affinity memoization may cause planning problems [[GH-10897](https://github.com/hashicorp/nomad/issues/10897)]
* cli: Fixed a bug where `-namespace` flag was not respected for `job run` and `job plan` commands. [[GH-10875](https://github.com/hashicorp/nomad/issues/10875)]
* client: Fixed a bug where a restarted client may start an already completed tasks in rare conditions [[GH-10907](https://github.com/hashicorp/nomad/issues/10907)]
* client: Fixed bug where meta blocks were not interpolated with task environment [[GH-10876](https://github.com/hashicorp/nomad/issues/10876)]
* cni: Fixed a bug where fingerprinting of CNI configuration failed with default `cni_config_dir` and `cni_path` [[GH-10870](https://github.com/hashicorp/nomad/issues/10870)]
* consul: Fixed a bug where services may incorrectly fail conflicting name validation [[GH-10868](https://github.com/hashicorp/nomad/issues/10868)]
* deps: Update `hashicorp/consul-template` to v0.25.2 to fix panic reading Vault secrets [[GH-10892](https://github.com/hashicorp/nomad/issues/10892)]
* drivers: Fixed bug where Nomad incorrectly reported tasks as recovered successfully even when they were not. [[GH-10849](https://github.com/hashicorp/nomad/issues/10849)]
* scheduler: Fixed a bug where updates to the `datacenters` field were not destructive. [[GH-10864](https://github.com/hashicorp/nomad/issues/10864)]
* volumes: Fix a bug where the HTTP server would crash if a `volume_mount` block was empty [[GH-10855](https://github.com/hashicorp/nomad/issues/10855)]

## 1.0.8 (June 22, 2021)

BUG FIXES:
* artifact: Fixed support for 5 part vhosted-style AWS S3 buckets. [[GH-10778](https://github.com/hashicorp/nomad/issues/10778)]
* artifact: HTTP requests made for artifacts will default to trying HTTP2 first. [[GH-10778](https://github.com/hashicorp/nomad/issues/10778)]
* client/fingerprint/java: Fixed a bug where java fingerprinter would not detect some Java distributions [[GH-10765](https://github.com/hashicorp/nomad/pull/10765)]
* consul: Fixed a bug where consul check parameters missing in group services [[GH-10764](https://github.com/hashicorp/nomad/pull/10764)]
* consul/connect: Fixed a bug where Connect upstreams would not be updated in-place [[GH-10776](https://github.com/hashicorp/nomad/pull/10776)]
* deployments: Fixed a bug where unnecessary goroutines were spawned whenever deployments were updated. [[GH-10756](https://github.com/hashicorp/nomad/issues/10756)]
* quotas (Enterprise): Fixed a bug where quotas were evaluated before constraints, resulting in quota capacity being used up by filtered nodes. [[GH-10753](https://github.com/hashicorp/nomad/issues/10753)]
* quotas (Enterprise): Fixed a bug where stopped allocations for a failed deployment can be double-credited to quota limits, resulting in a quota limit bypass. [[GH-10694](https://github.com/hashicorp/nomad/issues/10694)

## 1.0.7 (June 9, 2021)

BUG FIXES:
* api: Fixed event stream connection initialization when there are no events to send [[GH-10637](https://github.com/hashicorp/nomad/issues/10637)]
* cli: Fixed a bug where `plugin status` did not validate the passed `type` flag correctly [[GH-10712](https://github.com/hashicorp/nomad/pull/10712)]
* cli: Fixed a bug where `alloc exec` may fail with "unexpected EOF" without returning the exit code after a command [[GH-10657](https://github.com/hashicorp/nomad/issues/10657)]
* client: Fixed a bug where `alloc exec` sessions may terminate abruptly after a few minutes [[GH-10710](https://github.com/hashicorp/nomad/issues/10710)]
* drivers/exec: Fixed a bug where `exec` and `java` tasks inherit the Nomad agent's `oom_score_adj` value [[GH-10698](https://github.com/hashicorp/nomad/issues/10698)]
* ui: Fixed a bug where exec would not work across regions. [[GH-10539](https://github.com/hashicorp/nomad/issues/10539)]
* ui: Fixed global-search shortcut for non-english keyboards. [[GH-10714](https://github.com/hashicorp/nomad/issues/10714)]

## 1.0.6 (May 18, 2021)

BUG FIXES:
 * core (Enterprise): Update licensing library to v0.0.11 to include race condition fix. [[GH-10253](https://github.com/hashicorp/nomad/issues/10253)]
 * agent: Only allow querying Prometheus formatted metrics if Prometheus is enabled within the config [[GH-10140](https://github.com/hashicorp/nomad/pull/10140)]
 * api: Ensured that `api.LicenseGet` returned response meta data [[GH-10276](https://github.com/hashicorp/nomad/issues/10276)]
 * api: Added missing devices block to AllocatedTaskResources [[GH-10064](https://github.com/hashicorp/nomad/pull/10064)]
 * api: Fixed a panic that may occur on concurrent access to an SDK client [[GH-10302](https://github.com/hashicorp/nomad/issues/10302)]
 * cli: Fixed a bug where non-int proxy port would panic CLI [[GH-10072](https://github.com/hashicorp/nomad/issues/10072)]
 * cli: Fixed a bug where `snapshot agent` command panics on launch [[GH-10276](https://github.com/hashicorp/nomad/issues/10276)]
 * cli: Remove extra linefeeds in monitor.log files written by `nomad operator debug`. [[GH-10252](https://github.com/hashicorp/nomad/issues/10252)]
 * cli: Fixed a bug where parsing HCLv2 may panic on some variable interpolation syntax [[GH-10326](https://github.com/hashicorp/nomad/issues/10326)] [[GH-10419](https://github.com/hashicorp/nomad/issues/10419)]
 * cli: Fixed a bug where `nomad operator debug` incorrectly parsed https Consul API URLs. [[GH-10082](https://github.com/hashicorp/nomad/pull/10082)]
 * cli: Fixed a panic where `nomad job run` or `plan` would crash when supplied with non-existent `-var-file` files. [[GH-10569](https://github.com/hashicorp/nomad/issues/10569)]
 * client: Fixed log formatting when killing tasks. [[GH-10135](https://github.com/hashicorp/nomad/issues/10135)]
 * client: Added handling for cgroup-v2 memory metrics [[GH-10286](https://github.com/hashicorp/nomad/issues/10286)]
 * client: Only publish measured allocation memory metrics [[GH-10376](https://github.com/hashicorp/nomad/issues/10376)]
 * client: Fixed a bug where small files would be assigned the wrong content type. [[GH-10348](https://github.com/hashicorp/nomad/pull/10348)]
 * consul/connect: Fixed a bug where job plan always different when using expose checks. [[GH-10492](https://github.com/hashicorp/nomad/pull/10492)]
 * consul/connect: Fixed a bug where HTTP ingress gateways could not use wildcard names. [[GH-10457](https://github.com/hashicorp/nomad/pull/10457)]
 * cni: Fallback to an interface with an IP address if sandbox interface lacks one. [[GH-9895](https://github.com/hashicorp/nomad/issues/9895)]
 * csi: Fixed a bug where volume with IDs that are a substring prefix of another volume could use the wrong volume for feasibility checking. [[GH-10158](https://github.com/hashicorp/nomad/issues/10158)]
 * drivers/docker: Fixed a bug where Dockerfile `STOPSIGNAL` was not honored. [[GH-10441](https://github.com/hashicorp/nomad/issues/10441)]
 * drivers/raw_exec: Fixed a bug where exit codes could be dropped and return a spurious error. [[GH-10494](https://github.com/hashicorp/nomad/issues/10494)]
 * scheduler: Fixed a bug where Nomad reports negative or incorrect running children counts for periodic jobs. [[GH-10145](https://github.com/hashicorp/nomad/issues/10145)]
 * scheduler: Fixed a bug where jobs requesting multiple CSI volumes could be incorrectly scheduled if only one of the volumes passed feasibility checking. [[GH-10143](https://github.com/hashicorp/nomad/issues/10143)]
 * service: Fixed a bug where new script checks would not be added on job updates. [[GH-10403](https://github.com/hashicorp/nomad/issues/10403)]
 * server: Fixed a bug affecting periodic job summary counts [[GH-10145](https://github.com/hashicorp/nomad/issues/10145)]
 * server: Fixed a bug where draining a node may fail to migrate its allocations [[GH-10411](https://github.com/hashicorp/nomad/issues/10411)]
 * server: Fixed a bug where jobs may not run if submitted with ParentID field set [[GH-10424](https://github.com/hashicorp/nomad/issues/10424)]
 * server: Fixed a panic that may arise on submission of jobs containing invalid service checks [[GH-10154](https://github.com/hashicorp/nomad/issues/10154)]
 * ui: Fixed the rendering of interstitial components shown after processing a dynamic application sizing recommendation. [[GH-10094](https://github.com/hashicorp/nomad/pull/10094)]

## 1.0.5 (May 11, 2021)

SECURITY:
 * drivers/docker+exec+java: Disable `CAP_NET_RAW` linux capability by default to prevent ARP spoofing. CVE-2021-32575 [[GH-10568](https://github.com/hashicorp/nomad/issues/10568)](https://github.com/hashicorp/nomad/issues/10568)

## 1.0.4 (February 24, 2021)

FEATURES:
 * **Terminating Gateways**: Adds built-in support for running Consul Connect terminating gateways [[GH-9829](https://github.com/hashicorp/nomad/pull/9829)]

IMPROVEMENTS:
 * api: Added OSS handling for license request to stop spurious errors from appearing in the logs [[GH-9963](https://github.com/hashicorp/nomad/pull/9963)]
 * agent: Removed leading whitespace from JSON-formatted log output. [[GH-9795](https://github.com/hashicorp/nomad/issues/9795)]
 * cli: Added optional `-task <task-name>` flag to `alloc logs` to match `alloc exec` [[GH-10026](https://github.com/hashicorp/nomad/issues/10026)]
 * cli: Improved `scaling policy` commands with -verbose, auto-completion, and prefix-matching [[GH-9964](https://github.com/hashicorp/nomad/issues/9964)]
 * consul/connect: Enable custom sidecar tasks to use connect expose checks [[GH-9995](https://github.com/hashicorp/nomad/pull/9995)]
 * consul/connect: Added validation to prevent `connect` blocks from being added to task services. [[GH-9817](https://github.com/hashicorp/nomad/issues/9817)]
 * consul/connect: Made handling of sidecar task container image URLs consistent with the `docker` task driver. [[GH-9580](https://github.com/hashicorp/nomad/issues/9580)]
 * drivers/exec+java: Added client plugin and task configuration options to re-enable previous PID/IPC namespace behavior [[GH-9982](https://github.com/hashicorp/nomad/pull/9982)] [[GH-9990](https://github.com/hashicorp/nomad/pull/9990)]
 * ui: Added button to fail running deployments [[GH-9831](https://github.com/hashicorp/nomad/pull/9831)]
 * ui: Reduced bundle size by removing support for IE 11 [[GH-9578](https://github.com/hashicorp/nomad/pull/9578)]

BUG FIXES:
 * cli: Fixed a bug where some fields in `dynamic` blocks were not interpolated. [[GH-9921](https://github.com/hashicorp/nomad/issues/9921)]
 * cli: Fixed a bug where unset HCL2 variables would panic the CLI if the type was also not set. [[GH-10045](https://github.com/hashicorp/nomad/issues/10045)]
 * consul: Fixed a bug where failing tasks with group services would only cause the allocation to restart once instead of respecting the `restart` field. [[GH-9869](https://github.com/hashicorp/nomad/issues/9869)]
 * consul/connect: Fixed a bug where gateway proxy connection default timeout not set [[GH-9851](https://github.com/hashicorp/nomad/pull/9851)]
 * consul/connect: Fixed a bug preventing more than one connect gateway per Nomad client [[GH-9849](https://github.com/hashicorp/nomad/pull/9849)]
 * consul/connect: Fixed a bug where connect sidecar services would be re-registered unnecessarily. [[GH-10059](https://github.com/hashicorp/nomad/pull/10059)]
 * consul/connect: Fixed a bug where the sidecar health checks would fail if `host_network` was defined. [[GH-9975](https://github.com/hashicorp/nomad/issues/9975)]
 * consul/connect: Fixed a bug where tasks with connect services might be updated when no update necessary. [[GH-10077](https://github.com/hashicorp/nomad/issues/10077)]
 * deployments: Fixed a bug where deployments with multiple task groups and manual promotion would fail if promoted after the progress deadline. [[GH-10042](https://github.com/hashicorp/nomad/issues/10042)]
 * drivers/docker: Fixed a bug preventing multiple ports to be mapped to the same container port [[GH-9951](https://github.com/hashicorp/nomad/issues/9951)]
 * driver/qemu: Fixed a bug where network namespaces were not supported for QEMU workloads [[GH-9861](https://github.com/hashicorp/nomad/pull/9861)]
 * nomad/structs: Fixed a bug where static ports with the same value but different `host_network` were invalid [[GH-9946](https://github.com/hashicorp/nomad/issues/9946)]
 * scheduler: Fixed a bug where shared ports were not persisted during inplace updates for service jobs. [[GH-9830](https://github.com/hashicorp/nomad/issues/9830)]
 * scheduler: Fixed a bug where job statuses and summaries where duplicated and miscalculated when registering a job. [[GH-9768](https://github.com/hashicorp/nomad/issues/9768)]
 * scheduler: Fixed a bug that caused the scheduler not to detect changes for `host_network` port field. [[GH-9937](https://github.com/hashicorp/nomad/issues/9937)]
 * scheduler (Enterprise): Fixed a bug where the deprecated network `mbits` field was being considered as part of quota enforcement. [[GH-9920](https://github.com/hashicorp/nomad/issues/9920)]
 * ui: Fixed exec command escaping of emoji in task names [[GH-7813](https://github.com/hashicorp/nomad/pull/7813)]
 * ui: Consistently use the correct MHz shorthand throughout the UI [[GH-9896](https://github.com/hashicorp/nomad/issues/9896)]
 * ui: Fixed inconsistent namespace casing in the namespace selector [[GH-9876](https://github.com/hashicorp/nomad/issues/9876)]
 * ui: Always draw allocation associations if the alloc count is less than 10 [[GH-9769](https://github.com/hashicorp/nomad/issues/9769)]
 * ui: Fixed incorrect text alignment in the topology visualization in Firefox [[GH-9894](https://github.com/hashicorp/nomad/issues/9894)]
 * ui: Fixed node composite status so being down takes priority over being ineligible [[GH-9927](https://github.com/hashicorp/nomad/pull/9927)]
 * ui: Don't count reservations of terminal allocations in the topology visualization [[GH-9886](https://github.com/hashicorp/nomad/issues/9886)]
 * ui: Use server-sent error messages when applicable (e.g., when a task can't be stopped) [[GH-9909](https://github.com/hashicorp/nomad/issues/9909)]
 * ui: Send the region query param when making cross-region client/server monitor requests [[GH-9913](https://github.com/hashicorp/nomad/issues/9913)]
 * ui: Fixed a bug where namespaces were not being included when opening exec windows from allocations and tasks [[GH-9968](https://github.com/hashicorp/nomad/pull/9968)]
 * ui: Don't draw allocation associations in the topology visualization on window resize when the associations aren't supposed to be shown [[GH-9769](https://github.com/hashicorp/nomad/issues/9769)]
 * volumes: Fixed a bug where volume diffs were not displayed in the output of `nomad plan`. [[GH-9973](https://github.com/hashicorp/nomad/issues/9973)]

## 1.0.3 (January 28, 2021)

SECURITY:
 * drivers/exec+java: Modified exec-based drivers to run tasks in private PID/IPC namespaces. CVE-2021-3283 [[GH-9911](https://github.com/hashicorp/nomad/issues/9911)]

## 1.0.2 (January 14, 2021)

IMPROVEMENTS:
 * artifact: Added support for virtual host style AWS S3 paths. [[GH-9050](https://github.com/hashicorp/nomad/issues/9050)]
 * build: Updated to Go 1.15.6. [[GH-9686](https://github.com/hashicorp/nomad/issues/9686)]
 * client: Improve support for AWS Graviton instances [[GH-7989](https://github.com/hashicorp/nomad/issues/7989)]
 * consul/connect: Interpolate the connect, service meta, and service canary meta blocks with the task environment [[GH-9586](https://github.com/hashicorp/nomad/pull/9586)]
 * consul/connect: enable configuring custom gateway task [[GH-9639](https://github.com/hashicorp/nomad/pull/9639)]
 * cli: Added JSON/go template formatting to agent-info command. [[GH-9788](https://github.com/hashicorp/nomad/pull/9788)]


BUG FIXES:
 * client: Fixed a bug where non-`docker` tasks with network isolation were restarted on client restart. [[GH-9757](https://github.com/hashicorp/nomad/issues/9757)]
 * client: Fixed a bug where clients configured with `cpu_total_compute` did not update the `cpu.totalcompute` node attribute. [[GH-9532](https://github.com/hashicorp/nomad/issues/9532)]
 * client: Fixed an fingerprinter issue detecting bridge kernel module on RHEL [[GH-9776](https://github.com/hashicorp/nomad/issues/9776)]
 * core: Fixed a bug where an in place update dropped an allocations shared allocated resources [[GH-9736](https://github.com/hashicorp/nomad/issues/9736)]
 * consul: Fixed a bug where updating a task to include services would not work [[GH-9707](https://github.com/hashicorp/nomad/issues/9707)]
 * consul: Fixed alloc address mode port advertisement to use the mapped `to` port value [[GH-9730](https://github.com/hashicorp/nomad/issues/9730)]
 * consul/connect: Fixed a bug where absent ingress envoy proxy configuration could panic client [[GH-9669](https://github.com/hashicorp/nomad/issues/9669)]
 * consul/connect: Fixed a bug where in-place upgrade of Nomad client running Connect enabled jobs would panic [[GH-9738](https://github.com/hashicorp/nomad/issues/9738)]
 * lifecycle: Fixed a bug where poststop breaks deployments with consul service checks [[GH-9361](https://github.com/hashicorp/nomad/issues/9361)]
 * template: Fixed multiple issues in template src/dest and artifact dest interpolation [[GH-9671](https://github.com/hashicorp/nomad/issues/9671)]
 * template: Fixed a bug where dynamic secrets did not trigger the template `change_mode` after a client restart. [[GH-9636](https://github.com/hashicorp/nomad/issues/9636)]
 * scaling: Fixed a bug where job scaling endpoint did not enforce scaling policy min/max [[GH-9761](https://github.com/hashicorp/nomad/issues/9761)]
 * server: Fixed a bug where new servers may bootstrap prematurely when configured with `bootstrap_expect = 0` [[GH-9672](https://github.com/hashicorp/nomad/issues/9672)]
 * ui: The topology visualization will now render a subset of nodes instead of nothing when some nodes are running nomad <0.9.0 [[GH-9733](https://github.com/hashicorp/nomad/issues/9733)]

## 1.0.1 (December 16, 2020)

IMPROVEMENTS:
 * drivers/docker: Added a new syntax for specifying `mount` [[GH-9635](https://github.com/hashicorp/nomad/issues/9635)]

BUG FIXES:
 * core: Fixed a bug where ACLToken and ACLPolicy changes were ignored by the event stream [[GH-9595](https://github.com/hashicorp/nomad/issues/9595)]
 * core: Fixed a bug to honor HCL2 variables set by environment variables or variable files [[GH-9592](https://github.com/hashicorp/nomad/issues/9592)] [[GH-9623](https://github.com/hashicorp/nomad/issues/9623)]
 * cli: Fixed a bug in the node count for the `nomad operator debug` command. [[GH-9625](https://github.com/hashicorp/nomad/pull/9625)]
 * cni: Fixed a bug where plugins that do not set the interface sandbox value could crash the Nomad client. [[GH-9648](https://github.com/hashicorp/nomad/issues/9648)]
 * consul/connect: Fixed a bug where client meta.connect.sidecar_image configuration was ignored [[GH-9624](https://github.com/hashicorp/nomad/pull/9624)]
 * consul/connect: Fixed a bug where client meta.connect.proxy_concurrency was not applied to connect gateways [[GH-9611](https://github.com/hashicorp/nomad/pull/9611)]

## 1.0.0 (December 8, 2020)

FEATURES:

* **Event Stream**: Subscribe to change events as they occur in real time. [[GH-9013](https://github.com/hashicorp/nomad/issues/9013)]
* **Namespaces OSS**: Namespaces are now available in open source Nomad. [[GH-9135](https://github.com/hashicorp/nomad/issues/9135)]
* **Topology Visualization**: See all of the clients and allocations in a cluster at once. [[GH-9077](https://github.com/hashicorp/nomad/issues/9077)]
* **HCL 2**: Job files can contain variables, expressions, and advanced templating.
* **PostStop**: Tasks can now run after all other tasks have finished [[GH-8194](https://github.com/hashicorp/nomad/pull/8194)]

IMPROVEMENTS:
 * core: Improved job deregistration error logging. [[GH-8745](https://github.com/hashicorp/nomad/issues/8745)]
 * acl: Allow operators with `namespace:dispatch-job` capability to force periodic job invocation [[GH-9205](https://github.com/hashicorp/nomad/issues/9205)]
 * api: Added support for cancellation contexts to HTTP API. [[GH-8836](https://github.com/hashicorp/nomad/issues/8836)]
 * api: Job Register API now permits non-zero initial Version to accommodate multi-region deployments. [[GH-9071](https://github.com/hashicorp/nomad/issues/9071)]
 * api: Added ?resources=true query parameter to /v1/nodes and /v1/allocations to include resource allocations in listings. [[GH-9055](https://github.com/hashicorp/nomad/issues/9055)]
 * api: Added ?task_states=false query parameter to /v1/allocations to remove TaskStates from listings. Defaults to being included as before. [[GH-9055](https://github.com/hashicorp/nomad/issues/9055)]
 * build: Updated to Go 1.15.5. [[GH-9345](https://github.com/hashicorp/nomad/issues/9345)]
 * cli: Added autocompletion for `recommendation` commands [[GH-9317](https://github.com/hashicorp/nomad/issues/9317)]
 * cli: Added client node filtering arguments to `nomad operator debug` command. [[GH-9331](https://github.com/hashicorp/nomad/pull/9331)]
 * cli: Added goroutine debug pprof output and server-id=all to `nomad operator debug` capture. [[GH-9067](https://github.com/hashicorp/nomad/pull/9067)]
 * cli: Added metrics to `nomad operator debug` capture. [[GH-9034](https://github.com/hashicorp/nomad/pull/9034)]
 * cli: Added pprof duration and CSI details to `nomad operator debug` capture. [[GH-9346](https://github.com/hashicorp/nomad/pull/9346)]
 * cli: Added `scale` and `scaling-events` subcommands to the `job` command. [[GH-9023](https://github.com/hashicorp/nomad/pull/9023)]
 * cli: Added `scaling` command for interaction with the scaling API endpoint. [[GH-9025](https://github.com/hashicorp/nomad/pull/9025)]
 * client: Use ec2 CPU perf data from AWS API [[GH-7830](https://github.com/hashicorp/nomad/issues/7830)]
 * client: Added support for Azure fingerprinting. [[GH-8979](https://github.com/hashicorp/nomad/issues/8979)]
 * client: Batch state store writes to reduce disk IO. [[GH-9093](https://github.com/hashicorp/nomad/issues/9093)]
 * client: Reduce rate of sending allocation updates when servers are slow. [[GH-9435](https://github.com/hashicorp/nomad/issues/9435)]
 * client: Added support for fingerprinting the client node's Consul segment. [[GH-7214](https://github.com/hashicorp/nomad/issues/7214)]
 * client: Added `NOMAD_JOB_ID` and `NOMAD_PARENT_JOB_ID` environment variables to those made available to jobs. [[GH-8967](https://github.com/hashicorp/nomad/issues/8967)]
 * client: Updated consul-template to v0.25.1 - config `function_blacklist` deprecated and replaced with `function_denylist` [[GH-8988](https://github.com/hashicorp/nomad/pull/8988)]
 * config: Deprecated terms `blacklist` and `whitelist` from configuration and replaced them with `denylist` and `allowlist`. [[GH-9019](https://github.com/hashicorp/nomad/issues/9019)]
 * consul: Support advertising CNI and multi-host network addresses to consul [[GH-8801](https://github.com/hashicorp/nomad/issues/8801)]
 * consul: Support Consul namespace (Consul Enterprise) in client configuration. [[GH-8849](https://github.com/hashicorp/nomad/pull/8849)]
 * consul/connect: Dynamically select envoy sidecar at runtime [[GH-8945](https://github.com/hashicorp/nomad/pull/8945)]
 * consul/connect: Enable setting `datacenter` field on connect upstreams [[GH-8964](https://github.com/hashicorp/nomad/issues/8964)]
 * consul/connect: Envoy concurrency now defaults to 1 rather than number of cores [[GH-9341](https://github.com/hashicorp/nomad/issues/9341)]
 * csi: Support `nomad volume detach` with previously garbage-collected nodes. [[GH-9057](https://github.com/hashicorp/nomad/issues/9057)]
 * csi: Relaxed validation requirements when checking volume capabilities with controller plugins, to accommodate existing plugin behaviors. [[GH-9049](https://github.com/hashicorp/nomad/issues/9049)]
 * driver/docker: Upgrade pause container and detect architecture [[GH-8957](https://github.com/hashicorp/nomad/pull/8957)]
 * driver/docker: Support pinning tasks to specific CPUs with `cpuset_cpus` option. [[GH-8291](https://github.com/hashicorp/nomad/pull/8291)]
 * driver/raw_exec: Honor the task user setting when a user runs `nomad alloc exec` [[GH-9439](https://github.com/hashicorp/nomad/pull/9439)]
 * jobspec: Lowered minimum CPU allowed from 20 to 1. [[GH-8996](https://github.com/hashicorp/nomad/issues/8996)]
 * jobspec: Added support for `headers` option in `artifact` stanza [[GH-9306](https://github.com/hashicorp/nomad/issues/9306)]

__BACKWARDS INCOMPATIBILITIES:__
 * core: null characters are prohibited in region, datacenter, job name/ID, task group name, and task name [[GH-9020](https://github.com/hashicorp/nomad/issues/9020)]
 * csi: registering a CSI volume with a `block-device` attachment mode and `mount_options` now returns a validation error, instead of silently dropping the `mount_options`. [[GH-9044](https://github.com/hashicorp/nomad/issues/9044)]
 * driver/docker: Tasks are now issued SIGTERM instead of SIGINT when stopping [[GH-8932](https://github.com/hashicorp/nomad/issues/8932)]
 * telemetry: removed backwards compatible/untagged metrics deprecated in 0.7  [[GH-9080](https://github.com/hashicorp/nomad/issues/9080)]

BUG FIXES:

 * agent (Enterprise): Fixed a bug where audit logging caused websocket and streaming http endpoints to fail [[GH-9319](https://github.com/hashicorp/nomad/issues/9319)]
 * core: Fixed a bug where ACL handling prevented cross-namespace allocation listing [[GH-9278](https://github.com/hashicorp/nomad/issues/9278)]
 * core: Fixed a bug where AllocatedResources contained increasingly duplicated ports [[GH-9368](https://github.com/hashicorp/nomad/issues/9368)]
 * core: Fixed a bug where group level network ports not usable by task resource network stanza [[GH-8780](https://github.com/hashicorp/nomad/issues/8780)]
 * core: Fixed a bug where scaling policy filtering would ignore type query if job query was present [[GH-9312](https://github.com/hashicorp/nomad/issues/9312)]
 * core: Fixed a bug where a request to scale a job would fail if the job was not in the default namespace. [[GH-9296](https://github.com/hashicorp/nomad/pull/9296)]
 * core: Fixed a bug where blocking queries would not include the query's maximum wait time when calculating whether it was safe to retry. [[GH-8921](https://github.com/hashicorp/nomad/issues/8921)]
 * config (Enterprise): Fixed default enterprise config merging. [[GH-9083](https://github.com/hashicorp/nomad/pull/9083)]
 * client: Fixed an fingerprinter issue detecting bridge kernel module [[GH-9299](https://github.com/hashicorp/nomad/pull/9299)]
 * client: Fixed an issue with the Java fingerprinter on macOS causing pop-up notifications when no JVM installed. [[GH-9225](https://github.com/hashicorp/nomad/pull/9225)]
 * client: Fixed an issue in processing device plugin fingerprints which would temporarily hang nomad if no devices were found [[GH-9311](https://github.com/hashicorp/nomad/issues/9311)]
 * client: Fixed an in-place upgrade bug, where a Nomad client may fail to manage tasks that were started with pre-0.9 Nomad client. [[GH-9304](https://github.com/hashicorp/nomad/pull/9304)]
 * consul: Fixed a bug where canary_meta was not being interpolated with environment variables [[GH-9096](https://github.com/hashicorp/nomad/pull/9096)]
 * consul: Fixed a bug to correctly validate task when using script-checks in group-level services [[GH-8952](https://github.com/hashicorp/nomad/issues/8952)]
 * consul: Fixed a bug that caused connect sidecars to be re-registered in Consul every 30 seconds [[GH-9330](https://github.com/hashicorp/nomad/pull/9330)]
 * consul/connect: Fixed a bug to correctly trigger updates on jobspec changes [[GH-9029](https://github.com/hashicorp/nomad/pull/9029)]
 * csi: Fixed a bug where multi-writer volumes were allowed only 1 write claim. [[GH-9040](https://github.com/hashicorp/nomad/issues/9040)]
 * csi: Fixed a bug where garbage collection of plugins could prevent volume claim release. [[GH-9141](https://github.com/hashicorp/nomad/issues/9141)]
 * csi: Fixed a bug where concurrent updates to volumes could result in inconsistent state. [[GH-9239](https://github.com/hashicorp/nomad/issues/9239)]
 * csi: Fixed a bug where `nomad volume detach` would not accept prefixes for the node ID parameter. [[GH-9041](https://github.com/hashicorp/nomad/issues/9041)]
 * csi: Fixed a bug where `nomad alloc status -verbose` would display an error when querying volumes. [[GH-9354](https://github.com/hashicorp/nomad/issues/9354)]
 * csi: Fixed a bug where queries for CSI plugins could be interleaved, resulting in inconsistent counts of plugins. [[GH-9438](https://github.com/hashicorp/nomad/issues/9438)]
 * driver/docker: Fixed a bug where the Docker daemon could block longer than the `kill_timeout`. [[GH-9502](https://github.com/hashicorp/nomad/issues/9502)
 * driver/docker: Fixed a bug where the default `image_delay` configuration was ignored if the `gc` configuration was not set. [[GH-9101](https://github.com/hashicorp/nomad/issues/9101)]
 * driver/raw_exec: Fixed a bug where raw_exec attempts to create a freezer cgroups for the tasks even when `no_cgroups` is set. [[GH-9328](https://github.com/hashicorp/nomad/issues/9328)]
 * scheduler: Fixed a bug where where system jobs would bind on all interfaces instead of the specified `host_network`. [[GH-8822](https://github.com/hashicorp/nomad/issues/8822)]
 * ui: Fixed a bug in the volume list page where allocation counts were not displayed. [[GH-9495](https://github.com/hashicorp/nomad/issues/9495)]
 * ui: Fixed a bug in the volume status page where read allocations and write allocations were not displayed. [[GH-9377](https://github.com/hashicorp/nomad/issues/9377)]
 * ui: Fixed a bug in the CSI volume and plugin status pages where plugins that don't require controllers were shown as unhealthy. [[GH-9416](https://github.com/hashicorp/nomad/issues/9416)]

## 0.12.12 (May 11, 2021)

SECURITY:
 * drivers/docker+exec+java: Disable `CAP_NET_RAW` linux capability by default to prevent ARP spoofing. CVE-2021-32575 [[GH-10568](https://github.com/hashicorp/nomad/issues/10568)](https://github.com/hashicorp/nomad/issues/10568)

## 0.12.11 (March 18, 2021)

BUG FIXES:
 * server: _Backport from v1.0.2_ - Fixed a bug where new servers may bootstrap prematurely when configured with `bootstrap_expect = 0` [[GH-9672](https://github.com/hashicorp/nomad/issues/9672)]

## 0.12.10 (January 28, 2021)

SECURITY:
 * drivers/exec+java: Modified exec-based drivers to run tasks in private PID/IPC namespaces. CVE-2021-3283 [[GH-9911](https://github.com/hashicorp/nomad/issues/9911)]

## 0.12.9 (November 18, 2020)

BUG FIXES:
 * client: Fixed a regression where `NOMAD_{ALLOC,TASK,SECRETS}_DIR` variables would cause an error when interpolated into `template.source` stanzas. [[GH-9391](https://github.com/hashicorp/nomad/issues/9391)]

## 0.12.8 (November 10, 2020)

SECURITY:
 * docker: Fixed a bug where the `docker.volumes.enabled` configuration was not set to the default `false` if left unset. CVE-2020-28348 [[GH-9303](https://github.com/hashicorp/nomad/issues/9303)]
 * docker: Fixed a bug where Docker driver mounts of type "volume" (but not "bind") were not sandboxed when `docker.volumes.enabled` is set to `false`. The `docker.volumes.enabled` configuration will now disable Docker mounts with type "volume" when set to `false`. CVE-2020-28348 [[GH-9303](https://github.com/hashicorp/nomad/issues/9303)]

BUG FIXES:
 * client: Fixed an in-place upgrade bug, where a Nomad client may fail to manage tasks that were started with pre-0.9 Nomad client. [[GH-9304](https://github.com/hashicorp/nomad/pull/9304)]

## 0.12.7 (October 23, 2020)

BUG FIXES:
 * artifact: Fixed a regression in 0.12.6 where if the artifact `destination` field is an absolute path it is not appended to the task working directory, breaking the use of `NOMAD_SECRETS_DIR` as part of the destination path. [[GH-9148](https://github.com/hashicorp/nomad/issues/9148)]
 * template: Fixed a regression in 0.12.6 where if the template `destination` field is an absolute path it is not appended to the task working directory, breaking the use of `NOMAD_SECRETS_DIR` as part of the destination path. [[GH-9148](https://github.com/hashicorp/nomad/issues/9148)]

## 0.12.6 (October 21, 2020)

SECURITY:

 * artifact: Fixed a bug where interpolation can be used in the artifact `destination` field to write artifact payloads outside the allocation directory. CVE-2020-27195 [[GH-9129](https://github.com/hashicorp/nomad/issues/9129)]
 * template: Fixed a bug where interpolation can be used in the template `source` and `destination` fields to read or write files outside the allocation directory even when `disable_file_sandbox` was set to `false` (the default). CVE-2020-27195 [[GH-9129](https://github.com/hashicorp/nomad/issues/9129)]
 * template: Fixed a bug where the `disable_file_sandbox` configuration was only respected for the template `file` function and not the template `source` and `destination` fields. CVE-2020-27195 [[GH-9129](https://github.com/hashicorp/nomad/issues/9129)]

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
 * docker: Fixed a bug where configuring DNS options in `bridge` or `cni` mode would prevent the container from being created. [[GH-8600](https://github.com/hashicorp/nomad/issues/8600)]
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
 * driver/docker: The Docker driver's `port_map` configuration is deprecated in lieu of the `ports` field.
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
* networking: Omitting the `port.to` field defaults to mapping to the same port value as the dynamically assigned port. [[GH-8208](https://github.com/hashicorp/nomad/issues/8208)]
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

## 0.11.8 (November 19, 2020)

BUG FIXES:
 * client: _Backport from v0.12.9_ - Fixed a regression where `NOMAD_{ALLOC,TASK,SECRETS}_DIR` variables would cause an error when interpolated into `template.source` stanzas. [[GH-9402](https://github.com/hashicorp/nomad/issues/9402)]

## 0.11.7 (November 10, 2020)

SECURITY:
 * docker: _Backport from v0.12.8_ - Fixed a bug where Docker driver mounts of type "volume" (but not "bind") were not sandboxed when `docker.volumes.enabled` is set to `false`. The `docker.volumes.enabled` configuration will now disable Docker mounts with type "volume" when set to `false`. CVE-2020-28348 [[GH-9303](https://github.com/hashicorp/nomad/issues/9303)]

BUG FIXES:
 * client: _Backport from v0.12.8_ - Fixed an in-place upgrade bug, where a Nomad client may fail to manage tasks that were started with pre-0.9 Nomad client. [[GH-9304](https://github.com/hashicorp/nomad/pull/9304)]

## 0.11.6 (October 23, 2020)

BUG FIXES:
 * artifact: _Backport from v0.12.7_ - Fixed a regression in 0.11.5 where if the artifact `destination` field is an absolute path it is not appended to the task working directory, breaking the use of `NOMAD_SECRETS_DIR` as part of the destination path. [[GH-9148](https://github.com/hashicorp/nomad/issues/9148)]
 * template: _Backport from v0.12.7_ - Fixed a regression in 0.11.5 where if the template `destination` field is an absolute path it is not appended to the task working directory, breaking the use of `NOMAD_SECRETS_DIR` as part of the destination path. [[GH-9148](https://github.com/hashicorp/nomad/issues/9148)]

## 0.11.5 (October 21, 2020)

SECURITY:

 * artifact: _Backport from v0.12.6_ - Fixed a bug where interpolation can be used in the artifact `destination` field to write artifact payloads outside the allocation directory. CVE-2020-27195 [[GH-9129](https://github.com/hashicorp/nomad/issues/9129)]
 * template: _Backport from v0.12.6_ - Fixed a bug where interpolation can be used in the template `source` and `destination` fields to read or write files outside the allocation directory even when `disable_file_sandbox` was set to `false` (the default). CVE-2020-27195 [[GH-9129](https://github.com/hashicorp/nomad/issues/9129)]
 * template: _Backport from v0.12.6_ - Fixed a bug where the `disable_file_sandbox` configuration was only respected for the template `file` function and not the template `source` and `destination` fields. CVE-2020-27195 [[GH-9129](https://github.com/hashicorp/nomad/issues/9129)]

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
 * client: Updated consul-template library to v0.24.1 - added support for working with consul connect. [Deprecated vault_grace](https://developer.hashicorp.com/nomad/guides/upgrade/upgrade-specific/#nomad-0110) [[GH-7170](https://github.com/hashicorp/nomad/pull/7170)]
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

## 0.10.9 (November 19, 2020)

BUG FIXES:
 * client: _Backport from v0.12.9_ - Fixed a regression where `NOMAD_{ALLOC,TASK,SECRETS}_DIR` variables would cause an error when interpolated into `template.source` stanzas. [[GH-9405](https://github.com/hashicorp/nomad/issues/9405)]

## 0.10.8 (November 10, 2020)

SECURITY:
 * docker: _Backport from v0.12.8_ - Fixed a bug where Docker driver mounts of type "volume" (but not "bind") were not sandboxed when `docker.volumes.enabled` is set to `false`. The `docker.volumes.enabled` configuration will now disable Docker mounts with type "volume" when set to `false`. CVE-2020-28348 [[GH-9303](https://github.com/hashicorp/nomad/issues/9303)]

BUG FIXES:
 * client: _Backport from v0.12.8_ - Fixed an in-place upgrade bug, where a Nomad client may fail to manage tasks that were started with pre-0.9 Nomad client. [[GH-9304](https://github.com/hashicorp/nomad/pull/9304)]

## 0.10.7 (October 23, 2020)

BUG FIXES:
 * artifact: _Backport from v0.12.7_ - Fixed a regression in 0.10.6 where if the artifact `destination` field is an absolute path it is not appended to the task working directory, breaking the use of `NOMAD_SECRETS_DIR` as part of the destination path. [[GH-9148](https://github.com/hashicorp/nomad/issues/9148)]
 * template: _Backport from v0.12.7_ - Fixed a regression in 0.10.6 where if the template `destination` field is an absolute path it is not appended to the task working directory, breaking the use of `NOMAD_SECRETS_DIR` as part of the destination path. [[GH-9148](https://github.com/hashicorp/nomad/issues/9148)]

## 0.10.6 (October 21, 2020)

SECURITY:

 * artifact: _Backport from v0.12.6_ - Fixed a bug where interpolation can be used in the artifact `destination` field to write artifact payloads outside the allocation directory. CVE-2020-27195 [[GH-9129](https://github.com/hashicorp/nomad/issues/9129)]
 * template: _Backport from v0.12.6_ - Fixed a bug where interpolation can be used in the template `source` and `destination` fields to read or write files outside the allocation directory even when `disable_file_sandbox` was set to `false` (the default). CVE-2020-27195 [[GH-9129](https://github.com/hashicorp/nomad/issues/9129)]
 * template: _Backport from v0.12.6_ - Fixed a bug where the `disable_file_sandbox` configuration was only respected for the template `file` function and not the template `source` and `destination` fields. CVE-2020-27195 [[GH-9129](https://github.com/hashicorp/nomad/issues/9129)]

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
   https://developer.hashicorp.com/nomad/docs/commands/monitor.html
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
 * api: The [job deployments](https://developer.hashicorp.com/nomad/api/jobs.html#list-job-deployments) endpoint
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
 * code: Add `nomad alloc exec` command for debugging and running commands in an alloc [[GH-5632](https://github.com/hashicorp/nomad/pull/5632)]
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
 * core: Added preemption support for system jobs. System jobs can now preempt other jobs of lower priority. See [preemption](https://developer.hashicorp.com/nomad/docs/internals/scheduling/preemption.html) for more details. [[GH-4794](https://github.com/hashicorp/nomad/pull/4794)]
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
 * core: Add a new [progress_deadline](https://developer.hashicorp.com/nomad/docs/job-specification/update.html#progress_deadline) parameter to
   support rescheduling failed allocations during a deployment. This allows operators to specify a configurable deadline before which
   a deployment should see healthy allocations [[GH-4259](https://github.com/hashicorp/nomad/issues/4259)]
 * core: Add a new [job eval](https://developer.hashicorp.com/nomad/docs/commands/job/eval.html) CLI and API
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
   [migrate stanza](https://developer.hashicorp.com/nomad/docs/job-specification/migrate.html)
   allows jobs to specify how quickly task groups can be drained. A `-force`
   option can be used to emulate the old drain behavior.
 * jobspec: The default values for restart policy have changed. Restart policy
   mode defaults to "fail" and the attempts/time interval values have been
   changed to enable faster server side rescheduling. See [restart
   stanza](https://developer.hashicorp.com/nomad/docs/job-specification/restart.html) for
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
 * core: A set of features (Autopilot) has been added to allow for automatic operator-friendly management of Nomad servers. For more information about Autopilot, see the [Autopilot Guide](https://developer.hashicorp.com/nomad/guides/cluster/autopilot.html). [[GH-3670](https://github.com/hashicorp/nomad/pull/3670)]
 * core: Failed tasks are automatically rescheduled according to user specified criteria. For more information on configuration, see the [Reshedule Stanza](https://developer.hashicorp.com/nomad/docs/job-specification/reschedule.html) [[GH-3981](https://github.com/hashicorp/nomad/issues/3981)]
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
