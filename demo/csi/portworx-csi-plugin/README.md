## Portworx CSI Plugin

Author: @ggriffiths

[Portworx](https://portworx.com/) is a software defined storage overlay which may be used via a CSI Plugin.

## Official documentation
Before getting started, full documentation for using Portworx with Nomad can be found at [docs.portworx.com/install-with-other/nomad](https://docs.portworx.com/install-with-other/nomad).

## Prerequisites

* Portworx 2.8.0 or higher is required for using the Portworx CSI Driver on Nomad
* Nomad 1.1.0 or higher is recommended for the full suite of volume operations
* A minimum of 3 nodes is required for running Portworx on Nomad
* Connection to a consul cluster is required

## Configure your Nomad clients

The Portworx OCI-monitor container needs to run in privileged mode, so you need to configure your Nomad clients to allow docker containers running on privileged mode.

Add the following lines in your Nomad client configuration files and restart your clients:

```hcl
plugin "docker" {
  config {
    allow_privileged = true
    volumes {
      enabled = true
    }
  }
}
```

## Getting started

To start using the Portworx CSI Driver on Nomad, complete the follow steps:


1. Run the following command in this directory:

    ```
    nomad job run portworx-csi-plugin.hcl
    ```

2. Portworx will take a few minutes to startup. To check on the status, run the following command:

    ```
    nomad job status portworx
    ```

3. Once Portworx is running, create a volume with the following command:

    ```
    nomad volume create portworx-volume.hcl
    ```

## Portworx Openstorage CSI Driver repo:

The open source control plane for Portworx can be found at the following repository:
https://github.com/libopenstorage/openstorage/tree/master/csi
