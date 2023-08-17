# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

STORYBOOK_LINK=true ember build
mkdir -p ui-dist/ui
mv dist/* ui-dist/ui/

yarn build-storybook
mv storybook-static ui-dist/storybook/
