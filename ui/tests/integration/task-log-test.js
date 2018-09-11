import { run } from '@ember/runloop';
import { test, moduleForComponent } from 'ember-qunit';
import wait from 'ember-test-helpers/wait';
import { find, click } from 'ember-native-dom-helpers';
import hbs from 'htmlbars-inline-precompile';
import Pretender from 'pretender';
import { logEncode } from '../../mirage/data/logs';

const HOST = '1.1.1.1:1111';
const allowedConnectionTime = 100;
const commonProps = {
  interval: 200,
  allocation: {
    id: 'alloc-1',
    node: {
      httpAddr: HOST,
    },
  },
  task: 'task-name',
  clientTimeout: allowedConnectionTime,
  serverTimeout: allowedConnectionTime,
};

const logHead = ['HEAD'];
const logTail = ['TAIL'];
const streamFrames = ['one\n', 'two\n', 'three\n', 'four\n', 'five\n'];
let streamPointer = 0;

moduleForComponent('task-log', 'Integration | Component | task log', {
  integration: true,
  beforeEach() {
    const handler = ({ queryParams }) => {
      const { origin, offset, plain, follow } = queryParams;

      let frames;
      let data;

      if (origin === 'start' && offset === '0' && plain && !follow) {
        frames = logHead;
      } else if (origin === 'end' && plain && !follow) {
        frames = logTail;
      } else {
        frames = streamFrames;
      }

      if (frames === streamFrames) {
        data = queryParams.plain ? frames[streamPointer] : logEncode(frames, streamPointer);
        streamPointer++;
      } else {
        data = queryParams.plain ? frames.join('') : logEncode(frames, frames.length - 1);
      }

      return [200, {}, data];
    };

    this.server = new Pretender(function() {
      this.get(`http://${HOST}/v1/client/fs/logs/:allocation_id`, handler);
      this.get('/v1/client/fs/logs/:allocation_id', handler);
      this.get('/v1/regions', () => [200, {}, '[]']);
    });
  },
  afterEach() {
    this.server.shutdown();
    streamPointer = 0;
  },
});

test('Basic appearance', function(assert) {
  this.setProperties(commonProps);
  this.render(hbs`{{task-log allocation=allocation task=task}}`);

  assert.ok(find('[data-test-log-action="stdout"]'), 'Stdout button');
  assert.ok(find('[data-test-log-action="stderr"]'), 'Stderr button');
  assert.ok(find('[data-test-log-action="head"]'), 'Head button');
  assert.ok(find('[data-test-log-action="tail"]'), 'Tail button');
  assert.ok(find('[data-test-log-action="toggle-stream"]'), 'Stream toggle button');

  assert.ok(find('[data-test-log-box].is-full-bleed.is-dark'), 'Body is full-bleed and dark');

  assert.ok(find('pre.cli-window'), 'Cli is preformatted and using the cli-window component class');
});

test('Streaming starts on creation', function(assert) {
  run.later(run, run.cancelTimers, commonProps.interval);

  this.setProperties(commonProps);
  this.render(hbs`{{task-log allocation=allocation task=task}}`);

  const logUrlRegex = new RegExp(`${HOST}/v1/client/fs/logs/${commonProps.allocation.id}`);
  assert.ok(
    this.server.handledRequests.filter(req => logUrlRegex.test(req.url)).length,
    'Log requests were made'
  );

  return wait().then(() => {
    assert.equal(
      find('[data-test-log-cli]').textContent,
      streamFrames[0],
      'First chunk of streaming log is shown'
    );
  });
});

test('Clicking Head loads the log head', function(assert) {
  this.setProperties(commonProps);
  this.render(hbs`{{task-log allocation=allocation task=task}}`);

  click('[data-test-log-action="head"]');

  return wait().then(() => {
    assert.ok(
      this.server.handledRequests.find(
        ({ queryParams: qp }) => qp.origin === 'start' && qp.plain === 'true' && qp.offset === '0'
      ),
      'Log head request was made'
    );
    assert.equal(find('[data-test-log-cli]').textContent, logHead[0], 'Head of the log is shown');
  });
});

test('Clicking Tail loads the log tail', function(assert) {
  this.setProperties(commonProps);
  this.render(hbs`{{task-log allocation=allocation task=task}}`);

  click('[data-test-log-action="tail"]');

  return wait().then(() => {
    assert.ok(
      this.server.handledRequests.find(
        ({ queryParams: qp }) => qp.origin === 'end' && qp.plain === 'true'
      ),
      'Log tail request was made'
    );
    assert.equal(find('[data-test-log-cli]').textContent, logTail[0], 'Tail of the log is shown');
  });
});

test('Clicking toggleStream starts and stops the log stream', function(assert) {
  const { interval } = commonProps;
  this.setProperties(commonProps);
  this.render(hbs`{{task-log allocation=allocation task=task interval=interval}}`);

  run.later(() => {
    click('[data-test-log-action="toggle-stream"]');
  }, interval);

  return wait().then(() => {
    assert.equal(find('[data-test-log-cli]').textContent, streamFrames[0], 'First frame loaded');

    run.later(() => {
      assert.equal(
        find('[data-test-log-cli]').textContent,
        streamFrames[0],
        'Still only first frame'
      );
      click('[data-test-log-action="toggle-stream"]');
      run.later(run, run.cancelTimers, interval * 2);
    }, interval * 2);

    return wait().then(() => {
      assert.equal(
        find('[data-test-log-cli]').textContent,
        streamFrames[0] + streamFrames[0] + streamFrames[1],
        'Now includes second frame'
      );
    });
  });
});

test('Clicking stderr switches the log to standard error', function(assert) {
  this.setProperties(commonProps);
  this.render(hbs`{{task-log allocation=allocation task=task}}`);

  click('[data-test-log-action="stderr"]');
  run.later(run, run.cancelTimers, commonProps.interval);

  return wait().then(() => {
    assert.ok(
      this.server.handledRequests.filter(req => req.queryParams.type === 'stderr').length,
      'stderr log requests were made'
    );
  });
});

test('When the client is inaccessible, task-log falls back to requesting logs through the server', function(assert) {
  run.later(run, run.cancelTimers, allowedConnectionTime * 2);

  // override client response to timeout
  this.server.get(
    `http://${HOST}/v1/client/fs/logs/:allocation_id`,
    () => [400, {}, ''],
    allowedConnectionTime * 2
  );

  this.setProperties(commonProps);
  this.render(
    hbs`{{task-log
      allocation=allocation
      task=task
      clientTimeout=clientTimeout
      serverTimeout=serverTimeout}}`
  );

  const clientUrlRegex = new RegExp(`${HOST}/v1/client/fs/logs/${commonProps.allocation.id}`);
  assert.ok(
    this.server.handledRequests.filter(req => clientUrlRegex.test(req.url)).length,
    'Log request was initially made directly to the client'
  );

  return wait().then(() => {
    const serverUrl = `/v1/client/fs/logs/${commonProps.allocation.id}`;
    assert.ok(
      this.server.handledRequests.filter(req => req.url.startsWith(serverUrl)).length,
      'Log request was later made to the server'
    );
  });
});

test('When both the client and the server are inaccessible, an error message is shown', function(assert) {
  run.later(run, run.cancelTimers, allowedConnectionTime * 5);

  // override client and server responses to timeout
  this.server.get(
    `http://${HOST}/v1/client/fs/logs/:allocation_id`,
    () => [400, {}, ''],
    allowedConnectionTime * 2
  );
  this.server.get(
    '/v1/client/fs/logs/:allocation_id',
    () => [400, {}, ''],
    allowedConnectionTime * 2
  );

  this.setProperties(commonProps);
  this.render(
    hbs`{{task-log
      allocation=allocation
      task=task
      clientTimeout=clientTimeout
      serverTimeout=serverTimeout}}`
  );

  return wait().then(() => {
    const clientUrlRegex = new RegExp(`${HOST}/v1/client/fs/logs/${commonProps.allocation.id}`);
    assert.ok(
      this.server.handledRequests.filter(req => clientUrlRegex.test(req.url)).length,
      'Log request was initially made directly to the client'
    );
    const serverUrl = `/v1/client/fs/logs/${commonProps.allocation.id}`;
    assert.ok(
      this.server.handledRequests.filter(req => req.url.startsWith(serverUrl)).length,
      'Log request was later made to the server'
    );
    assert.ok(find('[data-test-connection-error]'), 'An error message is shown');
  });
});
