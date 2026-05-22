#!/usr/bin/env bash
# update_copyright.sh
#
# For every file modified on this branch (relative to main), check if the file
# has a copyright header whose last year is 2025, and if so update it to 2026.
#
# Usage: run from the root of the repository on the f-pointer-of-new branch.
#   ./update_copyright.sh [--dry-run]
#
# Pass --dry-run to only print what would change without touching any files.

set -euo pipefail

DRY_RUN=false
if [[ "${1:-}" == "--dry-run" ]]; then
  DRY_RUN=true
fi

# Detect the base branch to diff against.
BASE="${BASE_BRANCH:-main}"

# Collect the list of files that differ from the base branch.
mapfile -t changed_files < <(git diff --name-only "${BASE}...HEAD")

if [[ ${#changed_files[@]} -eq 0 ]]; then
  echo "No files changed relative to '${BASE}'. Nothing to do."
  exit 0
fi

updated=0
skipped_no_header=0
skipped_already_current=0

for file in "${changed_files[@]}"; do
  # Skip files that no longer exist (deleted on this branch).
  if [[ ! -f "${file}" ]]; then
    continue
  fi

  # Read only the first 5 lines to find the copyright header efficiently.
  # The header is virtually always within the first few lines of the file.
  header=$(head -5 "${file}")

  # Check whether the file has a copyright line at all.
  if ! echo "${header}" | grep -qiE 'copyright'; then
    skipped_no_header=$((skipped_no_header + 1))
    continue
  fi

  # Check whether the copyright line ends with ", 2025" (with optional trailing
  # whitespace). The regex matches lines like:
  #   // Copyright IBM Corp. 2015, 2025
  #   # Copyright (c) HashiCorp, Inc., 2025
  if ! echo "${header}" | grep -qE ',\s*2025\s*$'; then
    skipped_already_current=$((skipped_already_current + 1))
    continue
  fi

  # At this point the file has a copyright header whose last year is 2025.
  if "${DRY_RUN}"; then
    echo "[dry-run] Would update: ${file}"
  else
    # Use sed to replace only the copyright line's trailing year.
    # The pattern targets lines containing "copyright" (case-insensitive) and
    # replaces ", 2025" (with any surrounding whitespace) with ", 2026".
    sed -i '' -E '/[Cc]opyright/s/,([[:space:]]*)2025([[:space:]]*)$/,\12026\2/' "${file}"
    echo "Updated: ${file}"
  fi
  updated=$((updated + 1))
done

echo ""
if "${DRY_RUN}"; then
  echo "Dry-run complete."
  echo "  Would update : ${updated}"
else
  echo "Done."
  echo "  Updated      : ${updated}"
fi
echo "  No header    : ${skipped_no_header}"
echo "  Already 2026+: ${skipped_already_current}"
