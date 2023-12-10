#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# we can't simply run 'npm playwright test' because of a bug in either
# NPM or playwright where we get either an interactive prompt or (with
# --yes) a permission denied error when trying to invoke
# playwright. Rather than trying to fix the NPM ecosystem, we'll just
# install and run as separate script steps in the container.

[[ $1 == "--no-install" ]] || npm install -D @playwright/test
npx playwright test
