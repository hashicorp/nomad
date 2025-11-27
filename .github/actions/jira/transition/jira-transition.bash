#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

source "$(dirname "${BASH_SOURCE[0]}")/../shared.bash"

# Do all the checks up front so we just bail before
# doing any actual work.

if [ -z "${ISSUE}" ]; then
    error "Missing 'issue' input value"
    exit 1
fi

if [ -z "${TRANSITION}" ]; then
    error "Missing 'transition' input value"
    exit 1
fi

if [ -n "${FIXED_VERSION}" ] && [ -z "${RESOLUTION}" ]; then
    error "The 'fixed-version' value can only be set if 'resolution' is set"
    exit 1
fi

if [ -n "${FIXED_VERSION}" ] && [ -z "${PROJECT}" ]; then
    error "Missing 'project' input value (must be set if 'fixed-version' is set)"
    exit 1
fi

if [ "${FIXED_VERSION}" == "auto" ] && [ -z "${TIMELINE_URL}" ]; then
    error "Missing 'timeline-url' input value (must be set if 'fixed-version' is 'auto')"
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

# Create the initial payload. This will be updated as needed prior
# to sending the request.
template='{transition: {id: $transition_id}}'
payload="$(jq -n --arg transition_id "${transition_id}" "${template}")" || exit

info "Transitioning JIRA issue '%s' to %s (ID: %s)" "${ISSUE}" \
    "${TRANSITION}" "${transition_id}"

# If a resolution is set, find it
if [ -n "${RESOLUTION}" ]; then
    # Grab the resolution ID
    result="$(jira-request "${JIRA_BASE_URL}/rest/api/3/resolution")" || exit
    query="$(printf '.[] | select(.name == "%s").id' "${RESOLUTION}")"
    resolution_id="$(jq -r "${query}" <<< "${result}")"

    if [ -z "${resolution_id}" ]; then
        error "Could not find matching resolution with name matching '%s'" "${RESOLUTION}"
        exit 1
    fi

    # Render the data structure for the resolution
    template='{fields: {resolution: {id: $resolution_id}}}'
    rendered="$(jq -n --arg resolution_id "${resolution_id}" "${template}")" || exit

    # Add it to the payload
    payload="$(jq -s '.[0] * .[1]' <<< "${payload}${rendered}")"

    info "Resolving JIRA issue '%s' to %s (ID: %s)" "${ISSUE}" "${RESOLUTION}" "${resolution_id}"
fi

# Handle setting fixed versions if set
if [ -n "${FIXED_VERSION}" ]; then
    fixed_versions=()
    
    # First pull the valid versions from jira
    project="$(jira-request "${JIRA_BASE_URL}/rest/api/3/project/${PROJECT}")" || exit
    jira_versions="$(jq -r '.versions' <<< "${project}")"

    # If the fixed version value is auto, attempt to detect versions from pull linked pull request
    if [ "${FIXED_VERSION}" == "auto" ]; then
        timeline="$(curl -sL --show-error --fail-with-body -H "Accept: application/vnd.github+json" "${TIMELINE_URL}")" || exit
        filter='.[] | select(.event == "cross-referenced") | .source.issue.labels.[] | select(.name | startswith("backport")).name'
        readarray -t labels < <(jq -r "${filter}" <<< "${timeline}")

        for label in "${labels[@]}"; do
            # start with stripping off the start of the label (backport/ or backport/ent/)
            version_prefix="${label##*/}"
            # then strip off the end of the label
            version_prefix="${version_prefix%.x*}"

            filter="$(printf '.[] | select(.name | contains("%s")).name' "${version_prefix}")"
            readarray -t valid_versions < <(jq -r "${filter}" <<< "${jira_versions}")
            match="${version_prefix}.0"
            for v in "${valid_versions[@]}"; do
                if [ "${v##*.}" -gt "${match##*.}" ]; then
                    match="${v}"
                fi
            done

            filter="$(printf '.[] | select(.name | endswith("%s")).id' "${match}")"
            version_id="$(jq -r "${filter}" <<< "${jira_versions}")"
            if [ -z "${version_id}" ]; then
                printf "WARNING: Failed to find valid JIRA version to match label: '%s'\n" "${label}"
                continue
            fi
            fixed_versions+=("$(jq -n --arg version_id "${version_id}" '[{id: $version_id}]')")
        done
    else
        # Match version directly
        filter="$(printf '.[] | select(.name | endswith("%s")).id' "${FIXED_VERSION}")"
        version_id="$(jq -r "${filter}" <<< "${jira_versions}")"
        if [ -z "${version_id}" ]; then
            printf "WARNING: Failed to find valid JIRA version to match provided version: '%s'\n" "${FIXED_VERSION}"
        else
            fixed_versions+=(jq -n --arg version_id "${version_id}" '[{id: $version_id}]')
        fi
    fi

    # If fixed versions are availble, create the data structure
    if [ "${#fixed_versions[@]}" -gt "0" ]; then
        template='{fields: {fixVersions: $versions}}'
        # Combine all the versions into a single array
        versions="$(jq -s 'add' <<< "${fixed_versions[*]}")"
        # Render the data structure for the fix versions
        rendered="$(jq -n --argjson versions "${versions}" "${template}")" || exit

        # Add it to the payload
        payload="$(jq -s '.[0] * .[1]' <<< "${payload}${rendered}")"
    fi
fi

info "Transition payload:\n%s" "${payload}"

jira-request --request "POST" --data "${payload}" \
    "${JIRA_BASE_URL}/rest/api/3/issue/${ISSUE}/transitions" || exit

info "JIRA issue '%s' transitioned to %s" "${ISSUE}" "${TRANSITION}"
