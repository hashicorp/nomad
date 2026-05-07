#!/usr/bin/env bash
set -eu

which changelog-build > /dev/null || {
    echo "installing changelog-build from repo"
    go install github.com/hashicorp/go-changelog/cmd/changelog-build@latest
}

SRC_DIR=$(dirname "${BASH_SOURCE[0]}")
CHANGELOG_DIR="${SRC_DIR}/../.changelog"
PREVIOUS=${1:-undefined}
NEXT=${2:-undefined}

if [ $PREVIOUS == "--help" ]; then
    echo "./scripts/changelog.sh <PREVIOUS> <NEXT>"
    exit 0
fi

if [ $PREVIOUS == "undefined" ]; then
    echo "first argument should be previous release tag/ref"
    exit 1
fi

if [ $NEXT == "undefined" ]; then
    echo "second argument should be target release tag/ref"
    exit 1
fi

changelog-build \
    -entries-dir "$CHANGELOG_DIR" \
    -changelog-template "$CHANGELOG_DIR/changelog.tmpl" \
    -note-template  "$CHANGELOG_DIR/note.tmpl" \
    -local-fs \
    -last-release v2.0.0 \
    -this-release release/2.0.x
