/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { addDecorator, addParameters } from '@storybook/ember';
import { INITIAL_VIEWPORTS } from '@storybook/addon-viewport';
import theme from './theme.js';

addParameters({
  viewport: { viewports: INITIAL_VIEWPORTS },
  options: {
    showPanel: true,
    theme,
  },
});

addDecorator((storyFn) => {
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

  return {
    template,
    context,
    element: storybookElement,
  };
});
