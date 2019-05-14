import EmberObject from '@ember/object';
import { task, timeout } from 'ember-concurrency';
import AbstractLogger from './abstract-logger';
import { fetchFailure } from './log';

export default EmberObject.extend(AbstractLogger, {
  interval: 1000,

  start() {
    return this.poll
      .linked()
      .perform();
  },

  stop() {
    return this.poll.cancelAll();
  },

  poll: task(function*() {
    const { interval, logFetch } = this;
    while (true) {
      const url = this.fullUrl;
      let response = yield logFetch(url).then(res => res, fetchFailure(url));

      if (!response) {
        return;
      }

      let text = yield response.text();

      if (text) {
        const lines = text.replace(/\}\{/g, '}\n{').split('\n');
        const frames = lines
          .map(line => JSON.parse(line))
          .filter(frame => frame.Data != null && frame.Offset != null);

        if (frames.length) {
          frames.forEach(frame => (frame.Data = window.atob(frame.Data)));
          this.set('endOffset', frames[frames.length - 1].Offset);
          this.write(frames.mapBy('Data').join(''));
        }
      }

      yield timeout(interval);
    }
  }),
});
