This command allows plugin developers to interact with a plugin directly. The
command has subcommands for each plugin type. See the subcommand help text for
detailed usage information.

# Device Example

The `device` subcommand provides a way to interact and visualize the data being
returned by a device plugin. As an example we will run the example device
plugin. To use this command with your own device plugin substitute the example
plugin with your own.


```
# Current working directory should be the root folder: github.com/hashicorp/nomad

# Build the plugin launcher
$ go build github.com/hashicorp/nomad/plugins/shared/cmd/launcher/

# Build the example fs-device plugin
$ go build -o fs-device github.com/hashicorp/nomad/plugins/device/cmd/example/cmd

# Launch the plugin
$ ./launcher device ./fs-device
> Availabile commands are: exit(), fingerprint(), stop_fingerprint(), stats(), stop_stats(), reserve(id1, id2, ...)
>  2018-08-28T14:54:45.658-0700 [INFO ] nomad-plugin-launcher.fs-device: config set: @module=example-fs-device config="example.Config{Dir:".", ListPeriod:"5s", StatsPeriod:"5s", UnhealthyPerm:"-rwxrwxrwx"}" timestamp=2018-08-28T14:54:45.658-0700

^C
2018-08-28T14:54:54.727-0700 [ERROR] nomad-plugin-launcher: error interacting with plugin: error=interrupted

# Lets launch changing the configuration
$ cat <<\EOF >fs-device.config
> config {
>   dir = "./plugins"
> }
> EOF

$ ./launcher device ./fs-device ./fs-device.config
2018-08-28T14:59:45.886-0700 [INFO ] nomad-plugin-launcher.fs-device: config set: @module=example-fs-device config="example.Config{Dir:"./plugins", ListPeriod:"5s", StatsPeriod:"2s", UnhealthyPerm:"-rwxrwxrwx"}" timestamp=2018-08-28T14:59:45.886-0700
> Availabile commands are: exit(), fingerprint(), stop_fingerprint(), stats(), stop_stats(), reserve(id1, id2, ...)
>  fingerprint()
>  > fingerprint: &device.FingerprintResponse{
    Devices: {
        &device.DeviceGroup{
            Vendor:  "nomad",
            Type:    "file",
            Name:    "mock",
            Devices: {
                &device.Device{
                    ID:         "serve.go",
                    Healthy:    true,
                    HealthDesc: "",
                    HwLocality: (*device.DeviceLocality)(nil),
                },
            },
            Attributes: {},
        },
    },
    Error: nil,
}
^C
2018-08-28T15:00:00.329-0700 [ERROR] nomad-plugin-launcher: error interacting with plugin: error=interrupted
```
