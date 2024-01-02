# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

#/bin/bash
# This script takes the config file of the node being killed
# usage ./_kill_node.sh client1
if [ "$#" -ne 1 ]; then
    echo "expected usage - ./kill_node.sh <client|server><1|2>"
    exit 255
fi
CONFIG=$1
echo "Killing $CONFIG"
pid=`ps wwwaux | grep nomad | grep "$CONFIG.hcl" | awk 'BEGIN { FS = " " } ; { print $2 }'`
echo "killing pid $pid"
kill -9 $pid
