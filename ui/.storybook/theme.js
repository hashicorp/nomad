/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { create } from '@storybook/theming';

// From Bulma
let blackBis = 'hsl(0, 0%, 7%)';
let greyLight = 'hsl(0, 0%, 71%)';

// From product-colors.scss
let vagrantBlue = '#1563ff';

export default create({
  base: 'light',

  colorPrimary: blackBis,
  colorSecondary: vagrantBlue,

  // UI
  appBorderColor: greyLight,

  // Typography
  // From variables.scss
  fontBase:
    "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen-Sans, Ubuntu, Cantarell, 'Helvetica Neue', sans-serif",
  // From Bulma
  fontCode: 'monospace',

  // Text colors
  textColor: blackBis,

  // Toolbar default and active colors
  barTextColor: greyLight,
  barSelectedColor: 'white',
  barBg: blackBis,

  brandTitle: 'Nomad Storybook',
  brandUrl: 'https://www.nomadproject.io/',
});
