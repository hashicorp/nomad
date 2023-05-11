/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { isPresent, clickable, text } from 'ember-cli-page-object';

export default (scope) => ({
  scope,

  isPresent: isPresent(),
  dismiss: clickable('[data-test-dismiss]'),
  title: text('[data-test-title]'),
  message: text('[data-test-message]'),
});
