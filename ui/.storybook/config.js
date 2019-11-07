/* eslint-env node */
import { addDecorator, addParameters, configure } from '@storybook/ember';
import { INITIAL_VIEWPORTS } from '@storybook/addon-viewport';
import theme from './theme.js';

addParameters({
  viewport: { viewports: INITIAL_VIEWPORTS },
  options: {
    showPanel: true,
    theme
  },
});

addDecorator(storyFn => {
  const { template, context } = storyFn();

  // This is applied to a wrapper element just inside .ember-application
  const wrapperElementStyle = {
    margin: '20px',
  };

  const applicationWrapperElement = document.createElement('div');
  Object.assign(applicationWrapperElement.style, wrapperElementStyle);

  const storybookElement = document.createElement('div');
  storybookElement.setAttribute('id', 'storybook');

  const wormhole = document.createElement('div');
  wormhole.setAttribute('id', 'ember-basic-dropdown-wormhole');

  storybookElement.appendChild(wormhole);

  applicationWrapperElement.appendChild(storybookElement);
  storybookElement.appendTo = function appendTo(el) {
    el.appendChild(applicationWrapperElement);
  };

  return {
    template,
    context,
    element: storybookElement,
  };
});

// The order of import controls the sorting in the sidebar
configure([
  require.context('../stories/theme', true, /\.stories\.js$/),
  require.context('../stories/components', true, /\.stories\.js$/),
  require.context('../stories/charts', true, /\.stories\.js$/),
], module);
