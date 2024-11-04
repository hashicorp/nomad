## 1.9.1 (October 21, 2024)

IMPROVEMENTS:

* cli: Added synopsis for `operator root` and `operator gossip` command [[GH-23671](https://github.com/hashicorp/nomad/issues/23671)]
* cli: Updated example job specifications in nomad job init [[GH-24232](https://github.com/hashicorp/nomad/issues/24232)]

BUG FIXES:

* consul: Fixed a bug where broken Consul ACL tokens could block registration and deregistration of services and checks [[GH-24166](https://github.com/hashicorp/nomad/issues/24166)]
* consul: Fixed a bug where service deregistration could fail because Consul ACL tokens were revoked during allocation GC [[GH-24166](https://github.com/hashicorp/nomad/issues/24166)]
* docker: Always negotiate API version when initializing clients [[GH-24237](https://github.com/hashicorp/nomad/issues/24237)]
* docker: Fix incorrect auth parsing for private registries [[GH-24215](https://github.com/hashicorp/nomad/issues/24215)]
* docker: Fixed a bug where alloc exec could leak a goroutine [[GH-24244](https://github.com/hashicorp/nomad/issues/24244)]
* docker: Fixed a bug where alloc exec with stdin would hang [[GH-24202](https://github.com/hashicorp/nomad/issues/24202)]
* docker: Fixed a bug where task CPU stats were reported incorrectly [[GH-24229](https://github.com/hashicorp/nomad/issues/24229)]
* heartbeat: Fixed a bug where failed nodes would not be marked down [[GH-24241](https://github.com/hashicorp/nomad/issues/24241)]
* scheduler: fixes reconnecting allocations not getting picked correctly when replacements failed [[GH-24165](https://github.com/hashicorp/nomad/issues/24165)]
* ui: Fix an issue where a dropdown on the variables page would appear underneath table headers [[GH-24162](https://github.com/hashicorp/nomad/issues/24162)]
* ui: Put a max-width on token name so it doesn't collide with the search box in the top nav [[GH-24240](https://github.com/hashicorp/nomad/issues/24240)]
* windows: Fixed a bug where a crashed executor would orphan task processes [[GH-24214](https://github.com/hashicorp/nomad/issues/24214)]

## 1.9.0 (October 10, 2024)

BREAKING CHANGES:

* heartbeats: clients older than 1.6.0 will fail heartbeats to 1.9.0+ servers [[GH-23838](https://github.com/hashicorp/nomad/issues/23838)]
* jobspec: Removed support for HCLv1 [[GH-23912](https://github.com/hashicorp/nomad/issues/23912)]
* services: Clients older than 1.5.0 will fail to read Nomad native services via template blocks [[GH-23910](https://github.com/hashicorp/nomad/issues/23910)]
* tls: Removed deprecated `tls.prefer_server_cipher_suites` field from agent configuration [[GH-23712](https://github.com/hashicorp/nomad/issues/23712)]

SECURITY:

* security: Fixed a bug in client FS API where the check to prevent reads from the secrets dir could be bypassed on case-insensitive file systems [[GH-24125](https://github.com/hashicorp/nomad/issues/24125)]

IMPROVEMENTS:

* cli: Added redaction options to operator snapshot commands [[GH-24023](https://github.com/hashicorp/nomad/issues/24023)]
* cli: Increase default log level and duration when capturing logs with `operator debug` [[GH-23850](https://github.com/hashicorp/nomad/issues/23850)]
* deps: Upgraded yamux to v0.1.2 to fix a bug where RPC connections could deadlock [[GH-24058](https://github.com/hashicorp/nomad/issues/24058)]
* docker: Use official docker SDK instead of a 3rd party client [[GH-23966](https://github.com/hashicorp/nomad/issues/23966)]
* identity: Added filepath parameter to identity block for persisting workload identities [[GH-24038](https://github.com/hashicorp/nomad/issues/24038)]
* jobs: Added Version Tags to job versions, to prevent them from being garbage collected and allow for diffs [[GH-24055](https://github.com/hashicorp/nomad/issues/24055)]
* keyring: Stored wrapped data encryption keys in Raft [[GH-23977](https://github.com/hashicorp/nomad/issues/23977)]
* metrics: introduce client config to include alloc metadata as part of the base labels [[GH-23964](https://github.com/hashicorp/nomad/issues/23964)]
* networking: Added an option to ignore static port collisions when scheduling, for programs that use the SO_REUSEPORT unix socket option [[GH-23956](https://github.com/hashicorp/nomad/issues/23956)]
* networking: IPv6 can now be enabled on the Nomad bridge network mode [[GH-23882](https://github.com/hashicorp/nomad/issues/23882)]
* quotas (Enterprise): Added the possibility to set device count limits [[GH-23894](https://github.com/hashicorp/nomad/issues/23894)]
* raft: Bump raft to v1.7.1 which includes pre-vote. This should make servers more stable after network partitions [[GH-24029](https://github.com/hashicorp/nomad/issues/24029)]

BUG FIXES:

* bug: Allow client template config block to be parsed when using json config [[GH-24007](https://github.com/hashicorp/nomad/issues/24007)]
* cli: Fixed a bug in job status command where -t would act as though -json was also set [[GH-24054](https://github.com/hashicorp/nomad/issues/24054)]
* scaling: Fixed a bug where scaling policies would not get created during job submission unless namespace field was set in jobspec [[GH-24065](https://github.com/hashicorp/nomad/issues/24065)]
* state: Fixed a bug where compatibility updates for node topology for nodes older than 1.7.0 were not being correctly applied [[GH-24127](https://github.com/hashicorp/nomad/issues/24127)]
* task: adds node.pool attribute to interpretable values in task env [[GH-24052](https://github.com/hashicorp/nomad/issues/24052)]
* template: Fixed a panic on client restart when using change_mode=script [[GH-24057](https://github.com/hashicorp/nomad/issues/24057)]
* ui: Fixes an issue where variables paths would not let namespaced users write variables unless they also had wildcard namespace variable write permissions [[GH-24073](https://github.com/hashicorp/nomad/issues/24073)]

## 1.8.6 Enterprise(October 21, 2024)

IMPROVEMENTS:

* cli: Added synopsis for `operator root` and `operator gossip` command [[GH-23671](https://github.com/hashicorp/nomad/issues/23671)]

BUG FIXES:

* consul: Fixed a bug where broken Consul ACL tokens could block registration and deregistration of services and checks [[GH-24166](https://github.com/hashicorp/nomad/issues/24166)]
* consul: Fixed a bug where service deregistration could fail because Consul ACL tokens were revoked during allocation GC [[GH-24166](https://github.com/hashicorp/nomad/issues/24166)]
* deps: Fixed a bug where restarting Nomad could cause an unrelated process with the same PID as a failed executor to be killed [[GH-24265](https://github.com/hashicorp/nomad/issues/24265)]
* scheduler: fixes reconnecting allocations not getting picked correctly when replacements failed [[GH-24165](https://github.com/hashicorp/nomad/issues/24165)]
* windows: Fixed a bug where a crashed executor would orphan task processes [[GH-24214](https://github.com/hashicorp/nomad/issues/24214)]

## 1.8.5 Enterprise (October 10, 2024)

SECURITY:

* security: Fixed a bug in client FS API where the check to prevent reads from the secrets dir could be bypassed on case-insensitive file systems [[GH-24125](https://github.com/hashicorp/nomad/issues/24125)]

IMPROVEMENTS:

* cli: Increase default log level and duration when capturing logs with `operator debug` [[GH-23850](https://github.com/hashicorp/nomad/issues/23850)]

BUG FIXES:

* bug: Allow client template config block to be parsed when using json config [[GH-24007](https://github.com/hashicorp/nomad/issues/24007)]
* cli: Fixed a bug in job status command where -t would act as though -json was also set [[GH-24054](https://github.com/hashicorp/nomad/issues/24054)]
* licensing: Fixed a bug where environment variable to opt-out of reporting was not respected
* scaling: Fixed a bug where scaling policies would not get created during job submission unless namespace field was set in jobspec [[GH-24065](https://github.com/hashicorp/nomad/issues/24065)]
* state: Fixed a bug where compatibility updates for node topology for nodes older than 1.7.0 were not being correctly applied [[GH-24127](https://github.com/hashicorp/nomad/issues/24127)]
* task: adds node.pool attribute to interpretable values in task env [[GH-24052](https://github.com/hashicorp/nomad/issues/24052)]
* template: Fixed a panic on client restart when using change_mode=script [[GH-24057](https://github.com/hashicorp/nomad/issues/24057)]

## 1.8.4 (September 17, 2024)

BREAKING CHANGES:

* docker: The default infra_image for pause containers is now registry.k8s.io/pause [[GH-23927](https://github.com/hashicorp/nomad/issues/23927)]

IMPROVEMENTS:

* build: update to go1.22.6 [[GH-23805](https://github.com/hashicorp/nomad/issues/23805)]
* cgroups: Allow clients with delegated cgroups check that required cgroup v2 controllers exist [[GH-23803](https://github.com/hashicorp/nomad/issues/23803)]
* docker: Disable cpuset management for non-root clients [[GH-23804](https://github.com/hashicorp/nomad/issues/23804)]
* identity: Added support for server-configured additional claims on the Vault default_identity block [[GH-23675](https://github.com/hashicorp/nomad/issues/23675)]
* namespaces: Allow enabling/disabling allowed network modes per namespace [[GH-23813](https://github.com/hashicorp/nomad/issues/23813)]
* ui: Badge added for Scaled Down jobs [[GH-23829](https://github.com/hashicorp/nomad/issues/23829)]

DEPRECATIONS:

* api: the JobParseRequest.HCLv1 field will be removed in Nomad 1.9.0 [[GH-23913](https://github.com/hashicorp/nomad/issues/23913)]
* jobspec: using the -hcl1 flag for HCLv1 job specifications will now emit a warning at the command line. This feature will be removed in Nomad 1.9.0 [[GH-23913](https://github.com/hashicorp/nomad/issues/23913)]

BUG FIXES:

* identity: Fixed a bug where dispatch and periodic jobs would have their job ID and not parent job ID used when creating the subject claim [[GH-23902](https://github.com/hashicorp/nomad/issues/23902)]
* identity: Fixed a bug where dispatch and periodic jobs would have their job ID and not parent job ID used when interpolating vault.default_identity.extra_claims [[GH-23817](https://github.com/hashicorp/nomad/issues/23817)]
* node: Fixed bug where sysbatch allocations were started prematurely [[GH-23858](https://github.com/hashicorp/nomad/issues/23858)]
* ui: Fix an issue where cmd+click or ctrl+click would double-open a job [[GH-23832](https://github.com/hashicorp/nomad/issues/23832)]

## 1.8.3 (August 13, 2024)

SECURITY:

* security: Fix symlink escape during unarchiving by removing existing paths within the same allocdir. Compromising the Nomad client agent at the source allocation first is a prerequisite for leveraging this issue. [[GH-23738](https://github.com/hashicorp/nomad/issues/23738)]

IMPROVEMENTS:

* acl: Submitting a policy with a leading `/` in a variable path will now return an error to prevent improperly working policies. [[GH-23757](https://github.com/hashicorp/nomad/issues/23757)]
* cli: Added option to return original HCL in `job inspect` command [[GH-23699](https://github.com/hashicorp/nomad/issues/23699)]
* cli: Added support for updating the roles for an ACL token [[GH-18532](https://github.com/hashicorp/nomad/issues/18532)]
* cli: `acl token create` will now emit a warning if the token has a policy that does not yet exist [[GH-16437](https://github.com/hashicorp/nomad/issues/16437)]
* keyring: Added support for encrypting the keyring via Vault transit or external KMS [[GH-23580](https://github.com/hashicorp/nomad/issues/23580)]
* keyring: Added support for prepublishing keys [[GH-23577](https://github.com/hashicorp/nomad/issues/23577)]
* identity: Added support for server-configured additional claims on the Vault default_identity block [[GH-23675](https://github.com/hashicorp/nomad/issues/23675)]
* metrics: Added `client.tasks` metrics to track task states [[GH-23773](https://github.com/hashicorp/nomad/issues/23773)]
* resources: Added `resources.secrets` field to configure size of secrets directory on Linux [[GH-23696](https://github.com/hashicorp/nomad/issues/23696)]
* tls: Allow setting the `tls_min_version` field to `"tls13"` [[GH-23713](https://github.com/hashicorp/nomad/issues/23713)]
* ui: added a Pack badge to the jobs index page for jobs run via Nomad Pack [[GH-23404](https://github.com/hashicorp/nomad/issues/23404)]

BUG FIXES:

* api: Fixed a bug where an `api.Config` targeting a unix domain socket could not be reused between clients [[GH-23785](https://github.com/hashicorp/nomad/issues/23785)]
* cni: .conf and .json config files are now parsed properly [[GH-23629](https://github.com/hashicorp/nomad/issues/23629)]
* cni: network.cni jobspec updates now replace allocs to apply the new network config [[GH-23764](https://github.com/hashicorp/nomad/issues/23764)]
* docker: Fixed a bug where plugin SELinux labels would conflict with read-only `volume` options [[GH-23750](https://github.com/hashicorp/nomad/issues/23750)]
* identity: Fixed a bug where a missing default task identity could panic the leader [[GH-23763](https://github.com/hashicorp/nomad/issues/23763)]
* keyring: Fixed a bug where keys could be garbage collected before workload identities expire [[GH-23577](https://github.com/hashicorp/nomad/issues/23577)]
* keyring: Fixed a bug where keys would never exit the "rekeying" state after a rotation with the `-full` flag [[GH-23577](https://github.com/hashicorp/nomad/issues/23577)]
* keyring: Fixed a bug where periodic key rotation would not occur [[GH-23577](https://github.com/hashicorp/nomad/issues/23577)]
* networking: The same static port can now be used more than once on host networks with multiple IPs [[GH-23693](https://github.com/hashicorp/nomad/issues/23693)]
* scaling: Fixed a bug where state store corruption could occur when writing scaling events [[GH-23673](https://github.com/hashicorp/nomad/issues/23673)]
* template: Fixed a bug where change_mode = "script" would not execute after a client restart [[GH-23663](https://github.com/hashicorp/nomad/issues/23663)]
* ui: Fixed storage/plugin 404s by unescaping a slash character in the request URL [[GH-23625](https://github.com/hashicorp/nomad/issues/23625)]
* windows: Fix bug with containers capabilities on Docker CE [[GH-23599](https://github.com/hashicorp/nomad/issues/23599)]

## 1.8.2 (July 16, 2024)

BREAKING CHANGES:

* docker: default to hyper-v isolation mode on Windows [[GH-23452](https://github.com/hashicorp/nomad/issues/23452)]

SECURITY:

* build: Updated Go to 1.22.5 to address CVE-2024-24791 [[GH-23498](https://github.com/hashicorp/nomad/issues/23498)]
* migration: Added a check for relative paths escaping the allocation directory when unpacking archive during migration, to harden clients against compromised peer clients sending malicious archives [[GH-23319](https://github.com/hashicorp/nomad/issues/23319)]
* security: Removed insecure TLS cipher suites: `TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256`, `TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA25` and `TLS_RSA_WITH_AES_128_CBC_SHA256`. [[GH-23551](https://github.com/hashicorp/nomad/issues/23551)]

IMPROVEMENTS:

* client: add a preferred_address_family config to prefer ipv4 or ipv6 when deducing IP from network interface [[GH-23389](https://github.com/hashicorp/nomad/issues/23389)]
* cni: allow users to input CNI args in job specification [[GH-23538](https://github.com/hashicorp/nomad/issues/23538)]
* deps: Updated Consul API to 1.29.1. [[GH-23436](https://github.com/hashicorp/nomad/issues/23436)]
* deps: Updated consul-template to 0.39 to allow admin partition and sameness groups queries. [[GH-23436](https://github.com/hashicorp/nomad/issues/23436)]
* docker: Validate that unprivileged containers aren't running as ContainerAdmin on Windows [[GH-23443](https://github.com/hashicorp/nomad/issues/23443)]
* namespaces: Added warnings if deleting namespaces that have existing objects associated with them [[GH-23499](https://github.com/hashicorp/nomad/issues/23499)]
* quota (Enterprise): Allow CPU cores to be configured within a quota [[GH-23543](https://github.com/hashicorp/nomad/issues/23543)]
* scaling: Added `-check-index` support to `job scale` command [[GH-23457](https://github.com/hashicorp/nomad/issues/23457)]
* ui: Allow users to create Global ACL tokens from the Administration UI [[GH-23506](https://github.com/hashicorp/nomad/issues/23506)]
* ui: Update headers in the Admin section to use the HashiCorp Design System [[GH-23366](https://github.com/hashicorp/nomad/issues/23366)]
* ui: allow for multiple namespaces in jobs index filters [[GH-23468](https://github.com/hashicorp/nomad/issues/23468)]

BUG FIXES:

* api: Fixed bug where newlines in JobSubmission vars weren't encoded correctly [[GH-23560](https://github.com/hashicorp/nomad/issues/23560)]
* cli: Fixed bug where the `plugin status` command would fail if the plugin ID was a prefix of another plugin ID [[GH-23502](https://github.com/hashicorp/nomad/issues/23502)]
* cli: Fixed bug where the `quota status` and `quota inspect` commands would fail if the quota name was a prefix of another quota name [[GH-23502](https://github.com/hashicorp/nomad/issues/23502)]
* cli: Fixed bug where the `scaling policy info` command would fail if the policy ID was a prefix of another policy ID [[GH-23502](https://github.com/hashicorp/nomad/issues/23502)]
* cli: Fixed bug where the `service info` command would fail if the service name was a prefix of another service name in the same namespace [[GH-23502](https://github.com/hashicorp/nomad/issues/23502)]
* cli: Fixed bug where the `volume deregister`, `volume detach`, and `volume status` commands would fail if the volume ID was a prefix of another volume ID in the same namespace [[GH-23502](https://github.com/hashicorp/nomad/issues/23502)]
* consul: Fixed a bug where service registration and Envoy bootstrap would not wait for Consul ACL tokens and services to be replicated to the local agent [[GH-23381](https://github.com/hashicorp/nomad/issues/23381)]
* plugins: Fix panic on systems that don't support NUMA [[GH-23399](https://github.com/hashicorp/nomad/issues/23399)]
* qemu: Fixed a bug that prevented `qemu` tasks from running on Linux [[GH-23466](https://github.com/hashicorp/nomad/issues/23466)]
* quota (Enterprise): Fixed a bug where a task's resource core count was not translated to CPU MHz and checked against its quota when performing a job plan [[GH-18876](https://github.com/hashicorp/nomad/issues/18876)]
* scheduler: Fix a bug where reserved resources are not calculated correctly [[GH-23386](https://github.com/hashicorp/nomad/issues/23386)]
* server: Fixed a bug where expiring heartbeats for garbage collected nodes could panic the server [[GH-23383](https://github.com/hashicorp/nomad/issues/23383)]
* template: Fix template rendering on Windows [[GH-23432](https://github.com/hashicorp/nomad/issues/23432)]
* ui: Actions run from jobs with explicit name properties now work from the web UI [[GH-23553](https://github.com/hashicorp/nomad/issues/23553)]
* ui: Dont show keyboard nav hints when taking a screenshot [[GH-23365](https://github.com/hashicorp/nomad/issues/23365)]
* ui: Fix an issue where a remotely purged job would prevent redirect from taking place in the web UI [[GH-23492](https://github.com/hashicorp/nomad/issues/23492)]
* ui: Fix an issue where access to Job Templates in the UI was restricted to variable.write access [[GH-23458](https://github.com/hashicorp/nomad/issues/23458)]
* ui: Fix the Upload Jobspec button on the Run Job page [[GH-23548](https://github.com/hashicorp/nomad/issues/23548)]
* ui: Fixed support for namespace parameter on job statuses API [[GH-23456](https://github.com/hashicorp/nomad/issues/23456)]
* ui: fix an issue where gateway timeouts would cause the jobs list to revert to null, gives users a Pause Fetch option [[GH-23427](https://github.com/hashicorp/nomad/issues/23427)]
* vault: Fixed a bug where requests to derive or renew tokens could be sent to the wrong namespace [[GH-23491](https://github.com/hashicorp/nomad/issues/23491)]

## 1.8.1 (June 19, 2024)

SECURITY:

* build: Updated Go to 1.22.4 to address Go stdlib vulnerabilities CVE-2024-24789 and CVE-2024-24790 [[GH-23172](https://github.com/hashicorp/nomad/issues/23172)]

IMPROVEMENTS:

* api: Add support for setting Notes field for Consul health checks [[GH-22397](https://github.com/hashicorp/nomad/issues/22397)]
* cli: `operator snapshot inspect` now includes details of data in snapshot [[GH-18372](https://github.com/hashicorp/nomad/issues/18372)]
* docker: Added container_exists_attempts plugin configuration variable [[GH-22419](https://github.com/hashicorp/nomad/issues/22419)]
* docker: Added support for oom_score_adj [[GH-23297](https://github.com/hashicorp/nomad/issues/23297)]
* exec: Fixed a bug where `exec` driver tasks would fail on older versions of glibc [[GH-23331](https://github.com/hashicorp/nomad/issues/23331)]
* metrics (Enterprise): Publish quota utilization as metrics [[GH-22912](https://github.com/hashicorp/nomad/issues/22912)]
* raw_exec: Added support for oom_score_adj [[GH-23308](https://github.com/hashicorp/nomad/issues/23308)]
* ui: adds a Stopped label for jobs that a user has manually stopped [[GH-23328](https://github.com/hashicorp/nomad/issues/23328)]
* ui: namespace dropdown gets a search field and supports many namespaces [[GH-20626](https://github.com/hashicorp/nomad/issues/20626)]
* ui: shorten client/node metadata/attributes display and make parent-terminal attributes show up [[GH-23290](https://github.com/hashicorp/nomad/issues/23290)]

BUG FIXES:

* acl: Fix plugin policy validation when checking write permissions [[GH-23274](https://github.com/hashicorp/nomad/issues/23274)]
* api: (Enterprise) fixed Allocations.GetPauseState method discarding the task argument [[GH-23377](https://github.com/hashicorp/nomad/issues/23377)]
* client: Fixed a bug where empty task directories would be left behind [[GH-23237](https://github.com/hashicorp/nomad/issues/23237)]
* connect: fix validation with multiple socket paths [[GH-22312](https://github.com/hashicorp/nomad/issues/22312)]
* consul: (Enterprise) Fixed a bug where gateway config entries were written before Sentinel policies were enforced [[GH-22228](https://github.com/hashicorp/nomad/issues/22228)]
* consul: Fixed a bug where Consul admin partition was not used to login via Consul JWT auth method [[GH-22226](https://github.com/hashicorp/nomad/issues/22226)]
* consul: Fixed a bug where gateway config entries were written to the Nomad server agent's Consul partition and not the client's partition [[GH-22228](https://github.com/hashicorp/nomad/issues/22228)]
* driver: Fixed a bug where the exec, java, and raw_exec drivers would not configure cgroups to allow access to devices provided by device plugins [[GH-22518](https://github.com/hashicorp/nomad/issues/22518)]
* scheduler: Fixed a bug where rescheduled allocations that could not be placed would later ignore their reschedule policy limits [[GH-12319](https://github.com/hashicorp/nomad/issues/12319)]
* task schedule: Fixed a bug where schedules wrongly errored as invalid on the last day of the month [[GH-23329](https://github.com/hashicorp/nomad/issues/23329)]
* ui: unbind job detail running allocations count from job-summary endpoint [[GH-23306](https://github.com/hashicorp/nomad/issues/23306)]

## 1.8.0 (May 28, 2024)

IMPROVEMENTS:

* agent: Added support for systemd readiness notifications [[GH-20528](https://github.com/hashicorp/nomad/issues/20528)]
* api: new /v1/jobs/statuses endpoint collates details about jobs' allocs and latest deployment, intended for use in the updated UI jobs index page [[GH-20130](https://github.com/hashicorp/nomad/issues/20130)]
* artifact: Added support for downloading artifacts without validating the TLS certificate [[GH-20126](https://github.com/hashicorp/nomad/issues/20126)]
* autopilot: Added `operator autopilot health` command to review Autopilot health data [[GH-20156](https://github.com/hashicorp/nomad/issues/20156)]
* cli: Add `-jwks-ca-file` argument to `setup consul/vault` commands [[GH-20518](https://github.com/hashicorp/nomad/issues/20518)]
* client/volumes: Add a mount volume level option for selinux tags on volumes [[GH-19839](https://github.com/hashicorp/nomad/issues/19839)]
* client: expose network namespace bridge/cni configuration values as task env vars [[GH-11810](https://github.com/hashicorp/nomad/issues/11810)]
* connect: Added support for `volume_mount` blocks on sidecar task overrides [[GH-20575](https://github.com/hashicorp/nomad/issues/20575)]
* consul/connect: Attempt autodetection of podman task driver for Connect gateways [[GH-20611](https://github.com/hashicorp/nomad/issues/20611)]
* consul: provide tasks that have Consul tokens the CONSUL_HTTP_TOKEN environment variable [[GH-20519](https://github.com/hashicorp/nomad/issues/20519)]
* core: Do not create evaluations within batch deregister endpoint during job garbage collection [[GH-20510](https://github.com/hashicorp/nomad/issues/20510)]
* csi: Added support for wildcard namespace to `plugin status` command [[GH-20551](https://github.com/hashicorp/nomad/issues/20551)]
* deps: Update msgpack to v2 [[GH-20173](https://github.com/hashicorp/nomad/issues/20173)]
* deps: Updated `docker` dependency to 26.0.1 [[GH-20389](https://github.com/hashicorp/nomad/issues/20389)]
* driver/rawexec: Allow specifying custom cgroups [[GH-20481](https://github.com/hashicorp/nomad/issues/20481)]
* func: Allow custom paths to be added the the getter landlock [[GH-20315](https://github.com/hashicorp/nomad/issues/20315)]
* jobspec: Add a schedule{} block for time based task execution (Enterprise) [[GH-22201](https://github.com/hashicorp/nomad/issues/22201)]
* metrics: Added tracking of enqueue and dequeue times of evaluations to the broker [[GH-20329](https://github.com/hashicorp/nomad/issues/20329)]
* networking: Inject constraints on CNI plugins when using bridge networking [[GH-15473](https://github.com/hashicorp/nomad/issues/15473)]
* scheduler: Added a new configuration to avoid rescheduling allocations if a nodes misses one or more heartbits [[GH-19101](https://github.com/hashicorp/nomad/issues/19101)]
* server: Add new options for reconcilation in case of disconnected nodes [[GH-20029](https://github.com/hashicorp/nomad/issues/20029)]
* ui: Added a UI for creating, editing and deleting Sentinel Policies [[GH-20483](https://github.com/hashicorp/nomad/issues/20483)]
* ui: Added a copy button on Action output [[GH-19496](https://github.com/hashicorp/nomad/issues/19496)]
* ui: Added a new UI block to job spec in order to provide description and links in the Web UI [[GH-18292](https://github.com/hashicorp/nomad/issues/18292)]
* ui: Added token.name information to the top nav for ease of operator debugging [[GH-20539](https://github.com/hashicorp/nomad/issues/20539)]
* ui: Improve error and warning messages for invalid variable and job template paths/names [[GH-19989](https://github.com/hashicorp/nomad/issues/19989)]
* ui: Overhaul of the Jobs Index list page, with live updates, more informative statuses, filter expressions, and pagination [[GH-20452](https://github.com/hashicorp/nomad/issues/20452)]
* ui: Prompt a user before they close an exec window to prevent accidental close-browser-tab shortcuts that overlap with terminal ones [[GH-19985](https://github.com/hashicorp/nomad/issues/19985)]
* ui: Replaced single-line variable value fields with multi-line textarea blocks [[GH-19544](https://github.com/hashicorp/nomad/issues/19544)]
* ui: Updated the style of components in the Variables web ui [[GH-19544](https://github.com/hashicorp/nomad/issues/19544)]
* ui: change the State filter on clients page to split out eligibility and drain status [[GH-18607](https://github.com/hashicorp/nomad/issues/18607)]

BUG FIXES:

* cli: Fix handling of scaling jobs which don't generate evals [[GH-20479](https://github.com/hashicorp/nomad/issues/20479)]
* client: Fix unallocated CPU metric calculation when client reserved CPU is set [[GH-20543](https://github.com/hashicorp/nomad/issues/20543)]
* client: terminate old exec task processes before starting new ones, to avoid accidentally leaving running processes in case of an error [[GH-20500](https://github.com/hashicorp/nomad/issues/20500)]
* config: Fixed a panic triggered by registering a job specifying a Vault cluster that has not been configured within the server [[GH-22227](https://github.com/hashicorp/nomad/issues/22227)]
* core: Fix multiple incorrect type conversion for potential overflows [[GH-20553](https://github.com/hashicorp/nomad/issues/20553)]
* csi: Fixed a bug where concurrent mount and unmount operations could unstage volumes needed by another allocation [[GH-20550](https://github.com/hashicorp/nomad/issues/20550)]
* csi: Fixed a bug where plugins would not be deleted on GC if their job updated the plugin ID [[GH-20555](https://github.com/hashicorp/nomad/issues/20555)]
* csi: Fixed a bug where volumes in different namespaces but the same ID would fail to stage on the same client [[GH-20532](https://github.com/hashicorp/nomad/issues/20532)]
* job endpoint: fix implicit constraint mutation for task-level services [[GH-22229](https://github.com/hashicorp/nomad/issues/22229)]
* quota (Enterprise): Fixed a bug where quota usage would not be freed if a job was purged
* services: Added retry to Nomad service deregistration RPCs during alloc stop [[GH-20596](https://github.com/hashicorp/nomad/issues/20596)]
* services: Fixed bug where Nomad services might not be deregistered when nodes are marked down or allocations are terminal [[GH-20590](https://github.com/hashicorp/nomad/issues/20590)]
* structs: Fix job canonicalization for array type fields [[GH-20522](https://github.com/hashicorp/nomad/issues/20522)]
* ui: Fix a bug where the UI would prompt a user to promote a deployment with unplaced canaries [[GH-20408](https://github.com/hashicorp/nomad/issues/20408)]
* ui: Fixed an issue where keynav would not trigger evaluation sidebar expand [[GH-20047](https://github.com/hashicorp/nomad/issues/20047)]
* ui: Show the namespace in the web UI exec command hint [[GH-20218](https://github.com/hashicorp/nomad/issues/20218)]
* windows: Fixed a regression where scanning task processes was inefficient [[GH-20619](https://github.com/hashicorp/nomad/issues/20619)]

## 1.7.14 Enterprise (October 21, 2024)

IMPROVEMENTS:

* cli: Added synopsis for `operator root` and `operator gossip` command [[GH-23671](https://github.com/hashicorp/nomad/issues/23671)]

BUG FIXES:

* consul: Fixed a bug where broken Consul ACL tokens could block registration and deregistration of services and checks [[GH-24166](https://github.com/hashicorp/nomad/issues/24166)]
* consul: Fixed a bug where service deregistration could fail because Consul ACL tokens were revoked during allocation GC [[GH-24166](https://github.com/hashicorp/nomad/issues/24166)]
* plugins: Fix panic on systems that don't support NUMA [[GH-23399](https://github.com/hashicorp/nomad/issues/23399)]
* scheduler: fixes reconnecting allocations not getting picked correctly when replacements failed [[GH-24165](https://github.com/hashicorp/nomad/issues/24165)]
* windows: Fixed a bug where a crashed executor would orphan task processes [[GH-24214](https://github.com/hashicorp/nomad/issues/24214)]

## 1.7.13 Enterprise (October 10, 2024)

SECURITY:

* security: Fixed a bug in client FS API where the check to prevent reads from the secrets dir could be bypassed on case-insensitive file systems [[GH-24125](https://github.com/hashicorp/nomad/issues/24125)]

BUG FIXES:

* bug: Allow client template config block to be parsed when using json config [[GH-24007](https://github.com/hashicorp/nomad/issues/24007)]
* cli: Fixed a bug in job status command where -t would act as though -json was also set [[GH-24054](https://github.com/hashicorp/nomad/issues/24054)]
* licensing: Fixed a bug where environment variable to opt-out of reporting was not respected
* scaling: Fixed a bug where scaling policies would not get created during job submission unless namespace field was set in jobspec [[GH-24065](https://github.com/hashicorp/nomad/issues/24065)]
* state: Fixed a bug where compatibility updates for node topology for nodes older than 1.7.0 were not being correctly applied [[GH-24127](https://github.com/hashicorp/nomad/issues/24127)]
* template: Fixed a panic on client restart when using change_mode=script [[GH-24057](https://github.com/hashicorp/nomad/issues/24057)]

## 1.7.12 Enterprise (September 17, 2024)

BREAKING CHANGES:

* docker: The default infra_image for pause containers is now registry.k8s.io/pause [[GH-23927](https://github.com/hashicorp/nomad/issues/23927)]

IMPROVEMENTS:

* build: update to go1.22.6 [[GH-23805](https://github.com/hashicorp/nomad/issues/23805)]

BUG FIXES:

* node: Fixed bug where sysbatch allocations were started prematurely [[GH-23858](https://github.com/hashicorp/nomad/issues/23858)]

## 1.7.11 Enterprise (August 13, 2024)

SECURITY:

* security: Fix symlink escape during unarchiving by removing existing paths within the same allocdir. Compromising the Nomad client agent at the source allocation first is a prerequisite for leveraging this issue. [[GH-23738](https://github.com/hashicorp/nomad/issues/23738)]

IMPROVEMENTS:

* keyring: Added support for prepublishing keys [[GH-23577](https://github.com/hashicorp/nomad/issues/23577)]

BUG FIXES:

* api: Fixed a bug where an `api.Config` targeting a unix domain socket could not be reused between clients [[GH-23785](https://github.com/hashicorp/nomad/issues/23785)]
* cni: .conf and .json config files are now parsed properly [[GH-23629](https://github.com/hashicorp/nomad/issues/23629)]
* docker: Fixed a bug where plugin SELinux labels would conflict with read-only `volume` options [[GH-23750](https://github.com/hashicorp/nomad/issues/23750)]
* identity: Fixed a bug where a missing default task identity could panic the leader [[GH-23763](https://github.com/hashicorp/nomad/issues/23763)]
* keyring: Fixed a bug where keys could be garbage collected before workload identities expire [[GH-23577](https://github.com/hashicorp/nomad/issues/23577)]
* keyring: Fixed a bug where keys would never exit the "rekeying" state after a rotation with the `-full` flag [[GH-23577](https://github.com/hashicorp/nomad/issues/23577)]
* keyring: Fixed a bug where periodic key rotation would not occur [[GH-23577](https://github.com/hashicorp/nomad/issues/23577)]
* networking: The same static port can now be used more than once on host networks with multiple IPs [[GH-23693](https://github.com/hashicorp/nomad/issues/23693)]
* scaling: Fixed a bug where state store corruption could occur when writing scaling events [[GH-23673](https://github.com/hashicorp/nomad/issues/23673)]
* template: Fixed a bug where change_mode = "script" would not execute after a client restart [[GH-23663](https://github.com/hashicorp/nomad/issues/23663)]
* windows: Fix bug with containers capabilities on Docker CE [[GH-23599](https://github.com/hashicorp/nomad/issues/23599)]

## 1.7.10 Enterprise (July 16, 2024)

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
* consul: Fixed a bug where service registration and Envoy bootstrap would not wait for Consul ACL tokens and services to be replicated to the local agent [[GH-23381](https://github.com/hashicorp/nomad/issues/23381)]
* qemu: Fixed a bug that prevented `qemu` tasks from running on Linux [[GH-23466](https://github.com/hashicorp/nomad/issues/23466)]
* quota (Enterprise): Fixed a bug where a task's resource core count was not translated to CPU MHz and checked against its quota when performing a job plan [[GH-18876](https://github.com/hashicorp/nomad/issues/18876)]
* scheduler: Fix a bug where reserved resources are not calculated correctly [[GH-23386](https://github.com/hashicorp/nomad/issues/23386)]
* server: Fixed a bug where expiring heartbeats for garbage collected nodes could panic the server [[GH-23383](https://github.com/hashicorp/nomad/issues/23383)]
* template: Fix template rendering on Windows [[GH-23432](https://github.com/hashicorp/nomad/issues/23432)]

## 1.7.9 Enterprise (June 19, 2024)

SECURITY:

* build: Updated Go to 1.22.4 to address Go stdlib vulnerabilities CVE-2024-24789 and CVE-2024-24790 [[GH-23172](https://github.com/hashicorp/nomad/issues/23172)]

IMPROVEMENTS:

* cli: `operator snapshot inspect` now includes details of data in snapshot [[GH-18372](https://github.com/hashicorp/nomad/issues/18372)]
* docker: Added container_exists_attempts plugin configuration variable [[GH-22419](https://github.com/hashicorp/nomad/issues/22419)]
* exec: Fixed a bug where `exec` driver tasks would fail on older versions of glibc [[GH-23331](https://github.com/hashicorp/nomad/issues/23331)]

BUG FIXES:

* acl: Fix plugin policy validation when checking write permissions [[GH-23274](https://github.com/hashicorp/nomad/issues/23274)]
* connect: fix validation with multiple socket paths [[GH-22312](https://github.com/hashicorp/nomad/issues/22312)]
* consul: (Enterprise) Fixed a bug where gateway config entries were written before Sentinel policies were enforced [[GH-22228](https://github.com/hashicorp/nomad/issues/22228)]
* consul: Fixed a bug where Consul admin partition was not used to login via Consul JWT auth method [[GH-22226](https://github.com/hashicorp/nomad/issues/22226)]
* consul: Fixed a bug where gateway config entries were written to the Nomad server agent's Consul partition and not the client's partition [[GH-22228](https://github.com/hashicorp/nomad/issues/22228)]
* driver: Fixed a bug where the exec, java, and raw_exec drivers would not configure cgroups to allow access to devices provided by device plugins [[GH-22518](https://github.com/hashicorp/nomad/issues/22518)]
* scheduler: Fixed a bug where rescheduled allocations that could not be placed would later ignore their reschedule policy limits [[GH-12319](https://github.com/hashicorp/nomad/issues/12319)]

## 1.7.8 Enterprise (May 28, 2024)

SECURITY:

* deps: Updated `docker` dependency to 25.0.5 [[GH-20171](https://github.com/hashicorp/nomad/issues/20171)]

IMPROVEMENTS:

* auth: Add support for authenticating via Workload Identity to the quota and sentinel APIs
* autopilot: Added `operator autopilot health` command to review Autopilot health data [[GH-20156](https://github.com/hashicorp/nomad/issues/20156)]
* cli: Add `-jwks-ca-file` argument to `setup consul/vault` commands [[GH-20518](https://github.com/hashicorp/nomad/issues/20518)]
* client/volumes: Add a mount volume level option for selinux tags on volumes [[GH-19839](https://github.com/hashicorp/nomad/issues/19839)]
* consul: provide tasks that have Consul tokens the CONSUL_HTTP_TOKEN environment variable [[GH-20519](https://github.com/hashicorp/nomad/issues/20519)]
* ui: Improve error and warning messages for invalid variable and job template paths/names [[GH-19989](https://github.com/hashicorp/nomad/issues/19989)]
* ui: Prompt a user before they close an exec window to prevent accidental close-browser-tab shortcuts that overlap with terminal ones [[GH-19985](https://github.com/hashicorp/nomad/issues/19985)]

BUG FIXES:

* cli: Fix handling of scaling jobs which don't generate evals [[GH-20479](https://github.com/hashicorp/nomad/issues/20479)]
* client: Fix unallocated CPU metric calculation when client reserved CPU is set [[GH-20543](https://github.com/hashicorp/nomad/issues/20543)]
* client: terminate old exec task processes before starting new ones, to avoid accidentally leaving running processes in case of an error [[GH-20500](https://github.com/hashicorp/nomad/issues/20500)]
* config: Fixed a panic triggered by registering a job specifying a Vault cluster that has not been configured within the server [[GH-22227](https://github.com/hashicorp/nomad/issues/22227)]
* core: Fix multiple incorrect type conversion for potential overflows [[GH-20553](https://github.com/hashicorp/nomad/issues/20553)]
* csi: Fixed a bug where concurrent mount and unmount operations could unstage volumes needed by another allocation [[GH-20550](https://github.com/hashicorp/nomad/issues/20550)]
* csi: Fixed a bug where plugins would not be deleted on GC if their job updated the plugin ID [[GH-20555](https://github.com/hashicorp/nomad/issues/20555)]
* csi: Fixed a bug where volumes in different namespaces but the same ID would fail to stage on the same client [[GH-20532](https://github.com/hashicorp/nomad/issues/20532)]
* job endpoint: fix implicit constraint mutation for task-level services [[GH-22229](https://github.com/hashicorp/nomad/issues/22229)]
* quota (Enterprise): Fixed a bug where quota usage would not be freed if a job was purged
* services: Added retry to Nomad service deregistration RPCs during alloc stop [[GH-20596](https://github.com/hashicorp/nomad/issues/20596)]
* services: Fixed bug where Nomad services might not be deregistered when nodes are marked down or allocations are terminal [[GH-20590](https://github.com/hashicorp/nomad/issues/20590)]
* structs: Fix job canonicalization for array type fields [[GH-20522](https://github.com/hashicorp/nomad/issues/20522)]
* ui: Fix a bug where the UI would prompt a user to promote a deployment with unplaced canaries [[GH-20408](https://github.com/hashicorp/nomad/issues/20408)]
* ui: Fixed an issue where keynav would not trigger evaluation sidebar expand [[GH-20047](https://github.com/hashicorp/nomad/issues/20047)]
* ui: Show the namespace in the web UI exec command hint [[GH-20218](https://github.com/hashicorp/nomad/issues/20218)]
* windows: Fixed a regression where scanning task processes was inefficient [[GH-20619](https://github.com/hashicorp/nomad/issues/20619)]

## 1.7.7 (April 16, 2024)

SECURITY:

* artifact: Updated `go-getter` dependency to v1.7.4 to address CVE-2024-3817 [[GH-20391](https://github.com/hashicorp/nomad/issues/20391)]

IMPROVEMENTS:

* autopilot: add Enterprise health information to autopilot API [[GH-20153](https://github.com/hashicorp/nomad/issues/20153)]
* cli: Collect only one heap profile per `operator debug` interval [[GH-20219](https://github.com/hashicorp/nomad/issues/20219)]
* consul/connect: Added support for TLS configuration, headers configuration, and request limit configuration to ingress service block [[GH-16753](https://github.com/hashicorp/nomad/issues/16753)]
* consul/connect: Added support for destination partition in `upstream` block [[GH-20167](https://github.com/hashicorp/nomad/issues/20167)]
* scheduler: Record exhausted node metrics for devices when preemption fails to find an allocation to evict [[GH-20346](https://github.com/hashicorp/nomad/issues/20346)]
* ui: When you re-bind keyboard shortcuts they now correctly show up in shift-held hints [[GH-20235](https://github.com/hashicorp/nomad/issues/20235)]

BUG FIXES:

* agent: allow configuration of in-memory telemetry sink [[GH-20166](https://github.com/hashicorp/nomad/issues/20166)]
* api: Fixed a bug where `AllocDirStats` field was missing from Read Stats client API [[GH-20261](https://github.com/hashicorp/nomad/issues/20261)]
* cli: Fixed a bug where `operator debug` did not respect the `-pprof-interval` flag and would take only one profile [[GH-20206](https://github.com/hashicorp/nomad/issues/20206)]
* cni: Fixed a regression where default DNS set by `dockerd` or other task drivers was not respected [[GH-20189](https://github.com/hashicorp/nomad/issues/20189)]
* config: Fixed a bug where IPv6 addresses were not accepted without ports for `client.servers` blocks [[GH-20324](https://github.com/hashicorp/nomad/issues/20324)]
* consul: Fixed a bug where services with interpolation would not get correctly signed Workload Identities [[GH-20344](https://github.com/hashicorp/nomad/issues/20344)]
* deployments: Fixed a goroutine leak when jobs are purged [[GH-20348](https://github.com/hashicorp/nomad/issues/20348)]
* deps: Updated consul-template dependency to 0.37.4 to fix a resource leak [[GH-20234](https://github.com/hashicorp/nomad/issues/20234)]
* docker: Fixed a bug where cpuset cgroup would not be updated on cgroup v1 systems [[GH-20294](https://github.com/hashicorp/nomad/issues/20294)]
* docker: Fixed a bug where cpuset would not be updated on cgroup v2 systems using cgroupfs [[GH-20276](https://github.com/hashicorp/nomad/issues/20276)]
* drain: Fixed a bug where Workload Identity tokens could not be used to drain a node [[GH-20317](https://github.com/hashicorp/nomad/issues/20317)]
* namespace/node pool: Fixed a bug where the `-region` flag would not be respected for namespace and node pool updates if ACLs were disabled [[GH-20220](https://github.com/hashicorp/nomad/issues/20220)]
* state: Fixed a bug where restarting a server could fail if the Raft logs include a drain update that used a now-expired token [[GH-20317](https://github.com/hashicorp/nomad/issues/20317)]
* template: Fixed a bug where a partial `client.template` block would cause defaults for unspecified fields to be ignored [[GH-20165](https://github.com/hashicorp/nomad/issues/20165)]
* ui: Fix an issue where the job status box would error if an allocation had no task events [[GH-20383](https://github.com/hashicorp/nomad/issues/20383)]

## 1.7.6 (March 12, 2024)

SECURITY:

* build: Update to go1.22 to address Go standard library vulnerabilities CVE-2024-24783, CVE-2023-45290, and CVE-2024-24785. [[GH-20066](https://github.com/hashicorp/nomad/issues/20066)]
* deps: Upgrade protobuf library to 1.33.0 to avoid scan alerts for CVE-2024-24786, which Nomad is not vulnerable to [[GH-20100](https://github.com/hashicorp/nomad/issues/20100)]

IMPROVEMENTS:

* cli: Added -json option on job status command [[GH-18925](https://github.com/hashicorp/nomad/issues/18925)]
* fingerprint: Added a fingerprint for Consul DNS address and port [[GH-19969](https://github.com/hashicorp/nomad/issues/19969)]

BUG FIXES:

* cli: Fixed a bug where the `nomad job restart` command could crash if the job type was not present in a response from the server [[GH-20049](https://github.com/hashicorp/nomad/issues/20049)]
* client: Fixed a bug where corrupt client state could panic the client [[GH-19972](https://github.com/hashicorp/nomad/issues/19972)]
* cni: Fixed a bug where DNS set by CNI plugins was not provided to task drivers [[GH-20007](https://github.com/hashicorp/nomad/issues/20007)]
* connect: Fixed a bug where `expose` blocks would not appear in `job plan` diff output [[GH-19990](https://github.com/hashicorp/nomad/issues/19990)]
* server: Prevent NPE when service lacks identity [[GH-19986](https://github.com/hashicorp/nomad/issues/19986)]

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

## Unsupported Versions

Versions of Nomad before 1.6.0 are no longer supported. See [CHANGELOG-unsupported.md](./CHANGELOG-unsupported.md) for their changelogs.
