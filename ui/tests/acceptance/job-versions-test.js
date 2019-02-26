import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import Versions from 'nomad-ui/tests/pages/jobs/job/versions';
import moment from 'moment';

let job;
let versions;

moduleForAcceptance('Acceptance | job versions', {
  beforeEach() {
    job = server.create('job', { createAllocations: false });
    versions = server.db.jobVersions.where({ jobId: job.id });

    Versions.visit({ id: job.id });
  },
});

test('/jobs/:id/versions should list all job versions', function(assert) {
  assert.ok(Versions.versions.length, versions.length, 'Each version gets a row in the timeline');
});

test('each version mentions the version number, the stability, and the submitted time', function(assert) {
  const version = versions.sortBy('submitTime').reverse()[0];
  const formattedSubmitTime = moment(version.submitTime / 1000000).format(
    "MMM DD, 'YY HH:mm:ss ZZ"
  );
  const versionRow = Versions.versions.objectAt(0);

  assert.ok(versionRow.text.includes(`Version #${version.version}`), 'Version #');
  assert.equal(versionRow.stability, version.stable.toString(), 'Stability');
  assert.equal(versionRow.submitTime, formattedSubmitTime, 'Submit time');
});

test('when the job for the versions is not found, an error message is shown, but the URL persists', function(assert) {
  Versions.visit({ id: 'not-a-real-job' });

  andThen(() => {
    assert.equal(
      server.pretender.handledRequests.findBy('status', 404).url,
      '/v1/job/not-a-real-job',
      'A request to the nonexistent job is made'
    );
    assert.equal(currentURL(), '/jobs/not-a-real-job/versions', 'The URL persists');
    assert.ok(Versions.error.isPresent, 'Error message is shown');
    assert.equal(Versions.error.title, 'Not Found', 'Error message is for 404');
  });
});
