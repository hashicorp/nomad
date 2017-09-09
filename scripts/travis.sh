#!/usr/bin/env bash
set -e

export PING_SLEEP=60
bash -c "while true; do echo \$(date) - building ...; sleep $PING_SLEEP; done" &
PING_LOOP_PID=$!

trap 'kill ${PING_LOOP_PID}' EXIT HUP INT QUIT TERM

make check
make checkscripts
make test
TEST_OUTPUT=$?

kill $PING_LOOP_PID
exit $TEST_OUTPUT
