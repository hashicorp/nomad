import { currentURL } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import Versions from 'nomad-ui/tests/pages/jobs/job/versions';
import moment from 'moment';

let job;
let versions;

module('Acceptance | job versions', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function() {
    job = server.create('job', { createAllocations: false });
    versions = server.db.jobVersions.where({ jobId: job.id });

    await Versions.visit({ id: job.id });
  });

  test('it passes an accessibility audit', async function(assert) {
    await a11yAudit(assert);
  });

  test('/jobs/:id/versions should list all job versions', async function(assert) {
    assert.ok(Versions.versions.length, versions.length, 'Each version gets a row in the timeline');
    assert.equal(document.title, `Job ${job.name} versions - Nomad`);
  });

  test('each version mentions the version number, the stability, and the submitted time', async function(assert) {
    const version = versions.sortBy('submitTime').reverse()[0];
    const formattedSubmitTime = moment(version.submitTime / 1000000).format(
      "MMM DD, 'YY HH:mm:ss ZZ"
    );
    const versionRow = Versions.versions.objectAt(0);

    assert.ok(versionRow.text.includes(`Version #${version.version}`), 'Version #');
    assert.equal(versionRow.stability, version.stable.toString(), 'Stability');
    assert.equal(versionRow.submitTime, formattedSubmitTime, 'Submit time');
  });

  test('when the job for the versions is not found, an error message is shown, but the URL persists', async function(assert) {
    await Versions.visit({ id: 'not-a-real-job' });

    assert.equal(
      server.pretender.handledRequests
        .filter(request => !request.url.includes('policy'))
        .findBy('status', 404).url,
      '/v1/job/not-a-real-job',
      'A request to the nonexistent job is made'
    );
    assert.equal(currentURL(), '/jobs/not-a-real-job/versions', 'The URL persists');
    assert.ok(Versions.error.isPresent, 'Error message is shown');
    assert.equal(Versions.error.title, 'Not Found', 'Error message is for 404');
  });
});
