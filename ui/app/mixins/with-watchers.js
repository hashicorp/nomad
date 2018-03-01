import Mixin from '@ember/object/mixin';
import { computed } from '@ember/object';
import { assert } from '@ember/debug';

export default Mixin.create({
  watchers: computed(() => []),

  actions: {
    willTransition() {
      this.get('watchers').forEach(watcher => {
        assert('Watchers must be Ember Concurrency Tasks.', !!watcher.cancelAll);
        watcher.cancelAll();
      });
    },
  },
});
