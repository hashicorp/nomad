/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { run } from '@ember/runloop';
import { find, render, triggerKeyEvent } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import Pretender from 'pretender';
import { logEncode } from '../../../mirage/data/logs';
import fetch from 'nomad-ui/utils/fetch';
import Log from 'nomad-ui/utils/classes/log';

const { assign } = Object;
const A_KEY = 65;

const stringifyValues = (obj) =>
  Object.keys(obj).reduce((newObj, key) => {
    newObj[key] = obj[key].toString();
    return newObj;
  }, {});

const makeLogger = (url, params) =>
  Log.create({
    url,
    params,
    plainText: true,
    logFetch: (url) => fetch(url).then((res) => res),
  });

module('Integration | Component | streaming file', function (hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function () {
    this.server = new Pretender(function () {
      this.get('/file/endpoint', () => [200, {}, 'Hello World']);
      this.get('/file/stream', () => [200, {}, logEncode(['Hello World'], 0)]);
    });
  });

  hooks.afterEach(function () {
    this.server.shutdown();
  });

  const commonTemplate = hbs`
    <StreamingFile @logger={{logger}} @mode={{mode}} @isStreaming={{isStreaming}} />
  `;

  test('when mode is `head`, the logger signals head', async function (assert) {
    assert.expect(5);

    const url = '/file/endpoint';
    const params = { path: 'hello/world.txt', offset: 0, limit: 50000 };
    this.setProperties({
      logger: makeLogger(url, params),
      mode: 'head',
      isStreaming: false,
    });

    await render(commonTemplate);

    const request = this.server.handledRequests[0];
    assert.equal(this.server.handledRequests.length, 1, 'One request made');
    assert.equal(request.url.split('?')[0], url, `URL is ${url}`);
    assert.deepEqual(
      request.queryParams,
      stringifyValues(assign({ origin: 'start' }, params)),
      'Query params are correct'
    );
    assert.equal(find('[data-test-output]').textContent, 'Hello World');
    await componentA11yAudit(this.element, assert);
  });

  test('when mode is `tail`, the logger signals tail', async function (assert) {
    const url = '/file/endpoint';
    const params = { path: 'hello/world.txt', limit: 50000 };
    this.setProperties({
      logger: makeLogger(url, params),
      mode: 'tail',
      isStreaming: false,
    });

    await render(commonTemplate);

    const request = this.server.handledRequests[0];
    assert.equal(this.server.handledRequests.length, 1, 'One request made');
    assert.equal(request.url.split('?')[0], url, `URL is ${url}`);
    assert.deepEqual(
      request.queryParams,
      stringifyValues(assign({ origin: 'end', offset: 50000 }, params)),
      'Query params are correct'
    );
    assert.equal(find('[data-test-output]').textContent, 'Hello World');
  });

  test('when mode is `streaming` and `isStreaming` is true, streaming starts', async function (assert) {
    const url = '/file/stream';
    const params = { path: 'hello/world.txt', limit: 50000 };
    this.setProperties({
      logger: makeLogger(url, params),
      mode: 'streaming',
      isStreaming: true,
    });

    assert.ok(true);

    run.later(run, run.cancelTimers, 500);

    await render(commonTemplate);

    const request = this.server.handledRequests[0];
    assert.equal(request.url.split('?')[0], url, `URL is ${url}`);
    assert.equal(find('[data-test-output]').textContent, 'Hello World');
  });

  test('the ctrl+a/cmd+a shortcut selects only the text in the output window', async function (assert) {
    const url = '/file/endpoint';
    const params = { path: 'hello/world.txt', offset: 0, limit: 50000 };
    this.setProperties({
      logger: makeLogger(url, params),
      mode: 'head',
      isStreaming: false,
    });

    await render(hbs`
      Extra text
      <StreamingFile @logger={{logger}} @mode={{mode}} @isStreaming={{isStreaming}} />
      On either side
    `);

    // Windows and Linux shortcut
    await triggerKeyEvent('[data-test-output]', 'keydown', A_KEY, {
      ctrlKey: true,
    });
    assert.equal(
      window.getSelection().toString().trim(),
      find('[data-test-output]').textContent.trim()
    );

    window.getSelection().removeAllRanges();

    // MacOS shortcut
    await triggerKeyEvent('[data-test-output]', 'keydown', A_KEY, {
      metaKey: true,
    });
    assert.equal(
      window.getSelection().toString().trim(),
      find('[data-test-output]').textContent.trim()
    );
  });
});
