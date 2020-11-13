#!/usr/bin/env bash
set -euo pipefail

# read go packages out of ./tools.go
go_packages=()
while IFS='' read -r line; do
  go_packages+=("$line")
done < <(grep '^.*_ "' tools.go | sed 's/.*"\(.*\)"/\1/g')

# $go_packages must be an array for the expansion here to work
for tool in "${go_packages[@]}"; do
  echo "  $tool"
  go install "$tool"
done
