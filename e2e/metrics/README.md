Metrics End to End Tests
========================

This folder contains scripts used for triggering a load test on AWS. This is intended to be used before cutting a release to run the same
set of workloads against two different versions of Nomad. It currently uses the end to end test terraform setup that relies on OSS build artifacts
stored in AWS S3.

The current steps are:

1. Trigger a build on CircleCI:
   * Ensure that you have `CIRCLE_API_TOKEN` env var set.  It's available as `release-bot-token` in https://circleci.hashicorp.engineering/gh/hashicorp/nomad-enterprise/edit#api .  Use `envchain` or other tools if convenient
   * Run `./scripts/metric_runner.sh <nomad_oss_sha>`
2. Examine the output from the final step of the CircleCI job. It should show URLs to monitor where you can check various metrics via Prometheus

Future Work
===========
Standardize the set of metrics to use as a dashboard. This will require bringing Graphana or another graphing system into the test infrastructure managed by terraform.