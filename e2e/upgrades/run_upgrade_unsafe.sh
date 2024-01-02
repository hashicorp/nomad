#!/bin/sh
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

if [ $# -lt 2 ]; then
    echo "usage: $0 path-nomad-v1 path-nomad-v2" 1>&2
    exit 1
fi

v1="$1"; shift
v2="$1"; shift

# sh run_cluster.sh "$v1" >/dev/null &
sh run_cluster.sh "$v1" &

while true; do
    n=`"$v1" operator raft list-peers | grep -c '\bserver[1-3]\b'`
    [ "$n" = 3 ] && break
done

for i in {1,2,3}; do
    sh kill_node.sh server$i
    sh run_node.sh "$v2" server$i &
done
