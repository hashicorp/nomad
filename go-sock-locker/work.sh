#!/usr/bin/env bash

set -u

# set stuff up

ALLOC="${NOMAD_ALLOC_ID}"
LOCK="${NOMAD_LOCK_FILE:-local/lock}"

_log() { echo "$(date -I'seconds') $*"; }

check_lock() {
  if [ -f "$LOCK" ]; then
    _log "$(cat "$LOCK") $(grep -q "$ALLOC" "$LOCK" && echo 'it me!' || echo 'no :(')"
  else
    _log 'no lock'
  fi
}

_hup() {
  _log 'hup hup!'
  _log "lock file: $LOCK"
  check_lock
}
trap _hup SIGHUP # set in template stanza on variable change

_term() { _log 'got term, so exiting.'; exit; }
trap _term SIGTERM # default nomad kill signal

# do stuff

_log 'starting'
_log "lock file: $LOCK"

# lil loop just to show stuff happening
while true; do
  check_lock
  sleep 5
done
