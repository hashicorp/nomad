/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember/no-string-prototype-extensions */
import { run } from '@ember/runloop';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { find, render, settled } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import Pretender from 'pretender';
import sinon from 'sinon';
import { logEncode } from '../../../mirage/data/logs';
import {
  selectOpen,
  selectOpenChoose,
} from '../../utils/ember-power-select-extensions';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import { capitalize } from '@ember/string';

module('Integration | Component | agent-monitor', function (hooks) {
  setupRenderingTest(hooks);

  const LOG_MESSAGE = 'log message goes here';

  hooks.beforeEach(function () {
    // Normally this would be called server, but server is a prop of this component.
    this.pretender = new Pretender(function () {
      this.get('/v1/regions', () => [200, {}, '[]']);
      this.get('/v1/agent/monitor', ({ queryParams }) => [
        200,
        {},
        logEncode(
          [
            `[${(
              queryParams.log_level || 'info'
            ).toUpperCase()}] ${LOG_MESSAGE}\n`,
          ],
          0
        ),
      ]);
    });
  });

  hooks.afterEach(function () {
    this.pretender.shutdown();
  });

  const INTERVAL = 200;

  const commonTemplate = hbs`
    <AgentMonitor
      @level={{this.level}}
      @client={{this.client}}
      @server={{this.server}}
      @onLevelChange={{this.onLevelChange}} />
  `;

  test('basic appearance', async function (assert) {
    assert.expect(5);

    this.setProperties({
      level: 'info',
      client: { id: 'client1' },
    });

    run.later(run, run.cancelTimers, INTERVAL);

    await render(commonTemplate);

    assert.ok(find('[data-test-level-switcher-parent]'));
    assert.ok(find('[data-test-toggle]'));
    assert.ok(find('[data-test-log-box]'));
    assert.ok(find('[data-test-log-box].is-full-bleed.is-dark'));

    await componentA11yAudit(this.element, assert);
  });

  test('when provided with a client, AgentMonitor streams logs for the client', async function (assert) {
    this.setProperties({
      level: 'info',
      client: { id: 'client1', region: 'us-west-1' },
    });

    run.later(run, run.cancelTimers, INTERVAL);

    await render(commonTemplate);

    const logRequest = this.pretender.handledRequests[1];
    assert.ok(logRequest.url.startsWith('/v1/agent/monitor'));
    assert.ok(logRequest.url.includes('client_id=client1'));
    assert.ok(logRequest.url.includes('log_level=info'));
    assert.notOk(logRequest.url.includes('server_id'));
    assert.notOk(logRequest.url.includes('region='));
  });

  test('when provided with a server, AgentMonitor streams logs for the server', async function (assert) {
    this.setProperties({
      level: 'warn',
      server: { id: 'server1', region: 'us-west-1' },
    });

    run.later(run, run.cancelTimers, INTERVAL);

    await render(commonTemplate);

    const logRequest = this.pretender.handledRequests[1];
    assert.ok(logRequest.url.startsWith('/v1/agent/monitor'));
    assert.ok(logRequest.url.includes('server_id=server1'));
    assert.ok(logRequest.url.includes('log_level=warn'));
    assert.ok(logRequest.url.includes('region=us-west-1'));
    assert.notOk(logRequest.url.includes('client_id'));
  });

  test('switching levels calls onLevelChange and restarts the logger', async function (assert) {
    const onLevelChange = sinon.spy();
    const newLevel = 'trace';

    this.setProperties({
      level: 'info',
      client: { id: 'client1' },
      onLevelChange,
    });

    run.later(run, run.cancelTimers, INTERVAL);

    await render(commonTemplate);

    const contentId = await selectOpen('[data-test-level-switcher-parent]');
    run.later(run, run.cancelTimers, INTERVAL);
    await selectOpenChoose(contentId, capitalize(newLevel));
    await settled();

    assert.ok(onLevelChange.calledOnce);
    assert.ok(onLevelChange.calledWith(newLevel));

    const secondLogRequest = this.pretender.handledRequests[2];
    assert.ok(secondLogRequest.url.includes(`log_level=${newLevel}`));
  });

  test('when switching levels, the scrollback is preserved and annotated with a switch message', async function (assert) {
    const newLevel = 'trace';
    const onLevelChange = sinon.spy();

    this.setProperties({
      level: 'info',
      client: { id: 'client1' },
      onLevelChange,
    });

    run.later(run, run.cancelTimers, INTERVAL);

    await render(commonTemplate);

    assert.equal(
      find('[data-test-log-cli]').textContent,
      `[INFO] ${LOG_MESSAGE}\n`
    );

    const contentId = await selectOpen('[data-test-level-switcher-parent]');
    run.later(run, run.cancelTimers, INTERVAL);
    await selectOpenChoose(contentId, capitalize(newLevel));
    await settled();

    assert.equal(
      find('[data-test-log-cli]').textContent,
      `[INFO] ${LOG_MESSAGE}\n\n...changing log level to ${newLevel}...\n\n[TRACE] ${LOG_MESSAGE}\n`
    );
  });

  test('when switching levels and there is no scrollback, there is no appended switch message', async function (assert) {
    const newLevel = 'trace';
    const onLevelChange = sinon.spy();

    // Emit nothing for the first request
    this.pretender.get('/v1/agent/monitor', ({ queryParams }) => [
      200,
      {},
      queryParams.log_level === 'info'
        ? logEncode([''], 0)
        : logEncode(
            [
              `[${(
                queryParams.log_level || 'info'
              ).toUpperCase()}] ${LOG_MESSAGE}\n`,
            ],
            0
          ),
    ]);

    this.setProperties({
      level: 'info',
      client: { id: 'client1' },
      onLevelChange,
    });

    run.later(run, run.cancelTimers, INTERVAL);

    await render(commonTemplate);

    assert.equal(find('[data-test-log-cli]').textContent, '');

    const contentId = await selectOpen('[data-test-level-switcher-parent]');
    run.later(run, run.cancelTimers, INTERVAL);
    await selectOpenChoose(contentId, capitalize(newLevel));
    await settled();

    assert.equal(
      find('[data-test-log-cli]').textContent,
      `[TRACE] ${LOG_MESSAGE}\n`
    );
  });
});
