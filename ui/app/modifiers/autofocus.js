/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { modifier } from 'ember-modifier';

export default modifier(function autofocus(element, _positional, named) {
  const { ignore } = named;
  if (ignore) return;
  element.focus();
});
