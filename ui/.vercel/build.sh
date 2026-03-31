# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

echo $PWD
cd .. && pnpm exec ember build && cd .vercel
mkdir -p ui-dist/ui
mv dist/* ui-dist/ui/
