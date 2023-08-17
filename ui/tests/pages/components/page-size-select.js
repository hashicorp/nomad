/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { clickable, collection, isPresent, text } from 'ember-cli-page-object';

export default () => ({
  isPresent: isPresent(
    '[data-test-page-size-select-parent] .ember-power-select-trigger'
  ),
  open: clickable(
    '[data-test-page-size-select-parent] .ember-power-select-trigger'
  ),
  selectedOption: text(
    '[data-test-page-size-select-parent] .ember-power-select-selected-item'
  ),
  options: collection('.ember-power-select-option', {
    testContainer: '#ember-testing',
    resetScope: true,
    label: text(),
  }),
});
