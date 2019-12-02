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
  let { template, context } = storyFn();

  let wrapperElementStyle = {
    margin: '20px',
  };

  let applicationWrapperElement = document.createElement('div');
  Object.assign(applicationWrapperElement.style, wrapperElementStyle);

  let storybookElement = document.createElement('div');
  storybookElement.setAttribute('id', 'storybook');

  let wormhole = document.createElement('div');
  wormhole.setAttribute('id', 'ember-basic-dropdown-wormhole');

  storybookElement.appendChild(wormhole);

  applicationWrapperElement.appendChild(storybookElement);
  storybookElement.appendTo = function appendTo(el) {
    el.appendChild(applicationWrapperElement);
  };

  /**
   * Stories that require routing (table sorting/pagination) fail
   * with the default iframe setup with this error:
   *   Path /iframe.html does not start with the provided rootURL /ui/
   *
   * Changing ENV.rootURL fixes that but then HistoryLocation.getURL
   * fails because baseURL is undefined, which is usually set up by
   * Ember CLI configuring the base element. This adds the href for
   * Ember CLI to use.
   *
   * The default target="_parent" breaks table sorting and pagination
   * by trying to navigate when clicking the query-params-changing
   * elements. Removing the base target for the iframe means that
   * navigation-requiring links within stories need to have the
   * target themselves.
   */
  let baseElement = document.querySelector('base');
  baseElement.setAttribute('href', '/');
  baseElement.removeAttribute('target');

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
