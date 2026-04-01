# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1
sleep 10s
ember build
mkdir -p ui-dist/ui
mv dist/* ui-dist/ui/
