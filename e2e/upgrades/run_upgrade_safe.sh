#!/bin/sh
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

if [ $# -lt 2 ]; then
    echo "usage: $0 path-nomad-v1 path-nomad-v2" 1>&2
    exit 1
fi

v1="$1"; shift
v2="$1"; shift

sh run_cluster.sh "$v1" &

function peers () {
    $v1 operator raft list-peers | tail -n+2 | awk '{print $1 " " $2}'
}

while true; do
    n=`peers | grep -c '\bserver[1-3]\b'`
    [ "$n" = 3 ] && break
done

function wait_serf () {
    echo "wait $1 \c"; date
    while true; do
	  peers \
	      | egrep "$1.global [0-9a-f-][0-9a-f-]{35}$" \
	      && break
	  sleep 1
    done
    echo "done $1 \c"; date
}

for i in {3,2,1}; do
    sh kill_node.sh server$i
    sh run_node.sh "$v2" server$i &
    wait_serf server$i
done
