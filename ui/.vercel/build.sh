# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# Vercel seemingly has a bootstrap timing issue which causes the build to fail
# when we do not have this delay.
sleep 2s

ember build
mkdir -p ui-dist/ui
mv dist/* ui-dist/ui/
