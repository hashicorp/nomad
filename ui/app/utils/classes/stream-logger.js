import EmberObject, { computed } from '@ember/object';
import { task } from 'ember-concurrency';
import TextDecoder from 'nomad-ui/utils/classes/text-decoder';
import AbstractLogger from './abstract-logger';
import { fetchFailure } from './log';

export default EmberObject.extend(AbstractLogger, {
  reader: null,

  additionalParams: computed(() => ({
    follow: true,
  })),

  start() {
    return this.poll.perform();
  },

  stop() {
    const reader = this.reader;
    if (reader) {
      reader.cancel();
    }
    return this.poll.cancelAll();
  },

  poll: task(function*() {
    const url = this.fullUrl;
    const logFetch = this.logFetch;

    let streamClosed = false;
    let buffer = '';

    const decoder = new TextDecoder();
    const reader = yield logFetch(url).then(res => res.body.getReader(), fetchFailure(url));

    if (!reader) {
      return;
    }

    this.set('reader', reader);

    while (!streamClosed) {
      yield reader.read().then(({ value, done }) => {
        streamClosed = done;

        // There is no guarantee that value will be a complete JSON object,
        // so it needs to be buffered.
        buffer += decoder.decode(value, { stream: true });

        // Only when the buffer contains a close bracket can we be sure the buffer
        // is in a complete state
        if (buffer.indexOf('}') !== -1) {
          // The buffer can be one or more complete frames with additional text for the
          // next frame
          const [, chunk, newBuffer] = buffer.match(/(.*\})(.*)$/);

          // Peel chunk off the front of the buffer (since it represents complete frames)
          // and set the buffer to be the remainder
          buffer = newBuffer;

          // Assuming the logs endpoint never returns nested JSON (it shouldn't), at this
          // point chunk is a series of valid JSON objects with no delimiter.
          const lines = chunk.replace(/\}\{/g, '}\n{').split('\n');
          const frames = lines.map(line => JSON.parse(line)).filter(frame => frame.Data);

          if (frames.length) {
            frames.forEach(frame => (frame.Data = window.atob(frame.Data)));
            this.set('endOffset', frames[frames.length - 1].Offset);
            this.write(frames.mapBy('Data').join(''));
          }
        }
      });
    }
  }),
}).reopenClass({
  isSupported: !!window.ReadableStream && !isSafari(),
});

// Fetch streaming doesn't work in Safari yet despite all the primitives being in place.
// Bug: https://bugs.webkit.org/show_bug.cgi?id=185924
// Until this is fixed, Safari needs to be explicitly targeted for poll-based logging.
function isSafari() {
  const oldSafariTest = /constructor/i.test(window.HTMLElement);
  const newSafariTest = (function(p) {
    return p.toString() === '[object SafariRemoteNotification]';
  })(!window['safari'] || (typeof window.safari !== 'undefined' && window.safari.pushNotification));
  return oldSafariTest || newSafariTest;
}
