import Ember from 'ember';
import { test, moduleForComponent } from 'ember-qunit';
import wait from 'ember-test-helpers/wait';
import { find, click } from 'ember-native-dom-helpers';
import hbs from 'htmlbars-inline-precompile';
import Pretender from 'pretender';
import { logEncode } from '../../mirage/data/logs';

const { run } = Ember;

const HOST = '1.1.1.1:1111';
const commonProps = {
  interval: 50,
  allocation: {
    id: 'alloc-1',
    node: {
      httpAddr: HOST,
    },
  },
  task: 'task-name',
};

const logHead = ['HEAD'];
const logTail = ['TAIL'];
const streamFrames = ['one\n', 'two\n', 'three\n', 'four\n', 'five\n'];
let streamPointer = 0;

moduleForComponent('task-log', 'Integration | Component | task log', {
  integration: true,
  beforeEach() {
    this.server = new Pretender(function() {
      this.get(`http://${HOST}/v1/client/fs/logs/:allocation_id`, ({ queryParams }) => {
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
      });
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

  assert.ok(find('.action-stdout'), 'Stdout button');
  assert.ok(find('.action-stderr'), 'Stderr button');
  assert.ok(find('.action-head'), 'Head button');
  assert.ok(find('.action-tail'), 'Tail button');
  assert.ok(find('.action-toggle-stream'), 'Stream toggle button');

  assert.ok(find('.boxed-section-body.is-full-bleed.is-dark'), 'Body is full-bleed and dark');

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
      find('.cli-window').textContent,
      streamFrames[0],
      'First chunk of streaming log is shown'
    );
  });
});

test('Clicking Head loads the log head', function(assert) {
  this.setProperties(commonProps);
  this.render(hbs`{{task-log allocation=allocation task=task}}`);

  click('.action-head');

  return wait().then(() => {
    assert.ok(
      this.server.handledRequests.find(
        ({ queryParams: qp }) => qp.origin === 'start' && qp.plain === 'true' && qp.offset === '0'
      ),
      'Log head request was made'
    );
    assert.equal(find('.cli-window').textContent, logHead[0], 'Head of the log is shown');
  });
});

test('Clicking Tail loads the log tail', function(assert) {
  this.setProperties(commonProps);
  this.render(hbs`{{task-log allocation=allocation task=task}}`);

  click('.action-tail');

  return wait().then(() => {
    assert.ok(
      this.server.handledRequests.find(
        ({ queryParams: qp }) => qp.origin === 'end' && qp.plain === 'true'
      ),
      'Log tail request was made'
    );
    assert.equal(find('.cli-window').textContent, logTail[0], 'Tail of the log is shown');
  });
});

test('Clicking toggleStream starts and stops the log stream', function(assert) {
  const { interval } = commonProps;
  this.setProperties(commonProps);
  this.render(hbs`{{task-log allocation=allocation task=task interval=interval}}`);

  run.later(() => {
    click('.action-toggle-stream');
  }, interval);

  return wait().then(() => {
    assert.equal(find('.cli-window').textContent, streamFrames[0], 'First frame loaded');

    run.later(() => {
      assert.equal(find('.cli-window').textContent, streamFrames[0], 'Still only first frame');
      click('.action-toggle-stream');
      run.later(run, run.cancelTimers, interval * 2);
    }, interval * 2);

    return wait().then(() => {
      assert.equal(
        find('.cli-window').textContent,
        streamFrames[0] + streamFrames[0] + streamFrames[1],
        'Now includes second frame'
      );
    });
  });
});

test('Clicking stderr switches the log to standard error', function(assert) {
  this.setProperties(commonProps);
  this.render(hbs`{{task-log allocation=allocation task=task}}`);

  click('.action-stderr');
  run.later(run, run.cancelTimers, commonProps.interval);

  return wait().then(() => {
    assert.ok(
      this.server.handledRequests.filter(req => req.queryParams.type === 'stderr').length,
      'stderr log requests were made'
    );
  });
});
