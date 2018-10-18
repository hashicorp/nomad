# NVIDIA GPU Monitoring Tools

## NVML Go Bindings

[NVIDIA Management Library (NVML)](https://developer.nvidia.com/nvidia-management-library-nvml) is a C-based API for monitoring and managing NVIDIA GPU devices. 
NVML go bindings are taken from [nvidia-docker 1.0](https://github.com/NVIDIA/nvidia-docker/tree/1.0) with some improvements and additions. NVML headers are also added to the package to make it easy to use and build.

### NVML Samples
Three [samples](https://github.com/NVIDIA/gpu-monitoring-tools/blob/master/bindings/go/samples/nvml/README.md) are included to demonstrate how to use the NVML API.


## DCGM Go Bindings

[NVIDIA Data Center GPU Manager (DCGM)](https://developer.nvidia.com/data-center-gpu-manager-dcgm) is a set of tools for managing and monitoring NVIDIA GPUs in cluster environments. It's a low overhead tool suite that performs a variety of functions on each host system including active health monitoring, diagnostics, system validation, policies, power and clock management, group configuration and accounting.

DCGM go bindings makes administering and monitoring containerized GPU applications easy.

### DCGM Samples

DCGM can be run in different modes, seven [samples](https://github.com/NVIDIA/gpu-monitoring-tools/blob/master/bindings/go/samples/dcgm/README.md) and a [REST API](https://github.com/NVIDIA/gpu-monitoring-tools/blob/master/bindings/go/samples/dcgm/restApi/README.md) are included for showing how to use the DCGM API and run it in different modes.


## DCGM exporter

GPU metrics exporter for [Prometheus](https://prometheus.io/) leveraging [NVIDIA Data Center GPU Manager (DCGM)](https://developer.nvidia.com/data-center-gpu-manager-dcgm) is a simple shell script that starts nv-hostengine, reads GPU metrics every 1 second and converts it to a standard Prometheus format.

Find the installation and run instructions [here](https://github.com/NVIDIA/gpu-monitoring-tools/blob/master/exporters/prometheus-dcgm/README.md).

## Issues and Contributing

A signed copy of the [Contributor License Agreement](https://github.com/NVIDIA/gpu-monitoring-tools/blob/master/CLA) needs to be provided to <a href="mailto:digits@nvidia.com">digits@nvidia.com</a> before any change can be accepted.

* Please let us know by [filing a new issue](https://github.com/NVIDIA/gpu-monitoring-tools/issues/new)
* You can contribute by opening a [pull request](https://help.github.com/articles/using-pull-requests/)
