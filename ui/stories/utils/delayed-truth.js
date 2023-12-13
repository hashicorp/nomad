/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import EmberObject from '@ember/object';
import { next } from '@ember/runloop';

/**
 * This has a `complete` property that turns from false
 * to true in the next tick, which helps with some
 * Storybook race condition rendering problems.
 */

export default EmberObject.extend({
  init() {
    this._super(...arguments);
    this.set('complete', false);

    next(this, () => {
      this.set('complete', true);
    });
  },
});
