# CircleCI Release scripts

This folder contain scripts used for triggering release jobs on CircleCI.

1. Ensure that you have `CIRCLE_API_TOKEN` env var set.  It's available as `release-bot-token` in https://circleci.hashicorp.engineering/gh/hashicorp/nomad-enterprise/edit#api .  Use `envchain` or other tools if convenient
2. Run `./scripts/release/trigger-release-job <version> <oss_revision> <enterprise_revision>`


