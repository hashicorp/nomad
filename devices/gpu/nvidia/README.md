This package provides an implementation of nvidia device plugin

# Behavior

Nvidia device plugin uses NVML bindings to get data regarding available nvidia devices and will expose them via Fingerprint RPC. GPUs can be excluded from fingerprinting by setting the `ignored_gpu_ids` field. Plugin sends statistics for fingerprinted devices every `stats_period` period.

# Config

The configuration should be passed via an HCL file that begins with a top level `config` stanza:

```
config {
  ignored_gpu_ids = ["uuid1", "uuid2"]
  fingerprint_period = "5s"
}
```

The valid configuration options are:

* `ignored_gpu_ids` (`list(string)`: `[]`): list of GPU UUIDs strings that should not be exposed to nomad
* `fingerprint_period` (`string`: `"5s"`): The interval to repeat fingerprint process to identify possible changes.
