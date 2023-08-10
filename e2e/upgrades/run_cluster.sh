# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

#/bin/bash
# This script takes path to a binary and runs a 3 server, two node cluster
if [ "$#" -ne 1 ]; then
    echo "expected usage ./run_cluster.sh /path/to/nomad/binary"
    exit 255
fi
NOMAD_BINARY=$1

# make sure the directories exist so tee can create logs in them
mkdir -p /tmp/server{1,2,3} /tmp/client{1,2}

# launch server 
( ${NOMAD_BINARY} agent -config=server1.hcl 2>&1 | tee "/tmp/server1/log" ; echo "Exit code: $?" >> "/tmp/server1/log" ) &

( ${NOMAD_BINARY} agent -config=server2.hcl 2>&1 | tee "/tmp/server2/log" ; echo "Exit code: $?" >> "/tmp/server2/log" ) &

( ${NOMAD_BINARY}  agent -config=server3.hcl 2>&1 | tee "/tmp/server3/log" ; echo "Exit code: $?" >> "/tmp/server3/log" ) &

# launch client 1
( ${NOMAD_BINARY} agent -config=client1.hcl 2>&1 | tee "/tmp/client1/log" ; echo "Exit code: $?" >> "/tmp/client1/log" ) &

# launch client 2
( ${NOMAD_BINARY} agent -config=client2.hcl 2>&1 | tee "/tmp/client2/log" ; echo "Exit code: $?" >> "/tmp/client2/log" ) &

# launch consul
(consul agent -dev)&
