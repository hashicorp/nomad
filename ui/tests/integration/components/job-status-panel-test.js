/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { find, render, settled } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import percySnapshot from '@percy/ember';

module(
  'Integration | Component | job status panel | active deployment',
  function (hooks) {
    setupRenderingTest(hooks);

    hooks.beforeEach(function () {
      fragmentSerializerInitializer(this.owner);
      window.localStorage.clear();
      this.store = this.owner.lookup('service:store');
      this.server = startMirage();
      this.server.create('node-pool');
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
      <JobStatus::Panel @job={{this.job}} />)
    `);

      assert.notOk(find('.active-deployment'), 'No active deployment');
    });

    test('the latest deployment section shows up for the currently running deployment: Ungrouped Allocations (small cluster)', async function (assert) {
      assert.expect(24);

      this.server.create('node');

      const NUMBER_OF_GROUPS = 2;
      const ALLOCS_PER_GROUP = 10;
      const allocStatusDistribution = {
        running: 0.5,
        failed: 0.2,
        unknown: 0.1,
        lost: 0,
        complete: 0.1,
        pending: 0.1,
      };

      const job = await this.server.create('job', {
        type: 'service',
        createAllocations: true,
        noDeployments: true, // manually created below
        activeDeployment: true,
        groupTaskCount: ALLOCS_PER_GROUP,
        shallow: true,
        resourceSpec: Array(NUMBER_OF_GROUPS).fill('M: 257, C: 500'), // length of this array determines number of groups
        allocStatusDistribution,
      });

      const jobRecord = await this.store.find(
        'job',
        JSON.stringify([job.id, 'default'])
      );
      await this.server.create('deployment', false, 'active', {
        jobId: job.id,
        groupDesiredTotal: ALLOCS_PER_GROUP,
        versionNumber: 1,
        status: 'failed',
      });

      const OLD_ALLOCATIONS_TO_SHOW = 25;
      const OLD_ALLOCATIONS_TO_COMPLETE = 5;

      this.server.createList('allocation', OLD_ALLOCATIONS_TO_SHOW, {
        jobId: job.id,
        jobVersion: 0,
        clientStatus: 'running',
      });

      this.set('job', jobRecord);
      await this.get('job.allocations');

      await render(hbs`
        <JobStatus::Panel @job={{this.job}} />
      `);

      // Initially no active deployment
      assert.notOk(
        find('.active-deployment'),
        'Does not show an active deployment when latest is failed'
      );

      const deployment = await this.get('job.latestDeployment');

      await this.set('job.latestDeployment.status', 'running');

      assert.ok(
        find('.active-deployment'),
        'Shows an active deployment if latest status is Running'
      );

      // Half the shown allocations are running, 1 is pending, 1 is failed; none are canaries or healthy.
      // The rest (lost, unknown, etc.) all show up as "Unplaced"
      assert
        .dom('.new-allocations .allocation-status-row .represented-allocation')
        .exists(
          { count: NUMBER_OF_GROUPS * ALLOCS_PER_GROUP },
          'All allocations are shown (ungrouped)'
        );
      assert
        .dom(
          '.new-allocations .allocation-status-row .represented-allocation.running'
        )
        .exists(
          {
            count:
              NUMBER_OF_GROUPS *
              ALLOCS_PER_GROUP *
              allocStatusDistribution.running,
          },
          'Correct number of running allocations are shown'
        );
      assert
        .dom(
          '.new-allocations .allocation-status-row .represented-allocation.running.canary'
        )
        .exists({ count: 0 }, 'No running canaries shown by default');
      assert
        .dom(
          '.new-allocations .allocation-status-row .represented-allocation.running.healthy'
        )
        .exists({ count: 0 }, 'No running healthy shown by default');
      assert
        .dom(
          '.new-allocations .allocation-status-row .represented-allocation.failed'
        )
        .exists(
          {
            count:
              NUMBER_OF_GROUPS *
              ALLOCS_PER_GROUP *
              allocStatusDistribution.failed,
          },
          'Correct number of failed allocations are shown'
        );
      assert
        .dom(
          '.new-allocations .allocation-status-row .represented-allocation.failed.canary'
        )
        .exists({ count: 0 }, 'No failed canaries shown by default');
      assert
        .dom(
          '.new-allocations .allocation-status-row .represented-allocation.pending'
        )
        .exists(
          {
            count:
              NUMBER_OF_GROUPS *
              ALLOCS_PER_GROUP *
              allocStatusDistribution.pending,
          },
          'Correct number of pending allocations are shown'
        );
      assert
        .dom(
          '.new-allocations .allocation-status-row .represented-allocation.pending.canary'
        )
        .exists({ count: 0 }, 'No pending canaries shown by default');
      assert
        .dom(
          '.new-allocations .allocation-status-row .represented-allocation.unplaced'
        )
        .exists(
          {
            count:
              NUMBER_OF_GROUPS *
              ALLOCS_PER_GROUP *
              (allocStatusDistribution.lost +
                allocStatusDistribution.unknown +
                allocStatusDistribution.complete),
          },
          'Correct number of unplaced allocations are shown'
        );

      assert.equal(
        find('[data-test-new-allocation-tally] > span').textContent.trim(),
        `New allocations: ${
          this.job.allocations.filter(
            (a) =>
              a.clientStatus === 'running' &&
              a.deploymentStatus?.Healthy === true
          ).length
        }/${deployment.get('desiredTotal')} running and healthy`,
        'Summary text shows accurate numbers when 0 are running/healthy'
      );

      let NUMBER_OF_RUNNING_CANARIES = 2;
      let NUMBER_OF_RUNNING_HEALTHY = 5;
      let NUMBER_OF_FAILED_CANARIES = 1;
      let NUMBER_OF_PENDING_CANARIES = 1;

      // Set some allocs to canary, and to healthy
      this.get('job.allocations')
        .filter((a) => a.clientStatus === 'running')
        .slice(0, NUMBER_OF_RUNNING_CANARIES)
        .forEach((alloc) =>
          alloc.set('deploymentStatus', {
            Canary: true,
            Healthy: alloc.deploymentStatus?.Healthy,
          })
        );
      this.get('job.allocations')
        .filter((a) => a.clientStatus === 'running')
        .slice(0, NUMBER_OF_RUNNING_HEALTHY)
        .forEach((alloc) =>
          alloc.set('deploymentStatus', {
            Canary: alloc.deploymentStatus?.Canary,
            Healthy: true,
          })
        );
      this.get('job.allocations')
        .filter((a) => a.clientStatus === 'failed')
        .slice(0, NUMBER_OF_FAILED_CANARIES)
        .forEach((alloc) =>
          alloc.set('deploymentStatus', {
            Canary: true,
            Healthy: alloc.deploymentStatus?.Healthy,
          })
        );
      this.get('job.allocations')
        .filter((a) => a.clientStatus === 'pending')
        .slice(0, NUMBER_OF_PENDING_CANARIES)
        .forEach((alloc) =>
          alloc.set('deploymentStatus', {
            Canary: true,
            Healthy: alloc.deploymentStatus?.Healthy,
          })
        );

      await render(hbs`
        <JobStatus::Panel @job={{this.job}} />
      `);
      assert
        .dom(
          '.new-allocations .allocation-status-row .represented-allocation.running.canary'
        )
        .exists(
          { count: NUMBER_OF_RUNNING_CANARIES },
          'Running Canaries shown when deployment info dictates'
        );
      assert
        .dom(
          '.new-allocations .allocation-status-row .represented-allocation.running.healthy'
        )
        .exists(
          { count: NUMBER_OF_RUNNING_HEALTHY },
          'Running Healthy allocs shown when deployment info dictates'
        );
      assert
        .dom(
          '.new-allocations .allocation-status-row .represented-allocation.failed.canary'
        )
        .exists(
          { count: NUMBER_OF_FAILED_CANARIES },
          'Failed Canaries shown when deployment info dictates'
        );
      assert
        .dom(
          '.new-allocations .allocation-status-row .represented-allocation.pending.canary'
        )
        .exists(
          { count: NUMBER_OF_PENDING_CANARIES },
          'Pending Canaries shown when deployment info dictates'
        );

      assert.equal(
        find('[data-test-new-allocation-tally] > span').textContent.trim(),
        `New allocations: ${
          this.job.allocations.filter(
            (a) =>
              a.clientStatus === 'running' &&
              a.deploymentStatus?.Healthy === true
          ).length
        }/${deployment.get('desiredTotal')} running and healthy`,
        'Summary text shows accurate numbers when some are running/healthy'
      );

      assert.equal(
        find('[data-test-old-allocation-tally] > span').textContent.trim(),
        `Previous allocations: ${
          this.job.allocations.filter(
            (a) =>
              (a.clientStatus === 'running' || a.clientStatus === 'complete') &&
              a.jobVersion !== deployment.versionNumber
          ).length
        } running`,
        'Old Alloc Summary text shows accurate numbers'
      );

      assert.equal(
        find('[data-test-previous-allocations-legend]')
          .textContent.trim()
          .replace(/\s\s+/g, ' '),
        '25 Running 0 Complete'
      );

      await percySnapshot(
        "Job Status Panel: 'New' and 'Previous' allocations, initial deploying state"
      );

      // Try setting a few of the old allocs to complete and make sure number ticks down
      await Promise.all(
        this.get('job.allocations')
          .filter(
            (a) =>
              a.clientStatus === 'running' &&
              a.jobVersion !== deployment.versionNumber
          )
          .slice(0, OLD_ALLOCATIONS_TO_COMPLETE)
          .map(async (a) => await a.set('clientStatus', 'complete'))
      );

      assert
        .dom(
          '.previous-allocations .allocation-status-row .represented-allocation'
        )
        .exists(
          { count: OLD_ALLOCATIONS_TO_SHOW },
          'All old allocations are shown'
        );
      assert
        .dom(
          '.previous-allocations .allocation-status-row .represented-allocation.complete'
        )
        .exists(
          { count: OLD_ALLOCATIONS_TO_COMPLETE },
          'Correct number of old allocations are in completed state'
        );

      assert.equal(
        find('[data-test-old-allocation-tally] > span').textContent.trim(),
        `Previous allocations: ${
          this.job.allocations.filter(
            (a) =>
              (a.clientStatus === 'running' || a.clientStatus === 'complete') &&
              a.jobVersion !== deployment.versionNumber
          ).length - OLD_ALLOCATIONS_TO_COMPLETE
        } running`,
        'Old Alloc Summary text shows accurate numbers after some are marked complete'
      );

      assert.equal(
        find('[data-test-previous-allocations-legend]')
          .textContent.trim()
          .replace(/\s\s+/g, ' '),
        '20 Running 5 Complete'
      );

      await percySnapshot(
        "Job Status Panel: 'New' and 'Previous' allocations, some old marked complete"
      );

      await componentA11yAudit(
        this.element,
        assert,
        'scrollable-region-focusable'
      ); //keyframe animation fades from opacity 0
    });

    test('non-running allocations are grouped regardless of health', async function (assert) {
      this.server.create('node');

      const NUMBER_OF_GROUPS = 1;
      const ALLOCS_PER_GROUP = 100;
      const allocStatusDistribution = {
        running: 0.9,
        failed: 0.1,
        unknown: 0,
        lost: 0,
        complete: 0,
        pending: 0,
      };

      const job = await this.server.create('job', {
        type: 'service',
        createAllocations: true,
        noDeployments: true, // manually created below
        activeDeployment: true,
        groupTaskCount: ALLOCS_PER_GROUP,
        shallow: true,
        resourceSpec: Array(NUMBER_OF_GROUPS).fill('M: 257, C: 500'), // length of this array determines number of groups
        allocStatusDistribution,
      });

      const jobRecord = await this.store.find(
        'job',
        JSON.stringify([job.id, 'default'])
      );
      await this.server.create('deployment', false, 'active', {
        jobId: job.id,
        groupDesiredTotal: ALLOCS_PER_GROUP,
        versionNumber: 1,
        status: 'failed',
      });

      let activelyDeployingJobAllocs = server.schema.allocations
        .all()
        .filter((a) => a.jobId === job.id);

      activelyDeployingJobAllocs.models
        .filter((a) => a.clientStatus === 'failed')
        .slice(0, 10)
        .forEach((a) =>
          a.update({ deploymentStatus: { Healthy: true, Canary: false } })
        );

      this.set('job', jobRecord);

      await this.get('job.latestDeployment');
      await this.set('job.latestDeployment.status', 'running');

      await this.get('job.allocations');

      await render(hbs`
        <JobStatus::Panel @job={{this.job}} />
      `);

      assert
        .dom('.allocation-status-block .represented-allocation.failed')
        .exists({ count: 1 }, 'Failed block exists only once');
      assert
        .dom('.allocation-status-block .represented-allocation.failed')
        .hasClass('rest', 'Failed block is a summary block');

      await Promise.all(
        this.get('job.allocations')
          .filterBy('clientStatus', 'failed')
          .slice(0, 3)
          .map(async (a) => {
            await a.set('deploymentStatus', { Healthy: false, Canary: true });
          })
      );

      assert
        .dom('.represented-allocation.failed.rest')
        .exists(
          { count: 2 },
          'Now that some are canaries, they still make up two blocks'
        );
    });

    test('During a deployment with canaries, canary alerts are handled', async function (assert) {
      this.server.create('node');

      const NUMBER_OF_GROUPS = 1;
      const ALLOCS_PER_GROUP = 10;
      const allocStatusDistribution = {
        running: 0.9,
        failed: 0.1,
        unknown: 0,
        lost: 0,
        complete: 0,
        pending: 0,
      };

      const job = await this.server.create('job', {
        type: 'service',
        createAllocations: true,
        noDeployments: true, // manually created below
        activeDeployment: true,
        groupTaskCount: ALLOCS_PER_GROUP,
        shallow: true,
        resourceSpec: Array(NUMBER_OF_GROUPS).fill('M: 257, C: 500'), // length of this array determines number of groups
        allocStatusDistribution,
      });

      const jobRecord = await this.store.find(
        'job',
        JSON.stringify([job.id, 'default'])
      );
      const deployment = await this.server.create(
        'deployment',
        false,
        'active',
        {
          jobId: job.id,
          groupDesiredTotal: ALLOCS_PER_GROUP,
          versionNumber: 1,
          status: 'failed',
          // requiresPromotion: false,
        }
      );

      // requiresPromotion goes to false
      deployment.deploymentTaskGroupSummaries.models.forEach((d) => {
        d.update({
          desiredCanaries: 0,
          requiresPromotion: false,
          promoted: false,
        });
      });

      // All allocations set to Healthy and non-canary
      let activelyDeployingJobAllocs = server.schema.allocations
        .all()
        .filter((a) => a.jobId === job.id);

      activelyDeployingJobAllocs.models.forEach((a) => {
        a.update({ deploymentStatus: { Healthy: true, Canary: false } });
      });

      this.set('job', jobRecord);

      await this.get('job.latestDeployment');
      await this.set('job.latestDeployment.status', 'running');

      await this.get('job.allocations');

      await render(hbs`
        <JobStatus::Panel @job={{this.job}} />
      `);

      assert
        .dom(find('.legend-item .represented-allocation.running').parentElement)
        .hasText('9 Running');
      assert
        .dom(find('.legend-item .represented-allocation.healthy').parentElement)
        .hasText('9 Healthy');

      assert
        .dom('.canary-promotion-alert')
        .doesNotExist('No canary promotion alert when no canaries');

      // Set 3 allocations to health-pending canaries
      await Promise.all(
        this.get('job.allocations')
          .filterBy('clientStatus', 'running')
          .slice(0, 3)
          .map(async (a) => {
            await a.set('deploymentStatus', { Healthy: null, Canary: true });
          })
      );

      // Set the deployment's requiresPromotion to true
      await Promise.all(
        this.get('job.latestDeployment.taskGroupSummaries').map(async (a) => {
          await a.set('desiredCanaries', 3);
          await a.set('requiresPromotion', true);
        })
      );

      await settled();

      assert
        .dom('.canary-promotion-alert')
        .exists('Canary promotion alert when canaries are present');

      assert
        .dom('.canary-promotion-alert')
        .containsText('Checking Canary health');

      // Fail the health check on 1 canary
      await Promise.all(
        this.get('job.allocations')
          .filterBy('clientStatus', 'running')
          .slice(0, 1)
          .map(async (a) => {
            await a.set('deploymentStatus', { Healthy: false, Canary: true });
          })
      );

      assert
        .dom('.canary-promotion-alert')
        .containsText('Some Canaries have failed');

      // That 1 passes its health checks, but two peers remain pending
      await Promise.all(
        this.get('job.allocations')
          .filterBy('clientStatus', 'running')
          .slice(0, 1)
          .map(async (a) => {
            await a.set('deploymentStatus', { Healthy: true, Canary: true });
          })
      );
      await settled();
      assert
        .dom('.canary-promotion-alert')
        .containsText('Checking Canary health');

      // Fail one of the running canaries, but dont specifically touch its deploymentStatus.health
      await Promise.all(
        this.get('job.allocations')
          .filterBy('clientStatus', 'running')
          .slice(0, 1)
          .map(async (a) => {
            await a.set('clientStatus', 'failed');
          })
      );

      assert
        .dom('.canary-promotion-alert')
        .containsText('Some Canaries have failed');

      // Canaries all running and healthy
      await Promise.all(
        this.get('job.allocations')
          .slice(0, 3)
          .map(async (a) => {
            await a.setProperties({
              deploymentStatus: { Healthy: true, Canary: true },
              clientStatus: 'running',
            });
          })
      );

      await settled();

      assert
        .dom('.canary-promotion-alert')
        .containsText('Deployment requires promotion');
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
      <JobStatus::Panel @job={{this.job}} />
    `);

      assert.notOk(find('.active-deployment'), 'No active deployment');
      assert.ok(
        find('.running-allocs-title'),
        'Steady-state mode shown instead'
      );
    });
  }
);
