## 0.2.1 (UNRELEASED)

IMPROVEMENTS:

  * core: Can specify a whitelist for activating drivers [GH-467]
  * client/spawn: spawn package tests made portable (work on Windows) [GH-442]

BUG FIXES:

  * driver/docker: Expose the container port instead of the host port [GH-466]
  * driver/docker: Support `port_map` for static ports [GH-476]
  * driver/docker: Pass 0.2.0-style port environment variables to the docker container [GH-476]
  * client/service discovery: Make Service IDs unique [GH-479]

## 0.2.0 (November 18, 2015)

BACKWARDS INCOMPATIBILITIES:

  * core: HTTP API `/v1/node/<id>/allocations` returns full Allocation and not
    stub [GH-402]
  * core: Removed weight and hard/soft fields in constraints [GH-351]
  * drivers: Qemu and Java driver configurations have been updated to both use
    `artifact_source` as the source for external images/jars to be ran
  * New reserved and dynamic port specification [GH-415]
  * jobspec and drivers: Driver configuration supports arbitrary struct to be
    passed in jobspec [GH-415]

FEATURES:

  * core: Service block definition with Consul registration [GH-463, GH-460,
    GH-458, GH-455, GH-446, GH-425]
  * core: Blocking queries supported in API [GH-366]
  * core: distinctHost constraint ensures Task Groups are running on distinct
    clients [GH-321]
  * core: Regexp, version and lexical ordering constraints [GH-271]
  * core: System Scheduler that runs tasks on every node [GH-287]
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

