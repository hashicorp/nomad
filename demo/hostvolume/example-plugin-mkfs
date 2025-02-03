#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

version='0.0.1'
fingerprint() {
  printf '{"version": "%s"}' "$version"
}

help() {
  cat <<EOF
Dynamic Host Volume plugin which creates an ext4 loopback drive of
the minimum requested size, and mounts it at the provided path argument.

Note: Requires superuser access to mount.

Usage:
  $(basename "$0") [options] <create|delete|fingerprint> [path]

Options:
  -v|--verbose: Show shell commands (set -x)
  -h|--help: Print this help text and exit

Operations:
  create: Creates and mounts the device at path (required)
    required environment:
      CAPACITY_MIN_BYTES
  delete: Unmounts and deletes the device at path (required)
  version: Outputs this plugin's version: $version
  fingerprint: Outputs plugin metadata: $(fingerprint)

EOF
}

# parse args
[ $# -eq 0 ] && { help; exit 1; }
for arg in "$@"; do
  case $arg in
    -h|-help|--help) help; exit 0 ;;
    fingerprint|fingerprint) fingerprint; exit 0 ;;
    version|version) echo "$version"; exit 0 ;;
    -v|--verbose) set -x; shift; ;;
  esac
done

# OS detect
if [[ "$OSTYPE" == "linux-"* ]]; then
  ext=ext4
  mount=/usr/bin/mount
  mkfsExec() {
    dd if=/dev/zero of="$1".$ext bs=1M count="$2"
    mkfs.ext4 "$1".$ext 1>&2
  }
  mountExec() {
    $mount "$1".$ext "$1"
  }
  st() {
    stat --format='%s' "$1"
  }
elif [[ "$OSTYPE" == "darwin"* ]]; then
  ext=dmg
  mount=/sbin/mount
  mkfsExec() {
    hdiutil create -megabytes "$2" -layout NONE -fs apfs -volname "$1" "$1" 1>&2
  }
  mountExec() { 
    hdiutil attach "$1".$ext 1>&2
  }
  st() {
    stat -f %z "$1"
  }
else
  echo "$OSTYPE is an unsupported OS"
  exit 1
fi

validate_path() {
  local path="$1"
  if [[ ! "$path" =~ [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12} ]]; then
    1>&2 echo "expected uuid-lookin ID in target host path; got: '$path'"
    return 1
  fi
}

is_mounted() {
  $mount | grep -q " $1 "
}

create_volume() {
    local path="$1"
    validate_path "$path"
    local bytes="$2"

    # translate to mb for dd block size
    local megs=$((bytes / 1024 / 1024)) # lazy, approximate

    mkdir -p "$(dirname "$path")"
    # the extra conditionals are for idempotency
    if [ ! -f "$path.$ext" ]; then
      mkfsExec "$path" $megs
    fi
    if ! is_mounted "$path"; then
      mkdir -p "$path"
      mountExec "$path"
    fi
}

delete_volume() {
  local path="$1"
  validate_path "$path"
  is_mounted "$path" && umount "$path"
  rm -rf "$path"
  rm -f "$path"."$ext"
}

host_path="$DHV_VOLUMES_DIR/$DHV_VOLUME_ID"
case "$1" in
  "create")
    create_volume "$host_path" "$DHV_CAPACITY_MIN_BYTES"
    # output what Nomad expects
    bytes="$(st "$host_path".$ext)"
    printf '{"path": "%s", "bytes": %s}' "$host_path" "$bytes"
    ;;
  "delete")
    delete_volume "$host_path" ;;
  *)
    echo "unknown operation: $1" 1>&2
    exit 1 ;;
esac
