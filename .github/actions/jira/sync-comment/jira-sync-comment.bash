#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

source "$(dirname "${BASH_SOURCE[0]}")/../shared.bash"

# Check for required inputs
if [ -z "${ISSUE}" ]; then
    error "Missing 'issue' input value"
    exit 1
fi

if [ -z "${COMMENT}" ]; then
    error "Missing 'comment' input value"
    exit 1
fi

comment="$(convert-gfm-to-jira "${COMMENT}")" || exit
template='
{
  body: $comment
}
'
issue_comment="$(jq -n --arg comment "${comment}" "${template}")"

info "Adding comment to JIRA issue %s" "${ISSUE}"
info "Comment payload: %s" "${issue_comment}"

# Create the comment
# NOTE: The v2 API is used here for creating the comment. This is because
# the v3 API only supports the Atlassian Document Format for which pandoc
# currently does not have support (https://github.com/jgm/pandoc/issues/9898)
result="$(jira-request --request "POST" --data "${issue_comment}" "${JIRA_BASE_URL}/rest/api/2/issue/${ISSUE}/comment")" || exit
comment_id="$(jq -r .id <<< "${result}")"

info "JIRA issue ID %s updated with new comment ID %s" "${ISSUE}" "${comment_id}"

printf "comment-id=%s\n" "${comment_id}" >> "${GITHUB_OUTPUT}"
