#!/usr/bin/env bash

set -eu

COMMAND="$*"
if [ "$COMMAND" = "" ]; then
	COMMAND="make test-nomad"
fi

GIT_DESC=$(git describe)
TS=$(date +%s)
LOG_DIR=$(mktemp -d)/nomad-test-samples/$GIT_DESC-$TS
mkdir -p "$LOG_DIR"

echo
echo
echo "Collecting test failures from $COMMAND"
echo "Writing logs to $LOG_DIR"
echo "Press ctrl+c to stop."
echo
echo

I=0
FAILED_FILE="$LOG_DIR/failed_tests"
for ((;;)); do
	RUN_NUM=$(printf "%04d" $I)
	LOG_FILE="$LOG_DIR/run-$RUN_NUM"
	echo "RUN $RUN_NUM" >> "$FAILED_FILE" 
	(
	set +e -o pipefail
	$COMMAND 2>&1 | tee "$LOG_FILE"
	echo $? > test_exit_code
	) &
	echo $! > test_pid
	wait "$(cat test_pid)"
	
	grep -E '^--- FAIL:' "$LOG_FILE" >> "$FAILED_FILE"

	EXIT_CODE=$(cat test_exit_code)
	if [ "$EXIT_CODE" = 0 ]; then
		mv "$LOG_FILE" "$LOG_FILE-pass"
	else
		mv "$LOG_FILE" "$LOG_FILE-fail-$EXIT_CODE"
	fi
	I=$(( I + 1 ))
done

