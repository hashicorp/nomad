/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { modifier } from 'ember-modifier';

export default modifier(function autofocus(element, _positional, named) {
  const { ignore } = named;
  if (ignore) return;
  element.focus();
});
