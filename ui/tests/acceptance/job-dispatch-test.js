/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember/no-test-module-for */

import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import setupCodeMirror from 'nomad-ui/tests/helpers/codemirror';
import JobDispatch from 'nomad-ui/tests/pages/jobs/dispatch';
import JobDetail from 'nomad-ui/tests/pages/jobs/detail';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import { currentURL, waitFor, waitUntil } from '@ember/test-helpers';

const REQUIRED_INDICATOR = '*';

moduleForJobDispatch('Acceptance | job dispatch', (server) => {
  server.createList('namespace', 2);
  const namespace = server.db.namespaces[0];

  return server.create('job', 'parameterized', {
    status: 'running',
    namespaceId: namespace.name,
  });
});

moduleForJobDispatch('Acceptance | job dispatch (with namespace)', (server) => {
  server.createList('namespace', 2);
  const namespace = server.db.namespaces[1];

  return server.create('job', 'parameterized', {
    status: 'running',
    namespaceId: namespace.name,
  });
});

function moduleForJobDispatch(title, jobFactory) {
  let job, namespace, managementToken, clientToken;

  module(title, function (hooks) {
    setupApplicationTest(hooks);
    setupCodeMirror(hooks);
    setupMirage(hooks);

    hooks.beforeEach(function () {
      // Required for placing allocations (a result of dispatching jobs)
      this.server.create('node-pool');
      this.server.create('node');

      job = jobFactory(this.server);
      namespace = this.server.db.namespaces.find(job.namespaceId);

      managementToken = this.server.create('token');
      clientToken = this.server.create('token');

      window.localStorage.nomadTokenSecret = managementToken.secretId;
    });

    test('it passes an accessibility audit', async function (assert) {
      await JobDispatch.visit({ id: `${job.id}@${namespace.name}` });
      await a11yAudit(assert);
    });

    test('the dispatch button is displayed with management token', async function (assert) {
      await JobDetail.visit({ id: `${job.id}@${namespace.name}` });
      assert.notOk(JobDetail.dispatchButton.isDisabled);
    });

    test('the dispatch button is displayed when allowed', async function (assert) {
      window.localStorage.nomadTokenSecret = clientToken.secretId;

      const policy = this.server.create('policy', {
        id: 'dispatch',
        name: 'dispatch',
        rulesJSON: {
          Namespaces: [
            {
              Name: namespace.name,
              Capabilities: ['list-jobs', 'dispatch-job'],
            },
          ],
        },
      });

      clientToken.policyIds = [policy.id];
      clientToken.save();

      await JobDetail.visit({ id: `${job.id}@${namespace.name}` });
      assert.notOk(JobDetail.dispatchButton.isDisabled);

      // Reset clientToken policies.
      clientToken.policyIds = [];
      clientToken.save();
    });

    test('the dispatch button is disabled when not allowed', async function (assert) {
      window.localStorage.nomadTokenSecret = clientToken.secretId;

      await JobDetail.visit({ id: `${job.id}@${namespace.name}` });
      assert.ok(JobDetail.dispatchButton.isDisabled);
    });

    test('all meta fields are displayed', async function (assert) {
      await JobDispatch.visit({ id: `${job.id}@${namespace.name}` });
      assert.deepEqual(
        JobDispatch.metaFields.length,
        job.parameterizedJob.MetaOptional.length +
          job.parameterizedJob.MetaRequired.length,
      );
    });

    test('required meta fields are properly indicated', async function (assert) {
      await JobDispatch.visit({ id: `${job.id}@${namespace.name}` });

      JobDispatch.metaFields.forEach((f) => {
        const hasIndicator = f.label.includes(REQUIRED_INDICATOR);
        const isRequired = job.parameterizedJob.MetaRequired.includes(
          f.field.id,
        );

        if (isRequired) {
          assert.ok(hasIndicator, `${f.label} contains required indicator.`);
        } else {
          assert.notOk(
            hasIndicator,
            `${f.label} doesn't contain required indicator.`,
          );
        }
      });
    });

    test('job without meta fields', async function (assert) {
      const jobWithoutMeta = this.server.create('job', 'parameterized', {
        status: 'running',
        namespaceId: namespace.name,
        parameterizedJob: {
          MetaRequired: null,
          MetaOptional: null,
        },
      });

      await JobDispatch.visit({ id: `${jobWithoutMeta.id}@${namespace.name}` });
      assert.ok(JobDispatch.dispatchButton.isPresent);
    });

    test('payload text area is hidden when forbidden', async function (assert) {
      job.parameterizedJob.Payload = 'forbidden';
      job.save();

      await JobDispatch.visit({ id: `${job.id}@${namespace.name}` });

      assert.ok(JobDispatch.payload.emptyMessage.isPresent);
      assert.notOk(JobDispatch.payload.editor.isPresent);
    });

    test('payload is indicated as required', async function (assert) {
      const jobPayloadRequired = this.server.create('job', 'parameterized', {
        status: 'running',
        namespaceId: namespace.name,
        parameterizedJob: {
          Payload: 'required',
        },
      });
      const jobPayloadOptional = this.server.create('job', 'parameterized', {
        status: 'running',
        namespaceId: namespace.name,
        parameterizedJob: {
          Payload: 'optional',
        },
      });

      await JobDispatch.visit({
        id: `${jobPayloadRequired.id}@${namespace.name}`,
      });

      let payloadTitle = JobDispatch.payload.title;
      assert.ok(
        payloadTitle.includes(REQUIRED_INDICATOR),
        `${payloadTitle} contains required indicator.`,
      );

      await JobDispatch.visit({
        id: `${jobPayloadOptional.id}@${namespace.name}`,
      });

      payloadTitle = JobDispatch.payload.title;
      assert.notOk(
        payloadTitle.includes(REQUIRED_INDICATOR),
        `${payloadTitle} doesn't contain required indicator.`,
      );
    });

    test('dispatch a job', async function (assert) {
      function countDispatchChildren() {
        return this.server.db.jobs.where((j) => j.id.startsWith(`${job.id}/`))
          .length;
      }

      await JobDispatch.visit({ id: `${job.id}@${namespace.name}` });

      // Fill form.
      JobDispatch.metaFields.map((f) => f.field.input('meta value'));
      JobDispatch.payload.editor.fillIn('payload');

      const childrenCountBefore = countDispatchChildren.call(this);
      await JobDispatch.dispatchButton.click();
      const childrenCountAfter = countDispatchChildren.call(this);
      const dispatchedJobPath = `/jobs/${encodeURIComponent(`${job.id}/`)}`;

      assert.deepEqual(childrenCountAfter, childrenCountBefore + 1);
      await waitUntil(() => currentURL().startsWith(dispatchedJobPath));
      assert.ok(currentURL().startsWith(dispatchedJobPath));
      await waitUntil(() => !currentURL().includes('/dispatch'));
      await waitFor('[data-test-job-name]');
      assert.ok(JobDetail.jobName);
    });

    test('fail when required meta field is empty', async function (assert) {
      // Make sure we have a required meta param.
      job.parameterizedJob.MetaRequired = ['required'];
      job.parameterizedJob.Payload = 'forbidden';
      job.save();

      await JobDispatch.visit({ id: `${job.id}@${namespace.name}` });

      // Fill only optional meta params.
      JobDispatch.optionalMetaFields.map((f) => f.field.input('meta value'));

      await JobDispatch.dispatchButton.click();

      assert.ok(JobDispatch.hasError, 'Dispatch error message is shown');
    });

    test('fail when required payload is empty', async function (assert) {
      job.parameterizedJob.MetaRequired = [];
      job.parameterizedJob.Payload = 'required';
      job.save();

      await JobDispatch.visit({ id: `${job.id}@${namespace.name}` });
      await JobDispatch.dispatchButton.click();

      assert.ok(JobDispatch.hasError, 'Dispatch error message is shown');
    });
  });
}
