/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { isPresent, text } from 'ember-cli-page-object';

export default (scope) => ({
  scope,

  svgIsPresent: isPresent('[data-test-gauge-svg]'),
  label: text('[data-test-label]'),
  percentage: text('[data-test-percentage]'),
});
