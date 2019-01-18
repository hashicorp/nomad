#!/bin/bash

status=`curl -sS localhost:4646/v1/job/sleep/allocations | jq -r .[0].ClientStatus`

if [ $status != "running" ]; then
	echo "Allocation status was not running: $status"
	exit 1
fi

id=`curl -sS localhost:4646/v1/job/sleep/allocations | jq -r .[0].ID`
rss=`curl -sS localhost:4646/v1/client/allocation/$id/stats | jq .ResourceUsage.MemoryStats.RSS`
if [ "$rss" -le "0" ]; then
	echo "Stats failed for alloc"
	exit 1
fi
