/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { Promise } from 'rsvp';
import sinon from 'sinon';
import StreamLogger from 'nomad-ui/utils/classes/stream-logger';

module('Unit | Util | StreamLogger', function () {
  test('when a StreamLogger is stopped before the poll request responds, the request is immediately canceled upon completion', async function (assert) {
    const fetchMock = new FetchMock();
    const fetch = fetchMock.request();

    const logger = StreamLogger.create({
      logFetch: () => fetch,
    });

    logger.start();
    await logger.stop();

    assert.notOk(logger.poll.isRunning);
    assert.equal(fetchMock.reader.cancel.callCount, 0);

    fetchMock.closeRequest();
    await fetch;

    assert.equal(fetchMock.reader.cancel.callCount, 1);
  });

  test('when the streaming request sends the done flag, the poll task completes', async function (assert) {
    const fetchMock = new FetchMock();
    const fetch = fetchMock.request();

    const logger = StreamLogger.create({
      logFetch: () => fetch,
    });

    logger.start();

    assert.ok(logger.poll.isRunning);
    assert.equal(fetchMock.reader.readSpy.callCount, 0);

    fetchMock.closeRequest();
    await fetch;

    assert.notOk(logger.poll.isRunning);
    assert.equal(fetchMock.reader.readSpy.callCount, 1);
  });

  test('disable streaming if not supported', async function (assert) {
    window.ReadableStream = null;
    assert.false(StreamLogger.isSupported);
  });
});

class FetchMock {
  constructor() {
    this._closeRequest = null;
    this.reader = new ReadableStreamMock();
    this.response = new FetchResponseMock(this.reader);
  }

  request() {
    if (this._closeRequest) {
      throw new Error('Can only call FetchMock.request once');
    }
    return new Promise((resolve) => {
      this._closeRequest = resolve;
    });
  }

  closeRequest() {
    if (this._closeRequest) {
      this._closeRequest(this.response);
    } else {
      throw new Error(
        'Must call FetchMock.request() before FetchMock.closeRequest'
      );
    }
  }
}

class FetchResponseMock {
  constructor(reader) {
    this.reader = reader;
    this.body = {
      getReader() {
        return reader;
      },
    };
  }
}

class ReadableStreamMock {
  constructor() {
    this.cancel = sinon.spy();
    this.readSpy = sinon.spy();
  }

  read() {
    this.readSpy();
    return new Promise((resolve) => {
      resolve({ value: new ArrayBuffer(0), done: true });
    });
  }
}
