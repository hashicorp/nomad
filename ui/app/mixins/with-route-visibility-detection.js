/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Ember from 'ember';
import Mixin from '@ember/object/mixin';
import { assert } from '@ember/debug';
import { on } from '@ember/object/evented';

// eslint-disable-next-line ember/no-new-mixins
export default Mixin.create({
  visibilityHandler() {
    assert('visibilityHandler needs to be overridden in the Route', false);
  },

  setupDocumentVisibility: on('activate', function () {
    if (!Ember.testing) {
      this.set('_visibilityHandler', this.visibilityHandler.bind(this));
      document.addEventListener('visibilitychange', this._visibilityHandler);
    }
  }),

  removeDocumentVisibility: on('deactivate', function () {
    if (!Ember.testing) {
      document.removeEventListener('visibilitychange', this._visibilityHandler);
    }
  }),
});
