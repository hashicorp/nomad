import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import JobDetail from 'nomad-ui/tests/pages/jobs/detail';

export default function moduleForJob(title, jobFactory, additionalTests) {
  let job;

  moduleForAcceptance(title, {
    beforeEach() {
      server.create('node');
      job = jobFactory();
      JobDetail.visit({ id: job.id });
    },
  });

  test('visiting /jobs/:job_id', function(assert) {
    assert.equal(currentURL(), `/jobs/${job.id}`);
  });

  test('the subnav links to overview', function(assert) {
    JobDetail.tabFor('overview').visit();
    andThen(() => {
      assert.equal(currentURL(), `/jobs/${job.id}`);
    });
  });

  test('the subnav links to definition', function(assert) {
    JobDetail.tabFor('definition').visit();
    andThen(() => {
      assert.equal(currentURL(), `/jobs/${job.id}/definition`);
    });
  });

  test('the subnav links to versions', function(assert) {
    JobDetail.tabFor('versions').visit();
    andThen(() => {
      assert.equal(currentURL(), `/jobs/${job.id}/versions`);
    });
  });

  test('the subnav links to evaluations', function(assert) {
    JobDetail.tabFor('evaluations').visit();
    andThen(() => {
      assert.equal(currentURL(), `/jobs/${job.id}/evaluations`);
    });
  });

  for (var testName in additionalTests) {
    test(testName, function(assert) {
      additionalTests[testName](job, assert);
    });
  }
}
