#!/bin/sh
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


# Create the tarball used in TestDockerDriver_StopSignal
cat <<'EOF' | docker build -t busybox:1.29.3-stopsignal -
FROM busybox:1.29.3
STOPSIGNAL 19
EOF

docker save busybox:1.29.3-stopsignal > busybox_stopsignal.tar
