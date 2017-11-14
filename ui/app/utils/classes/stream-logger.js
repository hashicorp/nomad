import Ember from 'ember';
import queryString from 'npm:query-string';
import { task } from 'ember-concurrency';
import TextDecoder from 'nomad-ui/utils/classes/text-decoder';

const { Object: EmberObject, computed, assign } = Ember;

export default EmberObject.extend({
  url: '',
  params: computed(() => ({})),
  logFetch() {
    Ember.assert(
      'Loggers need a logFetch method, which should have an interface like window.fetch'
    );
  },

  reader: null,

  start() {
    return this.get('poll').perform();
  },

  stop() {
    const reader = this.get('reader');
    if (reader) {
      reader.cancel();
    }
    return this.get('poll').cancelAll();
  },

  poll: task(function*() {
    const queryParams = queryString.stringify(
      assign({}, this.get('params'), {
        plain: true,
        follow: true,
        origin: 'end',
        offset: 50000,
      })
    );
    const url = `${this.get('url')}?${queryParams}`;
    const logFetch = this.get('logFetch');
    let streamClosed = false;

    const reader = yield logFetch(url).then(res => res.body.getReader());
    this.set('reader', reader);

    const decoder = new TextDecoder();
    while (!streamClosed) {
      yield reader.read().then(({ value, done }) => {
        streamClosed = done;
        this.get('write')(decoder.decode(value, { stream: true }));
      });
    }
  }),
});
