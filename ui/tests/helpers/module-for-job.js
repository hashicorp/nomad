import { click, currentURL } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import JobDetail from 'nomad-ui/tests/pages/jobs/detail';

// eslint-disable-next-line ember/no-test-module-for
export default function moduleForJob(title, context, jobFactory, additionalTests) {
  let job;

  module(title, function(hooks) {
    setupApplicationTest(hooks);
    setupMirage(hooks);
    hooks.before(function() {
      if (context !== 'allocations' && context !== 'children' && context !== 'sysbatch') {
        throw new Error(
          `Invalid context provided to moduleForJob, expected either "allocations", "sysbatch" or "children", got ${context}`
        );
      }
    });

    hooks.beforeEach(async function() {
      if (context === 'sysbatch') {
        const clients = server.createList('node', 12, {
          datacenter: 'dc1',
          status: 'ready',
        });
        // Job with 1 task group.
        job = server.create('job', {
          status: 'running',
          datacenters: ['dc1', 'dc2'],
          type: 'sysbatch',
          resourceSpec: ['M: 256, C: 500'],
          createAllocations: false,
        });
        clients.forEach(c => {
          server.create('allocation', { jobId: job.id, nodeId: c.id });
        });
        await JobDetail.visit({ id: job.id });
      } else {
        server.create('node');
        job = jobFactory();
        await JobDetail.visit({ id: job.id });
      }
    });

    test('visiting /jobs/:job_id', async function(assert) {
      assert.equal(currentURL(), `/jobs/${encodeURIComponent(job.id)}`);
      assert.equal(document.title, `Job ${job.name} - Nomad`);
    });

    test('the subnav links to overview', async function(assert) {
      await JobDetail.tabFor('overview').visit();
      assert.equal(currentURL(), `/jobs/${encodeURIComponent(job.id)}`);
    });

    test('the subnav links to definition', async function(assert) {
      await JobDetail.tabFor('definition').visit();
      assert.equal(currentURL(), `/jobs/${encodeURIComponent(job.id)}/definition`);
    });

    test('the subnav links to versions', async function(assert) {
      await JobDetail.tabFor('versions').visit();
      assert.equal(currentURL(), `/jobs/${encodeURIComponent(job.id)}/versions`);
    });

    test('the subnav links to evaluations', async function(assert) {
      await JobDetail.tabFor('evaluations').visit();
      assert.equal(currentURL(), `/jobs/${encodeURIComponent(job.id)}/evaluations`);
    });

    test('the title buttons are dependent on job status', async function(assert) {
      if (job.status === 'dead') {
        assert.ok(JobDetail.start.isPresent);
        assert.notOk(JobDetail.stop.isPresent);
        assert.notOk(JobDetail.execButton.isPresent);
      } else {
        assert.notOk(JobDetail.start.isPresent);
        assert.ok(JobDetail.stop.isPresent);
        assert.ok(JobDetail.execButton.isPresent);
      }
    });

    if (context === 'sysbatch') {
      test('clients for the job are showing in the overview', async function(assert) {
        assert.ok(
          JobDetail.clientSummary.isPresent,
          'Client Summary Status Bar Chart is displayed in summary section'
        );
      });
      test('clicking a status bar in the chart takes you to a pre-filtered view of clients', async function(assert) {
        const bars = document.querySelectorAll('[data-test-client-status-bar] > svg > g > g');
        const status = bars[0].className.baseVal;
        await click(`[data-test-client-status-${status}="${status}"]`);
        const encodedStatus = statusList => encodeURIComponent(JSON.stringify(statusList));
        assert.equal(
          currentURL(),
          `/jobs/${job.name}/clients?status=${encodedStatus([status])}`,
          'Client Status Bar Chart links to client tab'
        );
      });
    }

    if (context === 'allocations') {
      test('allocations for the job are shown in the overview', async function(assert) {
        assert.ok(JobDetail.allocationsSummary, 'Allocations are shown in the summary section');
        assert.notOk(JobDetail.childrenSummary, 'Children are not shown in the summary section');
      });

      test('clicking in an allocation row navigates to that allocation', async function(assert) {
        const allocationRow = JobDetail.allocations[0];
        const allocationId = allocationRow.id;

        await allocationRow.visitRow();

        assert.equal(
          currentURL(),
          `/allocations/${allocationId}`,
          'Allocation row links to allocation detail'
        );
      });
    }

    if (context === 'children') {
      test('children for the job are shown in the overview', async function(assert) {
        assert.ok(JobDetail.childrenSummary, 'Children are shown in the summary section');
        assert.notOk(
          JobDetail.allocationsSummary,
          'Allocations are not shown in the summary section'
        );
      });
    }

    for (var testName in additionalTests) {
      test(testName, async function(assert) {
        await additionalTests[testName].call(this, job, assert);
      });
    }
  });
}
