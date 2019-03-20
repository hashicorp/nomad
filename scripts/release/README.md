# CircleCI Release scripts

This folder contain scripts used for triggering release jobs on CircleCI.  The release job would build all OSS, Enterprise, and Pro binary to be available as CircleCI artifacts.  They would also be available in S3 as `s3://nomad-team-test-binary/release-prep/${NOMAD_VERSION}/circleci-${CIRCLE_BUILD_NUM}/${ARTIFACT_NAME}`.

The general workflow for cutting a release would be:

1. Trigger a build on CircleCI:
   * Ensure that you have `CIRCLE_API_TOKEN` env var set.  It's available as `release-bot-token` in https://circleci.hashicorp.engineering/gh/hashicorp/nomad-enterprise/edit#api .  Use `envchain` or other tools if convenient
   * Run `./scripts/release/trigger-release-job <version> <oss_revision> <enterprise_revision>`
2. Inspect artifacts from the CircleCI job and run some manual tests on them
3. Download the repository tarball from S3, that contain generated files, and run `./scripts/dist.sh`, update changelog/new-version then `git push`

