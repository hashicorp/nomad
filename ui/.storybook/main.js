/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-env node */
module.exports = {
  addons: [
    '@storybook/addon-docs',
    '@storybook/addon-storysource',
    '@storybook/addon-knobs',
    '@storybook/addon-viewport',
  ],
  stories: [
    '../stories/theme/*.stories.js',
    '../stories/components/*.stories.js',
    '../stories/charts/*.stories.js',
  ],
};
