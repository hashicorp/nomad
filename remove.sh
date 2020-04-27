#!/bin/env sh

set -xe

version="v19.03.8"

libs="api/types \
    api/types/blkiodev \
    api/types/container \
    api/types/filters \
    api/types/mount \
    api/types/network \
    api/types/registry \
    api/types/strslice \
    api/types/swarm \
    api/types/swarm/runtime \
    api/types/versions \
    daemon/caps \
    errdefs \
    opts \
    pkg/archive \
    pkg/fileutils \
    pkg/homedir \
    pkg/idtools \
    pkg/ioutils \
    pkg/jsonmessage \
    pkg/longpath \
    pkg/mount \
    pkg/pools \
    pkg/stdcopy \
    pkg/stringid \
    pkg/system \
    pkg/tarsum \
    pkg/term \
    pkg/term/windows \
    registry \
    registry/resumable \
    volume \
    volume/mounts"

for lib in $libs
do
    d="github.com/docker/docker/$lib"
    if [ -d "vendor/$d" ]
    then
        govendor remove $d
    fi
done

for lib in $libs
do
    d="github.com/docker/docker/$lib"
    if [ ! -d "vendor/$d" ]
    then
        govendor fetch $d::github.com/moby/moby/$lib@$version
    fi
done
