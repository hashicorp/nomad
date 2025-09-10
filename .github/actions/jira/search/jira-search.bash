#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

source "$(dirname "${BASH_SOURCE[0]}")/../shared.bash"

# Check for required inputs
if [ -z "${JQL}" ]; then
    error "Missing 'jql' input value"
    exit 1
fi

info "Searching for existing JIRA issue..."
info "JQL: %s" "${JQL}"
template='{jql: $jql}'
search="$(jq -n --arg jql "${JQL}" "${template}")" || exit
result="$(jira-request --request "POST" --data "${search}" "${JIRA_BASE_URL}/rest/api/3/search/jql")" || exit
issue="$(jq -r '.issues[].id' <<< "${result}")"

if [ -z "${issue}" ]; then
    info "No existing issue found in JIRA"
    exit
fi

info "Existing JIRA issue found: %s" "${issue}"

# Make issue available in output
printf "issue=%s\n" "${issue}" >> "${GITHUB_OUTPUT}"
