# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

#/bin/bash
# this script runs a nomad node (client/server)
# first arg is the binary and second arg is of the format (<client|server><num>).
# this is only meant to be used within the context of the cluster created in run_cluster.sh 
# e.g usage ./run_node.sh nomad client1
if [ "$#" -ne 2 ]; then
    echo "Expected usage ./run_node.sh /path/to/binary <client|server><1|2>"
    exit 255
fi
NOMAD_BINARY=$1
NODE=$2
( $NOMAD_BINARY agent -config=${NODE}.hcl 2>&1 | tee -a "/tmp/$NODE/log" ; echo "Exit code: $?" >> "/tmp/$NODE/log" ) &
