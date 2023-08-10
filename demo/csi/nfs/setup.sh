#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


# Set up all the demo components.
# This can be run repeatedly as it is fairly idempotent.

set -xeuo pipefail

plugin='rocketduck-nfs'

# run nfs server
nomad run jobs/nfs.nomad.hcl

# run controller plugin
nomad run jobs/controller-plugin.nomad.hcl
while true; do
  nomad plugin status "$plugin" | grep 'Controllers Healthy.*1' && break
  sleep 5
done

# make a volume - the controller plugin handles this request
nomad volume status -t '{{.PluginID}}' csi-nfs 2>/dev/null \
|| nomad volume create volume.hcl

# run node plugin
nomad run jobs/node-plugin.nomad.hcl
while true; do
  nomad plugin status "$plugin" | grep 'Nodes Healthy.*1' && break
  sleep 10
done

# run demo web server, which prompts the node plugin to mount the volume
nomad run jobs/web.nomad.hcl

# show volume info now that it's all set up and in use
nomad volume status csi-nfs

# show the web service ports for convenience
nomad service info -t '{{ range . }}{{ .Port }} {{ end }}' web
