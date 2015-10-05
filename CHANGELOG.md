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

