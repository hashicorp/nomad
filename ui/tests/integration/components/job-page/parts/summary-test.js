/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';
import { find, click, render } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
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
    assert.expect(3);

    this.server.create('job', 'periodic', {
      createAllocations: false,
    });

    await this.store.findAll('job');

    this.set('job', this.store.peekAll('job').get('firstObject'));

    await render(hbs`
      <JobPage::Parts::Summary @job={{job}} />
    `);

    assert.ok(
      find('[data-test-children-status-bar]'),
      'Children status bar found'
    );
    assert.notOk(
      find('[data-test-allocation-status-bar]'),
      'Allocation status bar not found'
    );

    await componentA11yAudit(this.element, assert);
  });

  test('jobs without children use the allocations diagram', async function (assert) {
    assert.expect(3);

    this.server.create('job', {
      createAllocations: false,
    });

    await this.store.findAll('job');

    this.set('job', this.store.peekAll('job').get('firstObject'));

    await render(hbs`
      <JobPage::Parts::Summary @job={{job}} />
    `);

    assert.ok(
      find('[data-test-allocation-status-bar]'),
      'Allocation status bar found'
    );
    assert.notOk(
      find('[data-test-children-status-bar]'),
      'Children status bar not found'
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
      <JobPage::Parts::Summary @job={{job}} />
    `);

    assert.equal(
      find('[data-test-legend-value="queued"]').textContent,
      this.get('job.queuedAllocs'),
      `${this.get('job.queuedAllocs')} are queued`
    );

    assert.equal(
      find('[data-test-legend-value="starting"]').textContent,
      this.get('job.startingAllocs'),
      `${this.get('job.startingAllocs')} are starting`
    );

    assert.equal(
      find('[data-test-legend-value="running"]').textContent,
      this.get('job.runningAllocs'),
      `${this.get('job.runningAllocs')} are running`
    );

    assert.equal(
      find('[data-test-legend-value="complete"]').textContent,
      this.get('job.completeAllocs'),
      `${this.get('job.completeAllocs')} are complete`
    );

    assert.equal(
      find('[data-test-legend-value="failed"]').textContent,
      this.get('job.failedAllocs'),
      `${this.get('job.failedAllocs')} are failed`
    );

    assert.equal(
      find('[data-test-legend-value="lost"]').textContent,
      this.get('job.lostAllocs'),
      `${this.get('job.lostAllocs')} are lost`
    );
  });

  test('the children diagram lists all children status figures', async function (assert) {
    this.server.create('job', 'periodic', {
      createAllocations: false,
    });

    await this.store.findAll('job');

    this.set('job', this.store.peekAll('job').get('firstObject'));

    await render(hbs`
      <JobPage::Parts::Summary @job={{job}} />
    `);

    assert.equal(
      find('[data-test-legend-value="queued"]').textContent,
      this.get('job.pendingChildren'),
      `${this.get('job.pendingChildren')} are pending`
    );

    assert.equal(
      find('[data-test-legend-value="running"]').textContent,
      this.get('job.runningChildren'),
      `${this.get('job.runningChildren')} are running`
    );

    assert.equal(
      find('[data-test-legend-value="complete"]').textContent,
      this.get('job.deadChildren'),
      `${this.get('job.deadChildren')} are dead`
    );
  });

  test('the summary block can be collapsed', async function (assert) {
    this.server.create('job', {
      createAllocations: false,
    });

    await this.store.findAll('job');

    this.set('job', this.store.peekAll('job').get('firstObject'));

    await render(hbs`
      <JobPage::Parts::Summary @job={{job}} />
    `);

    await click('[data-test-accordion-toggle]');

    assert.notOk(find('[data-test-accordion-body]'), 'No accordion body');
    assert.notOk(find('[data-test-legend]'), 'No legend');
  });

  test('when collapsed, the summary block includes an inline version of the chart', async function (assert) {
    assert.expect(3);

    this.server.create('job', {
      createAllocations: false,
    });

    await this.store.findAll('job');

    await this.set('job', this.store.peekAll('job').get('firstObject'));

    await render(hbs`
      <JobPage::Parts::Summary @job={{job}} />
    `);

    await click('[data-test-accordion-toggle]');

    assert.ok(
      find('[data-test-allocation-status-bar]'),
      'Allocation bar still existed'
    );
    assert.ok(
      find('.inline-chart [data-test-allocation-status-bar]'),
      'Allocation bar is rendered in an inline-chart container'
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
      <JobPage::Parts::Summary @job={{job}} />
    `);

    assert.notOk(
      window.localStorage.nomadExpandJobSummary,
      'No value in localStorage yet'
    );
    await click('[data-test-accordion-toggle]');

    assert.equal(
      window.localStorage.nomadExpandJobSummary,
      'false',
      'Value is stored for the collapsed state'
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
      <JobPage::Parts::Summary @job={{job}} />
    `);

    assert.ok(
      find('[data-test-allocation-status-bar]'),
      'Allocation bar still existed'
    );
    assert.ok(
      find('.inline-chart [data-test-allocation-status-bar]'),
      'Allocation bar is rendered in an inline-chart container'
    );

    await click('[data-test-accordion-toggle]');

    assert.equal(
      window.localStorage.nomadExpandJobSummary,
      'true',
      'localStorage value still toggles'
    );
    assert.ok(find('[data-test-accordion-body]'), 'Summary still expands');
  });
});
