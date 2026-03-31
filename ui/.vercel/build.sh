# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

ember --version
ember build
mkdir -p ui-dist/ui
mv dist/* ui-dist/ui/
