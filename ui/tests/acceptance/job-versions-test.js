import { find, findAll, visit } from 'ember-native-dom-helpers';
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
    findAll('[data-test-version]').length,
    versions.length,
    'Each version gets a row in the timeline'
  );
});

test('each version mentions the version number, the stability, and the submitted time', function(
  assert
) {
  const version = versions.sortBy('submitTime').reverse()[0];
  const versionRow = find('[data-test-version]');

  assert.ok(versionRow.textContent.includes(`Version #${version.version}`), 'Version #');
  assert.equal(
    versionRow.querySelector('[data-test-version-stability]').textContent,
    version.stable.toString(),
    'Stability'
  );
  assert.equal(
    versionRow.querySelector('[data-test-version-submit-date]').textContent,
    moment(version.submitTime / 1000000).format('MM/DD/YY HH:mm:ss'),
    'Submit time'
  );
});
