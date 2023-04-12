#!/usr/bin/env bash

set -xeuo pipefail

plugin='org.democratic-csi.nfs'

# run controller plugin (and nfs server)
nomad run jobs/controller-plugin.nomad.hcl
while true; do
  nomad plugin status "$plugin" | grep 'Controllers Healthy.*1' && break
  sleep 5
done

# make a volume - the controller plugin handles this request
nomad volume status -t '{{.Context}}' my-nfs 2>/dev/null \
|| nomad volume create volume.hcl

# run node plugin
nomad run jobs/node-plugin.nomad.hcl
while true; do
  nomad plugin status "$plugin" | grep 'Nodes Healthy.*1' && break
  sleep 5
done

# run demo web server, which prompts the node plugin to mount the volume
nomad run jobs/web.nomad.hcl
