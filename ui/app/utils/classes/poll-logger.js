import Ember from 'ember';
import queryString from 'npm:query-string';
import { task, timeout } from 'ember-concurrency';

const { Object: EmberObject, computed, assign } = Ember;

export default EmberObject.extend({
  url: '',
  interval: 1000,
  params: computed(() => ({})),
  logFetch() {
    Ember.assert(
      'Loggers need a logFetch method, which should have an interface like window.fetch'
    );
  },

  endOffset: null,

  fullUrl: computed('url', 'endOffset', 'params', function() {
    const endOffset = this.get('endOffset');
    let additionalParams;
    if (endOffset) {
      additionalParams = {
        origin: 'start',
        offset: this.get('endOffset'),
      };
    } else {
      additionalParams = {
        origin: 'end',
        offset: 50000,
      };
    }
    const queryParams = queryString.stringify(assign({}, this.get('params'), additionalParams));
    return `${this.get('url')}?${queryParams}`;
  }),

  start() {
    return this.get('poll').perform();
  },

  stop() {
    return this.get('poll').cancelAll();
  },

  poll: task(function*() {
    const { interval, logFetch, fullUrl } = this.getProperties('interval', 'logFetch', 'fullUrl');
    while (true) {
      yield timeout(interval);
      let text = yield logFetch(fullUrl).then(res => res.text());

      const lines = text.replace(/\}\{/g, '}\n{').split('\n');
      const frames = lines.map(line => JSON.parse(line));
      frames.forEach(frame => (frame.Data = window.atob(frame.Data)));

      this.set('endOffset', frames[frames.length - 1].Offset);
      this.get('write')(frames.mapBy('Data').join(''));
    }
  }),
});
