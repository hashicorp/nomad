/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable qunit/require-expect */
/* eslint-disable qunit/no-conditional-assertions */
import { currentURL, click, typeIn } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import Versions from 'nomad-ui/tests/pages/jobs/job/versions';
import Layout from 'nomad-ui/tests/pages/layout';
import moment from 'moment';
import percySnapshot from '@percy/ember';
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
      noDeployments: true,
      type: 'service',
    });

    // Create some versions
    server.create('job-version', {
      job: job,
      version: 0,
    });
    server.create('job-version', {
      job: job,
      version: 1,
      versionTag: {
        Name: 'test-tag',
        Description: 'A tag with a brief description',
      },
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
  });

  test('version tags are displayed', async function (assert) {
    // Both a tagged version and an untagged version are present
    assert.dom('[data-test-tagged-version="true"]').exists();
    assert.dom('[data-test-tagged-version="false"]').exists();

    // The tagged version has a button indicating a tag name and description
    assert
      .dom('[data-test-tagged-version="true"] .tag-button-primary')
      .hasText('test-tag');
    assert
      .dom('[data-test-tagged-version="true"] .tag-description')
      .hasText('A tag with a brief description');

    // The untagged version has no tag button or description
    assert
      .dom('[data-test-tagged-version="false"] .tag-button-primary')
      .doesNotExist();
    assert
      .dom('[data-test-tagged-version="false"] .tag-description')
      .hasText('', 'Tag description is empty');

    await percySnapshot(assert, {
      percyCSS: `
        .timeline-note {
          display: none;
        }
        .submit-date {
          visibility: hidden;
        }
      `,
    });
  });

  test('existing version tags can be edited', async function (assert) {
    // Clicking the tag button puts it into edit mode
    assert
      .dom('[data-test-tagged-version="true"] .boxed-section-foot')
      .doesNotHaveClass('editing');
    await click('[data-test-tagged-version="true"] .tag-button-primary');
    assert
      .dom('[data-test-tagged-version="true"] .boxed-section-foot')
      .hasClass('editing');

    // equivalent of backspacing existing
    document.querySelector('[data-test-tag-name-input]').value = '';
    document.querySelector('[data-test-tag-description-input]').value = '';

    await typeIn(
      '[data-test-tagged-version="true"] [data-test-tag-name-input]',
      'new-tag'
    );
    await typeIn(
      '[data-test-tagged-version="true"] [data-test-tag-description-input]',
      'new-description'
    );

    // Clicking the save button commits the changes
    await click(
      '[data-test-tagged-version="true"] [data-test-tag-save-button]'
    );
    assert
      .dom('[data-test-tagged-version="true"] .tag-button-primary')
      .hasText('new-tag');
    assert
      .dom('[data-test-tagged-version="true"] .tag-description')
      .hasText('new-description');

    assert
      .dom('.flash-message.alert.alert-success')
      .exists('Shows a success toast notification on edit.');

    // Tag can subsequently be deleted
    await click('[data-test-tagged-version="true"] .tag-button-primary');
    await click(
      '[data-test-tagged-version="true"] [data-test-tag-delete-button]'
    );
    assert.dom('[data-test-tagged-version="true"]').doesNotExist();
  });

  test('new version tags can be created', async function (assert) {
    // Clicking the tag button puts it into edit mode
    assert
      .dom('[data-test-tagged-version="false"] .boxed-section-foot')
      .doesNotHaveClass('editing');
    await click('[data-test-tagged-version="false"] .tag-button-secondary');
    assert
      .dom('[data-test-tagged-version="false"] .boxed-section-foot')
      .hasClass('editing');

    assert
      .dom('[data-test-tagged-version="false"] [data-test-tag-delete-button]')
      .doesNotExist();

    // Clicking the save button commits the changes
    await click(
      '[data-test-tagged-version="false"] [data-test-tag-save-button]'
    );

    assert
      .dom('.flash-message.alert.alert-critical')
      .exists('Shows an error toast notification without a tag name.');

    await typeIn(
      '[data-test-tagged-version="false"] [data-test-tag-name-input]',
      'new-tag'
    );
    await typeIn(
      '[data-test-tagged-version="false"] [data-test-tag-description-input]',
      'new-description'
    );

    // Clicking the save button commits the changes
    await click(
      '[data-test-tagged-version="false"] [data-test-tag-save-button]'
    );

    assert
      .dom('[data-test-tagged-version="false"]')
      .doesNotExist('Both versions now have tags');

    assert
      .dom('.flash-message.alert.alert-success')
      .exists('Shows a success toast notification on edit.');

    await percySnapshot(assert, {
      percyCSS: `
        .timeline-note {
          display: none;
        }
        .submit-date {
          visibility: hidden;
        }
      `,
    });
  });
});

// Module for Clone and Edit
module('Acceptance | job versions (clone and edit)', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function () {
    server.create('node-pool');
    namespace = server.create('namespace');

    const managementToken = server.create('token');
    window.localStorage.nomadTokenSecret = managementToken.secretId;

    job = server.create('job', {
      createAllocations: false,
      version: 99,
      namespaceId: namespace.id,
    });
    // remove auto-created versions and create 3 of them, one with a tag
    server.db.jobVersions.remove();
    server.create('job-version', {
      job,
      version: 99,
      submitTime: 1731101785761339000,
    });
    server.create('job-version', {
      job,
      version: 98,
      submitTime: 1731101685761339000,
      versionTag: {
        Name: 'test-tag',
        Description: 'A tag with a brief description',
      },
    });
    server.create('job-version', {
      job,
      version: 0,
      submitTime: 1731101585761339000,
    });
    await Versions.visit({ id: job.id });
  });

  test('Clone and edit buttons are shown', async function (assert) {
    assert.dom('.job-version').exists({ count: 3 });
    assert
      .dom('[data-test-clone-and-edit]')
      .exists(
        { count: 2 },
        'Current job version doesnt have clone or revert buttons'
      );

    const versionBlock = '[data-test-job-version="98"]';

    assert
      .dom(`${versionBlock} [data-test-clone-as-new-version]`)
      .doesNotExist(
        'Confirmation-stage clone-as-new-version button doesnt exist on initial load'
      );
    assert
      .dom(`${versionBlock} [data-test-clone-as-new-job]`)
      .doesNotExist(
        'Confirmation-stage clone-as-new-job button doesnt exist on initial load'
      );

    await click(`${versionBlock} [data-test-clone-and-edit]`);

    assert
      .dom(`${versionBlock} [data-test-clone-as-new-version]`)
      .exists(
        'Confirmation-stage clone-as-new-version button exists after clicking clone and edit'
      );

    assert
      .dom(`${versionBlock} [data-test-clone-as-new-job]`)
      .exists(
        'Confirmation-stage clone-as-new-job button exists after clicking clone and edit'
      );

    assert
      .dom(`${versionBlock} [data-test-revert-to]`)
      .doesNotExist('Revert button is hidden when Clone buttons are expanded');

    assert
      .dom('[data-test-job-version="0"] [data-test-revert-to]')
      .exists('Revert button is visible for other versions');

    await click(`${versionBlock} [data-test-cancel-clone]`);

    assert
      .dom(`${versionBlock} [data-test-clone-as-new-version]`)
      .doesNotExist(
        'Confirmation-stage clone-as-new-version button doesnt exist after clicking cancel'
      );
  });

  test('Clone as new version', async function (assert) {
    const versionBlock = '[data-test-job-version="98"]';
    await click(`${versionBlock} [data-test-clone-and-edit]`);
    await click(`${versionBlock} [data-test-clone-as-new-version]`);

    assert.equal(
      currentURL(),
      `/jobs/${job.id}@${namespace.id}/definition?isEditing=true&version=98&view=job-spec`,
      'Taken to the definition page in edit mode'
    );

    await percySnapshot(assert);
  });

  test('Clone as new version when version is 0', async function (assert) {
    const versionBlock = '[data-test-job-version="0"]';
    await click(`${versionBlock} [data-test-clone-and-edit]`);
    await click(`${versionBlock} [data-test-clone-as-new-version]`);

    assert.equal(
      currentURL(),
      `/jobs/${job.id}@${namespace.id}/definition?isEditing=true&version=0&view=job-spec`,
      'Taken to the definition page in edit mode'
    );
  });

  test('Clone as a new version when no submission info is available', async function (assert) {
    server.pretender.get('/v1/job/:id/submission', () => [500, {}, '']);
    const versionBlock = '[data-test-job-version="98"]';
    await click(`${versionBlock} [data-test-clone-and-edit]`);
    await click(`${versionBlock} [data-test-clone-as-new-version]`);

    assert.equal(
      currentURL(),
      `/jobs/${job.id}@${namespace.id}/definition?isEditing=true&version=98&view=full-definition`,
      'Taken to the definition page in edit mode'
    );

    assert.dom('[data-test-json-warning]').exists();

    await percySnapshot(assert);
  });

  test('Clone as a new job', async function (assert) {
    const testString =
      'Test string that should appear in my sourceString url param';
    server.pretender.get('/v1/job/:id/submission', () => [
      200,
      {},
      JSON.stringify({
        Source: testString,
      }),
    ]);
    const versionBlock = '[data-test-job-version="98"]';
    await click(`${versionBlock} [data-test-clone-and-edit]`);
    await click(`${versionBlock} [data-test-clone-as-new-job]`);

    assert.equal(
      currentURL(),
      `/jobs/run?sourceString=${encodeURIComponent(testString)}`,
      'Taken to the new job page'
    );
    assert.dom('[data-test-job-name-warning]').exists();
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
