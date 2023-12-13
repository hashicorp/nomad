/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import EmberObject from '@ember/object';
import { task, timeout } from 'ember-concurrency';
import { decode } from 'nomad-ui/utils/stream-frames';
import AbstractLogger from './abstract-logger';
import { fetchFailure } from './log';
import classic from 'ember-classic-decorator';

@classic
export default class PollLogger extends EmberObject.extend(AbstractLogger) {
  interval = 1000;

  start() {
    return this.poll.linked().perform();
  }

  stop() {
    return this.poll.cancelAll();
  }

  @task(function* () {
    const { interval, logFetch } = this;
    while (true) {
      const url = this.fullUrl;
      let response = yield logFetch(url).then((res) => res, fetchFailure(url));

      if (!response) {
        return;
      }

      let text = yield response.text();

      if (text) {
        const { offset, message } = decode(text);
        if (message) {
          this.set('endOffset', offset);
          this.write(message);
        }
      }

      yield timeout(interval);
    }
  })
  poll;
}
