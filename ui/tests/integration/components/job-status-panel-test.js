import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { find, render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import faker from 'nomad-ui/mirage/faker';

module(
  'Integration | Component | job status panel | active deployment',
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
      <JobStatus::Panel @job={{this.job}} />)
    `);

      assert.notOk(find('.active-deployment'), 'No active deployment');
    });

    test('the latest deployment section shows up for the currently running deployment: Ungrouped Allocations (small cluster)', async function (assert) {
      assert.expect(19);
      faker.seed(0); // TODO: de-flake. Distribution doesnt look like it's using faker randomness.

      this.server.create('node');

      const NUMBER_OF_GROUPS = 2;
      const ALLOCS_PER_GROUP = 10;
      const allocStatusDistribution = {
        running: 0.5,
        failed: 0.05,
        unknown: 0.2,
        lost: 0.1,
        complete: 0.1,
        pending: 0.05,
      };

      const job = await this.server.create('job', {
        type: 'service',
        createAllocations: true,
        noDeployments: true, // manually created below
        activeDeployment: true,
        groupTaskCount: ALLOCS_PER_GROUP,
        shallow: true,
        resourceSpec: Array(NUMBER_OF_GROUPS).fill(['M: 257, C: 500']), // length of this array determines number of groups
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

      assert.ok(
        find('.active-deployment').classList.contains('is-info'),
        'Running deployment gets the is-info class'
      );
      console.log(
        'NUMBER OF JOBS IN STORE:',
        this.store.peekAll('job').length,
        this.server
      );
      // Half the shown allocations are running, 1 is pending, 1 is failed; none are canaries or healthy.
      // The rest (lost, unknown, etc.) all show up as "Unplaced"
      assert
        .dom('.allocation-status-row .represented-allocation')
        .exists(
          { count: NUMBER_OF_GROUPS * ALLOCS_PER_GROUP },
          'All allocations are shown (ungrouped)'
        );
      assert
        .dom('.allocation-status-row .represented-allocation.running')
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
        .dom('.allocation-status-row .represented-allocation.running.canary')
        .exists({ count: 0 }, 'No running canaries shown by default');
      assert
        .dom('.allocation-status-row .represented-allocation.running.healthy')
        .exists({ count: 0 }, 'No running healthy shown by default');
      assert
        .dom('.allocation-status-row .represented-allocation.failed')
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
        .dom('.allocation-status-row .represented-allocation.failed.canary')
        .exists({ count: 0 }, 'No failed canaries shown by default');
      assert
        .dom('.allocation-status-row .represented-allocation.pending')
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
        .dom('.allocation-status-row .represented-allocation.pending.canary')
        .exists({ count: 0 }, 'No pending canaries shown by default');
      assert
        .dom('.allocation-status-row .represented-allocation.unplaced')
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
        find('[data-test-new-allocation-tally]').textContent.trim(),
        `New allocations: ${
          this.job.allocations.filter(
            (a) =>
              a.clientStatus === 'running' &&
              a.deploymentStatus.Healthy === true
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
            Healthy: alloc.deploymentStatus.Healthy,
          })
        );
      this.get('job.allocations')
        .filter((a) => a.clientStatus === 'running')
        .slice(0, NUMBER_OF_RUNNING_HEALTHY)
        .forEach((alloc) =>
          alloc.set('deploymentStatus', {
            Canary: alloc.deploymentStatus.Canary,
            Healthy: true,
          })
        );
      this.get('job.allocations')
        .filter((a) => a.clientStatus === 'failed')
        .slice(0, NUMBER_OF_FAILED_CANARIES)
        .forEach((alloc) =>
          alloc.set('deploymentStatus', {
            Canary: true,
            Healthy: alloc.deploymentStatus.Healthy,
          })
        );
      this.get('job.allocations')
        .filter((a) => a.clientStatus === 'pending')
        .slice(0, NUMBER_OF_PENDING_CANARIES)
        .forEach((alloc) =>
          alloc.set('deploymentStatus', {
            Canary: true,
            Healthy: alloc.deploymentStatus.Healthy,
          })
        );

      await render(hbs`
        <JobStatus::Panel @job={{this.job}} />
      `);
      assert
        .dom('.allocation-status-row .represented-allocation.running.canary')
        .exists(
          { count: NUMBER_OF_RUNNING_CANARIES },
          'Running Canaries shown when deployment info dictates'
        );
      assert
        .dom('.allocation-status-row .represented-allocation.running.healthy')
        .exists(
          { count: NUMBER_OF_RUNNING_HEALTHY },
          'Running Healthy allocs shown when deployment info dictates'
        );
      assert
        .dom('.allocation-status-row .represented-allocation.failed.canary')
        .exists(
          { count: NUMBER_OF_FAILED_CANARIES },
          'Failed Canaries shown when deployment info dictates'
        );
      assert
        .dom('.allocation-status-row .represented-allocation.pending.canary')
        .exists(
          { count: NUMBER_OF_PENDING_CANARIES },
          'Pending Canaries shown when deployment info dictates'
        );

      assert.equal(
        find('[data-test-new-allocation-tally]').textContent.trim(),
        `New allocations: ${
          this.job.allocations.filter(
            (a) =>
              a.clientStatus === 'running' &&
              a.deploymentStatus.Healthy === true
          ).length
        }/${deployment.get('desiredTotal')} running and healthy`,
        'Summary text shows accurate numbers when some are running/healthy'
      );

      await componentA11yAudit(
        this.element,
        assert,
        'scrollable-region-focusable'
      ); //keyframe animation fades from opacity 0
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
