#!/usr/bin/env bash

export PING_SLEEP=30
bash -c "while true; do echo \$(date) - building ...; sleep $PING_SLEEP; done" &
PING_LOOP_PID=$!

make test
TEST_OUTPUT=$?

kill $PING_LOOP_PID
exit $TEST_OUTPUT
