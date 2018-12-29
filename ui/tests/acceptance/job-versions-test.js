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
  const formattedSubmitTime = moment(version.submitTime / 1000000).format('MM/DD/YY HH:mm:ss');
  const versionRow = Versions.versions.objectAt(0);

  assert.ok(versionRow.text.includes(`Version #${version.version}`), 'Version #');
  assert.equal(versionRow.stability, version.stable.toString(), 'Stability');
  assert.equal(versionRow.submitTime, formattedSubmitTime, 'Submit time');
});
