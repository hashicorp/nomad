#!/usr/bin/env bash

set -e
set -o pipefail

if [ "$#" -ne 1 ]
then
    echo "Usage: $0 NOMAD_SHA"
    exit 1
fi

if [[ -z "${CIRCLE_API_TOKEN}" ]]
then
    echo 'circle api token is not set as CIRCLE_API_TOKEN env-var.' >&2
    echo 'Token is available as release-bot-token in https://circleci.hashicorp.engineering/gh/hashicorp/nomad-enterprise/edit#api' >&2
    exit 1
fi

readonly nomad_sha="$1"

readonly script_dir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
echo $script_dir
readonly project_url=https://circleci.hashicorp.engineering/api/v1.1/project/github/hashicorp/nomad-enterprise/tree/master
echo $project_url

curl --fail --silent --show-error \
    --user "${CIRCLE_API_TOKEN}:" \
    --form "build_parameters[CIRCLE_JOB]=build" \
    --form "build_parameters[NOMAD_SHA]=${nomad_sha}" \
    --form "config=@${script_dir}/circleci-metrics.yaml" \
    ${project_url} | grep "build_url"