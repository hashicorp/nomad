#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

TEXT_RED='\e[31m'
TEXT_CLEAR='\e[0m'

# Make a request to the Jira API
function jira-request() {
  curl --show-error --location --fail-with-body \
    --user "${JIRA_USER_EMAIL}:${JIRA_API_TOKEN}" \
    --header "Accept: application/json" \
    --header "Content-Type: application/json" \
    "${@}"
}

# Write an informational message
function info() {
    local msg_template="${1}\n"
    local i=$(( ${#} - 1 ))
    local msg_args=("${@:2:$i}")

    #shellcheck disable=SC2059
    printf ">> ${msg_template}" "${msg_args[@]}" >&2
}

# Write an error message
function error() {
    local msg_template="${1}\n"
    local i=$(( ${#} - 1 ))
    local msg_args=("${@:2:$i}")

    #shellcheck disable=SC2059
    printf "%b!! ERROR:%b ${msg_template}%b" "${TEXT_RED}" "${TEXT_CLEAR}" "${msg_args[@]}" >&2
}

# Convert content from GitHub format to Jira format
function convert-gfm-to-jira() {
    local content="${1?Content value is required}"
    local src
    src="$(mktemp)" ||
        return 1
    printf "%s" "${content}" > "${src}"
    # NOTE: Using docker here instead of installing the pandoc package directly.
    # This is because when installing the pandoc package in CI the post install
    # tasks take multiple minutes to complete.
    docker run --rm -v "$(dirname "${src}"):/data" pandoc/core --from=gfm --to=jira "/data/$(basename "${src}")" ||
        return 1
    rm -f "${src}"
    return 0
}

# Check for environment variables that must always be set
if [ -z "${JIRA_BASE_URL}" ]; then
    error "Missing JIRA_BASE_URL environment variable"
    exit 1
fi

if [ -z "${JIRA_USER_EMAIL}" ]; then
    error "Missing JIRA_USER_EMAIL environment variable"
    exit 1
fi

if [ -z "${JIRA_API_TOKEN}" ]; then
    error "Missing JIRA_API_TOKEN environment variable"
    exit 1
fi
