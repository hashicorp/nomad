import Ember from 'ember';
import { task, timeout } from 'ember-concurrency';
import AbstractLogger from './abstract-logger';

const { Object: EmberObject } = Ember;

export default EmberObject.extend(AbstractLogger, {
  interval: 1000,

  start() {
    return this.get('poll').perform();
  },

  stop() {
    return this.get('poll').cancelAll();
  },

  poll: task(function*() {
    const { interval, logFetch } = this.getProperties('interval', 'logFetch');
    while (true) {
      let text = yield logFetch(this.get('fullUrl')).then(res => res.text());

      if (text) {
        const lines = text.replace(/\}\{/g, '}\n{').split('\n');
        const frames = lines.map(line => JSON.parse(line));
        frames.forEach(frame => (frame.Data = window.atob(frame.Data)));

        this.set('endOffset', frames[frames.length - 1].Offset);
        this.get('write')(frames.mapBy('Data').join(''));
      }

      yield timeout(interval);
    }
  }),
});
