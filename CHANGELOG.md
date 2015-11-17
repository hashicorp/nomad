## 0.2.0 (Unreleased)

FEATURES:

  * System Scheduler that runs tasks on every node [GH-287]
  * Restart policy for task groups enforced by the client [GH-369, GH-393]
  * distinctHost constraint ensures Task Groups are running on distinct clients [GH-321]
  * Regexp, version and lexical ordering constraints [GH-271]
  * Blocking queries supported in API [GH-366]
  * Add support for downloading external artifacts to execute for Exec, Raw exec drivers [GH-381]
  * Raw Fork/Exec Driver [GH-237]
  * Experimental Rkt Driver [GH-165, GH-247]
  * GCE Fingerprinting [GH-215]

IMPROVEMENTS:

  * Test Skip Detection [GH-221]
  * Use BlkioWeight rather than BlkioThrottleReadIopsDevice [GH-222]
  * Overlap plan verification and plan application for increased throughput [GH-272]
  * Mount task local and alloc directory to docker containers [GH-290]
  * Pass JVM options in java driver [GH-293, GH-297]
  * Show node attributes in `node-status` [GH-313]
  * Output of `server-members` is sorted [GH-323]
  * Network fingerprinter detects interface suitable for use, rather than
    defaulting to eth0 [GH-334, GH-356]
  * Configurable Node GC threshold [GH-362]
  * Client Restore State properly reattaches to tasks and recreates them as
    needed [GH-364, GH-380, GH-388, GH-392, GH-394, GH-397, GH-408]
  * Advanced docker driver options [GH-390]
  * Periodic Fingerprinting [GH-391]
  * Precise snapshotting of TaskRunner and AllocRunner [GH-403, GH-411]
  * Driver configuration supports arbitrary struct to be passed in jobspec [GH-415]
  * Task State is tracked by client [GH-416]
  * Output of `alloc-status` also displays task state [GH-424]
  * Docker hostname can be set [GH-426]

BUG FIXES:

  * Reap spawn-daemon process, avoiding a zombie process [GH-240]
  * Scheduler checks for updates to environment variables [GH-327]
  * Reset Nack timer in response to scheduler operations [GH-325]
  * Qemu fingerprint and tests work on both windows/linux [GH-352]
  * Use correct local interface on OS X [GH-361, GH-365]
  * Apply SELinux label for mounting directories in docker [GH-377]
  * Docker driver uses docker environment variables correctly [GH-407]
  * Nomad Client/Server RPC codec encodes strings properly [GH-420]
  * Fix crash when -config was given a directory or empty path [GH-119]
  * Resource exhausted errors because of link-speed zero [GH-146, GH-205]
  * Restarting Nomad Client leads to orphaned containers [GH-159]
  * Nomad Client doesn't restart failed containers [GH-198]
  * Docker driver exposes ports when creating container [GH-212, GH-412]

BACKWARDS INCOMPATIBILITIES:

  * Qemu and Java driver configurations have been updated to both use `artifact_source`
    as the source for external images/jars to be ran
  * Removed weight and hard/soft fields in constraints [GH-351]
  * Api /v1/node/\<id\>/allocations returns full Allocation and not stub [GH-402]

## 0.1.2 (October 6, 2015)

IMPROVEMENTS:

  * Nomad client cleans allocations on exit when in dev mode [GH-214]
  * drivers: Use go-getter for artifact retrieval, add artifact support to Exec,
    Raw Exec drivers [GH-288]

## 0.1.1 (October 5, 2015)

IMPROVEMENTS:

  * Docker networking mode is configurable [GH-184]
  * Set task environment variables [GH-206]
  * Native IP detection and user specifiable network interface for
    fingerprinting  [GH-189]
  * Nomad Client configurable from command-line [GH-191]

BUG FIXES:

  * Network fingerprinting failed if default network interface did not exist [GH-189]
  * Improved detection of Nomad binary [GH-181]
  * Docker dynamic port mapping were not being set properly [GH-199]
  * Fixed issue where network resources throughput would be set to 0 MBits if
    the link speed could not be determined [GH-205]

## 0.1.0 (September 28, 2015)

  * Initial release

