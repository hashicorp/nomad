import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { find, render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

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

    test.only('the latest deployment section shows up for the currently running deployment', async function (assert) {
      assert.expect(4);

      this.server.create('node');

      const job = await this.server.create('job', {
        type: 'service',
        createAllocations: true,
        noDeployments: true, // manually created below
        activeDeployment: true,
        groupTaskCount: 331,
        shallow: true,
        resourceSpec: ['M: 257, C: 500', 'M: 257, C: 500'], // length of this array determines number of groups
        allocStatusDistribution: {
          running: 0.5,
          failed: 0.05,
          unknown: 0.2,
          lost: 0.1,
          complete: 0.1,
          pending: 0.05,
        },
      });

      const jobRecord = await this.store.find(
        'job',
        JSON.stringify([job.id, 'default'])
      );
      await this.server.create('deployment', false, 'active', {
        jobId: job.id,
        groupDesiredTotal: 331,
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

      console.log('depl', deployment, this.get('job'));

      await this.set('job.latestDeployment.status', 'running');

      assert.ok(
        find('.active-deployment'),
        'Shows an active deployment if latest status is Running'
      );

      assert.ok(
        find('.active-deployment').classList.contains('is-info'),
        'Running deployment gets the is-info class'
      );

      assert.equal(
        find('[data-test-active-deployment-stat="id"]').textContent.trim(),
        deployment.get('shortId'),
        'The active deployment is the most recent running deployment'
      );

      assert.equal(
        find('[data-test-deployment-metric="canaries"]').textContent.trim(),
        `${deployment.get('placedCanaries')} / ${deployment.get(
          'desiredCanaries'
        )}`,
        'Canaries, both places and desired, are in the metrics'
      );

      // assert.equal(
      //   find('[data-test-deployment-metric="placed"]').textContent.trim(),
      //   deployment.get('placedAllocs'),
      //   'Placed allocs aggregates across task groups'
      // );

      // assert.equal(
      //   find('[data-test-deployment-metric="desired"]').textContent.trim(),
      //   deployment.get('desiredTotal'),
      //   'Desired allocs aggregates across task groups'
      // );

      // assert.equal(
      //   find('[data-test-deployment-metric="healthy"]').textContent.trim(),
      //   deployment.get('healthyAllocs'),
      //   'Healthy allocs aggregates across task groups'
      // );

      // assert.equal(
      //   find('[data-test-deployment-metric="unhealthy"]').textContent.trim(),
      //   deployment.get('unhealthyAllocs'),
      //   'Unhealthy allocs aggregates across task groups'
      // );

      // assert.equal(
      //   find('[data-test-deployment-notification]').textContent.trim(),
      //   deployment.get('statusDescription'),
      //   'Status description is in the metrics block'
      // );

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
