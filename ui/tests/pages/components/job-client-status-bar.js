/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { attribute, clickable, collection } from 'ember-cli-page-object';

export default (scope) => ({
  scope,

  slices: collection('svg .bars g', {
    label: attribute('data-test-slice-label'),
    click: clickable(),
  }),

  expand: {
    scope: '[data-test-accordion-toggle]',
    click: clickable(),
  },

  legend: {
    scope: '.legend',

    items: collection('li', {
      label: attribute('data-test-legend-label'),
    }),

    clickableItems: collection('li.is-clickable', {
      label: attribute('data-test-legend-label'),
      click: clickable('a'),
    }),
  },

  visitSlice: async function (label) {
    await this.slices.toArray().findBy('label', label).click();
  },

  visitLegend: async function (label) {
    await this.legend.clickableItems.toArray().findBy('label', label).click();
  },
});
