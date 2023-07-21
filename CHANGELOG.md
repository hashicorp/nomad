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


## Unsupported Versions

Versions of Nomad before 1.4.0 are no longer supported. See [CHANGELOG-unsupported.md](./CHANGELOG-unsupported.md) for their changelogs.
