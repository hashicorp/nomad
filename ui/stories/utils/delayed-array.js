/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { A } from '@ember/array';
import ArrayProxy from '@ember/array/proxy';
import { next } from '@ember/runloop';

/**
 * This is an array whose content is empty until the next
 * tick, which fixes Storybook race condition rendering
 * problems.
 */

export default ArrayProxy.extend({
  init(array) {
    this.set('content', A([]));
    this._super(...arguments);
    this[Symbol.iterator] = this.content[Symbol.iterator];

    next(this, () => {
      this.set('content', A(array));
      this[Symbol.iterator] = this.content[Symbol.iterator];
    });
  },
});
