/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { task, timeout } from 'ember-concurrency';
import jsonWithDefault from 'nomad-ui/utils/json-with-default';
import { assert } from '@ember/debug';
import classic from 'ember-classic-decorator';

@classic
export default class AbstractStatsTracker {
  // Get the passed poll_stats interval and convert to ms, or default to 2sec
  get pollInterval() {
    console.log('abstract poll interval', this.interval);
    return this.interval ? this.interval * 1000 : 2000;
  }

  get url() {
    assert('Url must be defined in the caller', this.url);
  }

  // The max number of data points tracked. Once the max is reached,
  // data points at the head of the list are removed in favor of new
  // data appended at the tail
  bufferSize = 500;

  // The number of consecutive request failures that can occur before an
  // empty frame is appended
  maxFrameMisses = 5;

  frameMisses = 0;

  fetch() {
    assert(
      'StatsTrackers need a fetch method, which should have an interface like window.fetch'
    );
  }

  append(/* frame */) {
    assert(
      'StatsTrackers need an append method, which takes the JSON response from a request to url as an argument'
    );
  }

  pause() {
    assert(
      'StatsTrackers need a pause method, which takes no arguments but adds a frame of data at the current timestamp with null as the value'
    );
  }

  handleResponse(frame) {
    if (frame.error) {
      this.incrementProperty('frameMisses');
      if (this.frameMisses >= this.maxFrameMisses) {
        // Missing enough data consecutively is effectively a pause
        this.pause();
        this.frameMisses = 0;
      }
      return;
    } else {
      this.frameMisses = 0;

      // Only append non-error frames
      this.append(frame);
    }
  }

  // Uses EC as a form of debounce to prevent multiple
  // references to the same tracker from flooding the tracker,
  // but also avoiding the issue where different places where the
  // same tracker is used needs to coordinate.
  @task *poll() {
    console.log('POLLING', this.pollInterval);
    // Interrupt any pause attempt
    this.signalPause.cancelAll();

    try {
      const url = this.url;
      assert('Url must be defined', url);

      yield this.fetch(url)
        .then(jsonWithDefault({ error: true }))
        .then((frame) => this.handleResponse(frame));
    } catch (error) {
      throw new Error(error);
    }
    yield timeout(Ember.testing ? 0 : this.pollInterval);
  }

  @task *signalPause() {
    // wait 2 seconds
    yield timeout(Ember.testing ? 0 : 2000);
    // if no poll called in 2 seconds, pause
    this.pause();
  }
}
