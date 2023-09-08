/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import EmberObject from '@ember/object';
import RSVP from 'rsvp';
import { run } from '@ember/runloop';
import sinon from 'sinon';
import { module, test } from 'qunit';
import _Log from 'nomad-ui/utils/classes/log';

import { settled } from '@ember/test-helpers';

let startSpy, stopSpy, initSpy, fetchSpy;

const MockStreamer = EmberObject.extend({
  init() {
    this.poll = {
      isRunning: false,
    };

    initSpy(...arguments);
  },

  start() {
    this.poll.isRunning = true;
    startSpy(...arguments);
  },

  stop() {
    this.poll.isRunning = true;
    stopSpy(...arguments);
  },

  step(chunk) {
    if (this.poll.isRunning) {
      this.write(chunk);
    }
  },
});

const Log = _Log.extend({
  init() {
    this._super();
    const props = this.logStreamer.getProperties(
      'url',
      'params',
      'logFetch',
      'write'
    );
    this.set('logStreamer', MockStreamer.create(props));
  },
});

module('Unit | Util | Log', function (hooks) {
  hooks.beforeEach(function () {
    initSpy = sinon.spy();
    startSpy = sinon.spy();
    stopSpy = sinon.spy();
    fetchSpy = sinon.spy();
  });

  const makeMocks = (output) => ({
    url: '/test-url/',
    params: {
      a: 'param',
      another: 'one',
    },
    logFetch: function () {
      fetchSpy(...arguments);
      return RSVP.Promise.resolve({
        text() {
          return output;
        },
      });
    },
  });

  test('logStreamer is created on init', async function (assert) {
    const log = Log.create(makeMocks(''));

    assert.ok(log.get('logStreamer'), 'logStreamer property is defined');
    assert.ok(initSpy.calledOnce, 'logStreamer init was called');
  });

  test('gotoHead builds the correct URL', async function (assert) {
    assert.expect(1);

    const mocks = makeMocks('');
    const expectedUrl = `${mocks.url}?a=param&another=one&offset=0&origin=start`;
    const log = Log.create(mocks);

    run(() => {
      log.get('gotoHead').perform();
      assert.ok(
        fetchSpy.calledWith(expectedUrl),
        `gotoHead URL was ${expectedUrl}`
      );
    });
  });

  test('When gotoHead returns too large of a log, the log is truncated', async function (assert) {
    const longLog = Array(50001).fill('a').join('');
    const encodedLongLog = `{"Offset":0,"Data":"${window.btoa(longLog)}"}`;
    const truncationMessage =
      '\n\n---------- TRUNCATED: Click "tail" to view the bottom of the log ----------';

    const mocks = makeMocks(encodedLongLog);
    const log = Log.create(mocks);

    run(() => {
      log.get('gotoHead').perform();
    });

    await settled();
    assert.ok(
      log.get('output').toString().endsWith(truncationMessage),
      'Truncation message is shown'
    );
    assert.equal(
      log.get('output').toString().length,
      50000 + truncationMessage.length,
      'Output is truncated the appropriate amount'
    );
  });

  test('gotoTail builds the correct URL', async function (assert) {
    assert.expect(1);

    const mocks = makeMocks('');
    const expectedUrl = `${mocks.url}?a=param&another=one&offset=50000&origin=end`;
    const log = Log.create(mocks);

    run(() => {
      log.get('gotoTail').perform();
      assert.ok(
        fetchSpy.calledWith(expectedUrl),
        `gotoTail URL was ${expectedUrl}`
      );
    });
  });

  test('startStreaming starts the log streamer', async function (assert) {
    const log = Log.create(makeMocks(''));

    log.startStreaming();
    assert.ok(startSpy.calledOnce, 'Streaming started');
    assert.equal(
      log.get('logPointer'),
      'tail',
      'Streaming points the log to the tail'
    );
  });

  test('When the log streamer calls `write`, the output is appended', async function (assert) {
    const log = Log.create(makeMocks(''));
    const chunk1 = 'Hello';
    const chunk2 = ' World';
    const chunk3 = '\n\nEOF';

    log.startStreaming();
    assert.equal(log.get('output'), '', 'No output yet');

    log.get('logStreamer').step(chunk1);
    assert.equal(log.get('output'), chunk1, 'First chunk written');

    log.get('logStreamer').step(chunk2);
    assert.equal(log.get('output'), chunk1 + chunk2, 'Second chunk written');

    log.get('logStreamer').step(chunk3);
    assert.equal(
      log.get('output'),
      chunk1 + chunk2 + chunk3,
      'Third chunk written'
    );
  });

  test('stop stops the log streamer', async function (assert) {
    const log = Log.create(makeMocks(''));

    log.stop();
    assert.ok(stopSpy.calledOnce, 'Streaming stopped');
  });
});
