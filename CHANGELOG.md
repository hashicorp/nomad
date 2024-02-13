## 1.7.5 (February 13, 2024)

SECURITY:

* windows: Remove `LazyDLL` calls for system modules to harden Nomad against attacks from the host [[GH-19925](https://github.com/hashicorp/nomad/issues/19925)]

IMPROVEMENTS:

* api: emit `JobDeregistered` event when job is deregistered with `purge` [[GH-19903](https://github.com/hashicorp/nomad/issues/19903)]

BUG FIXES:

* cli: Fix return code when `nomad job run` succeeds after a blocked eval [[GH-19876](https://github.com/hashicorp/nomad/issues/19876)]
* cli: Fixed a bug where the `nomad tls ca create` command failed when the `-domain` was used without other values [[GH-19892](https://github.com/hashicorp/nomad/issues/19892)]
* client: Ensure the value for CPU shares are within the allowed range [[GH-19935](https://github.com/hashicorp/nomad/issues/19935)]
* client: Prevent client from starting if cgroup initialization fails [[GH-19915](https://github.com/hashicorp/nomad/issues/19915)]
* connect: Fixed envoy sidecars being unable to restart after node reboots [[GH-19787](https://github.com/hashicorp/nomad/issues/19787)]
* driver/java: Ensure the OOM killed response is populated when the task exits [[GH-19818](https://github.com/hashicorp/nomad/issues/19818)]
* driver/qemu: Ensure the OOM killed response is populated when the task exits [[GH-19830](https://github.com/hashicorp/nomad/issues/19830)]
* driver/rawexec: Ensure the OOM killed response is populated when the task exits [[GH-19829](https://github.com/hashicorp/nomad/issues/19829)]
* exec: Fixed a bug in `alloc exec` where closing websocket streams could cause a panic [[GH-19932](https://github.com/hashicorp/nomad/issues/19932)]
* scheduler: Fixed a bug that caused blocked evaluations due to port conflict to not have a reason explaining why the evaluation was blocked [[GH-19933](https://github.com/hashicorp/nomad/issues/19933)]
* ui: Fix an issue where a same-named task from a different group could be selected when the user clicks Exec from a task group page where multiple allocations would be valid [[GH-19878](https://github.com/hashicorp/nomad/issues/19878)]

## 1.7.4 (February 08, 2024)

SECURITY:

* deps: Updated runc to 1.1.12 to address CVE-2024-21626 [[GH-19851](https://github.com/hashicorp/nomad/issues/19851)]
* migration: Fixed a bug where archives used for migration were not checked for symlinks that escaped the allocation directory [[GH-19887](https://github.com/hashicorp/nomad/issues/19887)]
* template: Fixed a bug where symlinks could force templates to read and write to arbitrary locations (CVE-2024-1329) [[GH-19888](https://github.com/hashicorp/nomad/issues/19888)]

## 1.7.3 (January 15, 2024)

IMPROVEMENTS:

* build: update to go 1.21.6 [[GH-19709](https://github.com/hashicorp/nomad/issues/19709)]
* cgroupslib: Consider CGroups OFF when essential controllers are missing [[GH-19176](https://github.com/hashicorp/nomad/issues/19176)]
* cli: Add new option `nomad setup vault -check` to help cluster operators migrate to workload identities for Vault [[GH-19720](https://github.com/hashicorp/nomad/issues/19720)]
* consul: Add fingerprint for Consul Enterprise admin partitions [[GH-19485](https://github.com/hashicorp/nomad/issues/19485)]
* consul: Added support for Consul Enterprise admin partitions [[GH-19665](https://github.com/hashicorp/nomad/issues/19665)]
* consul: Added support for failures_before_warning and failures_before_critical in Nomad agent services [[GH-19336](https://github.com/hashicorp/nomad/issues/19336)]
* consul: Added support for failures_before_warning in Consul service checks [[GH-19336](https://github.com/hashicorp/nomad/issues/19336)]
* drivers/exec: Added support for OOM detection in exec driver [[GH-19563](https://github.com/hashicorp/nomad/issues/19563)]
* drivers: Enable configuring a raw_exec task to not have an upper memory limit [[GH-19670](https://github.com/hashicorp/nomad/issues/19670)]
* identity: Added vault_role to JWT workload identity claims if specified in jobspec [[GH-19535](https://github.com/hashicorp/nomad/issues/19535)]
* ui: Added group name to allocation tooltips on job status panel [[GH-19601](https://github.com/hashicorp/nomad/issues/19601)]
* ui: Adds a warning message to pages in the Web UI when logs are disabled [[GH-18823](https://github.com/hashicorp/nomad/issues/18823)]
* ui: Hide token secret upon successful login [[GH-19529](https://github.com/hashicorp/nomad/issues/19529)]
* ui: when an Action has long output, anchor to the latest messages [[GH-19452](https://github.com/hashicorp/nomad/issues/19452)]
* vault: Add `allow_token_expiration` field to allow Vault tokens to expire without renewal for short-lived tasks [[GH-19691](https://github.com/hashicorp/nomad/issues/19691)]
* vault: Nomad clients will no longer attempt to renew Vault tokens that cannot be renewed [[GH-19691](https://github.com/hashicorp/nomad/issues/19691)]

BUG FIXES:

* acl: Fixed a bug where 1.5 and 1.6 clients could not access Nomad Variables and Services via templates [[GH-19578](https://github.com/hashicorp/nomad/issues/19578)]
* acl: Fixed auth method hashing which meant changing some fields would be silently ignored [[GH-19677](https://github.com/hashicorp/nomad/issues/19677)]
* auth: Added new optional OIDCDisableUserInfo setting for OIDC auth provider [[GH-19566](https://github.com/hashicorp/nomad/issues/19566)]
* client: Fixed a bug where where the environment variable / file for the Consul token weren't written. [[GH-19490](https://github.com/hashicorp/nomad/issues/19490)]
* consul (Enterprise): Fixed a bug where the group/task Consul cluster was assigned "default" when unset instead of the namespace-governed value
* core: Ensure job HCL submission data is persisted and restored during the FSM snapshot process [[GH-19605](https://github.com/hashicorp/nomad/issues/19605)]
* namespaces: Failed delete calls no longer return success codes [[GH-19483](https://github.com/hashicorp/nomad/issues/19483)]
* rawexec: Fixed a bug where oom_score_adj would be inherited from Nomad client [[GH-19515](https://github.com/hashicorp/nomad/issues/19515)]
* server: Fix panic when validating non-service reschedule block [[GH-19652](https://github.com/hashicorp/nomad/issues/19652)]
* server: Fix server not waiting for workers to submit nacks for dequeued evaluations before shutting down [[GH-19560](https://github.com/hashicorp/nomad/issues/19560)]
* state: Fixed a bug where purged jobs would not get new deployments [[GH-19609](https://github.com/hashicorp/nomad/issues/19609)]
* ui: Fix rendering of allocations table for jobs that don't have actions [[GH-19505](https://github.com/hashicorp/nomad/issues/19505)]
* vault: Fixed a bug that could cause errors during leadership transition when migrating to the new JWT and workload identity authentication workflow [[GH-19689](https://github.com/hashicorp/nomad/issues/19689)]
* vault: Fixed a bug where `allow_unauthenticated` was enforced when a `default_identity` was set [[GH-19585](https://github.com/hashicorp/nomad/issues/19585)]

## 1.7.2 (December 13, 2023)

FEATURES:

* **Reschedule on Lost**: Adds the ability to prevent tasks on down nodes from being rescheduled [[GH-16867](https://github.com/hashicorp/nomad/issues/16867)]

IMPROVEMENTS:

* audit (Enterprise): Added ACL token role links to audit log auth objects [[GH-19415](https://github.com/hashicorp/nomad/issues/19415)]
* ui: Added a new example template with Task Actions [[GH-19153](https://github.com/hashicorp/nomad/issues/19153)]
* ui: dont allow new jobspec download until template is populated, and remove group count from jobs index [[GH-19377](https://github.com/hashicorp/nomad/issues/19377)]
* ui: make the exec window look nicer on mobile screens [[GH-19332](https://github.com/hashicorp/nomad/issues/19332)]

BUG FIXES:

* auth: Fixed a bug where `tls.verify_server_hostname=false` was not respected, leading to authentication failures between Nomad agents [[GH-19425](https://github.com/hashicorp/nomad/issues/19425)]
* cli: Fix a bug in the `var put` command which prevented combining items as CLI arguments and other parameters as flags [[GH-19423](https://github.com/hashicorp/nomad/issues/19423)]
* client: Fix a panic in building CPU topology when inaccurate CPU data is provided [[GH-19383](https://github.com/hashicorp/nomad/issues/19383)]
* client: Fixed a bug where clients are unable to detect CPU topology in certain conditions [[GH-19457](https://github.com/hashicorp/nomad/issues/19457)]
* consul (Enterprise): Fixed a bug where implicit Consul constraints were not specific to non-default Consul clusters [[GH-19449](https://github.com/hashicorp/nomad/issues/19449)]
* consul: uses token namespace to fetch policies for verification [[GH-18516](https://github.com/hashicorp/nomad/issues/18516)]
* core: Fixed a bug where linux nodes with no reservable cores would panic the scheduler [[GH-19458](https://github.com/hashicorp/nomad/issues/19458)]
* csi: Added validation to `csi_plugin` blocks to prevent `stage_publish_base_dir` from being a subdirectory of `mount_dir` [[GH-19441](https://github.com/hashicorp/nomad/issues/19441)]
* metrics: Revert upgrade of `go-metrics` to fix an issue where metrics from dependencies, such as raft, were no longer emitted [[GH-19374](https://github.com/hashicorp/nomad/issues/19374)]
* ui: Fixed an issue where Accessor ID was masked by default when editing a token [[GH-19432](https://github.com/hashicorp/nomad/issues/19432)]
* vault: Fixed a bug that caused `template` blocks to ignore Nomad configuration for Vault and use the default address of `https://127.0.0.1:8200` when the job does not have a `vault` block defined [[GH-19439](https://github.com/hashicorp/nomad/issues/19439)]

## 1.7.1 (December 08, 2023)

BUG FIXES:

* cli: Fixed a bug that caused the `nomad agent` command to ignore the `VAULT_TOKEN` and `VAULT_NAMESPACE` environment variables [[GH-19349](https://github.com/hashicorp/nomad/issues/19349)]
* client: remove incomplete allocation entries from client state database during client restarts [[GH-16638](https://github.com/hashicorp/nomad/issues/16638)]
* connect: Fixed a bug where deployments would not wait for Connect sidecar task health checks to pass [[GH-19334](https://github.com/hashicorp/nomad/issues/19334)]
* keyring: Fixed a bug where RSA keys were not replicated to followers [[GH-19350](https://github.com/hashicorp/nomad/issues/19350)]

## 1.7.0 (December 07, 2023)

FEATURES:

* **Job Actions**: Introduces the action concept to jobspecs, the web UI, CLI and API. Operators can now define actions that Nomad users can execute against running allocations. [[GH-18794](https://github.com/hashicorp/nomad/issues/18794)]
* **Multiple Vault and Consul Clusters:** Nomad Enterprise can now use multiple Vault or Consul clusters. Each task or service can be registered with a different Consul cluster and each task can obtain secrets from a different Vault cluster. [[GH-5311](https://github.com/hashicorp/nomad/issues/5311)]
* **NUMA aware scheduling**: Nomad Enterprise now supports optimized scheduling on NUMA hardware [[GH-18681](https://github.com/hashicorp/nomad/issues/18681)]
* **Workload Identity IDP:** Nomad's workload identities may now be used with third parties that support JWT or OIDC IDPs such as the AWS IAM OIDC Provider. [[GH-18691](https://github.com/hashicorp/nomad/issues/18691)]
* **Workload Identity for Consul:** Jobs can now use workload identity to authenticate to Consul. [[GH-15618](https://github.com/hashicorp/nomad/issues/15618)]
* **Workload Identity for Vault:** Jobs can now use workload identity to authenticate to Vault. [[GH-15617](https://github.com/hashicorp/nomad/issues/15617)]

BREAKING CHANGES:

* client/fingerprint: The `cpu.numcores.power` node attribute has been renamed to `cpu.numcores.performance` on Apple Silicon nodes [[GH-18843](https://github.com/hashicorp/nomad/issues/18843)]
* client: the `unique.cgroup.mountpoint` node attribute has been removed [[GH-18371](https://github.com/hashicorp/nomad/issues/18371)]
* client: the `unique.cgroup.version` node attribute has been renamed to `os.cgroups.version` [[GH-18371](https://github.com/hashicorp/nomad/issues/18371)]
* core: Honor job's namespace when checking `distinct_hosts` feasibility [[GH-19004](https://github.com/hashicorp/nomad/issues/19004)]

SECURITY:

* build: Update to go1.21.4 to resolve Windows path validation CVE in Go [[GH-19013](https://github.com/hashicorp/nomad/issues/19013)]
* build: Update to go1.21.5 to resolve Windows path validation CVE in Go [[GH-19320](https://github.com/hashicorp/nomad/issues/19320)]

IMPROVEMENTS:

* api: Add JWKS HTTP API endpoint [[GH-18035](https://github.com/hashicorp/nomad/issues/18035)]
* api: Added support for Unix domain sockets [[GH-16872](https://github.com/hashicorp/nomad/issues/16872)]
* build (Enterprise): Support building s390x binaries. [[GH-18069](https://github.com/hashicorp/nomad/issues/18069)]
* cli: Add file prediction for operator raft/snapshot commands [[GH-18901](https://github.com/hashicorp/nomad/issues/18901)]
* cli: Added help text to `acl bootstrap` about reading the initial token from a file [[GH-18961](https://github.com/hashicorp/nomad/issues/18961)]
* cli: Added identities, networks, and volumes to the output of the `operator client-state` command [[GH-18996](https://github.com/hashicorp/nomad/issues/18996)]
* cli: Added support for prefix ID matching and wildcard namespaces to `service info` command [[GH-18836](https://github.com/hashicorp/nomad/issues/18836)]
* client: add support for NetBSD clients [[GH-18562](https://github.com/hashicorp/nomad/issues/18562)]
* client: enable detection of numa topology [[GH-18146](https://github.com/hashicorp/nomad/issues/18146)]
* config: Add `go-netaddrs` support to `server_join.retry_join` [[GH-18745](https://github.com/hashicorp/nomad/issues/18745)]
* consul: constraint for minimum version of Consul increased to 1.8.0 [[GH-19104](https://github.com/hashicorp/nomad/issues/19104)]
* deps: bumped `shirou/gopsutil` to v3.23.9 [[GH-18562](https://github.com/hashicorp/nomad/issues/18562)]
* fingerprint: clients now backoff after successfully fingerprinting Consul [[GH-18426](https://github.com/hashicorp/nomad/issues/18426)]
* identity: Add support for multiple workload identities [[GH-18123](https://github.com/hashicorp/nomad/issues/18123)]
* identity: Implement `change_mode` and `change_signal` for workload identities [[GH-18943](https://github.com/hashicorp/nomad/issues/18943)]
* identity: Support jwt expiration and rotation [[GH-18262](https://github.com/hashicorp/nomad/issues/18262)]
* identity: default to RS256 for new workload ids [[GH-18882](https://github.com/hashicorp/nomad/issues/18882)]
* sentinel (Enterprise): Add existing job information to Sentinel when available. [[GH-18553](https://github.com/hashicorp/nomad/issues/18553)]
* server: Added transfer-leadership API and CLI [[GH-17383](https://github.com/hashicorp/nomad/issues/17383)]
* sso: Allow adding a token name format to auth methods which can be used to generate token names when signing in via SSO [[GH-19135](https://github.com/hashicorp/nomad/issues/19135)]
* ui: color-code node and server status cells [[GH-18318](https://github.com/hashicorp/nomad/issues/18318)]
* ui: for system and sysbatch jobs, now show client name on hover in job panel [[GH-19051](https://github.com/hashicorp/nomad/issues/19051)]
* ui: nicer comment styles in UI example jobs [[GH-19037](https://github.com/hashicorp/nomad/issues/19037)]
* ui: show plan output warnings alongside placement failures and dry-run info when running a job through the web ui [[GH-19225](https://github.com/hashicorp/nomad/issues/19225)]
* ui: simplify presentation of task event times (10m2.230948s bceomes 10m2s etc.) [[GH-18595](https://github.com/hashicorp/nomad/issues/18595)]
* vars: Added a locking feature for Nomad Variables [[GH-18520](https://github.com/hashicorp/nomad/issues/18520)]

DEPRECATIONS:

* config: Loading plugins from `plugin_dir` without a `plugin` configuration block is deprecated [[GH-19189](https://github.com/hashicorp/nomad/issues/19189)]

BUG FIXES:

* agent: Correct websocket status code handling [[GH-19172](https://github.com/hashicorp/nomad/issues/19172)]
* api: Fix panic in `Allocation.Stub` method when `Job` is unset [[GH-19115](https://github.com/hashicorp/nomad/issues/19115)]
* cli: Fixed a bug that caused the `nomad job restart` command to miscount the allocations to restart [[GH-19155](https://github.com/hashicorp/nomad/issues/19155)]
* cli: Fixed a bug where the `operator client-state` command would crash if it reads an allocation without a task state [[GH-18996](https://github.com/hashicorp/nomad/issues/18996)]
* cli: Fixed a panic when the `nomad job restart` command received an interrupt signal while waiting for an answer [[GH-19154](https://github.com/hashicorp/nomad/issues/19154)]
* cli: Fixed the `nomad job restart` command to create replacements for batch and system jobs and to prevent sysbatch jobs from being rescheduled since they never create replacements [[GH-19147](https://github.com/hashicorp/nomad/issues/19147)]
* client: Fixed a bug where client API calls would fail incorrectly with permission denied errors when using ACL tokens with dangling policies [[GH-18972](https://github.com/hashicorp/nomad/issues/18972)]
* core: Fix incorrect submit time for stopped jobs [[GH-18967](https://github.com/hashicorp/nomad/issues/18967)]
* ui: Fixed an issue where purging a job with a namespace did not process correctly [[GH-19139](https://github.com/hashicorp/nomad/issues/19139)]
* ui: fix an issue where starting a stopped job with default-less variables would not retain those variables when done via the job page start button in the web ui [[GH-19220](https://github.com/hashicorp/nomad/issues/19220)]
* ui: fix the job auto-linked variable path name when user lacks variable write permissions [[GH-18598](https://github.com/hashicorp/nomad/issues/18598)]
* variables: Fixed a bug where poststop tasks were not allowed access to Variables [[GH-18754](https://github.com/hashicorp/nomad/issues/18754)]
* vault: Fixed a bug where poststop tasks would not get a Vault token [[GH-19268](https://github.com/hashicorp/nomad/issues/19268)]
* vault: Fixed an issue that could cause Nomad to attempt to renew a Vault token that is already expired [[GH-18985](https://github.com/hashicorp/nomad/issues/18985)]

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

## Unsupported Versions

Versions of Nomad before 1.5.0 are no longer supported. See [CHANGELOG-unsupported.md](./CHANGELOG-unsupported.md) for their changelogs.
