import Mixin from '@ember/object/mixin';
import { computed } from '@ember/object';
import { assert } from '@ember/debug';
import WithVisibilityDetection from './with-route-visibility-detection';

export default Mixin.create(WithVisibilityDetection, {
  watchers: computed(() => []),

  cancelAllWatchers() {
    this.get('watchers').forEach(watcher => {
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
    willTransition() {
      this.cancelAllWatchers();
    },
  },
});
