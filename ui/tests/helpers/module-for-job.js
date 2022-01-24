/* eslint-disable qunit/require-expect */
/* eslint-disable qunit/no-conditional-assertions */
import { currentURL } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import JobDetail from 'nomad-ui/tests/pages/jobs/detail';
import Tokens from 'nomad-ui/tests/pages/settings/tokens';

// eslint-disable-next-line ember/no-test-module-for
export default function moduleForJob(
  title,
  context,
  jobFactory,
  additionalTests
) {
  let job;

  module(title, function (hooks) {
    setupApplicationTest(hooks);
    setupMirage(hooks);
    hooks.before(function () {
      if (context !== 'allocations' && context !== 'children') {
        throw new Error(
          `Invalid context provided to moduleForJob, expected either "allocations" or "children", got ${context}`
        );
      }
    });

    hooks.beforeEach(async function () {
      server.create('node');
      job = jobFactory();
      if (!job.namespace || job.namespace === 'default') {
        await JobDetail.visit({ id: job.id });
      } else {
        await JobDetail.visit({ id: job.id, namespace: job.namespace });
      }
    });

    test('visiting /jobs/:job_id', async function (assert) {
      const expectedURL = new URL(
        urlWithNamespace(`/jobs/${encodeURIComponent(job.id)}`, job.namespace),
        window.location
      );
      const gotURL = new URL(currentURL(), window.location);

      assert.deepEqual(gotURL.path, expectedURL.path);
      assert.deepEqual(gotURL.searchParams, expectedURL.searchParams);
      assert.equal(document.title, `Job ${job.name} - Nomad`);
    });

    test('the subnav links to overview', async function (assert) {
      await JobDetail.tabFor('overview').visit();

      const expectedURL = new URL(
        urlWithNamespace(`/jobs/${encodeURIComponent(job.id)}`, job.namespace),
        window.location
      );
      const gotURL = new URL(currentURL(), window.location);

      assert.deepEqual(gotURL.path, expectedURL.path);
      assert.deepEqual(gotURL.searchParams, expectedURL.searchParams);
    });

    test('the subnav links to definition', async function (assert) {
      await JobDetail.tabFor('definition').visit();
      assert.equal(
        currentURL(),
        urlWithNamespace(
          `/jobs/${encodeURIComponent(job.id)}/definition`,
          job.namespace
        )
      );
    });

    test('the subnav links to versions', async function (assert) {
      await JobDetail.tabFor('versions').visit();
      assert.equal(
        currentURL(),
        urlWithNamespace(
          `/jobs/${encodeURIComponent(job.id)}/versions`,
          job.namespace
        )
      );
    });

    test('the subnav links to evaluations', async function (assert) {
      await JobDetail.tabFor('evaluations').visit();
      assert.equal(
        currentURL(),
        urlWithNamespace(
          `/jobs/${encodeURIComponent(job.id)}/evaluations`,
          job.namespace
        )
      );
    });

    test('the title buttons are dependent on job status', async function (assert) {
      if (job.status === 'dead') {
        assert.ok(JobDetail.start.isPresent);
        assert.notOk(JobDetail.stop.isPresent);
        assert.notOk(JobDetail.execButton.isPresent);
      } else {
        assert.notOk(JobDetail.start.isPresent);
        assert.ok(JobDetail.stop.isPresent);
        assert.ok(JobDetail.execButton.isPresent);
      }
    });

    if (context === 'allocations') {
      test('allocations for the job are shown in the overview', async function (assert) {
        assert.ok(
          JobDetail.allocationsSummary.isPresent,
          'Allocations are shown in the summary section'
        );
        assert.ok(
          JobDetail.childrenSummary.isHidden,
          'Children are not shown in the summary section'
        );
      });

      test('clicking in an allocation row navigates to that allocation', async function (assert) {
        const allocationRow = JobDetail.allocations[0];
        const allocationId = allocationRow.id;

        await allocationRow.visitRow();

        assert.equal(
          currentURL(),
          `/allocations/${allocationId}`,
          'Allocation row links to allocation detail'
        );
      });

      test('clicking legend item navigates to a pre-filtered allocations table', async function (assert) {
        const legendItem =
          JobDetail.allocationsSummary.legend.clickableItems[1];
        const status = legendItem.label;
        await legendItem.click();

        const encodedStatus = encodeURIComponent(JSON.stringify([status]));
        const expectedURL = new URL(
          urlWithNamespace(
            `/jobs/${job.name}/clients?status=${encodedStatus}`,
            job.namespace
          ),
          window.location
        );
        const gotURL = new URL(currentURL(), window.location);
        assert.deepEqual(gotURL.path, expectedURL.path);
        assert.deepEqual(gotURL.searchParams, expectedURL.searchParams);
      });

      test('clicking in a slice takes you to a pre-filtered allocations table', async function (assert) {
        const slice = JobDetail.allocationsSummary.slices[1];
        const status = slice.label;
        await slice.click();

        const encodedStatus = encodeURIComponent(JSON.stringify([status]));
        const expectedURL = new URL(
          urlWithNamespace(
            `/jobs/${encodeURIComponent(
              job.name
            )}/allocations?status=${encodedStatus}`,
            job.namespace
          ),
          window.location
        );
        const gotURL = new URL(currentURL(), window.location);
        assert.deepEqual(gotURL.pathname, expectedURL.pathname);

        // Sort and compare URL query params.
        gotURL.searchParams.sort();
        expectedURL.searchParams.sort();
        assert.equal(
          gotURL.searchParams.toString(),
          expectedURL.searchParams.toString()
        );
      });
    }

    if (context === 'children') {
      test('children for the job are shown in the overview', async function (assert) {
        assert.ok(
          JobDetail.childrenSummary.isPresent,
          'Children are shown in the summary section'
        );
        assert.ok(
          JobDetail.allocationsSummary.isHidden,
          'Allocations are not shown in the summary section'
        );
      });
    }

    for (var testName in additionalTests) {
      test(testName, async function (assert) {
        await additionalTests[testName].call(this, job, assert);
      });
    }
  });
}

// eslint-disable-next-line ember/no-test-module-for
export function moduleForJobWithClientStatus(
  title,
  jobFactory,
  additionalTests
) {
  let job;

  module(title, function (hooks) {
    setupApplicationTest(hooks);
    setupMirage(hooks);

    hooks.beforeEach(async function () {
      // Displaying the job status in client requires node:read permission.
      const policy = server.create('policy', {
        id: 'node-read',
        name: 'node-read',
        rulesJSON: {
          Node: {
            Policy: 'read',
          },
        },
      });
      const clientToken = server.create('token', { type: 'client' });
      clientToken.policyIds = [policy.id];
      clientToken.save();

      window.localStorage.clear();
      window.localStorage.nomadTokenSecret = clientToken.secretId;

      const clients = server.createList('node', 3, {
        datacenter: 'dc1',
        status: 'ready',
      });
      job = jobFactory();
      clients.forEach((c) => {
        server.create('allocation', { jobId: job.id, nodeId: c.id });
      });
      if (!job.namespace || job.namespace === 'default') {
        await JobDetail.visit({ id: job.id });
      } else {
        await JobDetail.visit({ id: job.id, namespace: job.namespace });
      }
    });

    test('job status summary is collapsed when not authorized', async function (assert) {
      const clientToken = server.create('token', { type: 'client' });
      await Tokens.visit();
      await Tokens.secret(clientToken.secretId).submit();

      await JobDetail.visit({ id: job.id, namespace: job.namespace });

      assert.ok(
        JobDetail.jobClientStatusSummary.toggle.isDisabled,
        'Job client status summar is disabled'
      );
      assert.equal(
        JobDetail.jobClientStatusSummary.toggle.tooltip,
        'You don’t have permission to read clients'
      );
    });

    test('the subnav links to clients', async function (assert) {
      await JobDetail.tabFor('clients').visit();
      assert.equal(
        currentURL(),
        urlWithNamespace(
          `/jobs/${encodeURIComponent(job.id)}/clients`,
          job.namespace
        )
      );
    });

    test('job status summary is shown in the overview', async function (assert) {
      assert.ok(
        JobDetail.jobClientStatusSummary.statusBar.isPresent,
        'Summary bar is displayed in the Job Status in Client summary section'
      );
    });

    test('clicking legend item navigates to a pre-filtered clients table', async function (assert) {
      const legendItem =
        JobDetail.jobClientStatusSummary.statusBar.legend.clickableItems[0];
      const status = legendItem.label;
      await legendItem.click();

      const encodedStatus = encodeURIComponent(JSON.stringify([status]));
      const expectedURL = new URL(
        urlWithNamespace(
          `/jobs/${job.name}/clients?status=${encodedStatus}`,
          job.namespace
        ),
        window.location
      );
      const gotURL = new URL(currentURL(), window.location);
      assert.deepEqual(gotURL.path, expectedURL.path);
      assert.deepEqual(gotURL.searchParams, expectedURL.searchParams);
    });

    test('clicking in a slice takes you to a pre-filtered clients table', async function (assert) {
      const slice = JobDetail.jobClientStatusSummary.statusBar.slices[0];
      const status = slice.label;
      await slice.click();

      const encodedStatus = encodeURIComponent(JSON.stringify([status]));
      const expectedURL = new URL(
        urlWithNamespace(
          `/jobs/${job.name}/clients?status=${encodedStatus}`,
          job.namespace
        ),
        window.location
      );
      const gotURL = new URL(currentURL(), window.location);
      assert.deepEqual(gotURL.pathname, expectedURL.pathname);

      // Sort and compare URL query params.
      gotURL.searchParams.sort();
      expectedURL.searchParams.sort();
      assert.equal(
        gotURL.searchParams.toString(),
        expectedURL.searchParams.toString()
      );
    });

    for (var testName in additionalTests) {
      test(testName, async function (assert) {
        await additionalTests[testName].call(this, job, assert);
      });
    }
  });
}

function urlWithNamespace(url, namespace) {
  if (!namespace || namespace === 'default') {
    return url;
  }

  const parts = url.split('?');
  const params = new URLSearchParams(parts[1]);
  params.set('namespace', namespace);

  return `${parts[0]}?${params.toString()}`;
}
