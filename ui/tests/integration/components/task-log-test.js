/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { run } from '@ember/runloop';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { find, click, render, settled } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import Pretender from 'pretender';
import { logEncode } from '../../../mirage/data/logs';

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
  taskState: 'task-name',
  clientTimeout: allowedConnectionTime,
  serverTimeout: allowedConnectionTime,
};

const logHead = [logEncode(['HEAD'], 0)];
const logTail = [logEncode(['TAIL'], 0)];
const streamFrames = ['one\n', 'two\n', 'three\n', 'four\n', 'five\n'];
let streamPointer = 0;
let logMode = null;

module('Integration | Component | task log', function (hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function () {
    const handler = ({ queryParams }) => {
      let frames;
      let data;

      if (logMode === 'head') {
        frames = logHead;
      } else if (logMode === 'tail') {
        frames = logTail;
      } else {
        frames = streamFrames;
      }

      if (frames === streamFrames) {
        data = queryParams.plain
          ? frames[streamPointer]
          : logEncode(frames, streamPointer);
        streamPointer++;
      } else {
        data = queryParams.plain
          ? frames.join('')
          : logEncode(frames, frames.length - 1);
      }

      return [200, {}, data];
    };

    this.server = new Pretender(function () {
      this.get(`http://${HOST}/v1/client/fs/logs/:allocation_id`, handler);
      this.get('/v1/client/fs/logs/:allocation_id', handler);
      this.get('/v1/regions', () => [200, {}, '[]']);
    });
  });

  hooks.afterEach(function () {
    window.localStorage.clear();
    this.server.shutdown();
    streamPointer = 0;
    logMode = null;
  });

  test('Basic appearance', async function (assert) {
    assert.expect(8);

    run.later(run, run.cancelTimers, commonProps.interval);

    this.setProperties(commonProps);
    await render(
      hbs`<TaskLog @allocation={{allocation}} @task={{taskState}} />`
    );

    assert.ok(find('[data-test-log-action="stdout"]'), 'Stdout button');
    assert.ok(find('[data-test-log-action="stderr"]'), 'Stderr button');
    assert.ok(find('[data-test-log-action="head"]'), 'Head button');
    assert.ok(find('[data-test-log-action="tail"]'), 'Tail button');
    assert.ok(
      find('[data-test-log-action="toggle-stream"]'),
      'Stream toggle button'
    );

    assert.ok(
      find('[data-test-log-box].is-full-bleed.is-dark'),
      'Body is full-bleed and dark'
    );

    assert.ok(
      find('pre.cli-window'),
      'Cli is preformatted and using the cli-window component class'
    );

    await componentA11yAudit(this.element, assert);
  });

  test('Streaming starts on creation', async function (assert) {
    assert.expect(3);

    run.later(run, run.cancelTimers, commonProps.interval);

    this.setProperties(commonProps);
    await render(
      hbs`<TaskLog @allocation={{allocation}} @task={{taskState}} />`
    );

    const logUrlRegex = new RegExp(
      `${HOST}/v1/client/fs/logs/${commonProps.allocation.id}`
    );
    assert.ok(
      this.server.handledRequests.filter((req) => logUrlRegex.test(req.url))
        .length,
      'Log requests were made'
    );

    await settled();
    assert.equal(
      find('[data-test-log-cli]').textContent,
      streamFrames[0],
      'First chunk of streaming log is shown'
    );

    await componentA11yAudit(this.element, assert);
  });

  test('Clicking Head loads the log head', async function (assert) {
    logMode = 'head';
    run.later(run, run.cancelTimers, commonProps.interval);

    this.setProperties(commonProps);
    await render(
      hbs`<TaskLog @allocation={{allocation}} @task={{taskState}} />`
    );

    click('[data-test-log-action="head"]');

    await settled();
    assert.ok(
      this.server.handledRequests.find(
        ({ queryParams: qp }) => qp.origin === 'start' && qp.offset === '0'
      ),
      'Log head request was made'
    );
    assert.equal(
      find('[data-test-log-cli]').textContent,
      logHead[0],
      'Head of the log is shown'
    );
  });

  test('Clicking Tail loads the log tail', async function (assert) {
    logMode = 'tail';
    run.later(run, run.cancelTimers, commonProps.interval);

    this.setProperties(commonProps);
    await render(
      hbs`<TaskLog @allocation={{allocation}} @task={{taskState}} />`
    );

    click('[data-test-log-action="tail"]');

    await settled();
    assert.ok(
      this.server.handledRequests.find(
        ({ queryParams: qp }) => qp.origin === 'end'
      ),
      'Log tail request was made'
    );
    assert.equal(
      find('[data-test-log-cli]').textContent,
      logTail[0],
      'Tail of the log is shown'
    );
  });

  test('Clicking toggleStream starts and stops the log stream', async function (assert) {
    assert.expect(3);

    run.later(run, run.cancelTimers, commonProps.interval);

    const { interval } = commonProps;
    this.setProperties(commonProps);
    await render(
      hbs`<TaskLog @allocation={{allocation}} @task={{taskState}} @interval={{interval}} />`
    );

    run.later(() => {
      click('[data-test-log-action="toggle-stream"]');
    }, interval);

    await settled();
    assert.equal(
      find('[data-test-log-cli]').textContent,
      streamFrames[0],
      'First frame loaded'
    );

    run.later(() => {
      assert.equal(
        find('[data-test-log-cli]').textContent,
        streamFrames[0],
        'Still only first frame'
      );
      click('[data-test-log-action="toggle-stream"]');
      run.later(run, run.cancelTimers, interval * 2);
    }, interval * 2);

    await settled();
    assert.equal(
      find('[data-test-log-cli]').textContent,
      streamFrames[0] + streamFrames[0] + streamFrames[1],
      'Now includes second frame'
    );
  });

  test('Clicking stderr switches the log to standard error', async function (assert) {
    run.later(run, run.cancelTimers, commonProps.interval);

    this.setProperties(commonProps);
    await render(
      hbs`<TaskLog @allocation={{allocation}} @task={{taskState}} />`
    );

    click('[data-test-log-action="stderr"]');
    run.later(run, run.cancelTimers, commonProps.interval);

    await settled();
    assert.ok(
      this.server.handledRequests.filter(
        (req) => req.queryParams.type === 'stderr'
      ).length,
      'stderr log requests were made'
    );
  });

  test('Clicking stderr/stdout mode buttons does nothing when the mode remains the same', async function (assert) {
    const { interval } = commonProps;

    run.later(() => {
      click('[data-test-log-action="stdout"]');
      run.later(run, run.cancelTimers, interval * 6);
    }, interval * 2);

    this.setProperties(commonProps);
    await render(
      hbs`<TaskLog @allocation={{allocation}} @task={{taskState}} />`
    );

    assert.equal(
      find('[data-test-log-cli]').textContent,
      streamFrames[0] + streamFrames[0] + streamFrames[1],
      'Now includes second frame'
    );
  });

  test('When the client is inaccessible, task-log falls back to requesting logs through the server', async function (assert) {
    run.later(run, run.cancelTimers, allowedConnectionTime * 2);

    // override client response to timeout
    this.server.get(
      `http://${HOST}/v1/client/fs/logs/:allocation_id`,
      () => [400, {}, ''],
      allowedConnectionTime * 2
    );

    this.setProperties(commonProps);
    await render(hbs`<TaskLog
      @allocation={{allocation}}
      @task={{taskState}}
      @clientTimeout={{clientTimeout}}
      @serverTimeout={{serverTimeout}} />`);

    const clientUrlRegex = new RegExp(
      `${HOST}/v1/client/fs/logs/${commonProps.allocation.id}`
    );
    assert.ok(
      this.server.handledRequests.filter((req) => clientUrlRegex.test(req.url))
        .length,
      'Log request was initially made directly to the client'
    );

    await settled();
    const serverUrl = `/v1/client/fs/logs/${commonProps.allocation.id}`;
    assert.ok(
      this.server.handledRequests.filter((req) => req.url.startsWith(serverUrl))
        .length,
      'Log request was later made to the server'
    );

    assert.ok(
      this.server.handledRequests.filter((req) =>
        clientUrlRegex.test(req.url)
      )[0].aborted,
      'Client log request was aborted'
    );
  });

  test('When both the client and the server are inaccessible, an error message is shown', async function (assert) {
    assert.expect(5);

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
    await render(hbs`<TaskLog
      @allocation={{allocation}}
      @task={{taskState}}
      @clientTimeout={{clientTimeout}}
      @serverTimeout={{serverTimeout}} />`);

    const clientUrlRegex = new RegExp(
      `${HOST}/v1/client/fs/logs/${commonProps.allocation.id}`
    );
    assert.ok(
      this.server.handledRequests.filter((req) => clientUrlRegex.test(req.url))
        .length,
      'Log request was initially made directly to the client'
    );
    const serverUrl = `/v1/client/fs/logs/${commonProps.allocation.id}`;
    assert.ok(
      this.server.handledRequests.filter((req) => req.url.startsWith(serverUrl))
        .length,
      'Log request was later made to the server'
    );
    assert.ok(
      find('[data-test-connection-error]'),
      'An error message is shown'
    );

    await click('[data-test-connection-error-dismiss]');
    assert.notOk(
      find('[data-test-connection-error]'),
      'The error message is dismissable'
    );

    await componentA11yAudit(this.element, assert);
  });

  test('When the client is inaccessible, the server is accessible, and stderr is pressed before the client timeout occurs, the no connection error is not shown', async function (assert) {
    // override client response to timeout
    this.server.get(
      `http://${HOST}/v1/client/fs/logs/:allocation_id`,
      () => [400, {}, ''],
      allowedConnectionTime * 2
    );

    // Click stderr before the client request responds
    run.later(() => {
      click('[data-test-log-action="stderr"]');
      run.later(run, run.cancelTimers, commonProps.interval * 5);
    }, allowedConnectionTime / 2);

    this.setProperties(commonProps);
    await render(hbs`<TaskLog
      @allocation={{allocation}}
      @task={{taskState}}
      @clientTimeout={{clientTimeout}}
      @serverTimeout={{serverTimeout}} />`);

    const clientUrlRegex = new RegExp(
      `${HOST}/v1/client/fs/logs/${commonProps.allocation.id}`
    );
    const clientRequests = this.server.handledRequests.filter((req) =>
      clientUrlRegex.test(req.url)
    );
    assert.ok(
      clientRequests.find((req) => req.queryParams.type === 'stdout'),
      'Client request for stdout'
    );
    assert.ok(
      clientRequests.find((req) => req.queryParams.type === 'stderr'),
      'Client request for stderr'
    );

    const serverUrl = `/v1/client/fs/logs/${commonProps.allocation.id}`;
    assert.ok(
      this.server.handledRequests
        .filter((req) => req.url.startsWith(serverUrl))
        .find((req) => req.queryParams.type === 'stderr'),
      'Server request for stderr'
    );

    assert.notOk(
      find('[data-test-connection-error]'),
      'An error message is not shown'
    );
  });

  test('The log streaming mode is persisted in localStorage', async function (assert) {
    window.localStorage.nomadLogMode = JSON.stringify('stderr');

    run.later(run, run.cancelTimers, commonProps.interval);

    this.setProperties(commonProps);
    await render(
      hbs`<TaskLog @allocation={{allocation}} @task={{taskState}} />`
    );

    assert.ok(
      this.server.handledRequests.filter(
        (req) => req.queryParams.type === 'stderr'
      ).length
    );
    assert.notOk(
      this.server.handledRequests.filter(
        (req) => req.queryParams.type === 'stdout'
      ).length
    );

    click('[data-test-log-action="stdout"]');
    run.later(run, run.cancelTimers, commonProps.interval);

    await settled();
    assert.ok(
      this.server.handledRequests.filter(
        (req) => req.queryParams.type === 'stdout'
      ).length
    );
    assert.equal(window.localStorage.nomadLogMode, JSON.stringify('stdout'));
  });
});
