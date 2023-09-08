/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable qunit/require-expect */
/* eslint-disable qunit/no-conditional-assertions */
import { currentURL } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import Versions from 'nomad-ui/tests/pages/jobs/job/versions';
import Layout from 'nomad-ui/tests/pages/layout';
import moment from 'moment';

let job;
let namespace;
let versions;

module('Acceptance | job versions', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function () {
    server.create('node-pool');
    server.create('namespace');
    namespace = server.create('namespace');

    job = server.create('job', {
      namespaceId: namespace.id,
      createAllocations: false,
    });
    versions = server.db.jobVersions.where({ jobId: job.id });

    const managementToken = server.create('token');
    window.localStorage.nomadTokenSecret = managementToken.secretId;

    await Versions.visit({ id: `${job.id}@${namespace.id}` });
  });

  test('it passes an accessibility audit', async function (assert) {
    await a11yAudit(assert);
  });

  test('/jobs/:id/versions should list all job versions', async function (assert) {
    assert.equal(
      Versions.versions.length,
      versions.length,
      'Each version gets a row in the timeline'
    );
    assert.equal(document.title, `Job ${job.name} versions - Nomad`);
  });

  test('each version mentions the version number, the stability, and the submitted time', async function (assert) {
    const version = versions.sortBy('submitTime').reverse()[0];
    const formattedSubmitTime = moment(version.submitTime / 1000000).format(
      "MMM DD, 'YY HH:mm:ss ZZ"
    );
    const versionRow = Versions.versions.objectAt(0);

    assert.ok(
      versionRow.text.includes(`Version #${version.version}`),
      'Version #'
    );
    assert.equal(versionRow.stability, version.stable.toString(), 'Stability');
    assert.equal(versionRow.submitTime, formattedSubmitTime, 'Submit time');
  });

  test('all versions but the current one have a button to revert to that version', async function (assert) {
    let versionRowToRevertTo;

    Versions.versions.forEach((versionRow) => {
      if (versionRow.number === job.version) {
        assert.ok(versionRow.revertToButton.isHidden);
      } else {
        assert.ok(versionRow.revertToButton.isPresent);

        versionRowToRevertTo = versionRow;
      }
    });

    if (versionRowToRevertTo) {
      const versionNumberRevertingTo = versionRowToRevertTo.number;
      await versionRowToRevertTo.revertToButton.idle();
      await versionRowToRevertTo.revertToButton.confirm();

      const revertRequest = this.server.pretender.handledRequests.find(
        (request) => request.url.includes('revert')
      );

      assert.equal(
        revertRequest.url,
        `/v1/job/${job.id}/revert?namespace=${namespace.id}`
      );

      assert.deepEqual(JSON.parse(revertRequest.requestBody), {
        JobID: job.id,
        JobVersion: versionNumberRevertingTo,
      });

      assert.equal(currentURL(), `/jobs/${job.id}@${namespace.id}`);
    }
  });

  test('when reversion fails, the error message from the API is piped through to the alert', async function (assert) {
    const versionRowToRevertTo = Versions.versions.filter(
      (versionRow) => versionRow.revertToButton.isPresent
    )[0];

    if (versionRowToRevertTo) {
      const message = 'A plaintext error message';
      server.pretender.post('/v1/job/:id/revert', () => [500, {}, message]);

      await versionRowToRevertTo.revertToButton.idle();
      await versionRowToRevertTo.revertToButton.confirm();

      assert.ok(Layout.inlineError.isShown);
      assert.ok(Layout.inlineError.isDanger);
      assert.ok(Layout.inlineError.title.includes('Could Not Revert'));
      assert.equal(Layout.inlineError.message, message);

      await Layout.inlineError.dismiss();

      assert.notOk(Layout.inlineError.isShown);
    } else {
      assert.expect(0);
    }
  });

  test('when reversion has no effect, the error message explains', async function (assert) {
    const versionRowToRevertTo = Versions.versions.filter(
      (versionRow) => versionRow.revertToButton.isPresent
    )[0];

    if (versionRowToRevertTo) {
      // The default Mirage implementation updates the job version as passed in, this does nothing
      server.pretender.post('/v1/job/:id/revert', () => [200, {}, '{}']);

      await versionRowToRevertTo.revertToButton.idle();
      await versionRowToRevertTo.revertToButton.confirm();

      assert.ok(Layout.inlineError.isShown);
      assert.ok(Layout.inlineError.isWarning);
      assert.ok(Layout.inlineError.title.includes('Reversion Had No Effect'));
      assert.equal(
        Layout.inlineError.message,
        'Reverting to an identical older version doesn’t produce a new version'
      );
    } else {
      assert.expect(0);
    }
  });

  test('when the job for the versions is not found, an error message is shown, but the URL persists', async function (assert) {
    await Versions.visit({ id: 'not-a-real-job' });

    assert.equal(
      server.pretender.handledRequests
        .filter((request) => !request.url.includes('policy'))
        .findBy('status', 404).url,
      '/v1/job/not-a-real-job',
      'A request to the nonexistent job is made'
    );
    assert.equal(
      currentURL(),
      '/jobs/not-a-real-job/versions',
      'The URL persists'
    );
    assert.ok(Versions.error.isPresent, 'Error message is shown');
    assert.equal(Versions.error.title, 'Not Found', 'Error message is for 404');
  });
});

module('Acceptance | job versions (with client token)', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function () {
    server.create('node-pool');
    job = server.create('job', { createAllocations: false });
    versions = server.db.jobVersions.where({ jobId: job.id });

    server.create('token');
    const clientToken = server.create('token');
    window.localStorage.nomadTokenSecret = clientToken.secretId;

    await Versions.visit({ id: job.id });
  });

  test('reversion buttons are disabled when the token lacks permissions', async function (assert) {
    const versionRowWithReversion = Versions.versions.filter(
      (versionRow) => versionRow.revertToButton.isPresent
    )[0];

    if (versionRowWithReversion) {
      assert.ok(versionRowWithReversion.revertToButtonIsDisabled);
    } else {
      assert.expect(0);
    }

    window.localStorage.clear();
  });

  test('reversion buttons are available when the client token has permissions', async function (assert) {
    const REVERT_NAMESPACE = 'revert-namespace';
    window.localStorage.clear();
    const clientToken = server.create('token');

    server.create('namespace', { id: REVERT_NAMESPACE });

    const job = server.create('job', {
      groupCount: 0,
      createAllocations: false,
      shallow: true,
      noActiveDeployment: true,
      namespaceId: REVERT_NAMESPACE,
    });

    const policy = server.create('policy', {
      id: 'something',
      name: 'something',
      rulesJSON: {
        Namespaces: [
          {
            Name: REVERT_NAMESPACE,
            Capabilities: ['submit-job'],
          },
        ],
      },
    });

    clientToken.policyIds = [policy.id];
    clientToken.save();

    window.localStorage.nomadTokenSecret = clientToken.secretId;

    versions = server.db.jobVersions.where({ jobId: job.id });
    await Versions.visit({ id: job.id, namespace: REVERT_NAMESPACE });
    const versionRowWithReversion = Versions.versions.filter(
      (versionRow) => versionRow.revertToButton.isPresent
    )[0];

    if (versionRowWithReversion) {
      assert.ok(versionRowWithReversion.revertToButtonIsDisabled);
    } else {
      assert.expect(0);
    }
  });
});
