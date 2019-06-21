import Mixin from '@ember/object/mixin';
import { computed } from '@ember/object';
import { assert } from '@ember/debug';
import { inject as service } from '@ember/service';
import WithVisibilityDetection from './with-route-visibility-detection';
import { on } from '@ember/object/evented';
import codesForError from 'nomad-ui/utils/codes-for-error';

export default Mixin.create(WithVisibilityDetection, {
  flashMessages: service(),

  watchers: computed(() => []),

  init() {
    this._super(...arguments);
    this.displayedFlashMessages = [];
  },

  cancelAllWatchers() {
    this.watchers.forEach(watcher => {
      assert('Watchers must be Ember Concurrency Tasks.', !!watcher.cancelAll);
      watcher.cancelAll();
    });
  },

  removeFlashMessages() {
    this.displayedFlashMessages.forEach(message => this.flashMessages.queue.removeObject(message));
    this.displayedFlashMessages = [];
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

  watchRecordErrored: on('watch:errored', function(taskInstance, { error, modelName }) {
    const statusIs404 = codesForError(error).includes('404');
    const errorIsEmptyResponse = error.message.includes('response did not have any data');

    if (statusIs404 || errorIsEmptyResponse) {
      const message = this.flashMessages
        .warning(`This ${modelName} no longer exists`, { sticky: true })
        .getFlashObject();

      if (this.displayedFlashMessages) {
        this.displayedFlashMessages.push(message);
      }
    }
  }),

  actions: {
    willTransition(transition) {
      // Don't cancel watchers if transitioning into a sub-route
      if (!transition.intent.name || !transition.intent.name.startsWith(this.routeName)) {
        this.cancelAllWatchers();
        this.removeFlashMessages();
      }

      // Bubble the action up to the application route
      return true;
    },
  },
});
