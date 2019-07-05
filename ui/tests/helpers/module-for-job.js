import { currentURL } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import setupMirage from 'ember-cli-mirage/test-support/setup-mirage';
import JobDetail from 'nomad-ui/tests/pages/jobs/detail';

export default function moduleForJob(title, context, jobFactory, additionalTests) {
  let job;

  module(title, function(hooks) {
    setupApplicationTest(hooks);
    setupMirage(hooks);
    hooks.before(function() {
      if (context !== 'allocations' && context !== 'children') {
        throw new Error(
          `Invalid context provided to moduleForJob, expected either "allocations" or "children", got ${context}`
        );
      }
    });

    hooks.beforeEach(async function() {
      server.create('node');
      job = jobFactory();
      await JobDetail.visit({ id: job.id });
    });

    test('visiting /jobs/:job_id', async function(assert) {
      assert.equal(currentURL(), `/jobs/${job.id}`);
      assert.equal(document.title, `Job ${job.name} - Nomad`);
    });

    test('the subnav links to overview', async function(assert) {
      await JobDetail.tabFor('overview').visit();
      assert.equal(currentURL(), `/jobs/${job.id}`);
    });

    test('the subnav links to definition', async function(assert) {
      await JobDetail.tabFor('definition').visit();
      assert.equal(currentURL(), `/jobs/${job.id}/definition`);
    });

    test('the subnav links to versions', async function(assert) {
      await JobDetail.tabFor('versions').visit();
      assert.equal(currentURL(), `/jobs/${job.id}/versions`);
    });

    test('the subnav links to evaluations', async function(assert) {
      await JobDetail.tabFor('evaluations').visit();
      assert.equal(currentURL(), `/jobs/${job.id}/evaluations`);
    });

    if (context === 'allocations') {
      test('allocations for the job are shown in the overview', async function(assert) {
        assert.ok(JobDetail.allocationsSummary, 'Allocations are shown in the summary section');
        assert.notOk(JobDetail.childrenSummary, 'Children are not shown in the summary section');
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
        await additionalTests[testName](job, assert);
      });
    }
  });
}
