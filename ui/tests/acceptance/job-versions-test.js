import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import moment from 'moment';

let job;
let versions;

moduleForAcceptance('Acceptance | job versions', {
  beforeEach() {
    job = server.create('job', { createAllocations: false });
    versions = server.db.jobVersions.where({ jobId: job.id });

    visit(`/jobs/${job.id}/versions`);
  },
});

test('/jobs/:id/versions should list all job versions', function(assert) {
  assert.ok(
    find('.timeline-object').length,
    versions.length,
    'Each version gets a row in the timeline'
  );
});

test('each version mentions the version number, the stability, and the submitted time', function(
  assert
) {
  const version = versions.sortBy('submitTime').reverse()[0];
  const versionRow = find('.timeline-object:eq(0)');

  assert.ok(versionRow.text().includes(`Version #${version.version}`), 'Version #');
  assert.equal(
    versionRow.find('.version-stability .badge').text(),
    version.stable.toString(),
    'Stability'
  );
  assert.equal(
    versionRow.find('.version-submit-date .submit-date').text(),
    moment(version.submitTime / 1000000).format('DD/MM/YY HH:mm:ss [UTC]'),
    'Submit time'
  );
});
