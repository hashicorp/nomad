#!/usr/bin/env bash
# Copyright IBM Corp. 2015, 2026
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODULE="github.com/hashicorp/nomad/v2"
OLD_MODULE="github.com/hashicorp/nomad"
EXIT_CODE=0

function check_mod() {
    MOD_LINE=$(head -1 "$REPO_ROOT/go.mod")
    if [[ "$MOD_LINE" != "module $MODULE" ]]; then
        echo "FAIL: go.mod declares '$MOD_LINE', expected 'module $MODULE'"
        EXIT_CODE=1
    else
        echo "OK: go.mod declares module $MODULE"
    fi
}

function check_imports() {

    # Match import lines that reference the old module path but NOT the new. The
    # find matches Go file extensions, but skip potentially larger directories.
    STALE_IMPORTS=$(find "$REPO_ROOT" \
        -type f -name '*.go' \
        ! -path '*/.git/*' \
        ! -path '*/ui/*' \
        -exec grep -Hn "\"${OLD_MODULE}/" {} \; \
        | grep -v "\"${MODULE}" \
        | grep -v "github.com/hashicorp/nomad/api" \
        || true)

    # If we found stale imports, output at least some of them to the user. If there
    # are more than 50, cut the output, as it'll be too much information on the
    # console.
    if [[ -n "$STALE_IMPORTS" ]]; then
        COUNT=$(echo "$STALE_IMPORTS" | wc -l | tr -d ' ')
        echo "FAIL: Found $COUNT file(s) with stale import path:"
        echo ""
        echo "$STALE_IMPORTS" | head -50
        if [[ $COUNT -gt 50 ]]; then
            echo "  ... and $((COUNT - 50)) more (showing first 50)"
        fi
        EXIT_CODE=1
    else
        echo "OK: No stale imports found"
    fi
}

function handle_exit() {
    if [[ $EXIT_CODE -eq 0 ]]; then
        echo ""
        echo "==> All checks passed. Module path is consistently $MODULE."
    else
        echo ""
        echo "==> Some checks FAILED. See above for details."
    fi

    exit $EXIT_CODE
}

main() {
    echo "==> Checking go.mod module declaration..."
    check_mod
    echo "==> Done"

    echo ""
    echo "==> Scanning for stale imports..."
    check_imports
    echo "==> Done"

    handle_exit
}

# Only run main if the script is executed, not sourced.
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
