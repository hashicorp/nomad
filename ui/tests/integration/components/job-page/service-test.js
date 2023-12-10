/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { assign } from '@ember/polyfills';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { click, find, render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import {
  startJob,
  stopJob,
  purgeJob,
  expectError,
  expectDeleteRequest,
  expectStartRequest,
  expectPurgeRequest,
} from './helpers';
import Job from 'nomad-ui/tests/pages/jobs/detail';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

module('Integration | Component | job-page/service', function (hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function () {
    fragmentSerializerInitializer(this.owner);
    window.localStorage.clear();
    this.store = this.owner.lookup('service:store');
    this.server = startMirage();
    this.server.create('namespace');
    this.server.create('node-pool');
    this.server.create('node');
  });

  hooks.afterEach(function () {
    this.server.shutdown();
    window.localStorage.clear();
  });

  const commonTemplate = hbs`
    <JobPage::Service
      @job={{job}}
      @sortProperty={{sortProperty}}
      @sortDescending={{sortDescending}}
      @currentPage={{currentPage}}
      @gotoJob={{gotoJob}}
      @statusMode={{statusMode}}
      @setStatusMode={{setStatusMode}}
      />
  `;

  const commonProperties = (job) => ({
    job,
    sortProperty: 'name',
    sortDescending: true,
    currentPage: 1,
    gotoJob() {},
    statusMode: 'current',
    setStatusMode() {},
  });

  const makeMirageJob = (server, props = {}) =>
    server.create(
      'job',
      assign(
        {
          type: 'service',
          createAllocations: false,
          status: 'running',
        },
        props
      )
    );

  test('Stopping a job sends a delete request for the job', async function (assert) {
    assert.expect(1);

    const mirageJob = makeMirageJob(this.server);
    await this.store.findAll('job');

    const job = this.store.peekAll('job').findBy('plainId', mirageJob.id);

    this.setProperties(commonProperties(job));
    await render(commonTemplate);

    await stopJob();
    expectDeleteRequest(assert, this.server, job);
  });

  test('Stopping a job without proper permissions shows an error message', async function (assert) {
    assert.expect(4);

    this.server.pretender.delete('/v1/job/:id', () => [403, {}, '']);

    const mirageJob = makeMirageJob(this.server);
    await this.store.findAll('job');

    const job = this.store.peekAll('job').findBy('plainId', mirageJob.id);

    this.setProperties(commonProperties(job));
    await render(commonTemplate);

    await stopJob();
    expectError(assert, 'Could Not Stop Job');

    await componentA11yAudit(this.element, assert);
  });

  test('Starting a job sends a post request for the job using the current definition', async function (assert) {
    assert.expect(1);

    const mirageJob = makeMirageJob(this.server, { status: 'dead' });
    await this.store.findAll('job');

    const job = this.store.peekAll('job').findBy('plainId', mirageJob.id);

    this.setProperties(commonProperties(job));
    await render(commonTemplate);

    await startJob();
    expectStartRequest(assert, this.server, job);
  });

  test('Starting a job without proper permissions shows an error message', async function (assert) {
    assert.expect(3);

    this.server.pretender.post('/v1/job/:id', () => [403, {}, '']);

    const mirageJob = makeMirageJob(this.server, { status: 'dead' });
    await this.store.findAll('job');

    const job = this.store.peekAll('job').findBy('plainId', mirageJob.id);

    this.setProperties(commonProperties(job));
    await render(commonTemplate);

    await startJob();
    await expectError(assert, 'Could Not Start Job');
  });

  test('Purging a job sends a purge request for the job', async function (assert) {
    assert.expect(1);

    const mirageJob = makeMirageJob(this.server, { status: 'dead' });
    await this.store.findAll('job');

    const job = this.store.peekAll('job').findBy('plainId', mirageJob.id);

    this.setProperties(commonProperties(job));
    await render(commonTemplate);

    await purgeJob();
    expectPurgeRequest(assert, this.server, job);
  });

  test('Recent allocations shows allocations in the job context', async function (assert) {
    assert.expect(3);

    this.server.create('node');
    const mirageJob = makeMirageJob(this.server, { createAllocations: true });
    await this.store.findAll('job');

    const job = this.store.peekAll('job').findBy('plainId', mirageJob.id);

    this.setProperties(commonProperties(job));
    await render(commonTemplate);

    const allocation = this.server.db.allocations
      .sortBy('modifyIndex')
      .reverse()[0];
    const allocationRow = Job.allocations.objectAt(0);

    assert.equal(allocationRow.shortId, allocation.id.split('-')[0], 'ID');
    assert.equal(
      allocationRow.taskGroup,
      allocation.taskGroup,
      'Task Group name'
    );

    await componentA11yAudit(this.element, assert);
  });

  test('Recent allocations caps out at five', async function (assert) {
    this.server.create('node');
    const mirageJob = makeMirageJob(this.server);
    this.server.createList('allocation', 10);

    await this.store.findAll('job');

    const job = this.store.peekAll('job').findBy('plainId', mirageJob.id);

    this.setProperties(commonProperties(job));
    await render(commonTemplate);

    assert.equal(Job.allocations.length, 5, 'Capped at 5 allocations');
    assert.ok(
      Job.viewAllAllocations.includes(job.get('allocations.length') + ''),
      `View link mentions ${job.get('allocations.length')} allocations`
    );
  });

  test('Recent allocations shows an empty message when the job has no allocations', async function (assert) {
    assert.expect(2);

    this.server.create('node');
    const mirageJob = makeMirageJob(this.server);

    await this.store.findAll('job');

    const job = this.store.peekAll('job').findBy('plainId', mirageJob.id);

    this.setProperties(commonProperties(job));
    await render(commonTemplate);

    assert.ok(
      Job.recentAllocationsEmptyState.headline.includes('No Allocations'),
      'No allocations empty message'
    );

    await componentA11yAudit(this.element, assert);
  });

  test('Active deployment can be promoted', async function (assert) {
    this.server.create('node');
    const mirageJob = makeMirageJob(this.server, { activeDeployment: true });

    await this.store.findAll('job');

    const job = this.store.peekAll('job').findBy('plainId', mirageJob.id);
    const deployment = await job.get('latestDeployment');

    this.setProperties(commonProperties(job));
    await render(commonTemplate);

    await click('[data-test-promote-canary]');

    const requests = this.server.pretender.handledRequests;

    assert.ok(
      requests
        .filterBy('method', 'POST')
        .findBy('url', `/v1/deployment/promote/${deployment.get('id')}`),
      'A promote POST request was made'
    );
  });

  test('When promoting the active deployment fails, an error is shown', async function (assert) {
    assert.expect(4);

    this.server.pretender.post('/v1/deployment/promote/:id', () => [
      403,
      {},
      '',
    ]);

    this.server.create('node');
    const mirageJob = makeMirageJob(this.server, { activeDeployment: true });

    await this.store.findAll('job');

    const job = this.store.peekAll('job').findBy('plainId', mirageJob.id);

    this.setProperties(commonProperties(job));
    await render(commonTemplate);

    await click('[data-test-promote-canary]');

    assert.equal(
      find('[data-test-job-error-title]').textContent,
      'Could Not Promote Deployment',
      'Appropriate error is shown'
    );
    assert.ok(
      find('[data-test-job-error-body]').textContent.includes('ACL'),
      'The error message mentions ACLs'
    );

    await componentA11yAudit(
      this.element,
      assert,
      'scrollable-region-focusable'
    ); //keyframe animation fades from opacity 0

    await click('[data-test-job-error-close]');

    assert.notOk(
      find('[data-test-job-error-title]'),
      'Error message is dismissable'
    );
  });

  test('Active deployment can be failed', async function (assert) {
    this.server.create('node');
    const mirageJob = makeMirageJob(this.server, { activeDeployment: true });

    await this.store.findAll('job');

    const job = this.store.peekAll('job').findBy('plainId', mirageJob.id);
    const deployment = await job.get('latestDeployment');

    this.setProperties(commonProperties(job));
    await render(commonTemplate);

    await click('.active-deployment [data-test-fail]');

    const requests = this.server.pretender.handledRequests;

    assert.ok(
      requests
        .filterBy('method', 'POST')
        .findBy('url', `/v1/deployment/fail/${deployment.get('id')}`),
      'A fail POST request was made'
    );
  });

  test('When failing the active deployment fails, an error is shown', async function (assert) {
    assert.expect(4);

    this.server.pretender.post('/v1/deployment/fail/:id', () => [403, {}, '']);

    this.server.create('node');
    const mirageJob = makeMirageJob(this.server, { activeDeployment: true });

    await this.store.findAll('job');

    const job = this.store.peekAll('job').findBy('plainId', mirageJob.id);

    this.setProperties(commonProperties(job));
    await render(commonTemplate);

    await click('.active-deployment [data-test-fail]');

    assert.equal(
      find('[data-test-job-error-title]').textContent,
      'Could Not Fail Deployment',
      'Appropriate error is shown'
    );
    assert.ok(
      find('[data-test-job-error-body]').textContent.includes('ACL'),
      'The error message mentions ACLs'
    );

    await componentA11yAudit(
      this.element,
      assert,
      'scrollable-region-focusable'
    ); //keyframe animation fades from opacity 0

    await click('[data-test-job-error-close]');

    assert.notOk(
      find('[data-test-job-error-title]'),
      'Error message is dismissable'
    );
  });
});
