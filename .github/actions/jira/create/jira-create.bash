#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

source "$(dirname "${BASH_SOURCE[0]}")/../shared.bash"

# Check for required input values
if [ -z "${ISSUE_TYPE}" ]; then
    error "Missing 'issueType' input value"
    exit 1
fi

if [ -z "${PROJECT}" ]; then
    error "Missing 'project' input value"
    exit 1
fi

if [ -z "${SUMMARY}" ]; then
    error "Missing 'summary' input value"
    exit 1
fi

# Grab the issue type ID
result="$(jira-request "${JIRA_BASE_URL}/rest/api/3/issuetype")" || exit
query="$(printf '.[] | select(.name == "%s").id' "${ISSUE_TYPE}")"
type_id="$(jq -r "${query}" <<< "${result}")"

if [ -z "${type_id}" ]; then
    error "Could not find issue type with name '%s'" "${ISSUE_TYPE}"
    exit 1
fi

info "Issue type ID for '%s': %s" "${ISSUE_TYPE}" "${type_id}"

if [ -n "${DESCRIPTION}" ]; then
  description="$(convert-gfm-to-jira "${DESCRIPTION}")" || exit
fi

# Base template for issue creation
template='
{
  description: $description,
  issuetype: {
    id: $issuetype
  },
  project: {
    key: $project
  },
  summary: $summary
}'
new_issue="$(jq -n --arg description "${description}" --arg issuetype "${type_id}" --arg project "${PROJECT}" --arg summary "${SUMMARY}" "${template}")" || exit

# If there are extra fields provided, merge them in
if [ -n "${EXTRA_FIELDS}" ]; then
    new_issue="$(printf "%s %s" "${new_issue}" "${EXTRA_FIELDS}" | jq -s add)"
fi

# Wrap the payload for submission
template='{fields: $fields}'
new_issue="$(jq -n --argjson fields "${new_issue}" "${template}")" || exit

info "JIRA new issue payload:\n%s" "${new_issue}"

# Create the issue
# NOTE: The v2 API is used here for creating the issue. This is because
# the v3 API only supports the Atlassian Document Format for which pandoc
# currently does not have support (https://github.com/jgm/pandoc/issues/9898)
result="$(jira-request --request "POST" --data "${new_issue}" "${JIRA_BASE_URL}/rest/api/2/issue")" || exit
key="$(jq -r '.key' <<< "${result}")"
id="$(jq -r '.id' <<< "${result}")"

printf "issue=%s\n" "${id}" >> "${GITHUB_OUTPUT}"
printf "issue-key=%s\n" "${key}" >> "${GITHUB_OUTPUT}"

info ">> New JIRA issue created: %s/browse/%s" "${JIRA_BASE_URL}" "${key}"
