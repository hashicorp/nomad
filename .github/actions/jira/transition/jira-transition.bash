#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

source "$(dirname "${BASH_SOURCE[0]}")/../shared.bash"

# Check for required inputs
if [ -z "${ISSUE}" ]; then
    error "Missing 'issue' input value"
    exit 1
fi

if [ -z "${TRANSITION}" ]; then
    error "Missing 'transition' input value"
    exit 1
fi

# Grab the transition ID
result="$(jira-request "${JIRA_BASE_URL}/rest/api/3/issue/${ISSUE}/transitions")" || exit
query="$(printf '.transitions[] | select(.name == "%s").id' "${TRANSITION}")"
transition_id="$(jq -r "${query}" <<< "${result}")"

if [ -z "${transition_id}" ]; then
    error "Could not find matching transition with name matching '%s'" "${TRANSITION}"
    exit 1
fi

# If a resolution is set, find it and generate the template
# with it included
if [ -n "${RESOLUTION}" ]; then
    # Grab the resolution ID
    result="$(jira-request "${JIRA_BASE_URL}/rest/api/3/resolution")" || exit
    query="$(printf '.[] | select(.name == "%s").id' "${RESOLUTION}")"
    resolution_id="$(jq -r "${query}" <<< "${result}")"

    if [ -z "${resolution_id}" ]; then
        error "Could not find matching resolution with name matching '%s'" "${RESOLUTION}"
        exit 1
    fi

    template='
{
  transition: {
    id: $transition
  },
  fields: {
    resolution: {
      id: $resolution
    }
  }
}
'
    issue_transition="$(jq -n --arg transition "${transition_id}" --arg resolution "${resolution_id}" "${template}")" || exit
else
    # No resolution so the template only includes the transition
    template='
{
  transition: {
    id: $transition
  }
}
'
    issue_transition="$(jq -n --arg transition "${transition_id}" "${template}")" || exit
fi

info "Transitioning JIRA issue '%s' to %s (ID: %s)" "${ISSUE}" \
    "${TRANSITION}" "${transition_id}"
if [ -n "${resolution_id}" ]; then
    info "Resolving JIRA issue '%s' as %s (ID: %s)" "${ISSUE}" \
        "${RESOLUTION}" "${resolution_id}"
fi

info "Transition payload:\n%s" "${issue_transition}"

jira-request --request "POST" --data "${issue_transition}" \
    "${JIRA_BASE_URL}/rest/api/3/issue/${ISSUE}/transitions" || exit

info "JIRA issue '%s' transitioned to %s" "${ISSUE}" "${TRANSITION}"
