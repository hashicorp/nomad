/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import EmberObject, { computed } from '@ember/object';
import { task } from 'ember-concurrency';
import TextDecoder from 'nomad-ui/utils/classes/text-decoder';
import { decode } from 'nomad-ui/utils/stream-frames';
import AbstractLogger from './abstract-logger';
import { fetchFailure } from './log';
import classic from 'ember-classic-decorator';

@classic
export default class StreamLogger extends EmberObject.extend(AbstractLogger) {
  reader = null;

  static get isSupported() {
    return !!window.ReadableStream;
  }

  @computed()
  get additionalParams() {
    return {
      follow: true,
    };
  }

  start() {
    return this.poll.perform();
  }

  stop() {
    const reader = this.reader;
    if (reader) {
      reader.cancel();
    }
    return this.poll.cancelAll();
  }

  @task(function* () {
    const url = this.fullUrl;
    const logFetch = this.logFetch;

    const reader = yield logFetch(url).then((res) => {
      const reader = res.body.getReader();
      // It's possible that the logger was stopped between the time
      // polling was started and the log request responded.
      // If the logger was stopped, the reader needs to be immediately
      // canceled to prevent an endless request running in the background.
      if (this.poll.isRunning) {
        return reader;
      }
      reader.cancel();
    }, fetchFailure(url));

    if (!reader) {
      return;
    }

    this.set('reader', reader);

    let streamClosed = false;
    let buffer = '';
    const decoder = new TextDecoder();

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
          const { offset, message } = decode(chunk);
          if (message) {
            this.set('endOffset', offset);
            this.write(message);
          }
        }
      });
    }
  })
  poll;
}
