/* eslint-env node */
import { addDecorator, addParameters, configure } from '@storybook/ember';
import { INITIAL_VIEWPORTS } from '@storybook/addon-viewport';
import theme from './theme.js';

addParameters({
  viewport: { viewports: INITIAL_VIEWPORTS },
  options: { theme },
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

configure(require.context('../stories', true, /\.stories\.js$/), module);
