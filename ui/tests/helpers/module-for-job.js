import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';

export default function moduleForJob(title, jobFactory, additionalTests) {
  let job;

  moduleForAcceptance(title, {
    beforeEach() {
      server.create('node');
      job = jobFactory();
      visit(`/jobs/${job.id}`);
    },
  });

  test('visiting /jobs/:job_id', function(assert) {
    assert.equal(currentURL(), `/jobs/${job.id}`);
  });

  test('the subnav links to overview', function(assert) {
    click(find('[data-test-tab="overview"] a'));
    andThen(() => {
      assert.equal(currentURL(), `/jobs/${job.id}`);
    });
  });

  test('the subnav links to definition', function(assert) {
    click(find('[data-test-tab="definition"] a'));
    andThen(() => {
      assert.equal(currentURL(), `/jobs/${job.id}/definition`);
    });
  });

  test('the subnav links to versions', function(assert) {
    click(find('[data-test-tab="versions"] a'));
    andThen(() => {
      assert.equal(currentURL(), `/jobs/${job.id}/versions`);
    });
  });

  for (var testName in additionalTests) {
    test(testName, function(assert) {
      additionalTests[testName](job, assert);
    });
  }
}
