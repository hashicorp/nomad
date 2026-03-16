/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { macroCondition, isTesting } from '@embroider/macros';
import Mixin from '@ember/object/mixin';
import { assert } from '@ember/debug';
import { on } from '@ember/object/evented';

// eslint-disable-next-line ember/no-new-mixins
export default Mixin.create({
  visibilityHandler() {
    assert('visibilityHandler needs to be overridden in the Component', false);
  },

  setupDocumentVisibility: on('init', function () {
    if (!macroCondition(isTesting())) {
      this.set('_visibilityHandler', this.visibilityHandler.bind(this));
      document.addEventListener('visibilitychange', this._visibilityHandler);
    }
  }),

  removeDocumentVisibility: on('init', function () {
    if (!macroCondition(isTesting())) {
      document.removeEventListener('visibilitychange', this._visibilityHandler);
    }
  }),
});
