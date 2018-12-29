import EmberObject from '@ember/object';
import RSVP from 'rsvp';
import { run } from '@ember/runloop';
import sinon from 'sinon';
import wait from 'ember-test-helpers/wait';
import { module, test } from 'ember-qunit';
import _Log from 'nomad-ui/utils/classes/log';

let startSpy, stopSpy, initSpy, fetchSpy;

const MockStreamer = EmberObject.extend({
  poll: {
    isRunning: false,
  },

  init() {
    initSpy(...arguments);
  },

  start() {
    this.get('poll').isRunning = true;
    startSpy(...arguments);
  },

  stop() {
    this.get('poll').isRunning = true;
    stopSpy(...arguments);
  },

  step(chunk) {
    if (this.get('poll').isRunning) {
      this.get('write')(chunk);
    }
  },
});

const Log = _Log.extend({
  init() {
    this._super();
    const props = this.get('logStreamer').getProperties('url', 'params', 'logFetch', 'write');
    this.set('logStreamer', MockStreamer.create(props));
  },
});

module('Unit | Util | Log', {
  beforeEach() {
    initSpy = sinon.spy();
    startSpy = sinon.spy();
    stopSpy = sinon.spy();
    fetchSpy = sinon.spy();
  },
});

const makeMocks = output => ({
  url: '/test-url/',
  params: {
    a: 'param',
    another: 'one',
  },
  logFetch: function() {
    fetchSpy(...arguments);
    return RSVP.Promise.resolve({
      text() {
        return output;
      },
    });
  },
});

test('logStreamer is created on init', function(assert) {
  const log = Log.create(makeMocks(''));

  assert.ok(log.get('logStreamer'), 'logStreamer property is defined');
  assert.ok(initSpy.calledOnce, 'logStreamer init was called');
});

test('gotoHead builds the correct URL', function(assert) {
  const mocks = makeMocks('');
  const expectedUrl = `${mocks.url}?a=param&another=one&offset=0&origin=start&plain=true`;
  const log = Log.create(mocks);

  run(() => {
    log.get('gotoHead').perform();
    assert.ok(fetchSpy.calledWith(expectedUrl), `gotoHead URL was ${expectedUrl}`);
  });
});

test('When gotoHead returns too large of a log, the log is truncated', function(assert) {
  const longLog = Array(50001)
    .fill('a')
    .join('');
  const truncationMessage =
    '\n\n---------- TRUNCATED: Click "tail" to view the bottom of the log ----------';

  const mocks = makeMocks(longLog);
  const log = Log.create(mocks);

  run(() => {
    log.get('gotoHead').perform();
  });

  return wait().then(() => {
    assert.ok(log.get('output').endsWith(truncationMessage), 'Truncation message is shown');
    assert.equal(
      log.get('output').length,
      50000 + truncationMessage.length,
      'Output is truncated the appropriate amount'
    );
  });
});

test('gotoTail builds the correct URL', function(assert) {
  const mocks = makeMocks('');
  const expectedUrl = `${mocks.url}?a=param&another=one&offset=50000&origin=end&plain=true`;
  const log = Log.create(mocks);

  run(() => {
    log.get('gotoTail').perform();
    assert.ok(fetchSpy.calledWith(expectedUrl), `gotoTail URL was ${expectedUrl}`);
  });
});

test('startStreaming starts the log streamer', function(assert) {
  const log = Log.create(makeMocks(''));

  log.startStreaming();
  assert.ok(startSpy.calledOnce, 'Streaming started');
  assert.equal(log.get('logPointer'), 'tail', 'Streaming points the log to the tail');
});

test('When the log streamer calls `write`, the output is appended', function(assert) {
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
  assert.equal(log.get('output'), chunk1 + chunk2 + chunk3, 'Third chunk written');
});

test('stop stops the log streamer', function(assert) {
  const log = Log.create(makeMocks(''));

  log.stop();
  assert.ok(stopSpy.calledOnce, 'Streaming stopped');
});
