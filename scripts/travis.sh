#!/usr/bin/env bash
set -e

export PING_SLEEP=60
bash -c "while true; do echo \$(date) - building ...; sleep $PING_SLEEP; done" &
PING_LOOP_PID=$!

trap 'kill ${PING_LOOP_PID}' EXIT HUP INT QUIT TERM

if [[ "$RUN_STATIC_CHECKS" ]]; then
    make check
    if [[ "$TRAVIS_OS_NAME" == "linux" ]]; then 
        make checkscripts
    fi
fi

make test
TEST_OUTPUT=$?

kill $PING_LOOP_PID
exit $TEST_OUTPUT
