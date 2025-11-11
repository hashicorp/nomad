
SECURITY:

* build: Update go-getter to 1.8.3 that prevents a partially written file from remaining on disk with permissions that didn't include the umask. [[GH-27034](https://github.com/hashicorp/nomad/issues/27034)]
* build: Update toolchain to Go 1.25.2 to address Go stdlib CVE-2025-61724, CVE-2025-61725, CVE-2025-58187, CVE-2025-61723, CVE-2025-47912, CVE-2025-58185, CVE-2025-58186, CVE-2025-58188, and CVE-2025-58183 [[GH-26909](https://github.com/hashicorp/nomad/issues/26909)]
* job: Disallow tasks using the name "alloc" which breaks inter-task filesystem isolation [[GH-27001](https://github.com/hashicorp/nomad/issues/27001)]

IMPROVEMENTS:

* build: Add tzdata to Docker container final image [[GH-26794](https://github.com/hashicorp/nomad/issues/26794)]
* build: Updated Go to 1.25.1 [[GH-26823](https://github.com/hashicorp/nomad/issues/26823)]
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
* multiregion (Enterprise): fixes a bug where multiregion deployments could become deadlocked
* multiregion: fixes a bug where unblocking region could make unnecessary queries to other regions
* scheduler: Fixed scheduling behavior of batch job allocations [[GH-26961](https://github.com/hashicorp/nomad/issues/26961)]
* scheduler: allow use of different vendor/models when checking for device counts while filtering feasible nodes [[GH-26649](https://github.com/hashicorp/nomad/issues/26649)]
* scheduler: fixes a bug selecting nodes for updated jobs with ephemeral disks when nodepool changes [[GH-26662](https://github.com/hashicorp/nomad/issues/26662)]
* state: Fixed a bug where the server could panic when attempting to remove unneeded evals from the eval broker [[GH-26872](https://github.com/hashicorp/nomad/issues/26872)]
* ui: Fixed a bug where action fly-outs would fail to open due to a missing module [[GH-26833](https://github.com/hashicorp/nomad/issues/26833)]
* windows: Fixed a bug where agents would not gracefully shut down on Ctrl-C [[GH-26780](https://github.com/hashicorp/nomad/issues/26780)]
