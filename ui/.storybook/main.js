/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

/* eslint-env node */
module.exports = {
  framework: '@storybook/ember',
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
  core: {
    builder: '@storybook/builder-webpack4',
  },
};
