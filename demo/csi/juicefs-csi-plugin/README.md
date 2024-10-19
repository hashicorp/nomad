## JucieFS CSI Plugin

Author: @kmott

[JuiceFS](https://juicefs.com/) is a high-performance cloud-native distributed filesystem which may be used via a CSI Plugin.

## Official documentation
Before getting started, full documentation for using JuiceFS with Nomad can be found at [juicefs.com/docs/csi/csi-in-nomad/](https://juicefs.com/docs/csi/csi-in-nomad/).

## Prerequisites

* JuiceFS CSI Driver v0.14.1 or higher is required for use on Nomad
* Nomad v0.12.0 or higher is recommended for the full suite of operations
* A JuiceFS metadata URL endpoint like REDIS, PostgreSQL, TiKV, etc.
* A JuiceFS block storage URL for data like AWS S3, Minio, etc.

## Configure your Nomad clients

The JuiceFS container needs to run in privileged mode, so you need to configure your Nomad clients to allow Docker containers running on privileged mode.

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

To start using the JuiceFS CSI Driver on Nomad, complete the follow steps:

1. Run the following command in this directory:

    ```
    nomad job run juicefs-controller-csi-plugin.hcl
    ```

2. JuiceFS CSI Controller will take a few minutes to startup. To check on the status, run the following command:

    ```
    nomad job status juicefs-controller
    ```

3. Once JuiceFS Controller is running, run JuiceFS Node job:

    ```
    nomad job run juicefs-node-csi-plugin.hcl
    ```

4. JuiceFS CSI Node will take a few minutes to startup. To check on the status, run the following command:

    ```
    nomad job status juicefs-node
    ```

5. Once JuiceFS Node is running, create a volume with the following command:

    ```
    nomad volume create juicefs-volume.hcl
    ```

6. Once the volume is created, you can create a task that uses the volume.  Note that the volume does not get
configured or formatted until consumed by a task.

## References

- https://juicefs.com/docs/csi/csi-in-nomad/
- https://juicefs.com/docs/community/security/trash/
