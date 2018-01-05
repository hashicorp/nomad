import { findAll, currentURL, visit } from 'ember-native-dom-helpers';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';

let job;

moduleForAcceptance('Acceptance | job definition', {
  beforeEach() {
    server.create('node');
    server.create('job');
    job = server.db.jobs[0];
    visit(`/jobs/${job.id}/definition`);
  },
});

test('visiting /jobs/:job_id/definition', function(assert) {
  assert.equal(currentURL(), `/jobs/${job.id}/definition`);
});

test('the job definition page contains a json viewer component', function(assert) {
  assert.ok(findAll('[data-test-definition-view]').length, 'JSON viewer found');
});

test('the job definition page requests the job to display in an unmutated form', function(assert) {
  const jobURL = `/v1/job/${job.id}`;
  const jobRequests = server.pretender.handledRequests
    .map(req => req.url.split('?')[0])
    .filter(url => url === jobURL);
  assert.ok(jobRequests.length === 2, 'Two requests for the job were made');
});
