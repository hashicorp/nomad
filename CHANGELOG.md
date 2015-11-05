## 0.2.0 (Unreleased)

FEATURES:

  * Blocking queries supported in API [GH-366]
  * Add support for downloading external artifacts to execute for Exec, Raw exec drivers [GH-381]

BACKWARDS INCOMPATIBILITIES:

  * Qemu and Java driver configurations have been updated to both use `artifact_source` as the source for external images/jars to be ran

## 0.1.2 (October 6, 2015)

IMPROVEMENTS:

  * Nomad client cleans allocations on exit when in dev mode
  * drivers: Use go-getter for artifact retrieval, add artifact support to Exec, Raw Exec drivers [GH-288]

## 0.1.1 (October 5, 2015)

IMPROVEMENTS:

  * Docker networking mode is configurable
  * Set task environment variables
  * Native IP detection and user specifiable network interface for
    fingerprinting
  * Nomad Client configurable from command-line

BUG FIXES:

  * Network fingerprinting failed if default network interface did not exist
  * Improved detection of Nomad binary
  * Docker dynamic port mapping were not being set properly
  * Fixed issue where network resources throughput would be set to 0 MBits if
    the link speed could not be determined

## 0.1.0 (September 28, 2015)

  * Initial release

