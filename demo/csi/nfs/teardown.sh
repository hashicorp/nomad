#!/bin/bash

set -x

purge() {
  nomad stop -purge "$1"
}

purge web
while true; do
  nomad volume status my-nfs 2>&1 | grep -E 'No (allocations|volumes)' && break
  sleep 2
done
purge node

nomad volume delete my-nfs
purge controller

nomad system gc
