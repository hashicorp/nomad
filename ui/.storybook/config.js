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

  // This adds styling to the Canvas tab.
  const styles = {
    style: {
      margin: '20px',
    },
  };

  // Create a div to wrap the Canvas tab with the applied styles.
  const element = document.createElement('div');
  Object.assign(element.style, styles.style);

  const innerElement = document.createElement('div');
  innerElement.setAttribute('id', 'storybook');
  const wormhole = document.createElement('div');
  wormhole.setAttribute('id', 'ember-basic-dropdown-wormhole');
  innerElement.appendChild(wormhole);

  element.appendChild(innerElement);
  innerElement.appendTo = function appendTo(el) {
    el.appendChild(element);
  };

  return {
    template,
    context,
    element: innerElement,
  };
});

// The order of import controls the sorting in the sidebar
configure([
  require.context('../stories/theme', true, /\.stories\.js$/),
  require.context('../stories/components', true, /\.stories\.js$/),
  require.context('../stories/charts', true, /\.stories\.js$/),
], module);
