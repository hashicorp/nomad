# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

import signal
import time

# Setup handler for sigterm so we can exit when docker stop is called.
def term(signum, stack_Frame):
    exit(1)

signal.signal(signal.SIGTERM, term)

print ("Starting")

max = 3
for i in range(max):
    time.sleep(1)
    print("Heartbeat {0}/{1}".format(i + 1, max))

print("Exiting")
