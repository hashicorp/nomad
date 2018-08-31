This package provides an example implementation of a device plugin for
reference.

# Behavior

The example device plugin models files within a specified directory as devices. The plugin will periodically scan the directory for changes and will expose them via the streaming Fingerprint RPC. Device health is set to unhealthy if the file has a specific filemode permission as described by the config `unhealthy_perm`. Further statistics are also collected on the detected devices.

# Config

The configuration should be passed via an HCL file that begins with a top level `config` stanza:

```
config {
  dir = "/my/path/to/scan"
  list_period = "1s"
  stats_period = "5s"
  unhealthy_perm = "-rw-rw-rw-"
}
```

The valid configuration options are:

* `dir` (`string`: `"."`): The directory to scan for files that will represent fake devices.
* `list_period` (`string`: `"5s"`): The interval to scan the directory for changes.
* `stats_period` (`string`: `"5s"`): The interval at which to emit statistics about the devices.
* `unhealthy_perm` (`string`: `"-rwxrwxrwx"`): The file mode permission that if set on a detected file will casue the device to be considered unhealthy.
