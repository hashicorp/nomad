## 2.0.0 (April 21, 2026)

FEATURES:

* config: add nonproduction config option for server, license, and reporting config [[GH-27646](https://github.com/hashicorp/nomad/issues/27646)]
* core (Enterprise): Enable parsing and reporting with IBM PAO licenses

SECURITY:

* build: upgrade Go to 1.26.2 [[GH-27831](https://github.com/hashicorp/nomad/issues/27831)]
* ui: Increased the client-side generated OIDC nonce entropy to 256-bit. [[GH-27749](https://github.com/hashicorp/nomad/issues/27749)]

IMPROVEMENTS:

* build (Enterprise): Added support for ppc64le CPU architecture on Linux
* build: Upgrade to Go 1.26 [[GH-27685](https://github.com/hashicorp/nomad/issues/27685)]
* metrics: adds a metric for total agent http connections [[GH-26756](https://github.com/hashicorp/nomad/issues/26756)]
* secrets: increase secrets plugin execution timeout to 60s [[GH-27779](https://github.com/hashicorp/nomad/issues/27779)]
* server: Added support for raft-WAL logstore [[GH-27493](https://github.com/hashicorp/nomad/issues/27493)]
* variables: Add variable events to the event stream [[GH-27637](https://github.com/hashicorp/nomad/issues/27637)]

BUG FIXES:

* agent: Fixed a potential panic in agents using systemd notification [[GH-27746](https://github.com/hashicorp/nomad/issues/27746)]
* agent: fix api.Job.Version used in job PUT actions [[GH-27768](https://github.com/hashicorp/nomad/issues/27768)]
* drivers: handle SIGPIPE in executor to handle possible write errors after client restart [[GH-27825](https://github.com/hashicorp/nomad/issues/27825)]
* identity: fix bug where client identity failed to renew after server upgrade to >=1.11.0 [[GH-27773](https://github.com/hashicorp/nomad/issues/27773)]
* oidc: Fixed a bug where the request cache could be corrupted by concurrent requests with the same nonce [[GH-27747](https://github.com/hashicorp/nomad/issues/27747)]
* tls: fix parsing of combined key files when creating tls expiry metric [[GH-27667](https://github.com/hashicorp/nomad/issues/27667)]

## 1.11.4 Enterprise (April 21, 2026)

FEATURES:

* config: add nonproduction config option for server, license, and reporting config [[GH-27646](https://github.com/hashicorp/nomad/issues/27646)]
* core (Enterprise): Enable parsing and reporting with IBM PAO licenses

SECURITY:

* build: upgrade Go to 1.26.2 [[GH-27831](https://github.com/hashicorp/nomad/issues/27831)]
* ui: Increased the client-side generated OIDC nonce entropy to 256-bit. [[GH-27749](https://github.com/hashicorp/nomad/issues/27749)]

IMPROVEMENTS:

* build: Upgrade to Go 1.26 [[GH-27685](https://github.com/hashicorp/nomad/issues/27685)]
* metrics: adds a metric for total agent http connections [[GH-26756](https://github.com/hashicorp/nomad/issues/26756)]
* secrets: increase secrets plugin execution timeout to 60s [[GH-27779](https://github.com/hashicorp/nomad/issues/27779)]
* variables: Add variable events to the event stream [[GH-27637](https://github.com/hashicorp/nomad/issues/27637)]

BUG FIXES:

* agent: Fixed a potential panic in agents using systemd notification [[GH-27746](https://github.com/hashicorp/nomad/issues/27746)]
* agent: fix api.Job.Version used in job PUT actions [[GH-27768](https://github.com/hashicorp/nomad/issues/27768)]
* drivers: handle SIGPIPE in executor to handle possible write errors after client restart [[GH-27825](https://github.com/hashicorp/nomad/issues/27825)]
* identity: fix bug where client identity failed to renew after server upgrade to >=1.11.0 [[GH-27773](https://github.com/hashicorp/nomad/issues/27773)]
* oidc: Fixed a bug where the request cache could be corrupted by concurrent requests with the same nonce [[GH-27747](https://github.com/hashicorp/nomad/issues/27747)]

## 1.11.3 (March 11, 2026)

SECURITY:

* security: Upgrade tooling to Go 1.25.8 [[GH-27653](https://github.com/hashicorp/nomad/issues/27653)]

IMPROVEMENTS:

* acl (Enterprise): Added `sentinel` policy block to allow managing Sentinel policies without a management token [[GH-27556](https://github.com/hashicorp/nomad/issues/27556)]
* acl: Added fine-grained ACL capabilities for saving snapshots and reading the Enterprise license [[GH-27525](https://github.com/hashicorp/nomad/issues/27525)]
* acl: Added fine-grained ACL capability for rotating the keyring [[GH-27526](https://github.com/hashicorp/nomad/issues/27526)]
* agent: Added `agent.tls.cert.expiration_seconds` and `agent.tls.ca.expiration_seconds` telemetry data points to track TLS certificate expiration. [[GH-27538](https://github.com/hashicorp/nomad/issues/27538)]
* cli: Added autocompletions for ACL auth method, binding rule, policy, and token subcommands [[GH-27505](https://github.com/hashicorp/nomad/issues/27505)]
* cli: Improved options autocompletions for various commands [[GH-27506](https://github.com/hashicorp/nomad/issues/27506)]
* cli: Reduced server overhead when dispatching jobs or forcing periodic jobs from the CLI [[GH-27631](https://github.com/hashicorp/nomad/issues/27631)]
* cli: Truncate results when job commands return a large set of jobs that match the provided ID prefix [[GH-27631](https://github.com/hashicorp/nomad/issues/27631)]
* consul (enterprise): adds ability to specify cluster specific consul tokens with environment variables [[GH-27574](https://github.com/hashicorp/nomad/issues/27574)]
* events: Added a Deleted flag to JobDeregistered event type to differentiate between stopped and deleted jobs [[GH-27614](https://github.com/hashicorp/nomad/issues/27614)]

BUG FIXES:

* acl: Fixed a bug where a bearer-token authenticated request could panic the handler for checking claims [[GH-27550](https://github.com/hashicorp/nomad/issues/27550)]
* artifact: Fix artifact inspection when using `file` mode [[GH-27552](https://github.com/hashicorp/nomad/issues/27552)]
* config: Fixed a bug where the keyring block could only be specified a maximum of two times [[GH-27579](https://github.com/hashicorp/nomad/issues/27579)]
* config: Fixed parsing of Vault and Consul blocks as JSON that included objects such as `task_identity` [[GH-27595](https://github.com/hashicorp/nomad/issues/27595)]
* consul: fixes bug where clients were passing node token to connect envoy container, causing acl not found errors [[GH-27574](https://github.com/hashicorp/nomad/issues/27574)]
* core: Fixed system jobs being rescheduled after a node is drained and marked eligible again [[GH-27499](https://github.com/hashicorp/nomad/issues/27499)]
* deployments: Fixed a bug where a task group dropped from a system job could cause deployment state to be overwritten incorrectly [[GH-27604](https://github.com/hashicorp/nomad/issues/27604)]
* deployments: Fixed a bug where system job canary state could be incorrectly changed after a promotion [[GH-27497](https://github.com/hashicorp/nomad/issues/27497)]
* deployments: Fixed a bug where system job deployments would not be marked healthy even though all allocations were healthy [[GH-27497](https://github.com/hashicorp/nomad/issues/27497)]
* drivers: Pass error when included in fingerprint response [[GH-27537](https://github.com/hashicorp/nomad/issues/27537)]
* dynamic host volumes: Fixed a bug with sticky volumes where replacement allocations would not use the previous volume claim [[GH-27613](https://github.com/hashicorp/nomad/issues/27613)]
* http: Ensure the correct HTTP protocol version is set on event stream responses [[GH-27586](https://github.com/hashicorp/nomad/issues/27586)]
* job status: Fixes regression setting job status when jobs have matching prefix [[GH-27516](https://github.com/hashicorp/nomad/issues/27516)]
* keyring (Enterprise): Fixed a bug where in mixed-version clusters with pre-1.9 servers, a keyring rotation that returns an error for an unavailable KMS could prevent future server restarts [[GH-27581](https://github.com/hashicorp/nomad/issues/27581)]
* scheduler: Fix a potential panic in the system scheduler when deploying jobs with multiple task groups and infeasible nodes that become feasible [[GH-27571](https://github.com/hashicorp/nomad/issues/27571)]
* scheduler: Fixed a bug where system deployments would not complete on clusters with pre-1.11.0 nodes [[GH-27605](https://github.com/hashicorp/nomad/issues/27605)]
* state: Fixed a potential state store corruption bug in the service/batch scheduler and deployment watcher [[GH-27548](https://github.com/hashicorp/nomad/issues/27548)]

## 1.11.2 (February 11, 2026)

SECURITY:

* build: Updated toolchain to Go 1.25.6 [[GH-27439](https://github.com/hashicorp/nomad/issues/27439)]
* build: Updated toolchain to Go 1.25.7 [[GH-27468](https://github.com/hashicorp/nomad/issues/27468)]

IMPROVEMENTS:

* acl: Add finer grain permissions for managing job submissions [[GH-27287](https://github.com/hashicorp/nomad/issues/27287)]
* build: Add dev-static and static-release build targets that disable CGO and offer statically-linked binaries [[GH-27310](https://github.com/hashicorp/nomad/issues/27310)]
* cli: Highlight missing driver message in alloc metrics output [[GH-27416](https://github.com/hashicorp/nomad/issues/27416)]
* cli: Improve command line completion of the `sentinel apply` command [[GH-27335](https://github.com/hashicorp/nomad/issues/27335)]
* cni: Added `/usr/libexec/cni` as an additional default path within the `client.cni_path` configuration option [[GH-27336](https://github.com/hashicorp/nomad/issues/27336)]
* cni: Search all paths in cni_path instead of stopping on first failure [[GH-27336](https://github.com/hashicorp/nomad/issues/27336)]
* deps: Migrate from archived dependency `github.com/mitchellh/mapstructure` to `github.com/go-viper/mapstructure/v2` [[GH-27444](https://github.com/hashicorp/nomad/issues/27444)]
* docker: Added support for reserved-only memory oversubscription without a hard limit [[GH-27354](https://github.com/hashicorp/nomad/issues/27354)]
* exec: Added support for reserved-only memory oversubscription without a hard limit [[GH-27354](https://github.com/hashicorp/nomad/issues/27354)]
* fingerprint: Added support for reloading the cpu, memory, network, CNI plugin, and cloud provider fingerprints without restarting the client agent [[GH-27452](https://github.com/hashicorp/nomad/issues/27452)]
* qemu: adds an emulator allowlist to qemu plugin config [[GH-27182](https://github.com/hashicorp/nomad/issues/27182)]
* quotas: Node pool level limits for resources
* reporting (Enterprise): Add device plugin usage to product usage metrics
* rpc: Submitting a plan no longer serializes the whole Job object [[GH-27424](https://github.com/hashicorp/nomad/issues/27424)]
* scheduler: Do not create node evals for terminal node allocs [[GH-27423](https://github.com/hashicorp/nomad/issues/27423)]
* scheduler: Do not create node evaluations for system jobs that are stopped [[GH-27419](https://github.com/hashicorp/nomad/issues/27419)]
* sentinel: Added a new `nomad_var` built-in import for fetching Nomad variables under the `nomad/sentinel` path for use in policy evaluation
* sentinel: Added opt-in support for the `http` module via the `sentinel.additional_enabled_modules` configuration
* state: avoid unneded allocation copy when building event payload [[GH-27311](https://github.com/hashicorp/nomad/issues/27311)]

BUG FIXES:

* acl: Fixed a bug where host-volume-delete capability was not allowed when writing a policy [[GH-27434](https://github.com/hashicorp/nomad/issues/27434)]
* api: exit EventStream.Stream on first error [[GH-27141](https://github.com/hashicorp/nomad/issues/27141)]
* api: only include running tasks in allocation resource usage [[GH-27317](https://github.com/hashicorp/nomad/issues/27317)]
* api: return proper 403 message when getting variables instead of swallowing error [[GH-27269](https://github.com/hashicorp/nomad/issues/27269)]
* artifact: Fixed a bug that prevented the sandbox from moving downloaded files to the target directory on Windows [[GH-27398](https://github.com/hashicorp/nomad/issues/27398)]
* checks: Fixed a bug where script checks with task-level interpolation would fail to heartbeat to Consul [[GH-27453](https://github.com/hashicorp/nomad/issues/27453)]
* client: Added a new `fingerprint` configuration block which allows users to specify retry behavior for the `env_aws`, `env_azure`, `env_digitalocean` and `env_gcp` fingerprinters. [[GH-27161](https://github.com/hashicorp/nomad/issues/27161)]
* client: Fix unchanged devices causing extraneous node updates [[GH-27363](https://github.com/hashicorp/nomad/issues/27363)]
* client: Fixed generation of the "NOMAD_ALLOC_ADDR_" environment variable when using static port assignments [[GH-27305](https://github.com/hashicorp/nomad/issues/27305)]
* core: Fixed a bug where follow-up evals could be created for failed evaluations of garbage collected jobs [[GH-27367](https://github.com/hashicorp/nomad/issues/27367)]
* csi: Sanitize volumes correctly upon sentinel policy eval
* deployment: Fixed a bug where deploying a system job could panic the leader [[GH-27262](https://github.com/hashicorp/nomad/issues/27262)]
* deployments: Fixed a bug where system deployments can violate update.max_parallel if another eval for the job is triggered while allocs are pending [[GH-27284](https://github.com/hashicorp/nomad/issues/27284)]
* disconnect: allocations with a `disconnect.lost_after > 0` and `replace = true` will now follow the reschedule block instead of immediately being replaced. [[GH-27053](https://github.com/hashicorp/nomad/issues/27053)]
* dispatch: Fixed a bug where concurrent dispatch requests could ignore the idempotency token [[GH-27353](https://github.com/hashicorp/nomad/issues/27353)]
* drivers: adds hostname to NetworkCreateRequest for external drivers [[GH-27273](https://github.com/hashicorp/nomad/issues/27273)]
* event broker: fix memory leak in methods that close subscriptions [[GH-27312](https://github.com/hashicorp/nomad/issues/27312)]
* event stream: Fixed a bug where the HTTP handler can block forever and cause high memory usage if an API client reads too slowly from the stream [[GH-27397](https://github.com/hashicorp/nomad/issues/27397)]
* host volumes: Fixed a bug where allocations that request volumes with sticky=true could not be placed if previous allocations in the job claimed volumes [[GH-27470](https://github.com/hashicorp/nomad/issues/27470)]
* job: Correctly validate any constraint attributes to ensure they conform to known formats [[GH-27355](https://github.com/hashicorp/nomad/issues/27355)]
* keyring (Enterprise): Fixed a bug where servers configured with high availability keyrings with pre-1.9.0 keystores would not start if one of the external KMS was unreachable [[GH-27279](https://github.com/hashicorp/nomad/issues/27279)]
* multiregion: fixes a bug where resubmitting an unchanged job would cause server handler to hang [[GH-27386](https://github.com/hashicorp/nomad/issues/27386)]
* numa: Fixed a bug where NUMA detection would cause a panic on hosts with discontinuous node IDs [[GH-27277](https://github.com/hashicorp/nomad/issues/27277)]
* qemu: change driver filesystem isolation to "None" for proper variable interpolation in job spec [[GH-27246](https://github.com/hashicorp/nomad/issues/27246)]
* qemu: fixes graceful_shutdown to wait kill_timeout before signalling process [[GH-27316](https://github.com/hashicorp/nomad/issues/27316)]
* ui: Tagging job versions in another namespace than the default-namespace resulted in an error [[GH-27282](https://github.com/hashicorp/nomad/issues/27282)]
* ui: fix bug preventing OIDC login when `iss` parameter is required [[GH-27248](https://github.com/hashicorp/nomad/issues/27248)]

## 1.11.1 (December 09, 2025)

BREAKING CHANGES:

* docker: removed deprecated email auth config parameter [[GH-27156](https://github.com/hashicorp/nomad/issues/27156)]

SECURITY:

* build: Updated toolchain to Go 1.25.5 [[GH-27186](https://github.com/hashicorp/nomad/issues/27186)]

IMPROVEMENTS:

* connect: allow configuring identities for sidecar_task [[GH-25877](https://github.com/hashicorp/nomad/issues/25877)]
* landlock: check paths exist on setup [[GH-27149](https://github.com/hashicorp/nomad/issues/27149)]
* oidc: add support for array-based OIDC claims [[GH-26958](https://github.com/hashicorp/nomad/issues/26958)]
* qemu: Adds config parameters to modify qemu emulator binary and machine types and removes some hardcoded KVM accelerator settings. Defaults to previously used values of qemu-system-x86_64 and pc. The driver no longer forces machine type "host", or the -smp flag when using resources.cores with the KVM accelerator. [[GH-27128](https://github.com/hashicorp/nomad/issues/27128)]
* secrets: Adds nomad job ID and namespace to plugin environment [[GH-27207](https://github.com/hashicorp/nomad/issues/27207)]

BUG FIXES:

* acl: Made /agent and /recommendations endpoints workload-identity-aware [[GH-27099](https://github.com/hashicorp/nomad/issues/27099)]
* acl: include additional necessary permissions in the course-grained "scale" policy for nomad-autoscaler [[GH-27061](https://github.com/hashicorp/nomad/issues/27061)]
* api: Fixed a bug in the Go API where an event stream request without a topic filter would require a management token [[GH-27065](https://github.com/hashicorp/nomad/issues/27065)]
* cli: Fixed the `var get` command which was incorrectly displaying the variable modify time as the create time [[GH-27208](https://github.com/hashicorp/nomad/issues/27208)]
* client: return 403 when the caller doesn't have log streaming capabilities [[GH-27098](https://github.com/hashicorp/nomad/issues/27098)]
* csi: Fixed a bug where reading a volume from the API or event stream could erase its secrets [[GH-27176](https://github.com/hashicorp/nomad/issues/27176)]
* drain: Fixed a bug where clients configured with `leave_on_terminate` or `leave_on_interrupt` and `drain_on_shutdown` would receive a permission denied error when attempting to leave the cluster and drain themselves [[GH-27115](https://github.com/hashicorp/nomad/issues/27115)]
* dynamic host volumes: Ensure requested directory permission is correctly applied [[GH-27068](https://github.com/hashicorp/nomad/issues/27068)]
* dynamic host volumes: fix Windows compatibility [[GH-27147](https://github.com/hashicorp/nomad/issues/27147)]
* fingerprint: simplify storage fingerprint calculation to just (total disk space - reserved disk) [[GH-27019](https://github.com/hashicorp/nomad/issues/27019)]
* keyring: Do not mark the key as inactive until all follow-up rekey evals have completed. [[GH-27193](https://github.com/hashicorp/nomad/issues/27193)]
* keyring: Ensure follow-up rekey evals can be successfully created. [[GH-27193](https://github.com/hashicorp/nomad/issues/27193)]
* oidc: Add support for RFC9207, requiring an issuer param in authorization response if the provider requires it [[GH-27168](https://github.com/hashicorp/nomad/issues/27168)]
* reconciler: fixes a bug where stopping a job does not stop all allocations [[GH-27175](https://github.com/hashicorp/nomad/issues/27175)]
* scheduler (Enterprise): Fixed a bug where tasks were not placed on same numa node as reserved device [[GH-27177](https://github.com/hashicorp/nomad/issues/27177)]
* scheduler: Fixed a bug that was previously patched incorrectly where rescheduled allocations that could not be placed would later ignore their reschedule policy limits [[GH-27129](https://github.com/hashicorp/nomad/issues/27129)]
* server: Fixed a bug where a large backlog of unblocking evals could cause backpressure on Raft writes [[GH-27184](https://github.com/hashicorp/nomad/issues/27184)]
* ui: Fixed the error message presented for invalid Variables definitions [[GH-26235](https://github.com/hashicorp/nomad/issues/26235)]

## 1.11.0 (November 11, 2025)

FEATURES:

* Client Identity: Nomad clients use identities for authenticating and authorizing itself when performing RPC calls. The identities are generated and rotated automatically by Nomad servers with configurable TTLs. [[GH-26291](https://github.com/hashicorp/nomad/issues/26291)]
* Client Introduction: Nomad clients can now be introduced to the cluster using a token-based approach. Nomad servers can be configured with introduction enforcement levels which dictate how clients can join the cluster resulting in logs and metrics to detail introduction violations. [[GH-26430](https://github.com/hashicorp/nomad/issues/26430)]
* scheduler: Enable deployments for system jobs [[GH-26708](https://github.com/hashicorp/nomad/issues/26708)]
* secrets: Adds secret block for fetching and interpolating secrets in job spec [[GH-26681](https://github.com/hashicorp/nomad/issues/26681)]

BREAKING CHANGES:

* metrics: Eval broker metrics that previously used the job ID as a label will now use the parent ID of dispatch and periodic jobs [[GH-26737](https://github.com/hashicorp/nomad/issues/26737)]
* sysbatch: Submitting a sysbatch job with a `reschedule` block will now return an error instead of being silently ignored [[GH-26279](https://github.com/hashicorp/nomad/issues/26279)]

SECURITY:

* build: Update go-getter to 1.8.3 that prevents a partially written file from remaining on disk with permissions that didn't include the umask. [[GH-27034](https://github.com/hashicorp/nomad/issues/27034)]
* build: Update toolchain to Go 1.25.2 to address Go stdlib CVE-2025-61724, CVE-2025-61725, CVE-2025-58187, CVE-2025-61723, CVE-2025-47912, CVE-2025-58185, CVE-2025-58186, CVE-2025-58188, and CVE-2025-58183 [[GH-26909](https://github.com/hashicorp/nomad/issues/26909)]
* job: Disallow tasks using the name "alloc" which breaks inter-task filesystem isolation [[GH-27001](https://github.com/hashicorp/nomad/issues/27001)]

IMPROVEMENTS:

* api: The `Evaluations.Info` method of the Go API now populates the `RelatedEvals` field. [[GH-26156](https://github.com/hashicorp/nomad/issues/26156)]
* build: Add tzdata to Docker container final image [[GH-26794](https://github.com/hashicorp/nomad/issues/26794)]
* build: Updated Go to 1.25.1 [[GH-26823](https://github.com/hashicorp/nomad/issues/26823)]
* cli: Add -preserve-resources flag for keeping resource block when updating jobs [[GH-26841](https://github.com/hashicorp/nomad/issues/26841)]
* cli: Added related evals and placed allocations tables to the eval status command, and exposed more fields without requiring the `-verbose` flag. [[GH-26156](https://github.com/hashicorp/nomad/issues/26156)]
* config: Added job_max_count option to limit number of allocs for a single job [[GH-26858](https://github.com/hashicorp/nomad/issues/26858)]
* consul connect: Allow cni/* network mode; use at your own risk [[GH-26449](https://github.com/hashicorp/nomad/issues/26449)]
* install (Enterprise): Updated license information displayed during post-install [[GH-26791](https://github.com/hashicorp/nomad/issues/26791)]
* metrics: Reduce memory usage on the Nomad leader for collecting eval broker metrics. [[GH-26737](https://github.com/hashicorp/nomad/issues/26737)]
* reporting (Enterprise): Include product usage metrics with license utilization reports [[GH-27005](https://github.com/hashicorp/nomad/issues/27005)]
* scheduler: Add reconciler annotations to the output of the `eval status` command [[GH-26188](https://github.com/hashicorp/nomad/issues/26188)]
* scheduler: Debug-level logs emitted by the scheduler are now single-line structured logs [[GH-26169](https://github.com/hashicorp/nomad/issues/26169)]
* scheduler: For service and batch jobs, the scheduler no longer includes stops for already-stopped canaries in plans it submits. [[GH-26292](https://github.com/hashicorp/nomad/issues/26292)]
* scheduler: For service and batch jobs, the scheduler treats a group.count=0 identically to removing the task group from the job, and will stop all non-terminal allocations. [[GH-26292](https://github.com/hashicorp/nomad/issues/26292)]

DEPRECATIONS:

* api: the `Resources` and `Reserved` fields on the `Node` struct in the Go API are deprecated and will be removed in Nomad 1.12.0. Use the `NodeResources` and `ReservedResources` fields instead [[GH-26951](https://github.com/hashicorp/nomad/issues/26951)]

BUG FIXES:

* acl: Fixed a bug where ACL policies would silently accept invalid or duplicate blocks [[GH-26836](https://github.com/hashicorp/nomad/issues/26836)]
* auth: Fixed a bug where workload identity tokens could not be used to list or get policies from the ACL API [[GH-26772](https://github.com/hashicorp/nomad/issues/26772)]
* build: Updated toolchain to Go 1.25.3 to address bug in TLS certificate validation [[GH-26949](https://github.com/hashicorp/nomad/issues/26949)]
* client: Fix unique identifiers for templates with same content [[GH-26880](https://github.com/hashicorp/nomad/issues/26880)]
* client: restore task network status on client restart so restarted tasks receive proper networking environment variables, hosts file, and resolv.conf. [[GH-26699](https://github.com/hashicorp/nomad/issues/26699)]
* consul (Enterprise): Fixed a bug where Consul fingerprinting would generate warning logs if there was no default cluster [[GH-26787](https://github.com/hashicorp/nomad/issues/26787)]
* core: Fixed a bug where GC batch sizes for jobs resulted in excessively large Raft logs [[GH-26974](https://github.com/hashicorp/nomad/issues/26974)]
* csi: Fixed a bug where multiple node plugin RPCs could be in-flight for a single volume [[GH-26832](https://github.com/hashicorp/nomad/issues/26832)]
* csi: Fixed a bug where volumes could be unmounted while in use by a task that was shutting down [[GH-26831](https://github.com/hashicorp/nomad/issues/26831)]
* docker: Fixed a bug where cpu usage percentage was incorrectly measured when container was stopped [[GH-26902](https://github.com/hashicorp/nomad/issues/26902)]
* keyring: fixes an issue with Vault transit configuration where tls_skip_verify was not defaulting to false [[GH-26664](https://github.com/hashicorp/nomad/issues/26664)]
* networking: Fixed network interface detection failure with bridge or CNI mode on IPv6-only interfaces [[GH-26910](https://github.com/hashicorp/nomad/issues/26910)]
* scheduler: Fixed scheduling behavior of batch job allocations [[GH-26961](https://github.com/hashicorp/nomad/issues/26961)]
* scheduler: allow use of different vendor/models when checking for device counts while filtering feasible nodes [[GH-26649](https://github.com/hashicorp/nomad/issues/26649)]
* scheduler: fixes a bug selecting nodes for updated jobs with ephemeral disks when nodepool changes [[GH-26662](https://github.com/hashicorp/nomad/issues/26662)]
* state: Fixed a bug where the server could panic when attempting to remove unneeded evals from the eval broker [[GH-26872](https://github.com/hashicorp/nomad/issues/26872)]
* ui: Fixed a bug where action fly-outs would fail to open due to a missing module [[GH-26833](https://github.com/hashicorp/nomad/issues/26833)]
* windows: Fixed a bug where agents would not gracefully shut down on Ctrl-C [[GH-26780](https://github.com/hashicorp/nomad/issues/26780)]

## 1.10.10 Enterprise (April 21, 2026)

FEATURES:

* core (Enterprise): Enable parsing and reporting with IBM PAO licenses

SECURITY:

* build: upgrade Go to 1.26.2 [[GH-27831](https://github.com/hashicorp/nomad/issues/27831)]
* ui: Increased the client-side generated OIDC nonce entropy to 256-bit. [[GH-27749](https://github.com/hashicorp/nomad/issues/27749)]

IMPROVEMENTS:

* build: Upgrade to Go 1.26 [[GH-27685](https://github.com/hashicorp/nomad/issues/27685)]

BUG FIXES:

* agent: Fixed a potential panic in agents using systemd notification [[GH-27746](https://github.com/hashicorp/nomad/issues/27746)]
* agent: fix api.Job.Version used in job PUT actions [[GH-27768](https://github.com/hashicorp/nomad/issues/27768)]
* drivers: handle SIGPIPE in executor to handle possible write errors after client restart [[GH-27825](https://github.com/hashicorp/nomad/issues/27825)]
* oidc: Fixed a bug where the request cache could be corrupted by concurrent requests with the same nonce [[GH-27747](https://github.com/hashicorp/nomad/issues/27747)]

## 1.10.9 Enterprise (March 11, 2026)

SECURITY:

* security: Upgrade tooling to Go 1.25.8 [[GH-27653](https://github.com/hashicorp/nomad/issues/27653)]

IMPROVEMENTS:

* consul (enterprise): adds ability to specify cluster specific consul tokens with environment variables [[GH-27574](https://github.com/hashicorp/nomad/issues/27574)]

BUG FIXES:

* acl: Fixed a bug where a bearer-token authenticated request could panic the handler for checking claims [[GH-27550](https://github.com/hashicorp/nomad/issues/27550)]
* artifact: Fix artifact inspection when using `file` mode [[GH-27552](https://github.com/hashicorp/nomad/issues/27552)]
* config: Fixed a bug where the keyring block could only be specified a maximum of two times [[GH-27579](https://github.com/hashicorp/nomad/issues/27579)]
* config: Fixed parsing of Vault and Consul blocks as JSON that included objects such as `task_identity` [[GH-27595](https://github.com/hashicorp/nomad/issues/27595)]
* consul: fixes bug where clients were passing node token to connect envoy container, causing acl not found errors [[GH-27574](https://github.com/hashicorp/nomad/issues/27574)]
* drivers: Pass error when included in fingerprint response [[GH-27537](https://github.com/hashicorp/nomad/issues/27537)]
* http: Ensure the correct HTTP protocol version is set on event stream responses [[GH-27586](https://github.com/hashicorp/nomad/issues/27586)]
* job status: Fixes regression setting job status when jobs have matching prefix [[GH-27516](https://github.com/hashicorp/nomad/issues/27516)]
* keyring (Enterprise): Fixed a bug where in mixed-version clusters with pre-1.9 servers, a keyring rotation that returns an error for an unavailable KMS could prevent future server restarts [[GH-27581](https://github.com/hashicorp/nomad/issues/27581)]
* state: Fixed a potential state store corruption bug in the service/batch scheduler and deployment watcher [[GH-27548](https://github.com/hashicorp/nomad/issues/27548)]

## 1.10.8 Enterprise (February 11, 2026)

SECURITY:

* build: Updated toolchain to Go 1.25.6 [[GH-27439](https://github.com/hashicorp/nomad/issues/27439)]
* build: Updated toolchain to Go 1.25.7 [[GH-27468](https://github.com/hashicorp/nomad/issues/27468)]

IMPROVEMENTS:

* build: Add dev-static and static-release build targets that disable CGO and offer statically-linked binaries [[GH-27310](https://github.com/hashicorp/nomad/issues/27310)]
* deps: Migrate from archived dependency `github.com/mitchellh/mapstructure` to `github.com/go-viper/mapstructure/v2` [[GH-27444](https://github.com/hashicorp/nomad/issues/27444)]
* reporting (Enterprise): Add device plugin usage to product usage metrics
* state: avoid unneded allocation copy when building event payload [[GH-27311](https://github.com/hashicorp/nomad/issues/27311)]

BUG FIXES:

* acl: Fixed a bug where host-volume-delete capability was not allowed when writing a policy [[GH-27434](https://github.com/hashicorp/nomad/issues/27434)]
* api: only include running tasks in allocation resource usage [[GH-27317](https://github.com/hashicorp/nomad/issues/27317)]
* api: return proper 403 message when getting variables instead of swallowing error [[GH-27269](https://github.com/hashicorp/nomad/issues/27269)]
* artifact: Fixed a bug that prevented the sandbox from moving downloaded files to the target directory on Windows [[GH-27398](https://github.com/hashicorp/nomad/issues/27398)]
* checks: Fixed a bug where script checks with task-level interpolation would fail to heartbeat to Consul [[GH-27453](https://github.com/hashicorp/nomad/issues/27453)]
* client: Fix unchanged devices causing extraneous node updates [[GH-27363](https://github.com/hashicorp/nomad/issues/27363)]
* client: Fixed generation of the "NOMAD_ALLOC_ADDR_" environment variable when using static port assignments [[GH-27305](https://github.com/hashicorp/nomad/issues/27305)]
* core: Fixed a bug where follow-up evals could be created for failed evaluations of garbage collected jobs [[GH-27367](https://github.com/hashicorp/nomad/issues/27367)]
* csi: Sanitize volumes correctly upon sentinel policy eval
* dispatch: Fixed a bug where concurrent dispatch requests could ignore the idempotency token [[GH-27353](https://github.com/hashicorp/nomad/issues/27353)]
* drivers: adds hostname to NetworkCreateRequest for external drivers [[GH-27273](https://github.com/hashicorp/nomad/issues/27273)]
* event broker: fix memory leak in methods that close subscriptions [[GH-27312](https://github.com/hashicorp/nomad/issues/27312)]
* event stream: Fixed a bug where the HTTP handler can block forever and cause high memory usage if an API client reads too slowly from the stream [[GH-27397](https://github.com/hashicorp/nomad/issues/27397)]
* host volumes: Fixed a bug where allocations that request volumes with sticky=true could not be placed if previous allocations in the job claimed volumes [[GH-27470](https://github.com/hashicorp/nomad/issues/27470)]
* job: Correctly validate any constraint attributes to ensure they conform to known formats [[GH-27355](https://github.com/hashicorp/nomad/issues/27355)]
* keyring (Enterprise): Fixed a bug where servers configured with high availability keyrings with pre-1.9.0 keystores would not start if one of the external KMS was unreachable [[GH-27279](https://github.com/hashicorp/nomad/issues/27279)]
* multiregion: fixes a bug where resubmitting an unchanged job would cause server handler to hang [[GH-27386](https://github.com/hashicorp/nomad/issues/27386)]
* numa: Fixed a bug where NUMA detection would cause a panic on hosts with discontinuous node IDs [[GH-27277](https://github.com/hashicorp/nomad/issues/27277)]
* qemu: fixes graceful_shutdown to wait kill_timeout before signalling process [[GH-27316](https://github.com/hashicorp/nomad/issues/27316)]
* ui: Tagging job versions in another namespace than the default-namespace resulted in an error [[GH-27282](https://github.com/hashicorp/nomad/issues/27282)]
* ui: fix bug preventing OIDC login when `iss` parameter is required [[GH-27248](https://github.com/hashicorp/nomad/issues/27248)]

## 1.10.7 Enterprise (December 09, 2025)

BREAKING CHANGES:

* docker: removed deprecated email auth config parameter [[GH-27156](https://github.com/hashicorp/nomad/issues/27156)]

SECURITY:

* build: Updated toolchain to Go 1.25.5 [[GH-27186](https://github.com/hashicorp/nomad/issues/27186)]

IMPROVEMENTS:

* landlock: check paths exist on setup [[GH-27149](https://github.com/hashicorp/nomad/issues/27149)]

BUG FIXES:

* acl: Made /agent and /recommendations endpoints workload-identity-aware [[GH-27099](https://github.com/hashicorp/nomad/issues/27099)]
* acl: include additional necessary permissions in the course-grained "scale" policy for nomad-autoscaler [[GH-27061](https://github.com/hashicorp/nomad/issues/27061)]
* api: Fixed a bug in the Go API where an event stream request without a topic filter would require a management token [[GH-27065](https://github.com/hashicorp/nomad/issues/27065)]
* cli: Fixed the `var get` command which was incorrectly displaying the variable modify time as the create time [[GH-27208](https://github.com/hashicorp/nomad/issues/27208)]
* client: return 403 when the caller doesn't have log streaming capabilities [[GH-27098](https://github.com/hashicorp/nomad/issues/27098)]
* csi: Fixed a bug where reading a volume from the API or event stream could erase its secrets [[GH-27176](https://github.com/hashicorp/nomad/issues/27176)]
* dynamic host volumes: Ensure requested directory permission is correctly applied [[GH-27068](https://github.com/hashicorp/nomad/issues/27068)]
* dynamic host volumes: fix Windows compatibility [[GH-27147](https://github.com/hashicorp/nomad/issues/27147)]
* keyring: Do not mark the key as inactive until all follow-up rekey evals have completed. [[GH-27193](https://github.com/hashicorp/nomad/issues/27193)]
* keyring: Ensure follow-up rekey evals can be successfully created. [[GH-27193](https://github.com/hashicorp/nomad/issues/27193)]
* multiregion (Enterprise): fixes a bug where multiregion deployments could become deadlocked
* multiregion: fixes a bug where unblocking region could make unnecessary queries to other regions
* oidc: Add support for RFC9207, requiring an issuer param in authorization response if the provider requires it [[GH-27168](https://github.com/hashicorp/nomad/issues/27168)]
* scheduler (Enterprise): Fixed a bug where tasks were not placed on same numa node as reserved device [[GH-27177](https://github.com/hashicorp/nomad/issues/27177)]
* scheduler: Fixed a bug that was previously patched incorrectly where rescheduled allocations that could not be placed would later ignore their reschedule policy limits [[GH-27129](https://github.com/hashicorp/nomad/issues/27129)]
* server: Fixed a bug where a large backlog of unblocking evals could cause backpressure on Raft writes [[GH-27184](https://github.com/hashicorp/nomad/issues/27184)]
* ui: Fixed the error message presented for invalid Variables definitions [[GH-26235](https://github.com/hashicorp/nomad/issues/26235)]

## 1.10.6 Enterprise (November 11, 2025)

SECURITY:

* build: Update go-getter to 1.8.3 that prevents a partially written file from remaining on disk with permissions that didn't include the umask. [[GH-27034](https://github.com/hashicorp/nomad/issues/27034)]
* build: Update toolchain to Go 1.25.2 to address Go stdlib CVE-2025-61724, CVE-2025-61725, CVE-2025-58187, CVE-2025-61723, CVE-2025-47912, CVE-2025-58185, CVE-2025-58186, CVE-2025-58188, and CVE-2025-58183 [[GH-26909](https://github.com/hashicorp/nomad/issues/26909)]
* job: Disallow tasks using the name "alloc" which breaks inter-task filesystem isolation [[GH-27001](https://github.com/hashicorp/nomad/issues/27001)]

IMPROVEMENTS:

* build: Add tzdata to Docker container final image [[GH-26794](https://github.com/hashicorp/nomad/issues/26794)]
* build: Updated Go to 1.25.1 [[GH-26823](https://github.com/hashicorp/nomad/issues/26823)]
* cli: Add -preserve-resources flag for keeping resource block when updating jobs [[GH-26841](https://github.com/hashicorp/nomad/issues/26841)]
* install (Enterprise): Updated license information displayed during post-install [[GH-26791](https://github.com/hashicorp/nomad/issues/26791)]
* reporting (Enterprise): Include product usage metrics with license utilization reports [[GH-27005](https://github.com/hashicorp/nomad/issues/27005)]

BUG FIXES:

* acl: Fixed a bug where ACL policies would silently accept invalid or duplicate blocks [[GH-26836](https://github.com/hashicorp/nomad/issues/26836)]
* auth: Fixed a bug where workload identity tokens could not be used to list or get policies from the ACL API [[GH-26772](https://github.com/hashicorp/nomad/issues/26772)]
* build: Updated toolchain to Go 1.25.3 to address bug in TLS certificate validation [[GH-26949](https://github.com/hashicorp/nomad/issues/26949)]
* client: Fix unique identifiers for templates with same content [[GH-26880](https://github.com/hashicorp/nomad/issues/26880)]
* client: restore task network status on client restart so restarted tasks receive proper networking environment variables, hosts file, and resolv.conf. [[GH-26699](https://github.com/hashicorp/nomad/issues/26699)]
* consul (Enterprise): Fixed a bug where Consul fingerprinting would generate warning logs if there was no default cluster [[GH-26787](https://github.com/hashicorp/nomad/issues/26787)]
* core: Fixed a bug where GC batch sizes for jobs resulted in excessively large Raft logs [[GH-26974](https://github.com/hashicorp/nomad/issues/26974)]
* csi: Fixed a bug where multiple node plugin RPCs could be in-flight for a single volume [[GH-26832](https://github.com/hashicorp/nomad/issues/26832)]
* csi: Fixed a bug where volumes could be unmounted while in use by a task that was shutting down [[GH-26831](https://github.com/hashicorp/nomad/issues/26831)]
* docker: Fixed a bug where cpu usage percentage was incorrectly measured when container was stopped [[GH-26902](https://github.com/hashicorp/nomad/issues/26902)]
* keyring: fixes an issue with Vault transit configuration where tls_skip_verify was not defaulting to false [[GH-26664](https://github.com/hashicorp/nomad/issues/26664)]
* networking: Fixed network interface detection failure with bridge or CNI mode on IPv6-only interfaces [[GH-26910](https://github.com/hashicorp/nomad/issues/26910)]
* scheduler: Fixed scheduling behavior of batch job allocations [[GH-26961](https://github.com/hashicorp/nomad/issues/26961)]
* scheduler: allow use of different vendor/models when checking for device counts while filtering feasible nodes [[GH-26649](https://github.com/hashicorp/nomad/issues/26649)]
* scheduler: fixes a bug selecting nodes for updated jobs with ephemeral disks when nodepool changes [[GH-26662](https://github.com/hashicorp/nomad/issues/26662)]
* state: Fixed a bug where the server could panic when attempting to remove unneeded evals from the eval broker [[GH-26872](https://github.com/hashicorp/nomad/issues/26872)]
* ui: Fixed a bug where action fly-outs would fail to open due to a missing module [[GH-26833](https://github.com/hashicorp/nomad/issues/26833)]
* windows: Fixed a bug where agents would not gracefully shut down on Ctrl-C [[GH-26780](https://github.com/hashicorp/nomad/issues/26780)]

## 1.10.5 (September 09, 2025)

SECURITY:

* build: Update Go to 1.24.7 to address CVE-2025-47910 [[GH-26713](https://github.com/hashicorp/nomad/issues/26713)]
* build: Update go-getter to 1.7.9 to address CVE-2025-8959. Nomad Client Agents with Landlock support are not impacted by this vulnerability. [[GH-26533](https://github.com/hashicorp/nomad/issues/26533)]
* client: inspect artifacts for sandbox escape when landlock is unavailable [[GH-26608](https://github.com/hashicorp/nomad/issues/26608)]

IMPROVEMENTS:

* agent: Allow agent logging to the Windows Event Log [[GH-26441](https://github.com/hashicorp/nomad/issues/26441)]
* cli: Add commands for installing and uninstalling Windows system service [[GH-26442](https://github.com/hashicorp/nomad/issues/26442)]
* config: Validate the `keyring` configuration block label against supported values on agent startup [[GH-26673](https://github.com/hashicorp/nomad/issues/26673)]
* scheduling: Improve performance of scheduling when checking reserved ports usage [[GH-26712](https://github.com/hashicorp/nomad/issues/26712)]

BUG FIXES:

* consul: Fixed a bug where restarting the Nomad agent would cause Consul ACL tokens to be recreated [[GH-26604](https://github.com/hashicorp/nomad/pull/26604)]
* csi: fix EOF error when registering volumes [[GH-26642](https://github.com/hashicorp/nomad/issues/26642)]
* dispatch: Fixed a bug where evaluations were not created atomically with dispatched jobs, which could prevent dispatch jobs from creating allocations [[GH-26710](https://github.com/hashicorp/nomad/issues/26710)]
* exec: Adjust USER and HOME env vars when user value is set [[GH-25859](https://github.com/hashicorp/nomad/issues/25859)]
* exec: Correctly set the `LOGNAME` env var when the job specification user value is set [[GH-26703](https://github.com/hashicorp/nomad/issues/26703)]
* logs: skip logging SIGPIPE [[GH-26582](https://github.com/hashicorp/nomad/issues/26582)]

## 1.10.4 (August 13, 2025)

SECURITY:

* build: Update Go to 1.24.3 to address CVE-2025-47906 [[GH-26451](https://github.com/hashicorp/nomad/issues/26451)]

IMPROVEMENTS:

* cli: Added monitor export cli command to retrieve journald logs or the contents of the Nomad log file for a given Nomad agent [[GH-26178](https://github.com/hashicorp/nomad/issues/26178)]
* command: Add historical log capture to `nomad operator debug` command with `-log-lookback` and `-log-file-export` flags [[GH-26410](https://github.com/hashicorp/nomad/issues/26410)]
* metrics: Added node_pool label to blocked_evals metrics [[GH-26215](https://github.com/hashicorp/nomad/issues/26215)]
* sentinel (Enterprise): Added policy scope for csi-volumes [[GH-26438](https://github.com/hashicorp/nomad/issues/26438)]

BUG FIXES:

* alloc exec: Fixed executor panic when exec-ing a rootless raw_exec task [[GH-26401](https://github.com/hashicorp/nomad/issues/26401)]
* cli: Fixed a bug where `acl policy self` command would output all policies when used with a management token [[GH-26396](https://github.com/hashicorp/nomad/issues/26396)]
* client: run all allocrunner postrun (cleanup) hooks, even if any of them error [[GH-26271](https://github.com/hashicorp/nomad/issues/26271)]
* consul: Add AllocIPv6 option to allow IPv6 address being used for service registration [[GH-25632](https://github.com/hashicorp/nomad/issues/25632)]
* jobspec: Validate required hook field in lifecycle block [[GH-26285](https://github.com/hashicorp/nomad/issues/26285)]
* services: Fixed a bug where Nomad services were deleted if a node missed heartbeats and recovered before allocs were migrated [[GH-26424](https://github.com/hashicorp/nomad/issues/26424)]

## 1.10.3 (July 08, 2025)

IMPROVEMENTS:

* consul: Added kind field to service block for Consul service registrations [[GH-26170](https://github.com/hashicorp/nomad/issues/26170)]
* docker: Added support for cgroup namespaces in the task config [[GH-25927](https://github.com/hashicorp/nomad/issues/25927)]
* task environment: new NOMAD_UNIX_ADDR env var points to the task API unix socket, for use with workload identity [[GH-25598](https://github.com/hashicorp/nomad/issues/25598)]

BUG FIXES:

* agent: Fixed a bug to prevent a possible panic during graceful shutdown [[GH-26018](https://github.com/hashicorp/nomad/issues/26018)]
* agent: Fixed a bug to prevent panic during graceful server shutdown [[GH-26171](https://github.com/hashicorp/nomad/issues/26171)]
* agent: Fixed bug where agent would exit early from graceful shutdown when managed by systemd [[GH-26023](https://github.com/hashicorp/nomad/issues/26023)]
* cli: Fix panic when restarting stopped job with no scaling policies [[GH-26131](https://github.com/hashicorp/nomad/issues/26131)]
* cli: Fixed a bug in the `tls cert create` command that always added ``"<role>.global.nomad"` to the certificate DNS names, even when the specified region was not ``"global"`. [[GH-26086](https://github.com/hashicorp/nomad/issues/26086)]
* cli: Fixed a bug where the `acl token self` command only performed lookups for tokens set as environment variables and not by the `-token` flag. [[GH-26183](https://github.com/hashicorp/nomad/issues/26183)]
* client: Attempt to rollback directory creation when the `mkdir` plugin fails to perform ownership changes on it [[GH-26194](https://github.com/hashicorp/nomad/issues/26194)]
* client: Fixed bug where drained batch jobs would not be rescheduled if no eligible nodes were immediately available [[GH-26025](https://github.com/hashicorp/nomad/issues/26025)]
* docker: Fixed a bug where very low resources.cpu values could generate invalid cpu weights on hosts with very large client.cpu_total_compute values [[GH-26081](https://github.com/hashicorp/nomad/issues/26081)]
* host volumes: Fixed a bug where volumes with server-terminal allocations could be deleted from clients but not the state store [[GH-26213](https://github.com/hashicorp/nomad/issues/26213)]
* tls: Fixed a bug where reloading the Nomad server process with an updated `tls.verify_server_hostname` configuration parameter would not apply an update to internal RPC handler verification and require a full server restart [[GH-26107](https://github.com/hashicorp/nomad/issues/26107)]
* vault: Fixed a bug where non-periodic tokens would not have their TTL incremented to the lease duration [[GH-26041](https://github.com/hashicorp/nomad/issues/26041)]

## 1.10.2 (June 09, 2025)

BREAKING CHANGES:

* template: Support for the following non-hermetic sprig functions has been removed: sprig_date, sprig_dateInZone, sprig_dateModify, sprig_htmlDate, sprig_htmlDateInZone, sprig_dateInZone, sprig_dateModify, sprig_randAlphaNum, sprig_randAlpha, sprig_randAscii, sprig_randNumeric, sprig_randBytes, sprig_uuidv4, sprig_env, sprig_expandenv, and sprig_getHostByName. [[GH-25998](https://github.com/hashicorp/nomad/issues/25998)]

SECURITY:

* identity: Fixed bug where workflow identity policies are matched by job ID prefix (CVE-2025-4922) [[GH-25869](https://github.com/hashicorp/nomad/issues/25869)]
* template: Bump the consul-template version to resolve CVE-2025-27144, CVE-2025-22869, CVE-2025-22870 and CVE-2025-22872. [[GH-25998](https://github.com/hashicorp/nomad/issues/25998)]
* template: Removed support to the non-hermetic sprig_env, sprig_expandenv, and sprig_getHostByName sprig functions to prevent potential leakage of environment or network information, since they can allow reading environment variables or resolving domain names to IP addresses. [[GH-25998](https://github.com/hashicorp/nomad/issues/25998)]

IMPROVEMENTS:

* cli: Added job start command to allow starting a stopped job from the cli [[GH-24150](https://github.com/hashicorp/nomad/issues/24150)]
* client: Add gc_volumes_on_node_gc configuration to delete host volumes when nodes are garbage collected [[GH-25903](https://github.com/hashicorp/nomad/issues/25903)]
* client: add ability to set maximum allocation count by adding node_max_allocs to client configuration [[GH-25785](https://github.com/hashicorp/nomad/issues/25785)]
* host volumes: Add -force flag to volume delete command for removing volumes from GC'd nodes [[GH-25902](https://github.com/hashicorp/nomad/issues/25902)]
* identity: Allow ACL policies to be applied to a namespace [[GH-25871](https://github.com/hashicorp/nomad/issues/25871)]
* ipv6: bind and advertise addresses are now made to adhere to RFC-5942 §4 (reference: https://www.rfc-editor.org/rfc/rfc5952.html#section-4) [[GH-25921](https://github.com/hashicorp/nomad/issues/25921)]
* reporting (Enterprise): Added support for offline utilization reporting [[GH-25844](https://github.com/hashicorp/nomad/issues/25844)]
* template: adds ability to specify once mode for job templates [[GH-25922](https://github.com/hashicorp/nomad/issues/25922)]
* wi: new API endpoint for listing workload-attached ACL policies [[GH-25588](https://github.com/hashicorp/nomad/issues/25588)]

BUG FIXES:

* api: Fixed pagination bug which could result in duplicate results [[GH-25792](https://github.com/hashicorp/nomad/issues/25792)]
* client: Fixed a bug where disconnect.stop_on_client_after timeouts were extended or ignored [[GH-25946](https://github.com/hashicorp/nomad/issues/25946)]
* csi: Fixed -secret values not being sent with the `nomad volume snapshot delete` command [[GH-26022](https://github.com/hashicorp/nomad/issues/26022)]
* disconnect: Fixed a bug where pending evals for reconnected allocs were not cancelled [[GH-25923](https://github.com/hashicorp/nomad/issues/25923)]
* driver: Allow resources.cpu values above the maximum cpu.share value on Linux [[GH-25963](https://github.com/hashicorp/nomad/issues/25963)]
* job: Ensure sidecar task volume_mounts are added to planning diff object [[GH-25878](https://github.com/hashicorp/nomad/issues/25878)]
* reconnecting client: fix issue where reconcile strategy was sometimes ignored [[GH-25799](https://github.com/hashicorp/nomad/issues/25799)]
* scaling: Set the scaling policies to disabled when a job is stopped [[GH-25911](https://github.com/hashicorp/nomad/issues/25911)]
* scheduler: Fixed a bug where a node with no affinity could be selected over a node with low affinity [[GH-25800](https://github.com/hashicorp/nomad/issues/25800)]
* scheduler: Fixed a bug where planning or running a system job with constraints & previously running allocations would return a failed allocation error [[GH-25850](https://github.com/hashicorp/nomad/issues/25850)]
* telemetry: Fix excess CPU consumption from alloc stats collection [[GH-25870](https://github.com/hashicorp/nomad/issues/25870)]
* telemetry: Fixed a bug where alloc stats were still collected (but not published) if telemetry.publish_allocation_metrics=false. [[GH-25870](https://github.com/hashicorp/nomad/issues/25870)]
* ui: Fix incorrect calculation of permissions when ACLs are disabled which meant actions such as client drains were incorrectly blocked [[GH-25881](https://github.com/hashicorp/nomad/issues/25881)]

## 1.10.1 (May 13, 2025)

BREAKING CHANGES:

* api: The non-functional option -peer-address has been removed from the operator raft remove-peer command and equivalent API [[GH-25599](https://github.com/hashicorp/nomad/issues/25599)]
* core: Errors encountered when reloading agent configuration will now cause agents to exit. Before configuration errors during reloads were only logged. This could lead to agents running but unable to communicate [[GH-25721](https://github.com/hashicorp/nomad/issues/25721)]

SECURITY:

* build: Update Go to 1.24.3 to address CVE-2025-22873 [[GH-25818](https://github.com/hashicorp/nomad/issues/25818)]
* sentinel (Enterprise): Fixed a bug where in some cases hard-mandatory policies could be overridden with -policy-override. CVE-2025-3744 [[GH-2618](https://github.com/hashicorp/nomad-enterprise/pull/2618)]

IMPROVEMENTS:

* command: added priority flag to job dispatch command [[GH-25622](https://github.com/hashicorp/nomad/issues/25622)]

BUG FIXES:

* agent: Fixed a bug where reloading the agent with systemd notification enabled would cause the agent to be killed by system [[GH-25636](https://github.com/hashicorp/nomad/issues/25636)]
* cli: Respect NOMAD_REGION environment variable in operator debug command [[GH-25716](https://github.com/hashicorp/nomad/issues/25716)]
* client: fix failure cleaning up namespace on batch jobs [[GH-25714](https://github.com/hashicorp/nomad/issues/25714)]
* docker: Fix missing stats for rss, cache and swap memory for cgroups v1 [[GH-25741](https://github.com/hashicorp/nomad/issues/25741)]
* encrypter: Refactor startup decryption task handling to avoid timing problems with task addition on FSM restore [[GH-25795](https://github.com/hashicorp/nomad/issues/25795)]
* java: Fixed a bug where the default task user was set to 'nobody' on Windows [[GH-25648](https://github.com/hashicorp/nomad/issues/25648)]
* metrics: Fixed a bug where RSS and cache stats would not be reported for docker, exec, and java drivers under Linux cgroups v2 [[GH-25751](https://github.com/hashicorp/nomad/issues/25751)]
* scheduler: Fixed a bug in accounting for resources.cores that could prevent placements on nodes with available cores [[GH-25705](https://github.com/hashicorp/nomad/issues/25705)]
* scheduler: Fixed a bug where draining a node with canaries could result in a stuck deployment [[GH-25726](https://github.com/hashicorp/nomad/issues/25726)]
* scheduler: Fixed a bug where updating the rescheduler tracker could corrupt the state store [[GH-25698](https://github.com/hashicorp/nomad/issues/25698)]
* scheduler: Use core ID when selecting cores. This fixes a panic in the scheduler when the `reservable_cores` is not a contiguous list of core IDs. [[GH-25340](https://github.com/hashicorp/nomad/issues/25340)]
* server: Added a new server configuration option named `start_timeout` with a default value of `30s`. This duration is used to monitor the server setup and startup processes which must complete before it is considered healthy, such as keyring decryption. If these processes do not complete before the timeout is reached, the server process will exit. [[GH-25803](https://github.com/hashicorp/nomad/issues/25803)]
* ui: Fixed a bug where the job list page incorrectly calculated if a job had paused tasks. [[GH-25742](https://github.com/hashicorp/nomad/issues/25742)]

## 1.10.0 (April 09, 2025)

FEATURES:

* **Dynamic Host Volumes:** Nomad now supports creating host volumes via the API [[GH-24479](https://github.com/hashicorp/nomad/issues/24479)]
* **OIDC Login:** Nomad now enables PKCE for OIDC logins, and supports the private key JWT / client assertion option in the OIDC authentication flow. [[GH-25231](https://github.com/hashicorp/nomad/issues/25231)]
* **Stateful Deployments:** Nomad now supports stateful deployments when using dynamic host volumes. [[GH-24993](https://github.com/hashicorp/nomad/issues/24993)]

BREAKING CHANGES:

* agent: Plugins stored within the `plugin_dir` will now only be executed when they have a corresponding `plugin` configuration block. Any plugin found without a corresponding configuration block will be skipped. [[GH-18530](https://github.com/hashicorp/nomad/issues/18530)]
* api: QuotaSpec.RegionLimit is now of type QuotaResources instead of Resources [[GH-24785](https://github.com/hashicorp/nomad/issues/24785)]
* consul: Identities are no longer added to tasks by default when they include a template block.
Please see [Nomad's upgrade guide](https://developer.hashicorp.com/nomad/docs/upgrade/upgrade-specific)
for more detail. [[GH-25298](https://github.com/hashicorp/nomad/issues/25298)]
* consul: The deprecated token-based authentication workflow for allocations has been removed. Please see [Nomad's upgrade guide](https://developer.hashicorp.com/nomad/docs/upgrade/upgrade-specific) for more detail. [[GH-25217](https://github.com/hashicorp/nomad/issues/25217)]
* disconnected nodes: ignore the previously deprecated disconnect group fields in favor of the disconnect block introduced in Nomad 1.8 [[GH-25284](https://github.com/hashicorp/nomad/issues/25284)]
* drivers: remove remote task support for task drivers [[GH-24909](https://github.com/hashicorp/nomad/issues/24909)]
* sentinel: The sentinel apply command now requires the -scope option [[GH-24601](https://github.com/hashicorp/nomad/issues/24601)]
* vault: The deprecated token-based authentication workflow for allocations has been removed. Please
see [Nomad's upgrade guide](https://developer.hashicorp.com/nomad/docs/upgrade/upgrade-specific) for
more detail. [[GH-25155](https://github.com/hashicorp/nomad/issues/25155)]

IMPROVEMENTS:

* build: Updated Go to 1.24.2 [[GH-25623](https://github.com/hashicorp/nomad/issues/25623)]
* cli: Add -group option to `alloc exec`, `alloc logs`, `alloc fs` commands [[GH-25568](https://github.com/hashicorp/nomad/issues/25568)]
* cli: Added UI URL hints to the end of common CLI commands and a `-ui` flag to auto-open them [[GH-24454](https://github.com/hashicorp/nomad/issues/24454)]
* client: Fixed a bug where JSON formatted logs would not show the requested and overlapping cores when failing to reserve cores [[GH-25523](https://github.com/hashicorp/nomad/issues/25523)]
* client: Improve memory usage by dropping references to task environment [[GH-25373](https://github.com/hashicorp/nomad/issues/25373)]
* cni: Add a warning log when CNI check commands fail [[GH-25581](https://github.com/hashicorp/nomad/issues/25581)]
* csi: Accept ID prefixes and wildcard namespace for the volume delete command [[GH-24997](https://github.com/hashicorp/nomad/issues/24997)]
* csi: Added CSI volume and plugin events to the event stream [[GH-24724](https://github.com/hashicorp/nomad/issues/24724)]
* csi: Show volume capabilities in the volume status command [[GH-25173](https://github.com/hashicorp/nomad/issues/25173)]
* drivers/docker: adds image_pull_timeout to plugin config options [[GH-25489](https://github.com/hashicorp/nomad/issues/25489)]
* drivers/rawexec: adds denied_envvars to driver and task config options [[GH-25511](https://github.com/hashicorp/nomad/issues/25511)]
* rawexec: add support for setting the task user on windows platform [[GH-25496](https://github.com/hashicorp/nomad/issues/25496)]
* rpc: Added ability to configure yamux session parameters [[GH-25466](https://github.com/hashicorp/nomad/issues/25466)]
* ui: Added Dynamic Host Volumes to the web UI [[GH-25224](https://github.com/hashicorp/nomad/issues/25224)]
* ui: Added a scope selector for sentinel policy page [[GH-25390](https://github.com/hashicorp/nomad/issues/25390)]
* ui: Makes jobs list filtering case-insensitive [[GH-25378](https://github.com/hashicorp/nomad/issues/25378)]
* ui: Updated icons to the newest design system [[GH-25353](https://github.com/hashicorp/nomad/issues/25353)]

DEPRECATIONS:

* api: QuotaSpec.VariablesLimit field is deprecated and will be removed in Nomad 1.12.0. Use QuotaSpec.RegionLimit.Storage.Variables instead. [[GH-24785](https://github.com/hashicorp/nomad/issues/24785)]
* quotas: the variables_limit field in the quota specification is deprecated and replaced by a new storage block under the region_limit block, with a variables field. The variables_limit field will be removed in Nomad 1.12.0 [[GH-24785](https://github.com/hashicorp/nomad/issues/24785)]

BUG FIXES:

* client: fixed a bug where AMD CPUs were not correctly fingerprinting base speed [[GH-24415](https://github.com/hashicorp/nomad/issues/24415)]
* client: remove blocking call during client gc [[GH-25123](https://github.com/hashicorp/nomad/issues/25123)]
* client: skip a task groups shutdown_delay when all tasks have already been deregistered [[GH-25157](https://github.com/hashicorp/nomad/issues/25157)]
* csi: Fixed a CSI ExpandVolume bug where the namespace was left out of the staging path [[GH-25253](https://github.com/hashicorp/nomad/issues/25253)]
* csi: Fixed a bug where GC would attempt and fail to delete plugins that had volumes [[GH-25432](https://github.com/hashicorp/nomad/issues/25432)]
* csi: Fixed a bug where cleaning up volume claims on GC'd nodes would cause errors on the leader [[GH-25428](https://github.com/hashicorp/nomad/issues/25428)]
* csi: Fixed a bug where in-flight CSI RPCs would not be cancelled on client GC or dev agent shutdown [[GH-25472](https://github.com/hashicorp/nomad/issues/25472)]
* drivers: set -1 exit code in case of executor failure for the exec, raw_exec, java, and qemu task drivers [[GH-25453](https://github.com/hashicorp/nomad/issues/25453)]
* job: Ensure migrate block difference is added to planning diff object [[GH-25528](https://github.com/hashicorp/nomad/issues/25528)]
* scheduler: Fixed a bug that made affinity and spread updates destructive [[GH-25109](https://github.com/hashicorp/nomad/issues/25109)]
* server: Validate `num_schedulers` configuration parameter is between 0 and the number of CPUs available on the machine [[GH-25441](https://github.com/hashicorp/nomad/issues/25441)]
* services: Fixed a bug where Nomad native services would not be correctly interpolated during in-place updates [[GH-25373](https://github.com/hashicorp/nomad/issues/25373)]
* services: Fixed a bug where task-level services, checks, and identities could interpolate jobspec values from other tasks in the same group [[GH-25373](https://github.com/hashicorp/nomad/issues/25373)]

## Unsupported Versions

Versions of Nomad before 1.10.0 are no longer supported. See [CHANGELOG-unsupported.md](./CHANGELOG-unsupported.md) for their changelogs.
