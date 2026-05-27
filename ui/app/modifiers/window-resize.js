/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { modifier } from 'ember-modifier';

export default modifier(function windowResize(element, [handler]) {
  const boundHandler = (ev) => handler(element, ev);
  window.addEventListener('resize', boundHandler);

  return () => {
    window.removeEventListener('resize', boundHandler);
  };
});
