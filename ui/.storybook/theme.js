import { create } from '@storybook/theming';

// From Bulma
const blackBis = 'hsl(0, 0%, 7%)';
const greyLight = 'hsl(0, 0%, 71%)';

// From product-colors.scss
const packerBlue = '#1563ff';

export default create({
  base: 'light',

  colorPrimary: blackBis,
  colorSecondary: packerBlue,

  // UI
  appBorderColor: greyLight,

  // Typography
  // From variables.scss
  fontBase: "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen-Sans, Ubuntu, Cantarell, 'Helvetica Neue', sans-serif",
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
