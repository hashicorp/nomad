/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { clickable, collection, text, attribute } from 'ember-cli-page-object';
import {
  selectChoose,
  clickTrigger,
} from 'ember-power-select/test-support/helpers';

export const multiFacet = (scope) => ({
  scope,

  toggle: clickable('[data-test-dropdown-trigger]'),

  options: collection('[data-test-dropdown-option]', {
    testContainer: '#ember-testing .ember-basic-dropdown-content',
    resetScope: true,
    label: text(),
    key: attribute('data-test-dropdown-option'),
    toggle: clickable('label'),
  }),
});

export const singleFacet = (scope) => ({
  scope,

  async toggle() {
    await clickTrigger(this.scope);
  },

  options: collection('.ember-power-select-option', {
    testContainer: '#ember-testing .ember-basic-dropdown-content',
    resetScope: true,
    label: text('[data-test-dropdown-option]'),
    key: attribute('data-test-dropdown-option', '[data-test-dropdown-option]'),
    async select() {
      // __parentTreeNode is clearly meant to be private in the page object API,
      // but it seems better to use that and keep the API symmetry across singleFacet
      // and multiFacet compared to moving select to the parent.
      const parentScope = this.__parentTreeNode.scope;
      await selectChoose(parentScope, this.label);
    },
  }),
});
