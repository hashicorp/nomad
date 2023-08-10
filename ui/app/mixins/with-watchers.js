/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Mixin from '@ember/object/mixin';
import { computed } from '@ember/object';
import { assert } from '@ember/debug';
import WithVisibilityDetection from './with-route-visibility-detection';

// eslint-disable-next-line ember/no-new-mixins
export default Mixin.create(WithVisibilityDetection, {
  watchers: computed(function () {
    return [];
  }),

  cancelAllWatchers() {
    this.watchers.forEach((watcher) => {
      assert('Watchers must be Ember Concurrency Tasks.', !!watcher.cancelAll);
      watcher.cancelAll();
    });
  },

  startWatchers() {
    assert('startWatchers needs to be overridden in the Route', false);
  },

  setupController() {
    this.startWatchers(...arguments);
    return this._super(...arguments);
  },

  visibilityHandler() {
    if (document.hidden) {
      this.cancelAllWatchers();
    } else {
      this.startWatchers(this.controller, this.controller.get('model'));
    }
  },

  actions: {
    willTransition(transition) {
      // Don't cancel watchers if transitioning into a sub-route
      // Make sure, if it starts with the route name, that it's not the same route
      if (
        !transition.intent.name ||
        !(
          transition.intent.name.startsWith(this.routeName) &&
          this.routeName !== transition.intent.name
        )
      ) {
        this.cancelAllWatchers();
      }
      // Bubble the action up to the application route
      return true;
    },
  },
});
