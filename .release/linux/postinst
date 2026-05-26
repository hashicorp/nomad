#!/bin/bash

mkdir -p /opt/nomad/data
chown nomad:nomad /opt/nomad/data
chown -R nomad:nomad /etc/nomad.d

if [ -d /run/systemd/system ]; then
    systemctl --system daemon-reload >/dev/null || true
fi

if [[ $(nomad version) == *+ent* ]]; then
    echo "
Agreement

The following shall apply unless your organization has a separately signed 
agreement governing your use of the software made available here:

The software is subject to the license terms or community license (i.e. Mozilla
Public License 2.0 or Business Source License), as applicable, located in the 
download package for the software, the IBM International Program License 
Agreement, the IBM International License Agreement for Evaluation of Programs 
(for evaluation uses), or the IBM International License Agreement for Early 
Release of Programs (alpha and beta releases), and the applicable License 
Information, copies of which are also available at https://www.ibm.com/terms. 
In the event of a conflict between the license file in the download package and
the noted IBM licenses above, the relevant IBM terms will apply. Please refer 
to the license terms prior to using the software. Your installation and use of
the software constitute your acceptance of those terms. If you do not accept 
the terms, do not use the software.
"
fi
