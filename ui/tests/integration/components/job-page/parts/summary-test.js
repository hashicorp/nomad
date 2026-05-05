/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { hbs } from 'ember-cli-htmlbars';
import { find, click, render } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { startMirage } from 'nomad-ui/tests/helpers/start-mirage';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

module('Integration | Component | job-page/parts/summary', function (hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function () {
    fragmentSerializerInitializer(this.owner);
    window.localStorage.clear();
    this.store = this.owner.lookup('service:store');
    this.server = startMirage();
    this.server.create('namespace');
    this.server.create('node-pool');
  });

  hooks.afterEach(function () {
    this.server.shutdown();
    window.localStorage.clear();
  });

  test('jobs with children use the children diagram', async function (assert) {
    this.server.create('job', 'periodic', {
      createAllocations: false,
    });

    await this.store.findAll('job');

    this.set('job', this.store.peekAll('job').get('firstObject'));

    await render(hbs`
      <JobPage::Parts::Summary @job={{this.job}} />
    `);

    assert.ok(
      find('[data-test-children-status-bar]'),
      'Children status bar found',
    );
    assert.notOk(
      find('[data-test-allocation-status-bar]'),
      'Allocation status bar not found',
    );

    await componentA11yAudit(this.element, assert);
  });

  test('jobs without children use the allocations diagram', async function (assert) {
    this.server.create('job', {
      createAllocations: false,
    });

    await this.store.findAll('job');

    this.set('job', this.store.peekAll('job').get('firstObject'));

    await render(hbs`
      <JobPage::Parts::Summary @job={{this.job}} />
    `);

    assert.ok(
      find('[data-test-allocation-status-bar]'),
      'Allocation status bar found',
    );
    assert.notOk(
      find('[data-test-children-status-bar]'),
      'Children status bar not found',
    );

    await componentA11yAudit(this.element, assert);
  });

  test('the allocations diagram lists all allocation status figures', async function (assert) {
    this.server.create('job', {
      createAllocations: false,
    });

    await this.store.findAll('job');

    this.set('job', this.store.peekAll('job').get('firstObject'));

    await render(hbs`
      <JobPage::Parts::Summary @job={{this.job}} />
    `);

    assert.strictEqual(
      Number(find('[data-test-legend-value="queued"]').textContent.trim()),
      this.job.queuedAllocs,
      `${this.job.queuedAllocs} are queued`,
    );

    assert.strictEqual(
      Number(find('[data-test-legend-value="starting"]').textContent.trim()),
      this.job.startingAllocs,
      `${this.job.startingAllocs} are starting`,
    );

    assert.strictEqual(
      Number(find('[data-test-legend-value="running"]').textContent.trim()),
      this.job.runningAllocs,
      `${this.job.runningAllocs} are running`,
    );

    assert.strictEqual(
      Number(find('[data-test-legend-value="complete"]').textContent.trim()),
      this.job.completeAllocs,
      `${this.job.completeAllocs} are complete`,
    );

    assert.strictEqual(
      Number(find('[data-test-legend-value="failed"]').textContent.trim()),
      this.job.failedAllocs,
      `${this.job.failedAllocs} are failed`,
    );

    assert.strictEqual(
      Number(find('[data-test-legend-value="lost"]').textContent.trim()),
      this.job.lostAllocs,
      `${this.job.lostAllocs} are lost`,
    );
  });

  test('the children diagram lists all children status figures', async function (assert) {
    this.server.create('job', 'periodic', {
      createAllocations: false,
    });

    await this.store.findAll('job');

    this.set('job', this.store.peekAll('job').get('firstObject'));

    await render(hbs`
      <JobPage::Parts::Summary @job={{this.job}} />
    `);

    assert.strictEqual(
      Number(find('[data-test-legend-value="queued"]').textContent.trim()),
      this.job.pendingChildren,
      `${this.job.pendingChildren} are pending`,
    );

    assert.strictEqual(
      Number(find('[data-test-legend-value="running"]').textContent.trim()),
      this.job.runningChildren,
      `${this.job.runningChildren} are running`,
    );

    assert.strictEqual(
      Number(find('[data-test-legend-value="complete"]').textContent.trim()),
      this.job.deadChildren,
      `${this.job.deadChildren} are dead`,
    );
  });

  test('the summary block can be collapsed', async function (assert) {
    this.server.create('job', {
      createAllocations: false,
    });

    await this.store.findAll('job');

    this.set('job', this.store.peekAll('job').get('firstObject'));

    await render(hbs`
      <JobPage::Parts::Summary @job={{this.job}} />
    `);

    await click('[data-test-accordion-toggle]');

    assert.notOk(find('[data-test-accordion-body]'), 'No accordion body');
    assert.notOk(find('[data-test-legend]'), 'No legend');
  });

  test('when collapsed, the summary block includes an inline version of the chart', async function (assert) {
    this.server.create('job', {
      createAllocations: false,
    });

    await this.store.findAll('job');

    await this.set('job', this.store.peekAll('job').get('firstObject'));

    await render(hbs`
      <JobPage::Parts::Summary @job={{this.job}} />
    `);

    await click('[data-test-accordion-toggle]');

    assert.ok(
      find('[data-test-allocation-status-bar]'),
      'Allocation bar still existed',
    );
    assert.ok(
      find('.inline-chart [data-test-allocation-status-bar]'),
      'Allocation bar is rendered in an inline-chart container',
    );

    await componentA11yAudit(this.element, assert);
  });

  test('the collapsed/expanded state is persisted to localStorage', async function (assert) {
    this.server.create('job', {
      createAllocations: false,
    });

    await this.store.findAll('job');

    this.set('job', this.store.peekAll('job').get('firstObject'));

    await render(hbs`
      <JobPage::Parts::Summary @job={{this.job}} />
    `);

    assert.notOk(
      window.localStorage.nomadExpandJobSummary,
      'No value in localStorage yet',
    );
    await click('[data-test-accordion-toggle]');

    assert.deepEqual(
      window.localStorage.nomadExpandJobSummary,
      'false',
      'Value is stored for the collapsed state',
    );
  });

  test('the collapsed/expanded state from localStorage is used for the initial state when available', async function (assert) {
    this.server.create('job', {
      createAllocations: false,
    });

    await this.store.findAll('job');

    window.localStorage.nomadExpandJobSummary = 'false';

    this.set('job', this.store.peekAll('job').get('firstObject'));

    await render(hbs`
      <JobPage::Parts::Summary @job={{this.job}} />
    `);

    assert.ok(
      find('[data-test-allocation-status-bar]'),
      'Allocation bar still existed',
    );
    assert.ok(
      find('.inline-chart [data-test-allocation-status-bar]'),
      'Allocation bar is rendered in an inline-chart container',
    );

    await click('[data-test-accordion-toggle]');

    assert.deepEqual(
      window.localStorage.nomadExpandJobSummary,
      'true',
      'localStorage value still toggles',
    );
    assert.ok(find('[data-test-accordion-body]'), 'Summary still expands');
  });
});
