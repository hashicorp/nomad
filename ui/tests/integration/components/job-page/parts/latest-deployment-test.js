/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { click, find, render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import moment from 'moment';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

module(
  'Integration | Component | job-page/parts/latest-deployment',
  function (hooks) {
    setupRenderingTest(hooks);

    hooks.beforeEach(function () {
      fragmentSerializerInitializer(this.owner);
      window.localStorage.clear();
      this.store = this.owner.lookup('service:store');
      this.server = startMirage();
      this.server.create('namespace');
    });

    hooks.afterEach(function () {
      this.server.shutdown();
      window.localStorage.clear();
    });

    test('there is no latest deployment section when the job has no deployments', async function (assert) {
      this.server.create('job', {
        type: 'service',
        noDeployments: true,
        createAllocations: false,
      });

      await this.store.findAll('job');

      this.set('job', this.store.peekAll('job').get('firstObject'));
      await render(hbs`
      <JobPage::Parts::LatestDeployment @job={{job}} />)
    `);

      assert.notOk(
        find('[data-test-active-deployment]'),
        'No active deployment'
      );
    });

    test('the latest deployment section shows up for the currently running deployment', async function (assert) {
      assert.expect(11);

      this.server.create('job', {
        type: 'service',
        createAllocations: false,
        activeDeployment: true,
      });

      await this.store.findAll('job');

      this.set('job', this.store.peekAll('job').get('firstObject'));
      await render(hbs`
      <JobPage::Parts::LatestDeployment @job={{job}} />
    `);

      const deployment = await this.get('job.latestDeployment');
      const version = await deployment.get('version');

      assert.ok(find('[data-test-active-deployment]'), 'Active deployment');
      assert.ok(
        find('[data-test-active-deployment]').classList.contains('is-info'),
        'Running deployment gets the is-info class'
      );
      assert.equal(
        find('[data-test-active-deployment-stat="id"]').textContent.trim(),
        deployment.get('shortId'),
        'The active deployment is the most recent running deployment'
      );

      assert.equal(
        find(
          '[data-test-active-deployment-stat="submit-time"]'
        ).textContent.trim(),
        moment(version.get('submitTime')).fromNow(),
        'Time since the job was submitted is in the active deployment header'
      );

      assert.equal(
        find('[data-test-deployment-metric="canaries"]').textContent.trim(),
        `${deployment.get('placedCanaries')} / ${deployment.get(
          'desiredCanaries'
        )}`,
        'Canaries, both places and desired, are in the metrics'
      );

      assert.equal(
        find('[data-test-deployment-metric="placed"]').textContent.trim(),
        deployment.get('placedAllocs'),
        'Placed allocs aggregates across task groups'
      );

      assert.equal(
        find('[data-test-deployment-metric="desired"]').textContent.trim(),
        deployment.get('desiredTotal'),
        'Desired allocs aggregates across task groups'
      );

      assert.equal(
        find('[data-test-deployment-metric="healthy"]').textContent.trim(),
        deployment.get('healthyAllocs'),
        'Healthy allocs aggregates across task groups'
      );

      assert.equal(
        find('[data-test-deployment-metric="unhealthy"]').textContent.trim(),
        deployment.get('unhealthyAllocs'),
        'Unhealthy allocs aggregates across task groups'
      );

      assert.equal(
        find('[data-test-deployment-notification]').textContent.trim(),
        deployment.get('statusDescription'),
        'Status description is in the metrics block'
      );

      await componentA11yAudit(this.element, assert);
    });

    test('when there is no running deployment, the latest deployment section shows up for the last deployment', async function (assert) {
      this.server.create('job', {
        type: 'service',
        createAllocations: false,
        noActiveDeployment: true,
      });

      await this.store.findAll('job');

      this.set('job', this.store.peekAll('job').get('firstObject'));
      await render(hbs`
      <JobPage::Parts::LatestDeployment @job={{job}} />
    `);

      assert.ok(find('[data-test-active-deployment]'), 'Active deployment');
      assert.notOk(
        find('[data-test-active-deployment]').classList.contains('is-info'),
        'Non-running deployment does not get the is-info class'
      );
    });

    test('the latest deployment section can be expanded to show task groups and allocations', async function (assert) {
      assert.expect(5);

      this.server.create('node');
      this.server.create('job', { type: 'service', activeDeployment: true });

      await this.store.findAll('job');

      this.set('job', this.store.peekAll('job').get('firstObject'));
      await render(hbs`
      <JobPage::Parts::LatestDeployment @job={{job}} />
    `);

      assert.notOk(
        find('[data-test-deployment-task-groups]'),
        'Task groups not found'
      );
      assert.notOk(
        find('[data-test-deployment-allocations]'),
        'Allocations not found'
      );

      await click('[data-test-deployment-toggle-details]');

      assert.ok(
        find('[data-test-deployment-task-groups]'),
        'Task groups found'
      );
      assert.ok(
        find('[data-test-deployment-allocations]'),
        'Allocations found'
      );

      await componentA11yAudit(this.element, assert);
    });

    test('each task group in the expanded task group section shows task group details', async function (assert) {
      this.server.create('node');
      this.server.create('job', { type: 'service', activeDeployment: true });

      await this.store.findAll('job');

      const job = this.store.peekAll('job').get('firstObject');

      this.set('job', job);
      await render(hbs`
      <JobPage::Parts::LatestDeployment @job={{job}} />
    `);

      await click('[data-test-deployment-toggle-details]');

      const task = job.get('runningDeployment.taskGroupSummaries.firstObject');
      const findForTaskGroup = (selector) =>
        find(`[data-test-deployment-task-group-${selector}]`);
      assert.equal(
        findForTaskGroup('name').textContent.trim(),
        task.get('name')
      );
      assert.equal(
        findForTaskGroup('progress-deadline').textContent.trim(),
        moment(task.get('requireProgressBy')).format("MMM DD, 'YY HH:mm:ss ZZ")
      );
    });
  }
);
